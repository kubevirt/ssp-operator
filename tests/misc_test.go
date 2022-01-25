package tests

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
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
		updateSsp(func(foundSsp *sspv1beta1.SSP) {
			foundSsp.Spec.TemplateValidator.Replicas = &newValidatorReplicas
		})

		// Watch changes until above change
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return *updatedSsp.Spec.TemplateValidator.Replicas == newValidatorReplicas &&
				updatedSsp.Generation > updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())

		// Watch changes until SSP operator updates ObservedGeneration
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return *updatedSsp.Spec.TemplateValidator.Replicas == newValidatorReplicas &&
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:6060] should update observed generation when removing CR", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		ssp := getSsp()
		Expect(apiClient.Delete(ctx, ssp)).ToNot(HaveOccurred())

		// Check for deletion timestamp before the SSP operator notices change
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return updatedSsp.DeletionTimestamp != nil &&
				updatedSsp.Generation > updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())

		// SSP operator enters Deleting phase
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return updatedSsp.DeletionTimestamp != nil &&
				updatedSsp.Status.Phase == lifecycleapi.PhaseDeleting &&
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("SSPOperatorReconcilingProperly metric", func() {
	var (
		deploymentRes testResource
		deployment    = &apps.Deployment{}
		finalizerName = "ssp.kubernetes.io/temp-protection"
	)

	AfterEach(func() {
		Eventually(func() error {
			Expect(apiClient.Get(ctx, deploymentRes.GetKey(), deployment)).ToNot(HaveOccurred())
			// remove the finalizer so everything can go back to normal
			controllerutil.RemoveFinalizer(deployment, finalizerName)
			return apiClient.Update(ctx, deployment)
		}, shortTimeout, time.Second).ShouldNot(HaveOccurred())
		strategy.RevertToOriginalSspCr()
		waitUntilDeployed()
	})

	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
		expectedLabels := expectedLabelsFor("template-validator", common.AppComponentTemplating)
		deploymentRes = testResource{
			Name:           validator.DeploymentName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &apps.Deployment{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(deployment *apps.Deployment) {
				deployment.Spec.Replicas = pointer.Int32Ptr(0)
			},
			EqualsFunc: func(old *apps.Deployment, new *apps.Deployment) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}

		waitUntilDeployed()
	})

	It("[test_id:7369] should set SSPOperatorReconcilingProperly metrics to 0 on failing to reconcile", func() {
		foundSsp := getSsp()
		Expect(foundSsp.Status.Phase).To(Equal(lifecycleapi.PhaseDeployed), "SSP should be in phase Deployed")
		// add a finalizer to the validator deployment, do that it can't be deleted
		Expect(apiClient.Get(ctx, deploymentRes.GetKey(), deployment)).ToNot(HaveOccurred())
		controllerutil.AddFinalizer(deployment, finalizerName)
		Expect(apiClient.Update(ctx, deployment)).ToNot(HaveOccurred())
		// send a request to delete the validator deployment
		Expect(apiClient.Delete(ctx, deployment)).ToNot(HaveOccurred())
		// try to change the number of validator pods
		var newValidatorReplicas int32 = 3
		updateSsp(func(foundSsp *sspv1beta1.SSP) {
			foundSsp.Spec.TemplateValidator.Replicas = &newValidatorReplicas
		})
		// the reconcile cycle should now be failing, so the ssp_operator_reconciling_properly metric should be 0
		Eventually(func() int {
			return sspOperatorReconcilingProperly()
		}, shortTimeout, time.Second).Should(Equal(0))
	})
})

var _ = Describe("SCC annotation", func() {
	const (
		sccAnnotation = "openshift.io/scc"
		sccRestricted = "restricted"
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
			Expect(pod.Annotations).To(HaveKeyWithValue(sccAnnotation, sccRestricted), "Expected pod %s/%s to have scc 'restricted'", pod.Namespace, pod.Name)
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
			Expect(pod.Annotations).To(HaveKeyWithValue(sccAnnotation, sccRestricted), "Expected pod %s/%s to have scc 'restricted'", pod.Namespace, pod.Name)
		}
	})
})
