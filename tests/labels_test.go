package tests

import (
	"encoding/json"
	"fmt"
	"strings"

	"kubevirt.io/ssp-operator/internal/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func expectedLabelsFor(name string, component common.AppComponent) map[string]string {
	return map[string]string{
		common.AppKubernetesNameLabel:      name,
		common.AppKubernetesManagedByLabel: "ssp-operator",
		common.AppKubernetesPartOfLabel:    strategy.GetPartOfLabel(),
		common.AppKubernetesVersionLabel:   strategy.GetVersionLabel(),
		common.AppKubernetesComponentLabel: component.String(),
	}
}

func expectAppLabels(res *testResource) {
	waitForLabelMatch(res.NewResource(), res.GetKey(), res.ExpectedLabels)
}

func expectAppLabelsRestoreAfterUpdate(res *testResource) {
	resource := res.NewResource()
	key := res.GetKey()
	waitForLabelMatch(resource, key, res.ExpectedLabels)

	operations := newLabelOperations(res.ExpectedLabels)
	for label := range res.ExpectedLabels {
		operations = append(operations, labelPatchOperationFor("replace", label, "wrong"))
	}
	patch := encodePatch(operations)
	err := apiClient.Patch(ctx, resource, patch)
	Expect(err).NotTo(HaveOccurred())
	waitForLabelMatch(resource, key, res.ExpectedLabels)
}

func newLabelOperations(labels map[string]string) []jsonpatch.Operation {
	operations := make([]jsonpatch.Operation, 0, len(labels)+1)
	operations = append(operations, jsonpatch.NewPatch("add", "/metadata/labels", struct{}{}))
	return operations
}

func labelPatchOperationFor(op, label, value string) jsonpatch.Operation {
	return jsonpatch.NewPatch(op, fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(label, "/", "~1")), value)
}

func encodePatch(operations []jsonpatch.Operation) client.Patch {
	patchBytes, err := json.Marshal(operations)
	Expect(err).NotTo(HaveOccurred())

	fmt.Fprintf(GinkgoWriter, "sending patch: %s", string(patchBytes))
	return client.RawPatch(types.JSONPatchType, patchBytes)
}

func waitForLabelMatch(resource controllerutil.Object, key client.ObjectKey, expectedLabels map[string]string) {
	Eventually(func() bool {
		err := apiClient.Get(ctx, key, resource)
		if err != nil {
			fmt.Fprintln(GinkgoWriter, err)
			return false
		}
		return labelsMatch(expectedLabels, resource)
	}, shortTimeout).Should(BeTrue(), "app labels were not added")
}

func labelsMatch(expectedLabels map[string]string, obj controllerutil.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	for label, value := range expectedLabels {
		foundValue, foundLabel := labels[label]
		if !foundLabel || foundValue != value {
			fmt.Fprintf(GinkgoWriter, "expected label %s=%s, got: %s\n", label, value, foundValue)
			return false
		}
	}

	return true
}
