package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/controllers"
	"kubevirt.io/ssp-operator/tests/env"
)

var _ = Describe("Webhook controller", func() {
	var webhook *admissionv1.ValidatingWebhookConfiguration

	BeforeEach(func() {
		webhook = &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ssp.kubevirt.io-test-webhook-",
				Labels: map[string]string{
					controllers.OlmNameLabel: controllers.OlmNameLabelValue,
				},
			},
			Webhooks: []admissionv1.ValidatingWebhook{{
				Name: "test.webhook.ssp.kubevirt.io",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Name:      "non-existing-service",
						Namespace: "non-existing-namespace",
					},
				},
				Rules: []admissionv1.RuleWithOperations{{
					// Using "Delete" so it does not conflict with existing SSP webhook
					Operations: []admissionv1.OperationType{admissionv1.Delete},
					Rule: admissionv1.Rule{
						APIGroups:   []string{ssp.GroupVersion.Group},
						APIVersions: []string{ssp.GroupVersion.Version},
						Resources:   []string{"ssps"},
					},
				}},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ssp-webhook-test-label": "ssp-webhook-test-label-vale",
					},
				},
				SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			}},
		}
	})

	It("[test_id:TODO] should delete namespaceSelector from webhook configuration with OLM label", func() {
		Expect(apiClient.Create(ctx, webhook)).To(Succeed())
		DeferCleanup(func() {
			Expect(apiClient.Delete(ctx, webhook)).To(Succeed())
		})

		Eventually(func(g Gomega) {
			updatedWebhook := &admissionv1.ValidatingWebhookConfiguration{}
			g.Expect(apiClient.Get(ctx, client.ObjectKeyFromObject(webhook), updatedWebhook)).To(Succeed())
			g.Expect(updatedWebhook.Webhooks).To(HaveLen(1))

			namespaceSelector := updatedWebhook.Webhooks[0].NamespaceSelector
			if namespaceSelector != nil {
				g.Expect(namespaceSelector.MatchLabels).To(BeEmpty())
				g.Expect(namespaceSelector.MatchExpressions).To(BeEmpty())
			}
		}, env.ShortTimeout(), time.Second).Should(Succeed())
	})

	It("[test_id:TODO] should not delete namespaceSelector from webhook configuration without OLM label", func() {
		webhook.Labels = nil

		Expect(apiClient.Create(ctx, webhook)).To(Succeed())
		DeferCleanup(func() {
			Expect(apiClient.Delete(ctx, webhook)).To(Succeed())
		})

		Consistently(func(g Gomega) {
			updatedWebhook := &admissionv1.ValidatingWebhookConfiguration{}
			g.Expect(apiClient.Get(ctx, client.ObjectKeyFromObject(webhook), updatedWebhook)).To(Succeed())
			g.Expect(updatedWebhook.Webhooks).To(HaveLen(1))

			namespaceSelector := updatedWebhook.Webhooks[0].NamespaceSelector
			g.Expect(namespaceSelector).ToNot(BeNil())
			g.Expect(namespaceSelector.MatchLabels).ToNot(BeEmpty())
		}, 10*time.Second, time.Second).Should(Succeed())
	})
})
