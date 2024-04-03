package main

import (
	"testing"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/common"
)

var _ = Describe("csv generator", func() {
	flags := generatorFlags{
		dumpCRDs:        true,
		removeCerts:     false,
		csvVersion:      "9.9.9",
		namespace:       "testOperatorNamespace",
		operatorVersion: "testOperatorVersion",

		operatorImage:       "test",
		validatorImage:      "test",
		virtioImage:         "testVirtioImage",
		vmConsoleProxyImage: "testVmConsoleProxyImage",
	}
	envValues := []v1.EnvVar{
		{Name: common.TemplateValidatorImageKey},
		{Name: common.OperatorVersionKey},
		{Name: common.VirtioImageKey},
	}

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
		Expect(err).ToNot(HaveOccurred())
		//test csv name
		Expect(csv.Name).To(Equal("ssp-operator.v9.9.9"))
		//test csv version
		v, err := semver.New(flags.csvVersion)
		Expect(err).ToNot(HaveOccurred())
		Expect(csv.Spec.Version).To(Equal(version.OperatorVersion{Version: *v}))

		for _, container := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Containers {
			if container.Name == "manager" {
				Expect(container.Image).To(Equal(flags.operatorImage))

				for _, envVariable := range container.Env {
					if envVariable.Name == common.TemplateValidatorImageKey {
						Expect(envVariable.Value).To(Equal(flags.validatorImage))
					}
					if envVariable.Name == common.OperatorVersionKey {
						Expect(envVariable.Value).To(Equal(flags.operatorVersion))
					}
					if envVariable.Name == common.VirtioImageKey {
						Expect(envVariable.Value).To(Equal(flags.virtioImage))
					}
					if envVariable.Name == common.VmConsoleProxyImageKey {
						Expect(envVariable.Value).To(Equal(flags.vmConsoleProxyImage))
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
