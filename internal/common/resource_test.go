package common

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1alpha1"
)

var log = logf.Log.WithName("common_operand_package")

const (
	namespace = "kubevirt"
	name      = "test-ssp"
)

var _ = Describe("Create or update resource", func() {
	var (
		request Request
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())

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
			Logger: log,
		}
	})

	It("should create resource", func() {
		Expect(createOrUpdateTestResource(&request)).ToNot(HaveOccurred())
		expectEqualResourceExists(newTestResource(namespace), &request)
	})

	It("should update resource", func() {
		resource := newTestResource(namespace)
		resource.Spec.Ports[0].Name = "changed-name"
		resource.Annotations["test-annotation"] = "test-changed"
		resource.Labels["test-label"] = "new-change"
		Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

		Expect(createOrUpdateTestResource(&request)).ToNot(HaveOccurred())
		expectEqualResourceExists(newTestResource(namespace), &request)
	})

	It("should set owner reference", func() {
		Expect(createOrUpdateTestResource(&request)).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newTestResource(namespace))
		Expect(err).ToNot(HaveOccurred())

		found := &v1.Service{}
		Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

		Expect(found.GetOwnerReferences()).To(HaveLen(1))
		owner := found.GetOwnerReferences()[0]

		Expect(owner.Kind).To(Equal("SSP"))
	})

	It("should set owner annotations", func() {
		Expect(CreateOrUpdateClusterResource(&request,
			newTestResource(""),
			func(expected, found controllerutil.Object) {
				found.(*v1.Service).Spec = expected.(*v1.Service).Spec
			},
		)).ToNot(HaveOccurred())

		key, err := client.ObjectKeyFromObject(newTestResource(""))
		Expect(err).ToNot(HaveOccurred())

		found := &v1.Service{}
		Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

		Expect(found.GetAnnotations()).To(HaveKeyWithValue(libhandler.TypeAnnotation, "SSP.ssp.kubevirt.io"))
		Expect(found.GetAnnotations()).To(HaveKey(libhandler.NamespacedNameAnnotation))
	})
})

func createOrUpdateTestResource(request *Request) error {
	return CreateOrUpdateResource(request,
		newTestResource(namespace),
		func(expected, found controllerutil.Object) {
			found.(*v1.Service).Spec = expected.(*v1.Service).Spec
		},
	)
}

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

func expectEqualResourceExists(resource controllerutil.Object, request *Request) {
	key, err := client.ObjectKeyFromObject(resource)
	Expect(err).ToNot(HaveOccurred())

	found := newEmptyResource(resource)
	Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

	resource.SetResourceVersion(found.GetResourceVersion())
	resource.SetOwnerReferences(found.GetOwnerReferences())

	Expect(found).To(Equal(resource))
}

func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Suite")
}
