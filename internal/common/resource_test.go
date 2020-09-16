package common

import (
	"context"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/ssp-operator/pkg/apis"
	sspv1 "kubevirt.io/ssp-operator/pkg/apis/ssp/v1"
)

var log = logf.Log.WithName("common_operand_package")

var _ = Describe("Create or update resource", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var (
		request Request
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(apis.AddToScheme(s)).ToNot(HaveOccurred())

		client := fake.NewFakeClientWithScheme(s)
		request = Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Scheme:  s,
			Context: context.Background(),
			Instance: &sspv1.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger: log,
		}
	})

	It("should create resource", func() {
		Expect(CreateOrUpdateResource(&request,
			newTestResource(namespace),
			newEmptyResource(),
			NoUpdate,
		)).ToNot(HaveOccurred())
		expectEqualResourceExists(newTestResource(namespace), request)
	})

	It("should update resource", func() {
		resource := newTestResource(namespace)
		resource.Spec.Ports[0].Name = "changed-name"
		resource.Annotations["test-annotation"] = "test-changed"
		resource.Labels["test-label"] = "new-change"

		Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

		Expect(CreateOrUpdateResource(&request,
			newTestResource(namespace),
			newEmptyResource(),
			func(newRes Resource, foundRes Resource) bool {
				newService := newRes.(*v1.Service)
				foundService := foundRes.(*v1.Service)
				foundService.Spec = newService.Spec
				return true
			},
		)).ToNot(HaveOccurred())

		expectEqualResourceExists(newTestResource(namespace), request)
	})

	It("should set owner reference", func() {
		Expect(CreateOrUpdateResource(&request,
			newTestResource(namespace),
			newEmptyResource(),
			NoUpdate,
		)).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newTestResource(namespace))
		Expect(err).ToNot(HaveOccurred())

		found := newEmptyResource()
		Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

		Expect(found.GetOwnerReferences()).To(HaveLen(1))
		owner := found.GetOwnerReferences()[0]

		Expect(owner.Kind).To(Equal("SSP"))
	})

	It("should set owner annotations", func() {
		Expect(CreateOrUpdateClusterResource(&request,
			newTestResource(""),
			newEmptyResource(),
			NoUpdate,
		)).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newTestResource(""))
		Expect(err).ToNot(HaveOccurred())

		found := newEmptyResource()
		Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

		Expect(found.GetAnnotations()).To(HaveKeyWithValue(libhandler.TypeAnnotation, "SSP.ssp.kubevirt.io"))
		Expect(found.GetAnnotations()).To(HaveKey(libhandler.NamespacedNameAnnotation))
	})
})

func newTestResource(namespace string) *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testservice",
			Namespace: namespace,
			Labels: map[string]string{
				"test-label": "value1",
			},
			Annotations: map[string]string{
				"test-annotation": "value2",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Name:       "webhook",
				Port:       443,
				TargetPort: intstr.FromInt(8443),
			}},
			Selector: map[string]string{
				"kubevirtIo": "virtTemplateValidator",
			},
		},
	}
}

func newEmptyResource() *v1.Service {
	return &v1.Service{}
}

func expectEqualResourceExists(resource Resource, request Request) {
	key, err := client.ObjectKeyFromObject(resource)
	Expect(err).ToNot(HaveOccurred())

	found := newEmptyResource()
	Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

	resource.SetResourceVersion(found.GetResourceVersion())
	resource.SetOwnerReferences(found.GetOwnerReferences())

	Expect(found).To(Equal(resource))
}

func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Suite")
}
