package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	securityv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/tests/decorators"
)

var _ = Describe("Required SCC annotation", func() {
	It("[test_id:TODO] SSP pods should have 'openshift.io/required-scc' annotation", decorators.Conformance, func() {
		deployment := &apps.Deployment{}
		Expect(apiClient.Get(ctx, types.NamespacedName{
			Name:      strategy.GetSSPDeploymentName(),
			Namespace: strategy.GetSSPDeploymentNameSpace(),
		}, deployment)).To(Succeed())

		selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
		Expect(err).ToNot(HaveOccurred())

		pods := &core.PodList{}
		Expect(apiClient.List(ctx, pods, client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
		Expect(pods.Items).ToNot(BeEmpty())

		for _, pod := range pods.Items {
			Expect(pod.Annotations).To(HaveKeyWithValue(securityv1.RequiredSCCAnnotation, common.RequiredSCCAnnotationValue),
				"SSP pod %s/%s does not have required annotation",
				pod.Namespace, pod.Name,
			)
		}
	})
})
