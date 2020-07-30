package metrics

import (
	"context"
	"testing"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/pkg/apis"
	v1 "kubevirt.io/ssp-operator/pkg/apis/ssp/v1"
)

var log = logf.Log.WithName("metrics_operand")

var _ = Describe("Metrics operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var (
		request common.Request
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(apis.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(AddWatchTypesToScheme(s)).ToNot(HaveOccurred())

		client := fake.NewFakeClientWithScheme(s)

		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Scheme:  s,
			Context: context.Background(),
			Instance: &v1.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger: log,
		}
	})

	It("should create the prometheus rule", func() {
		Expect(Reconcile(&request)).ToNot(HaveOccurred())
		expectEqualRuleExists(newPrometheusRule(namespace), request)
	})

	It("should update the prometheus rule", func() {
		existingRule := newPrometheusRule(namespace)
		existingRule.Spec.Groups[0].Name = "changed-name"
		existingRule.Spec.Groups[0].Rules = nil
		Expect(request.Client.Create(request.Context, existingRule)).ToNot(HaveOccurred())

		Expect(Reconcile(&request)).ToNot(HaveOccurred())

		expectEqualRuleExists(newPrometheusRule(namespace), request)
	})
})

func expectEqualRuleExists(rule *promv1.PrometheusRule, request common.Request) {
	key, err := client.ObjectKeyFromObject(rule)
	Expect(err).ToNot(HaveOccurred())

	var found promv1.PrometheusRule
	Expect(request.Client.Get(request.Context, key, &found)).ToNot(HaveOccurred())
	Expect(found.Spec).To(Equal(rule.Spec))
}

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}
