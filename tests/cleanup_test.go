package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	common_instancetypes "kubevirt.io/ssp-operator/internal/operands/common-instancetypes"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	tekton_pipelines "kubevirt.io/ssp-operator/internal/operands/tekton-pipelines"
	tekton_tasks "kubevirt.io/ssp-operator/internal/operands/tekton-tasks"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	vm_console_proxy "kubevirt.io/ssp-operator/internal/operands/vm-console-proxy"
)

var _ = Describe("Cleanup", func() {
	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	It("[test_id:7394] should cleanup all deployed resources when SSP is deleted", func() {
		var allWatchTypes []operands.WatchType
		for _, f := range []func() []operands.WatchType{
			common_templates.WatchClusterTypes,
			common_instancetypes.WatchClusterTypes,
			data_sources.WatchClusterTypes,
			metrics.WatchTypes,
			metrics.WatchClusterTypes,
			template_validator.WatchTypes,
			template_validator.WatchClusterTypes,
			vm_console_proxy.WatchClusterTypes,
			tekton_pipelines.WatchClusterTypes,
			tekton_tasks.WatchClusterTypes,
		} {
			allWatchTypes = append(allWatchTypes, f()...)
		}

		sspObj := getSsp()

		Expect(apiClient.Delete(ctx, sspObj)).To(Succeed())
		waitForDeletion(client.ObjectKeyFromObject(sspObj), &ssp.SSP{})

		// Check that all deployed resources were deleted
		for _, watchType := range allWatchTypes {
			gvk, err := apiutil.GVKForObject(watchType.Object, testScheme)
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
