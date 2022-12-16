package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// List of legacy CRDs and their corresponding kinds, copied from ssp_controller.go
var kvsspCRDs = map[string]string{
	"kubevirtmetricsaggregations.ssp.kubevirt.io":    "KubevirtMetricsAggregation",
	"kubevirttemplatevalidators.ssp.kubevirt.io":     "KubevirtTemplateValidator",
	"kubevirtcommontemplatesbundles.ssp.kubevirt.io": "KubevirtCommonTemplatesBundle",
	"kubevirtnodelabellerbundles.ssp.kubevirt.io":    "KubevirtNodeLabellerBundle",
}

func listExistingCRDKinds() []string {
	// Get the list of all CRDs and build a list of the SSP ones
	crds := &unstructured.UnstructuredList{}
	crds.SetKind("CustomResourceDefinition")
	crds.SetAPIVersion("apiextensions.k8s.io/v1")
	err := apiClient.List(ctx, crds)
	foundKinds := make([]string, 0, len(kvsspCRDs))
	if err == nil {
		for _, item := range crds.Items {
			name := item.GetName()
			for crd, kind := range kvsspCRDs {
				if crd == name {
					foundKinds = append(foundKinds, kind)
					break
				}
			}
		}
	}

	return foundKinds
}

var _ = Describe("Upgrade", func() {
	BeforeEach(func() {
		waitUntilDeployed()
	})

	Context("from kubevirt-ssp-operator", func() {
		It("[test_id:5566] should have added a paused annotation to existing CRs", func() {
			By("listing legacy CRDs")
			kinds := listExistingCRDKinds()

			// If kinds is empty, this test is not running after an upgrade and will just succeed
			// A skip could be added for `len(kinds) == 0` to show nothing happened

			By("listing CRs for each legacy CRD and expecting the paused annotation")
			for _, kind := range kinds {
				crs := &unstructured.UnstructuredList{}
				crs.SetKind(kind)
				crs.SetAPIVersion("ssp.kubevirt.io/v1")
				err := apiClient.List(ctx, crs)
				Expect(err).ToNot(HaveOccurred())
				for _, item := range crs.Items {
					annotations := item.GetAnnotations()
					Expect(annotations["kubevirt.io/operator.paused"]).To(Equal("true"))
				}
			}
		})
	})
})
