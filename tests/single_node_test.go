package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	policy "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	"kubevirt.io/ssp-operator/tests/env"
)

var _ = Describe("Single Node Topology", func() {
	BeforeEach(func() {
		strategy.SkipUnlessSingleReplicaTopologyMode()
		waitUntilDeployed()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	It("[test_id:7792] Number of Template Validator replicas is not bigger then one", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		var newValidatorReplicas int32 = 3
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
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration && updatedSsp.Status.Phase == api.PhaseDeployed
		}, env.ShortTimeout())
		Expect(err).ToNot(HaveOccurred())

		deployment := getTemplateValidatorDeployment()
		Expect(int(deployment.Status.Replicas)).Should(Equal(1), "In Single Mode Topology the number of replicas is at most 1")
	})

	It("[test_id:7793] Number of Template Validator replicas can be set to 0", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		var newValidatorReplicas int32 = 0
		updateSsp(func(foundSsp *ssp.SSP) {
			foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
				Replicas: &newValidatorReplicas,
			}
		})

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
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration && updatedSsp.Status.Phase == api.PhaseDeployed
		}, env.ShortTimeout())
		Expect(err).ToNot(HaveOccurred())
		deployment := getTemplateValidatorDeployment()
		Expect(int(deployment.Status.Replicas)).Should(Equal(0), "In Single Mode Topology the number of replicas is at most 1")
	})

	It("[test_id:TODO] PodDisruptionBudget should not be created", func() {
		key := client.ObjectKey{Name: validator.DeploymentName, Namespace: strategy.GetNamespace()}
		pdb := &policy.PodDisruptionBudget{}
		Expect(apiClient.Get(ctx, key, pdb)).To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
	})
})
