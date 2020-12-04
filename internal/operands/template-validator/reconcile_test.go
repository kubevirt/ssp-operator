package template_validator

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
)

var log = logf.Log.WithName("validator_operand")

var _ = Describe("Template validator operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
		replicas  = 2
	)

	var (
		request common.Request
		operand = GetOperand()
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())

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
				TypeMeta: meta.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: meta.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ssp.SSPSpec{
					TemplateValidator: ssp.TemplateValidator{
						Replicas: replicas,
					},
				},
			},
			Logger:               log,
			ResourceVersionCache: common.VersionCache{},
		}
	})

	It("should create validator resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newClusterRole(namespace), request)
		ExpectResourceExists(newServiceAccount(namespace), request)
		ExpectResourceExists(newClusterRoleBinding(namespace), request)
		ExpectResourceExists(newService(namespace), request)
		ExpectResourceExists(newDeployment(namespace, replicas, "test-img"), request)
		ExpectResourceExists(newValidatingWebhook(namespace), request)
	})

	It("should not update webhook CA bundle", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newValidatingWebhook(namespace))
		Expect(err).ToNot(HaveOccurred())
		webhook := &admission.ValidatingWebhookConfiguration{}
		Expect(request.Client.Get(request.Context, key, webhook)).ToNot(HaveOccurred())

		const testCaBundle = "testCaBundle"
		webhook.Webhooks[0].ClientConfig.CABundle = []byte(testCaBundle)
		Expect(request.Client.Update(request.Context, webhook)).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		updatedWebhook := &admission.ValidatingWebhookConfiguration{}
		Expect(request.Client.Get(request.Context, key, updatedWebhook)).ToNot(HaveOccurred())
		Expect(updatedWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(testCaBundle)))
	})

	It("should not update service cluster IP", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newService(namespace))
		Expect(err).ToNot(HaveOccurred())
		service := &core.Service{}
		Expect(request.Client.Get(request.Context, key, service)).ToNot(HaveOccurred())

		const testClusterIp = "1.2.3.4"
		service.Spec.ClusterIP = testClusterIp
		Expect(request.Client.Update(request.Context, service)).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		updatedService := &core.Service{}
		Expect(request.Client.Get(request.Context, key, updatedService)).ToNot(HaveOccurred())
		Expect(updatedService.Spec.ClusterIP).To(Equal(testClusterIp))
	})

	It("should remove cluster resources on cleanup", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newClusterRole(namespace), request)
		ExpectResourceExists(newClusterRoleBinding(namespace), request)
		ExpectResourceExists(newValidatingWebhook(namespace), request)

		Expect(operand.Cleanup(&request)).ToNot(HaveOccurred())

		ExpectResourceNotExists(newClusterRole(namespace), request)
		ExpectResourceNotExists(newClusterRoleBinding(namespace), request)
		ExpectResourceNotExists(newValidatingWebhook(namespace), request)
	})

	It("should report status", func() {
		statuses, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// All resources should be progressing
		for _, status := range statuses {
			Expect(status.NotAvailable).ToNot(BeNil())
			Expect(status.Progressing).ToNot(BeNil())
			Expect(status.Degraded).ToNot(BeNil())
		}

		statuses, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Only deployment should be progressing
		for _, status := range statuses {
			if _, ok := status.Resource.(*apps.Deployment); ok {
				Expect(status.NotAvailable).ToNot(BeNil())
				Expect(status.Progressing).ToNot(BeNil())
				Expect(status.Degraded).ToNot(BeNil())
			} else {
				Expect(status.NotAvailable).To(BeNil())
				Expect(status.Progressing).To(BeNil())
				Expect(status.Degraded).To(BeNil())
			}
		}

		key, err := client.ObjectKeyFromObject(newDeployment(namespace, replicas, "test-img"))
		deployment := &apps.Deployment{}
		Expect(request.Client.Get(request.Context, key, deployment)).ToNot(HaveOccurred())

		deployment.Status.Replicas = replicas
		deployment.Status.ReadyReplicas = replicas
		deployment.Status.AvailableReplicas = replicas
		deployment.Status.UpdatedReplicas = replicas
		Expect(request.Client.Update(request.Context, deployment)).ToNot(HaveOccurred())

		statuses, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// All resources should be available
		for _, status := range statuses {
			Expect(status.NotAvailable).To(BeNil())
			Expect(status.Progressing).To(BeNil())
			Expect(status.Degraded).To(BeNil())
		}
	})
})

func TestValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Validator Suite")
}
