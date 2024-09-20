package common

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
)

var log = logf.Log.WithName("common_operand_package")

const (
	namespace = "kubevirt"
	name      = "test-ssp"
)

// TODO -- add unit tests for the watch label

var _ = Describe("Resource", func() {
	var (
		request Request
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())

		client := fake.NewClientBuilder().WithScheme(s).Build()
		request = Request{
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
					Kind:       sspResourceKind,
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger:       log,
			VersionCache: VersionCache{},
		}
	})

	Context("CreateOrUpdate", func() {
		It("should create resource", func() {
			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(newTestResource(namespace), &request)
		})

		It("should update resource", func() {
			resource := newTestResource(namespace)
			resource.Spec.Ports[0].Name = "changed-name"
			resource.Annotations["test-annotation"] = "test-changed"
			resource.Labels["test-label"] = "new-change"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(newTestResource(namespace), &request)
		})

		It("should set owner reference", func() {
			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(newTestResource(namespace))

			found := &v1.Service{}
			Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

			Expect(found.GetOwnerReferences()).To(HaveLen(1))
			owner := found.GetOwnerReferences()[0]

			Expect(owner.Kind).To(Equal("SSP"))
		})

		It("should set owner annotations", func() {
			_, err := CreateOrUpdate(&request).
				ClusterResource(newTestResource("")).
				UpdateFunc(func(expected, found client.Object) {
					found.(*v1.Service).Spec = expected.(*v1.Service).Spec
				}).
				Reconcile()

			Expect(err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(newTestResource(""))
			found := &v1.Service{}
			Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

			Expect(found.GetAnnotations()).To(HaveKeyWithValue(libhandler.TypeAnnotation, "SSP.ssp.kubevirt.io"))
			Expect(found.GetAnnotations()).To(HaveKey(libhandler.NamespacedNameAnnotation))
		})

		It("should not update resource with cached version", func() {
			resource := newTestResource(namespace)
			resource.Spec.Ports[0].Name = "changed-name"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			request.VersionCache.Add(resource)

			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(resource, &request)
		})

		It("should not update resource with cached generation", func() {
			resource := newTestResource(namespace)
			resource.Generation = 1
			resource.Spec.Ports[0].Name = "changed-name"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			request.VersionCache.Add(resource)

			resource.Spec.Ports[0].Name = "changed-name-2"
			Expect(request.Client.Update(request.Context, resource)).ToNot(HaveOccurred())

			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(resource, &request)
		})

		It("should update resource with different version in cache", func() {
			resource := newTestResource(namespace)
			resource.Spec.Ports[0].Name = "changed-name"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			request.VersionCache.Add(resource)

			resource.Spec.Ports[0].Name = "changed-name-2"
			Expect(request.Client.Update(request.Context, resource)).ToNot(HaveOccurred())

			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(newTestResource(namespace), &request)
		})

		It("should update resource when AlwaysCallUpdateFunc is set", func() {
			resource := newTestResource(namespace)
			resource.Spec.Ports[0].Name = "changed-name"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			request.VersionCache.Add(resource)

			_, err := CreateOrUpdate(&request).
				NamespacedResource(newTestResource(namespace)).
				Options(ReconcileOptions{AlwaysCallUpdateFunc: true}).
				UpdateFunc(func(expected, found client.Object) {
					found.(*v1.Service).Spec = expected.(*v1.Service).Spec
				}).
				Reconcile()

			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(newTestResource(namespace), &request)
		})

		It("should delete immutable resource on spec update", func() {
			resource := newTestResource(namespace)
			resource.Spec.Ports[0].Name = "changed-name"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			_, err := CreateOrUpdate(&request).
				NamespacedResource(newTestResource(namespace)).
				UpdateFunc(func(expected, found client.Object) {
					found.(*v1.Service).Spec = expected.(*v1.Service).Spec
				}).
				ImmutableSpec(func(resource client.Object) interface{} {
					return resource.(*v1.Service).Spec
				}).
				Reconcile()

			Expect(err).ToNot(HaveOccurred())

			err = request.Client.Get(request.Context, client.ObjectKeyFromObject(resource), resource)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should not delete immutable resource on metadata update", func() {
			resource := newTestResource(namespace)
			resource.Annotations["test-annotation"] = "test-changed"
			resource.Labels["test-label"] = "new-change"
			Expect(request.Client.Create(request.Context, resource)).ToNot(HaveOccurred())

			_, err := CreateOrUpdate(&request).
				NamespacedResource(newTestResource(namespace)).
				UpdateFunc(func(expected, found client.Object) {
					found.(*v1.Service).Spec = expected.(*v1.Service).Spec
				}).
				ImmutableSpec(func(resource client.Object) interface{} {
					return resource.(*v1.Service).Spec
				}).
				Reconcile()

			Expect(err).ToNot(HaveOccurred())
			expectEqualResourceExists(newTestResource(namespace), &request)
		})
	})

	Context("Cleanup", func() {
		It("should succeed Cleanup, if no resource is present", func() {
			nonexistingResource := newTestResource(namespace)
			cleanupResult, err := Cleanup(&request, nonexistingResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(cleanupResult.Deleted).To(BeTrue())
		})

		It("should not delete, if the resource is not owned by SSP CR", func() {
			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())

			resource := newTestResource(namespace)
			err = request.Client.Get(request.Context, client.ObjectKeyFromObject(resource), resource)
			Expect(err).ToNot(HaveOccurred())

			resource.SetOwnerReferences(nil)
			resource.SetAnnotations(nil)

			err = request.Client.Update(request.Context, resource)
			Expect(err).ToNot(HaveOccurred())

			cleanupResult, err := Cleanup(&request, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(cleanupResult.Deleted).To(BeTrue())

			// Object should still exist
			err = request.Client.Get(request.Context, client.ObjectKeyFromObject(resource), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource.GetDeletionTimestamp().IsZero()).To(BeTrue(), "Deletion timestamp should not be set")
		})

		It("should not delete if resource is already being deleted", func() {
			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())

			resource := newTestResource(namespace)
			err = request.Client.Get(request.Context, client.ObjectKeyFromObject(resource), resource)
			Expect(err).ToNot(HaveOccurred())

			resource.Finalizers = append(resource.Finalizers, "testfinalizer")

			err = request.Client.Update(request.Context, resource)
			Expect(err).ToNot(HaveOccurred())

			err = request.Client.Delete(request.Context, resource)
			Expect(err).ToNot(HaveOccurred())

			cleanupResult, err := Cleanup(&request, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(cleanupResult.Deleted).To(BeFalse())

			err = request.Client.Get(request.Context, client.ObjectKeyFromObject(resource), resource)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete resource", func() {
			_, err := createOrUpdateTestResource(&request)
			Expect(err).ToNot(HaveOccurred())

			resource := newTestResource(namespace)
			cleanupResult, err := Cleanup(&request, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(cleanupResult.Deleted).To(BeFalse())

			// Deleting second time will make sure that the resource does not exist
			cleanupResult, err = Cleanup(&request, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(cleanupResult.Deleted).To(BeTrue())
		})
	})
})

func createOrUpdateTestResource(request *Request) (ReconcileResult, error) {
	return CreateOrUpdate(request).
		NamespacedResource(newTestResource(namespace)).
		UpdateFunc(func(expected, found client.Object) {
			found.(*v1.Service).TypeMeta = expected.(*v1.Service).TypeMeta
			found.(*v1.Service).Spec = expected.(*v1.Service).Spec
		}).
		Reconcile()
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

func expectEqualResourceExists(resource client.Object, request *Request) {
	key := client.ObjectKeyFromObject(resource)
	found := newEmptyResource(resource)
	Expect(request.Client.Get(request.Context, key, found)).ToNot(HaveOccurred())

	resource.SetGeneration(found.GetGeneration())
	resource.SetResourceVersion(found.GetResourceVersion())
	resource.SetOwnerReferences(found.GetOwnerReferences())

	if resource.GetLabels() == nil {
		resource.SetLabels(map[string]string{})
	}
	resource.GetLabels()[WatchedObjectLabel] = "true"

	ExpectWithOffset(1, found).To(Equal(resource))
}
