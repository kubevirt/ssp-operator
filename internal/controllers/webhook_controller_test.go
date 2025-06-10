package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/common"
)

var _ = Describe("Webhook controller", func() {

	var (
		webhookConfig  *admissionv1.ValidatingWebhookConfiguration
		fakeClient     client.Client
		testController *webhookCtrl
		testRequest    reconcile.Request
	)

	BeforeEach(func() {
		webhookConfig = &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-webhook",
				Labels: map[string]string{
					OlmNameLabel: OlmNameLabelValue,
				},
			},
			Webhooks: []admissionv1.ValidatingWebhook{{
				Name: "test-ssp-webhook",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "test-namespace",
						Name:      "test-name",
						Path:      ptr.To("/webhook"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{{
					Rule: admissionv1.Rule{
						APIGroups:   []string{ssp.GroupVersion.Group},
						APIVersions: []string{ssp.GroupVersion.Version},
						Resources:   []string{"ssps"},
					},
					Operations: []admissionv1.OperationType{
						admissionv1.Create, admissionv1.Update,
					},
				}},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test-namespace-label": "some-value",
					},
				},
				SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
			}},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(common.Scheme).Build()

		testController = NewWebhookConfigurationController().(*webhookCtrl)
		testController.apiClient = fakeClient

		testRequest = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: webhookConfig.Name,
			},
		}
	})

	It("should remove namespaceSelector from webhook", func() {
		Expect(fakeClient.Create(context.Background(), webhookConfig)).To(Succeed())

		_, err := testController.Reconcile(context.Background(), testRequest)
		Expect(err).ToNot(HaveOccurred())

		updatedConfig := &admissionv1.ValidatingWebhookConfiguration{}
		Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(webhookConfig), updatedConfig)).To(Succeed())

		Expect(updatedConfig.Webhooks).ToNot(BeEmpty())
		for _, webhook := range updatedConfig.Webhooks {
			Expect(webhook.NamespaceSelector).To(BeNil())
		}
	})

	It("should not remove namespaceSelector from non SSP webhook", func() {
		webhookConfig.Webhooks[0].Rules[0].APIGroups = []string{"non-ssp-api-group"}

		Expect(fakeClient.Create(context.Background(), webhookConfig)).To(Succeed())

		_, err := testController.Reconcile(context.Background(), testRequest)
		Expect(err).ToNot(HaveOccurred())

		updatedConfig := &admissionv1.ValidatingWebhookConfiguration{}
		Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(webhookConfig), updatedConfig)).To(Succeed())

		Expect(updatedConfig).To(Equal(webhookConfig))
	})

	It("should not remove namespaceSelector from webhook without labels", func() {
		webhookConfig.Labels = nil

		Expect(fakeClient.Create(context.Background(), webhookConfig)).To(Succeed())

		_, err := testController.Reconcile(context.Background(), testRequest)
		Expect(err).ToNot(HaveOccurred())

		updatedConfig := &admissionv1.ValidatingWebhookConfiguration{}
		Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(webhookConfig), updatedConfig)).To(Succeed())

		Expect(updatedConfig).To(Equal(webhookConfig))
	})
})
