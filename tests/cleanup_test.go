package tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	nodelabeller "kubevirt.io/ssp-operator/internal/operands/node-labeller"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
)

var _ = Describe("Cleanup", func() {
	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	It("[test_id:7394] should cleanup all deployed resources when SSP is deleted", func() {
		var allResourceTypes []client.Object
		for _, f := range []func() []client.Object{
			common_templates.WatchClusterTypes,
			data_sources.WatchClusterTypes,
			metrics.WatchTypes,
			metrics.WatchClusterTypes,
			nodelabeller.WatchTypes,
			nodelabeller.WatchClusterTypes,
			template_validator.WatchTypes,
			template_validator.WatchClusterTypes,
		} {
			allResourceTypes = append(allResourceTypes, f()...)
		}

		ssp := getSsp()

		Expect(apiClient.Delete(ctx, ssp)).To(Succeed())
		waitForDeletion(client.ObjectKeyFromObject(ssp), &sspv1beta1.SSP{})

		// Check that all deployed resources were deleted
		for _, resource := range allResourceTypes {
			gvk, err := apiutil.GVKForObject(resource, testScheme)
			Expect(err).ToNot(HaveOccurred())

			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(gvk)

			Expect(apiClient.List(ctx, list, client.MatchingLabels{
				common.AppKubernetesManagedByLabel: "ssp-operator",
			})).To(Succeed())
			Expect(list.Items).To(BeEmpty(), "Some resources were not deleted.")
		}
	})
})
