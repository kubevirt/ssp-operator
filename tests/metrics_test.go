package tests

import (
	"reflect"
	"time"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/operands/metrics"
)

var _ = Describe("Metrics", func() {
	var (
		ruleKey = client.ObjectKey{
			Name:      metrics.PrometheusRuleName,
			Namespace: testNamespace,
		}
	)

	It("[test_id:4665] should create prometheus rule", func() {
		Expect(apiClient.Get(ctx, ruleKey, &promv1.PrometheusRule{})).ToNot(HaveOccurred())
	})

	It("should recreate deleted prometheus rule", func() {
		err := apiClient.DeleteAllOf(ctx, &promv1.PrometheusRule{},
			client.InNamespace(testNamespace),
			client.HasLabels{"kubevirt.io"})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			return apiClient.Get(ctx, ruleKey, &promv1.PrometheusRule{})
		}, timeout, time.Second).ShouldNot(HaveOccurred())
	})

	It("[test_id:4666] should restore modified prometheus rule", func() {
		originalRule := promv1.PrometheusRule{}
		err := apiClient.Get(ctx, ruleKey, &originalRule)
		Expect(err).ToNot(HaveOccurred())

		ruleCopy := originalRule.DeepCopy()
		ruleCopy.Spec.Groups[0].Name = "changed-name"
		ruleCopy.Spec.Groups[0].Rules = []promv1.Rule{}
		Expect(apiClient.Update(ctx, ruleCopy)).ToNot(HaveOccurred())

		Eventually(func() bool {
			newRule := promv1.PrometheusRule{}
			err := apiClient.Get(ctx, ruleKey, &newRule)
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(newRule.Spec, originalRule.Spec)
		}, timeout, time.Second).Should(BeTrue())
	})
})
