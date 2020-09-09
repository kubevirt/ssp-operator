package tests

import (
	"reflect"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/ssp-operator/internal/operands/metrics"
)

var _ = Describe("Metrics", func() {
	var prometheusRuleRes = &testResource{
		Name:       metrics.PrometheusRuleName,
		Namsespace: testNamespace,
		resource:   &promv1.PrometheusRule{},
	}

	BeforeEach(func() {
		waitUntilDeployed()
	})

	It("[test_id:4665] should create prometheus rule", func() {
		Expect(apiClient.Get(ctx, prometheusRuleRes.GetKey(), &promv1.PrometheusRule{})).ToNot(HaveOccurred())
	})

	It("should recreate deleted prometheus rule", func() {
		expectRecreateAfterDelete(prometheusRuleRes)
	})

	It("[test_id:4666] should restore modified prometheus rule", func() {
		expectRestoreAfterUpdate(prometheusRuleRes,
			func(rule *promv1.PrometheusRule) {
				rule.Spec.Groups[0].Name = "changed-name"
				rule.Spec.Groups[0].Rules = []promv1.Rule{}
			},
			func(old, new *promv1.PrometheusRule) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			})
	})
})
