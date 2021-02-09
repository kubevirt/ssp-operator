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
	var lastResult badLabels
	Eventually(func() bool {
		err := apiClient.Get(ctx, key, resource)
		if err != nil {
			fmt.Fprintln(GinkgoWriter, err)
			return false
		}
		badLabels := labelsMatch(expectedLabels, resource)
		if len(badLabels) > 0 {
			lastResult = badLabels
			return false
		}
		return true
	}, shortTimeout).Should(BeTrue(), func() string {
		return lastResult.String()
	})
}

func labelsMatch(expectedLabels map[string]string, obj controllerutil.Object) badLabels {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	badLabels := make(badLabels, len(expectedLabels))
	for label, value := range expectedLabels {
		foundValue, foundLabel := labels[label]
		if !foundLabel || foundValue != value {
			badLabels[label] = badLabel{expected: value, got: foundValue, missing: !foundLabel}
		}
	}

	return badLabels
}

type badLabel struct {
	expected, got string
	missing       bool
}
type badLabels map[string]badLabel

func (labels badLabels) String() string {
	str := strings.Builder{}
	str.WriteString("labels not matching expectations:\n")
	for label, badLabel := range labels {
		if badLabel.missing {
			str.WriteString(fmt.Sprintf("%s: missing\n", label))
			continue
		}
		str.WriteString(fmt.Sprintf("%s: expected: '%s', got: '%s'\n", label, badLabel.expected, badLabel.got))
	}
	return str.String()
}
