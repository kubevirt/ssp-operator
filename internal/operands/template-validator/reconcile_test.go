package template_validator

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var log = logf.Log.WithName("validator_operand")

var _ = Describe("Template validator operand", func() {
	const (
		namespace       = "kubevirt"
		name            = "test-ssp"
		replicas  int32 = 2
	)

	var (
		request common.Request
		operand = New()
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())

		client := fake.NewClientBuilder().WithScheme(s).Build()
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Context: context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: meta.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: meta.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ssp.SSPSpec{
					TemplateValidator: &ssp.TemplateValidator{
						Replicas: ptr.To(replicas),
						Placement: &lifecycleapi.NodePlacement{
							Affinity: &core.Affinity{},
						},
					},
				},
			},
			Logger:       log,
			VersionCache: common.VersionCache{},
		}
	})

	It("should create validator resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newClusterRole(), request)
		ExpectResourceExists(newServiceAccount(namespace), request)
		ExpectResourceExists(newClusterRoleBinding(namespace), request)
		ExpectResourceExists(newService(namespace), request)
		ExpectResourceExists(newConfigMap(namespace, ""), request)
		ExpectResourceExists(newDeployment(namespace, replicas, "test-img"), request)
		ExpectResourceExists(newValidatingWebhook(namespace), request)
		ExpectResourceExists(newPrometheusService(namespace), request)
	})

	It("should not update webhook CA bundle", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		key := client.ObjectKeyFromObject(newValidatingWebhook(namespace))
		webhook := &admission.ValidatingWebhookConfiguration{}
		Expect(request.Client.Get(request.Context, key, webhook)).ToNot(HaveOccurred())

		const testCaBundle = "testCaBundle"
		webhook.Webhooks[0].ClientConfig.CABundle = []byte(testCaBundle)
		Expect(request.Client.Update(request.Context, webhook)).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		updatedWebhook := &admission.ValidatingWebhookConfiguration{}
		Expect(request.Client.Get(request.Context, key, updatedWebhook)).ToNot(HaveOccurred())
		Expect(updatedWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(testCaBundle)))
	})

	It("should not update service cluster IP", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		key := client.ObjectKeyFromObject(newService(namespace))
		service := &core.Service{}
		Expect(request.Client.Get(request.Context, key, service)).ToNot(HaveOccurred())

		// This address is from a range of IP addresses reserved for documentation.
		const testClusterIp = "198.51.100.42"

		service.Spec.ClusterIP = testClusterIp
		Expect(request.Client.Update(request.Context, service)).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		updatedService := &core.Service{}
		Expect(request.Client.Get(request.Context, key, updatedService)).ToNot(HaveOccurred())
		Expect(updatedService.Spec.ClusterIP).To(Equal(testClusterIp))
	})

	It("should remove cluster resources on cleanup", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newClusterRole(), request)
		ExpectResourceExists(newClusterRoleBinding(namespace), request)
		ExpectResourceExists(newValidatingWebhook(namespace), request)

		_, err = operand.Cleanup(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceNotExists(newClusterRole(), request)
		ExpectResourceNotExists(newClusterRoleBinding(namespace), request)
		ExpectResourceNotExists(newValidatingWebhook(namespace), request)
	})

	It("should report status", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Set status for deployment
		key := client.ObjectKeyFromObject(newDeployment(namespace, replicas, "test-img"))
		updateDeploymentStatus(key, &request, func(deploymentStatus *apps.DeploymentStatus) {
			deploymentStatus.Replicas = replicas
			deploymentStatus.ReadyReplicas = 0
			deploymentStatus.AvailableReplicas = 0
			deploymentStatus.UpdatedReplicas = 0
			deploymentStatus.UnavailableReplicas = replicas
		})

		reconcileResults, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Only deployment should be progressing
		for _, reconcileResult := range reconcileResults {
			if _, ok := reconcileResult.Resource.(*apps.Deployment); ok {
				Expect(reconcileResult.Status.NotAvailable).ToNot(BeNil())
				Expect(reconcileResult.Status.Progressing).ToNot(BeNil())
				Expect(reconcileResult.Status.Degraded).ToNot(BeNil())
			} else {
				Expect(reconcileResult.Status.NotAvailable).To(BeNil())
				Expect(reconcileResult.Status.Progressing).To(BeNil())
				Expect(reconcileResult.Status.Degraded).To(BeNil())
			}
		}

		updateDeploymentStatus(key, &request, func(deploymentStatus *apps.DeploymentStatus) {
			deploymentStatus.Replicas = replicas
			deploymentStatus.ReadyReplicas = replicas
			deploymentStatus.AvailableReplicas = replicas
			deploymentStatus.UpdatedReplicas = replicas
			deploymentStatus.UnavailableReplicas = 0
		})

		reconcileResults, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// All resources should be available
		for _, reconcileResult := range reconcileResults {
			Expect(reconcileResult.Status.NotAvailable).To(BeNil())
			Expect(reconcileResult.Status.Progressing).To(BeNil())
			Expect(reconcileResult.Status.Degraded).To(BeNil())
		}
	})

	Context("should create correct deployment affinity", func() {

		const kubernetesHostnameTopologyKey = "kubernetes.io/hostname"

		nodeAffinity := &core.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: core.NodeSelectorTerm{
						MatchExpressions: []core.NodeSelectorRequirement{
							{
								Key:      "prefNodeAffinityKey",
								Operator: core.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: &core.NodeSelector{
				NodeSelectorTerms: []core.NodeSelectorTerm{
					{
						MatchExpressions: []core.NodeSelectorRequirement{
							{
								Key:      "reqNodeAffinityKey",
								Operator: core.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		}
		podAffinity := &core.PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				{
					Weight: 1,
					PodAffinityTerm: core.PodAffinityTerm{
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key:      "prefPodAffinityKey",
									Operator: meta.LabelSelectorOpIn,
									Values:   []string{"true"},
								},
							},
						},
						TopologyKey: kubernetesHostnameTopologyKey,
					},
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []core.PodAffinityTerm{
				{
					LabelSelector: &meta.LabelSelector{
						MatchExpressions: []meta.LabelSelectorRequirement{
							{
								Key:      "reqPodAffinityKey",
								Operator: meta.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					TopologyKey: kubernetesHostnameTopologyKey,
				},
			},
		}
		antiAffinity := &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				{
					Weight: 1,
					PodAffinityTerm: core.PodAffinityTerm{
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key:      "prefAntiAffinityKey",
									Operator: meta.LabelSelectorOpIn,
									Values:   []string{"true"},
								},
							},
						},
						TopologyKey: kubernetesHostnameTopologyKey,
					},
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []core.PodAffinityTerm{
				{
					LabelSelector: &meta.LabelSelector{
						MatchExpressions: []meta.LabelSelectorRequirement{
							{
								Key:      "reqAntiAffinityKey",
								Operator: meta.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					TopologyKey: kubernetesHostnameTopologyKey,
				},
			},
		}
		defaultPodAntiAffinity := &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				{
					Weight: 1,
					PodAffinityTerm: core.PodAffinityTerm{
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key:      "kubevirt.io",
									Operator: meta.LabelSelectorOpIn,
									Values:   []string{"virt-template-validator"},
								},
							},
						},
						TopologyKey: kubernetesHostnameTopologyKey,
					},
				},
			},
		}
		mergedPodAntiAffinity := &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				{
					Weight: 1,
					PodAffinityTerm: core.PodAffinityTerm{
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key:      "kubevirt.io",
									Operator: meta.LabelSelectorOpIn,
									Values:   []string{"virt-template-validator"},
								},
							},
						},
						TopologyKey: kubernetesHostnameTopologyKey,
					},
				},
				{
					Weight: 1,
					PodAffinityTerm: core.PodAffinityTerm{
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key:      "prefAntiAffinityKey",
									Operator: meta.LabelSelectorOpIn,
									Values:   []string{"true"},
								},
							},
						},
						TopologyKey: kubernetesHostnameTopologyKey,
					},
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []core.PodAffinityTerm{
				{
					LabelSelector: &meta.LabelSelector{
						MatchExpressions: []meta.LabelSelectorRequirement{
							{
								Key:      "reqAntiAffinityKey",
								Operator: meta.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					TopologyKey: kubernetesHostnameTopologyKey,
				},
			},
		}

		var setPodAntiAffinity = func(request *common.Request) {
			request.Instance.Spec.TemplateValidator.Placement.Affinity.PodAntiAffinity = antiAffinity
		}
		var setPodAffinity = func(request *common.Request) {
			request.Instance.Spec.TemplateValidator.Placement.Affinity.PodAffinity = podAffinity
		}
		var setNodeAffinity = func(request *common.Request) {
			request.Instance.Spec.TemplateValidator.Placement.Affinity.NodeAffinity = nodeAffinity
		}

		DescribeTable("with different configuration", func(requestAdjustFunctions []func(*common.Request), expectedNodeAffinity *core.NodeAffinity, expectedPodAffinity *core.PodAffinity, expectedPodAntiAffinity *core.PodAntiAffinity) {
			for _, f := range requestAdjustFunctions {
				f(&request)
			}
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())
			deployment := &apps.Deployment{}
			key := client.ObjectKeyFromObject(newDeployment(namespace, replicas, "test-img"))
			Expect(request.Client.Get(request.Context, key, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Affinity.NodeAffinity).To(Equal(expectedNodeAffinity))
			Expect(deployment.Spec.Template.Spec.Affinity.PodAffinity).To(Equal(expectedPodAffinity))
			Expect(deployment.Spec.Template.Spec.Affinity.PodAntiAffinity).To(Equal(expectedPodAntiAffinity))
		},
			Entry("with specific nodeAffinity", []func(*common.Request){setNodeAffinity}, nodeAffinity, nil, defaultPodAntiAffinity),
			Entry("with specific podAffinity", []func(*common.Request){setPodAffinity}, nil, podAffinity, defaultPodAntiAffinity),
			Entry("with specific podAntiAffinity", []func(*common.Request){setPodAntiAffinity}, nil, nil, mergedPodAntiAffinity),
			Entry("with specific nodeAffinity and podAffinity", []func(*common.Request){setNodeAffinity, setPodAffinity}, nodeAffinity, podAffinity, defaultPodAntiAffinity),
			Entry("with specific nodeAffinity and podAntiAffinity", []func(*common.Request){setNodeAffinity, setPodAntiAffinity}, nodeAffinity, nil, mergedPodAntiAffinity),
			Entry("with specific podAffinity and podAntiAffinity", []func(*common.Request){setPodAffinity, setPodAntiAffinity}, nil, podAffinity, mergedPodAntiAffinity),
			Entry("with specific nodeAffinity, podAffinity and podAntiAffinity", []func(*common.Request){setNodeAffinity, setPodAffinity, setPodAntiAffinity}, nodeAffinity, podAffinity, mergedPodAntiAffinity),
		)
	})
})

func updateDeploymentStatus(key client.ObjectKey, request *common.Request, updateFunc func(deploymentStatus *apps.DeploymentStatus)) {
	deployment := &apps.Deployment{}
	Expect(request.Client.Get(request.Context, key, deployment)).ToNot(HaveOccurred())
	updateFunc(&deployment.Status)
	Expect(request.Client.Status().Update(request.Context, deployment)).ToNot(HaveOccurred())
}

func TestValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Validator Suite")
}
