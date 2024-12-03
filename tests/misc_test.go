package tests

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	"kubevirt.io/ssp-operator/tests/env"
)

var _ = Describe("Observed generation", func() {
	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
		waitUntilDeployed()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
		waitUntilDeployed()
	})

	It("[test_id:6058] after deployment observedGeneration equals generation", func() {
		ssp := getSsp()
		Expect(ssp.Status.ObservedGeneration).To(Equal(ssp.Generation))
	})

	It("[test_id:6059] should update observed generation after CR update", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		var newValidatorReplicas int32 = 0
		updateSsp(func(foundSsp *ssp.SSP) {
			foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
				Replicas: &newValidatorReplicas,
			}
		})

		// Watch changes until above change
		err = WatchChangesUntil(watch, func(updatedSsp *ssp.SSP) bool {
			return updatedSsp.Spec.TemplateValidator != nil &&
				*updatedSsp.Spec.TemplateValidator.Replicas == newValidatorReplicas &&
				updatedSsp.Generation > updatedSsp.Status.ObservedGeneration
		}, env.ShortTimeout())
		Expect(err).ToNot(HaveOccurred())

		// Watch changes until SSP operator updates ObservedGeneration
		err = WatchChangesUntil(watch, func(updatedSsp *ssp.SSP) bool {
			return updatedSsp.Spec.TemplateValidator != nil &&
				*updatedSsp.Spec.TemplateValidator.Replicas == newValidatorReplicas &&
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration
		}, env.ShortTimeout())
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:6060] should update observed generation when removing CR", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		sspObj := getSsp()
		Expect(apiClient.Delete(ctx, sspObj)).ToNot(HaveOccurred())

		// Check for deletion timestamp before the SSP operator notices change
		err = WatchChangesUntil(watch, func(updatedSsp *ssp.SSP) bool {
			return updatedSsp.DeletionTimestamp != nil &&
				updatedSsp.Generation > updatedSsp.Status.ObservedGeneration
		}, env.ShortTimeout())
		Expect(err).ToNot(HaveOccurred())

		// SSP operator enters Deleting phase
		err = WatchChangesUntil(watch, func(updatedSsp *ssp.SSP) bool {
			return updatedSsp.DeletionTimestamp != nil &&
				updatedSsp.Status.Phase == lifecycleapi.PhaseDeleting &&
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration
		}, env.ShortTimeout())
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("SSPOperatorReconcileSucceeded metric", func() {
	var (
		deploymentRes testResource
		finalizerName = "ssp.kubernetes.io/temp-protection"
	)

	AfterEach(func() {
		removeFinalizer(deploymentRes, finalizerName)
		strategy.RevertToOriginalSspCr()
		waitUntilDeployed()
	})

	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
		deploymentRes = testDeploymentResource()

		waitUntilDeployed()
	})

	It("[test_id:7369] should set SSPOperatorReconcileSucceeded metrics to 0 on failing to reconcile", func() {
		// add a finalizer to the validator deployment, do that it can't be deleted
		addFinalizer(deploymentRes, finalizerName)
		// send a request to delete the validator deployment
		deleteDeployment(deploymentRes)
		validateSspIsFailingToReconcileMetric()
	})
})

func removeFinalizer(deploymentRes testResource, finalizerName string) {
	Eventually(func() error {
		deployment := &apps.Deployment{}
		err := apiClient.Get(ctx, deploymentRes.GetKey(), deployment)
		if err != nil {
			return err
		}
		// remove the finalizer so everything can go back to normal
		controllerutil.RemoveFinalizer(deployment, finalizerName)
		return apiClient.Update(ctx, deployment)
	}, env.ShortTimeout(), time.Second).ShouldNot(HaveOccurred())
}

func addFinalizer(deploymentRes testResource, finalizerName string) {
	Eventually(func() error {
		deployment := &apps.Deployment{}
		err := apiClient.Get(ctx, deploymentRes.GetKey(), deployment)
		if err != nil {
			return err
		}
		controllerutil.AddFinalizer(deployment, finalizerName)
		return apiClient.Update(ctx, deployment)
	}, env.ShortTimeout(), time.Second).ShouldNot(HaveOccurred())
}

func deleteDeployment(deploymentRes testResource) {
	deployment := &apps.Deployment{}
	deployment.Name = deploymentRes.Name
	deployment.Namespace = deploymentRes.Namespace
	Expect(apiClient.Delete(ctx, deployment)).ToNot(HaveOccurred())
}

func validateSspIsFailingToReconcileMetric() {
	// try to change the number of validator pods
	var newValidatorReplicas int32 = 3
	updateSsp(func(foundSsp *ssp.SSP) {
		foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
			Replicas: &newValidatorReplicas,
		}
	})
	// the reconcile cycle should now be failing, so the kubevirt_ssp_operator_reconcile_succeeded metric should be 0
	Eventually(func() int {
		return sspOperatorReconcileSucceededCount()
	}, env.ShortTimeout(), time.Second).Should(Equal(0))
}

var _ = Describe("SCC annotation", func() {
	const (
		sccAnnotation = "openshift.io/scc"
		sccRestricted = "^restricted*"
	)

	BeforeEach(func() {
		waitUntilDeployed()
	})

	It("[test_id:7162] operator pod should have 'restricted' scc annotation", func() {
		pods := &core.PodList{}
		err := apiClient.List(ctx, pods, client.MatchingLabels{"control-plane": "ssp-operator"})

		Expect(err).ToNot(HaveOccurred())
		Expect(pods.Items).ToNot(BeEmpty())

		for _, pod := range pods.Items {
			Expect(pod.Annotations).To(HaveKeyWithValue(sccAnnotation, MatchRegexp(sccRestricted)), "Expected pod %s/%s to have scc 'restricted'", pod.Namespace, pod.Name)
		}
	})

	It("[test_id:7163] template validator pods should have 'restricted' scc annotation", func() {
		pods := &core.PodList{}
		err := apiClient.List(ctx, pods,
			client.InNamespace(strategy.GetNamespace()),
			client.MatchingLabels{validator.KubevirtIo: validator.VirtTemplateValidator})

		Expect(err).ToNot(HaveOccurred())
		Expect(pods.Items).ToNot(BeEmpty())

		for _, pod := range pods.Items {
			Expect(pod.Annotations).To(HaveKeyWithValue(sccAnnotation, MatchRegexp(sccRestricted)), "Expected pod %s/%s to have scc 'restricted'", pod.Namespace, pod.Name)
		}
	})
})

var _ = Describe("RHEL VM creation", func() {
	const (
		rhel8Image = "docker://registry.redhat.io/rhel8/rhel-guest-image"
		rhel9Image = "docker://registry.redhat.io/rhel9/rhel-guest-image"
	)

	var (
		vm *kubevirtv1.VirtualMachine
	)

	AfterEach(func() {
		if vm != nil {
			err := apiClient.Delete(ctx, vm)
			expectSuccessOrNotFound(err)
			waitForDeletion(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})
			vm = nil
		}
	})

	JustAfterEach(func() {
		if vm == nil {
			return
		}

		logObject(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})

		dvName := vm.Spec.DataVolumeTemplates[0].Name
		logObject(client.ObjectKey{
			Name:      dvName,
			Namespace: vm.GetNamespace(),
		}, &cdiv1beta1.DataVolume{})

		logObject(client.ObjectKey{
			Name:      dvName,
			Namespace: vm.GetNamespace(),
		}, &core.PersistentVolumeClaim{})
	})

	DescribeTable("should be able to start VM", func(imageUrl string) {
		const diskName = "disk0"
		const sshPort = 22

		var always = kubevirtv1.RunStrategyAlways
		var terminateGracePeriod int64 = 0
		var pullMethodNode = cdiv1beta1.RegistryPullNode
		var vmName = "test-vm-" + rand.String(5)

		vm = &kubevirtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmName,
				Namespace: strategy.GetNamespace(),
			},
			Spec: kubevirtv1.VirtualMachineSpec{
				RunStrategy: &always,
				Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
					Spec: kubevirtv1.VirtualMachineInstanceSpec{
						Domain: kubevirtv1.DomainSpec{
							Resources: kubevirtv1.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
							CPU: &kubevirtv1.CPU{Sockets: 1, Cores: 1, Threads: 1},
							Devices: kubevirtv1.Devices{
								Rng: &kubevirtv1.Rng{},
								Interfaces: []kubevirtv1.Interface{{
									Name: "default",
									InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
										Masquerade: &kubevirtv1.InterfaceMasquerade{},
									},
								}},
								Disks: []kubevirtv1.Disk{{
									Name: diskName,
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{Bus: "virtio"},
									},
								}},
								NetworkInterfaceMultiQueue: ptr.To(true),
							},
						},
						TerminationGracePeriodSeconds: &terminateGracePeriod,
						Networks:                      []kubevirtv1.Network{*kubevirtv1.DefaultPodNetwork()},
						Volumes: []kubevirtv1.Volume{{
							Name: diskName,
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: vmName,
								},
							},
						}},
						ReadinessProbe: &kubevirtv1.Probe{
							Handler: kubevirtv1.Handler{
								TCPSocket: &core.TCPSocketAction{
									Port: intstr.FromInt(sshPort),
								},
							},
							InitialDelaySeconds: 5,
							TimeoutSeconds:      1,
							PeriodSeconds:       5,
						},
					},
				},
				DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{{
					ObjectMeta: metav1.ObjectMeta{
						Name: vmName,
					},
					Spec: cdiv1beta1.DataVolumeSpec{
						Source: &cdiv1beta1.DataVolumeSource{
							Registry: &cdiv1beta1.DataVolumeSourceRegistry{
								URL:        &imageUrl,
								PullMethod: &pullMethodNode,
							},
						},
						Storage: &cdiv1beta1.StorageSpec{
							Resources: core.VolumeResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("10Gi"),
								},
							},
						},
					},
				}},
			},
		}

		Expect(apiClient.Create(ctx, vm)).To(Succeed())

		// Wait for DataVolume to finish importing
		dvName := vm.Spec.DataVolumeTemplates[0].Name
		Eventually(func(g Gomega) {
			foundDv := &cdiv1beta1.DataVolume{}
			err := apiClient.Get(ctx, client.ObjectKey{Name: dvName, Namespace: vm.Namespace}, foundDv)
			if err != nil {
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			} else {
				g.Expect(foundDv.Status.Phase).To(Equal(cdiv1beta1.Succeeded))
			}
			foundPvc := &core.PersistentVolumeClaim{}
			err = apiClient.Get(ctx, client.ObjectKey{Name: dvName, Namespace: vm.Namespace}, foundPvc)
			g.Expect(err).ToNot(HaveOccurred())
		}, 2*env.Timeout(), time.Second).Should(Succeed())

		// Wait for VMI to be ready
		Eventually(func(g Gomega) bool {
			foundVmi := &kubevirtv1.VirtualMachineInstance{}
			err := apiClient.Get(ctx, client.ObjectKeyFromObject(vm), foundVmi)
			g.Expect(err).ToNot(HaveOccurred())

			for _, condition := range foundVmi.Status.Conditions {
				if condition.Type == kubevirtv1.VirtualMachineInstanceReady {
					return condition.Status == core.ConditionTrue
				}
			}
			return false
		}, env.Timeout(), time.Second).Should(BeTrue())
	},
		Entry("[test_id:8299] with RHEL 8 image", rhel8Image),
		Entry("[test_id:8300] with RHEL 9 image", rhel9Image),
	)
})

func logObject(key client.ObjectKey, obj client.Object) {
	gvk, err := apiutil.GVKForObject(obj, testScheme)
	if err != nil {
		panic(err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)

	err = apiClient.Get(ctx, key, obj)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "Failed to get %s: %s\n", gvk.Kind, err)
	} else {
		objJson, err := json.MarshalIndent(obj, "", "    ")
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(GinkgoWriter, "Found %s:\n%s\n", gvk.Kind, objJson)
	}
}
