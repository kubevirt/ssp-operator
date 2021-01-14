package tests

import (
	"context"
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
	testExpectedLabels(res.NewResource(), res.GetKey(), res.ExpectedLabels)
}

func expectAppLabelsRestoreAfterUpdate(res *testResource) {
	testExpectedLabelsRestoreAfterUpdate(res.NewResource(), res.GetKey(), res.ExpectedLabels)
}

func testExpectedLabels(resource controllerutil.Object, key client.ObjectKey, expectedLabels map[string]string) {
	patchAppLabelsIntoSSP(expectedLabels)
	waitForLabelMatch(resource, key, expectedLabels)
}

func testExpectedLabelsRestoreAfterUpdate(resource controllerutil.Object, key client.ObjectKey, expectedLabels map[string]string) {
	testExpectedLabels(resource, key, expectedLabels)

	operations := newLabelOperations(expectedLabels)
	for label := range expectedLabels {
		operations = append(operations, labelPatchOperationFor("replace", label, "wrong"))
	}
	patch := encodePatch(operations)
	err := apiClient.Patch(context.TODO(), resource, patch)
	Expect(err).NotTo(HaveOccurred())
	Expect(labelsMatch(expectedLabels, resource)).To(BeFalse())
}

func patchAppLabelsIntoSSP(expectedLabels map[string]string) {
	ssp := getSsp()
	Expect(ssp).NotTo(BeNil())

	if ssp.Labels != nil && labelsMatch(expectedLabels, ssp) {
		return
	}

	newLabels := map[string]string{
		common.AppKubernetesPartOfLabel:  expectedLabels[common.AppKubernetesPartOfLabel],
		common.AppKubernetesVersionLabel: expectedLabels[common.AppKubernetesVersionLabel],
	}

	patch := buildLabelsPatchAdding(newLabels)
	err := apiClient.Patch(context.TODO(), ssp, patch)
	Expect(err).NotTo(HaveOccurred(), "app labels could not be added to SSP CR")
}

func buildLabelsPatchAdding(labels map[string]string) client.Patch {
	return buildLabelsPatch("add", labels)
}

func buildLabelsPatch(operation string, labels map[string]string) client.Patch {
	operations := newLabelOperations(labels)

	for label, value := range labels {
		operations = append(operations, labelPatchOperationFor(operation, label, value))
	}

	return encodePatch(operations)
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
		err := apiClient.Get(context.TODO(), key, resource)
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
