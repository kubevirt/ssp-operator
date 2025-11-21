package metrics

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

var log = logf.Log.WithName("metrics_operand")

var _ = Describe("Metrics operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var (
		request common.Request
		operand = New()
	)

	BeforeEach(func() {
		Expect(rules.SetupRules()).To(Succeed())

		client := fake.NewClientBuilder().WithScheme(common.Scheme).Build()
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Context: context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger:       log,
			VersionCache: common.VersionCache{},
		}
	})

	AfterEach(func() {
		Expect(os.Unsetenv(runbookURLTemplateEnv)).To(Succeed())
	})

	It("should create metrics resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		prometheusRule, err := rules.BuildPrometheusRule(namespace)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(prometheusRule, request)
		ExpectResourceExists(newSspServiceMonitor(&request), request)
		ExpectResourceExists(newValidatorServiceMonitor(&request), request)
		ExpectResourceExists(newMonitoringClusterRole(), request)
		ExpectResourceExists(newMonitoringClusterRoleBinding(), request)
	})

	DescribeTable("runbook URL template",
		func(template string) {
			if template != defaultRunbookURLTemplate {
				Expect(os.Setenv(runbookURLTemplateEnv, template)).To(Succeed())
			}

			err := rules.SetupRules()

			if strings.Count(template, "%s") != 1 || strings.Count(template, "%") != 1 {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(err).ToNot(HaveOccurred())

			prometheusRule, err := rules.BuildPrometheusRule(namespace)
			Expect(err).ToNot(HaveOccurred())

			for _, group := range prometheusRule.Spec.Groups {
				for _, rule := range group.Rules {
					if rule.Alert != "" {
						if rule.Annotations["runbook_url"] != "" {
							Expect(rule.Annotations["runbook_url"]).To(Equal(fmt.Sprintf(template, rule.Alert)))
						}
					}
				}
			}
		},
		Entry("should use the default template when no ENV variable is set", defaultRunbookURLTemplate),
		Entry("should use the ENV variable when a valid value is set", "valid/runbookURL/template/%s"),
		Entry("should throw an error when the ENV variable value doesn't contain a string format placeholder", "invalid/runbookURL/template/"),
		Entry("should throw an error when the ENV variable value contains an integer format placeholder", "invalid/runbookURL/template/%d"),
	)
})

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}
