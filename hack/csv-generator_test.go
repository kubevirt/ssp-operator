package main

import (
	"testing"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("csv_generator")

var _ = Describe("csv generator", func() {
	flags := generatorFlags{
		csvVersion:        "9.9.9",
		operatorImage:     "test",
		kvmInfoImage:      "test",
		cpuPlugin:         "test",
		validatorImage:    "test",
		nodeLabellerImage: "test",
		virtLauncher:      "test",
	}
	envValues := []v1.EnvVar{{Name: "KVM_IMAGE"}, {Name: "VALIDATOR_IMAGE"}, {Name: "VIRT_LAUNCHER_IMAGE"}, {Name: "OPERATOR_VERSION"}, {Name: "NODE_LABELLER_IMAGE"}, {Name: "CPU_PLUGIN_IMAGE"}}

	csv := csvv1.ClusterServiceVersion{
		Spec: csvv1.ClusterServiceVersionSpec{
			InstallStrategy: csvv1.NamedInstallStrategy{
				StrategySpec: csvv1.StrategyDetailsDeployment{
					DeploymentSpecs: []csvv1.StrategyDeploymentSpec{
						{
							Spec: appsv1.DeploymentSpec{
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Name: "manager",
												Env:  envValues,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	It("should update csv", func() {
		err := replaceVariables(flags, &csv)
		Expect(err).To(BeNil())
		//test csv name
		Expect(csv.Name).To(Equal("ssp-operator.v9.9.9"))
		//test csv version
		v, err := semver.New(flags.csvVersion)
		Expect(err).To(BeNil())
		Expect(csv.Spec.Version).To(Equal(version.OperatorVersion{Version: *v}))

		for _, container := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Containers {
			if container.Name == "manager" {
				Expect(container.Image).To(Equal(flags.operatorImage))

				for _, envVariable := range container.Env {
					if envVariable.Name == "KVM_IMAGE" {
						Expect(envVariable.Value).To(Equal(flags.kvmInfoImage))
					}
					if envVariable.Name == "VALIDATOR_IMAGE" {
						Expect(envVariable.Value).To(Equal(flags.validatorImage))
					}
					if envVariable.Name == "VIRT_LAUNCHER_IMAGE" {
						Expect(envVariable.Value).To(Equal(flags.virtLauncher))
					}
					if envVariable.Name == "NODE_LABELLER_IMAGE" {
						Expect(envVariable.Value).To(Equal(flags.nodeLabellerImage))
					}
					if envVariable.Name == "CPU_PLUGIN_IMAGE" {
						Expect(envVariable.Value).To(Equal(flags.cpuPlugin))
					}
					if envVariable.Name == "OPERATOR_VERSION" {
						Expect(envVariable.Value).To(Equal(flags.operatorVersion))
					}
				}
				break
			}
		}
	})
})

func TestCsvGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csv generator Suite")
}
