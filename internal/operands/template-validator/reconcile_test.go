package template_validator

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admission "k8s.io/api/admissionregistration/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/pkg/apis"
	ssp "kubevirt.io/ssp-operator/pkg/apis/ssp/v1"
)

var log = logf.Log.WithName("validator_operand")

var _ = Describe("Template validator operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var request common.Request

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(apis.AddToScheme(s)).ToNot(HaveOccurred())

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
			Instance: &ssp.SSP{
				ObjectMeta: meta.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger: log,
		}
	})

	It("should create validator resources", func() {
		Expect(Reconcile(&request)).ToNot(HaveOccurred())

		expectResourceExists(newClusterRole(namespace), request)
		expectResourceExists(newServiceAccount(namespace), request)
		expectResourceExists(newClusterRoleBinding(namespace), request)
		expectResourceExists(newService(namespace), request)
		expectResourceExists(newDeployment(namespace, 2, "test-img"), request)
		expectResourceExists(newValidatingWebhook(namespace), request)
	})

	It("should not update webhook CA bundle", func() {
		Expect(Reconcile(&request)).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newValidatingWebhook(namespace))
		Expect(err).ToNot(HaveOccurred())
		webhook := &admission.ValidatingWebhookConfiguration{}
		Expect(request.Client.Get(request.Context, key, webhook)).ToNot(HaveOccurred())

		const testCaBundle = "testCaBundle"
		webhook.Webhooks[0].ClientConfig.CABundle = []byte(testCaBundle)
		Expect(request.Client.Update(request.Context, webhook)).ToNot(HaveOccurred())

		Expect(Reconcile(&request)).ToNot(HaveOccurred())

		updatedWebhook := &admission.ValidatingWebhookConfiguration{}
		Expect(request.Client.Get(request.Context, key, updatedWebhook)).ToNot(HaveOccurred())
		Expect(updatedWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(testCaBundle)))
	})

	It("should not update service cluster IP", func() {
		Expect(Reconcile(&request)).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newService(namespace))
		Expect(err).ToNot(HaveOccurred())
		service := &core.Service{}
		Expect(request.Client.Get(request.Context, key, service)).ToNot(HaveOccurred())

		const testClusterIp = "1.2.3.4"
		service.Spec.ClusterIP = testClusterIp
		Expect(request.Client.Update(request.Context, service)).ToNot(HaveOccurred())

		Expect(Reconcile(&request)).ToNot(HaveOccurred())

		updatedService := &core.Service{}
		Expect(request.Client.Get(request.Context, key, updatedService)).ToNot(HaveOccurred())
		Expect(updatedService.Spec.ClusterIP).To(Equal(testClusterIp))
	})
})

func expectResourceExists(resource common.Resource, request common.Request) {
	key, err := client.ObjectKeyFromObject(resource)
	Expect(err).ToNot(HaveOccurred())
	Expect(request.Client.Get(request.Context, key, resource)).ToNot(HaveOccurred())
}

func TestValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Validator Suite")
}
