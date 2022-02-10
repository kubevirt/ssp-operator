package tests

import (
	"net/http"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
)

var _ = Describe("Metrics", func() {
	var prometheusRuleRes testResource

	BeforeEach(func() {
		prometheusRuleRes = testResource{
			Name:           metrics.PrometheusRuleName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &promv1.PrometheusRule{},
			ExpectedLabels: expectedLabelsFor("metrics", common.AppComponentMonitoring),
			UpdateFunc: func(rule *promv1.PrometheusRule) {
				rule.Spec.Groups[0].Name = "changed-name"
				rule.Spec.Groups[0].Rules = []promv1.Rule{}
			},
			EqualsFunc: func(old, new *promv1.PrometheusRule) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}

		waitUntilDeployed()
	})

	It("[test_id:4665] should create prometheus rule", func() {
		Expect(apiClient.Get(ctx, prometheusRuleRes.GetKey(), &promv1.PrometheusRule{})).ToNot(HaveOccurred())
	})

	It("[test_id:6055] should recreate deleted prometheus rule", func() {
		expectRecreateAfterDelete(&prometheusRuleRes)
	})

	It("[test_id:4666] should restore modified prometheus rule", func() {
		expectRestoreAfterUpdate(&prometheusRuleRes)
	})

	Context("with pause", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()
		})

		JustAfterEach(func() {
			unpauseSsp()
		})

		It("[test_id:5397] should recreate modified prometheus rule after pause", func() {
			expectRestoreAfterUpdateWithPause(&prometheusRuleRes)
		})
	})

	Context("app labels", func() {
		It("[test_id:5790]adds app labels from SSP CR", func() {
			expectAppLabels(&prometheusRuleRes)
		})

		It("[test_id:5790]restores modified app labels", func() {
			expectAppLabelsRestoreAfterUpdate(&prometheusRuleRes)
		})
	})

	Context("alerts", func() {
		It("[test_id:7851]should have available runbook URLs", func() {
			promRule := &promv1.PrometheusRule{}
			Expect(apiClient.Get(ctx, prometheusRuleRes.GetKey(), promRule)).To(Succeed())
			for _, group := range promRule.Spec.Groups {
				for _, rule := range group.Rules {
					if len(rule.Alert) > 0 {
						Expect(rule.Annotations).ToNot(BeNil())
						url, ok := rule.Annotations["runbook_url"]
						Expect(ok).To(BeTrue())
						resp, err := http.Head(url)
						Expect(err).ToNot(HaveOccurred())
						Expect(resp.StatusCode).Should(Equal(http.StatusOK))
					}
				}
			}
		})
	})
})
