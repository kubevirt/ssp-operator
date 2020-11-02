package tests

import (
	"reflect"
	"time"

	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-lib/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type testResource struct {
	Name       string
	Namsespace string
	resource   controllerutil.Object
}

func (r *testResource) NewResource() controllerutil.Object {
	return r.resource.DeepCopyObject().(controllerutil.Object)
}

func (r *testResource) GetKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namsespace,
	}
}

func expectRecreateAfterDelete(res *testResource) {
	resource := res.NewResource()
	resource.SetName(res.Name)
	resource.SetNamespace(res.Namsespace)
	Expect(apiClient.Delete(ctx, resource)).ToNot(HaveOccurred())

	Eventually(func() error {
		return apiClient.Get(ctx, client.ObjectKey{
			Name: res.Name, Namespace: res.Namsespace,
		}, resource)
	}, timeout, time.Second).ShouldNot(HaveOccurred())
}

func expectRestoreAfterUpdate(res *testResource, updateFunc interface{}, equalsFunc interface{}) {
	key := res.GetKey()
	original := res.NewResource()
	Expect(apiClient.Get(ctx, key, original)).ToNot(HaveOccurred())

	changed := original.DeepCopyObject()
	reflect.ValueOf(updateFunc).Call([]reflect.Value{reflect.ValueOf(changed)})
	Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

	newRes := res.NewResource()
	Eventually(func() bool {
		Expect(apiClient.Get(ctx, key, newRes)).ToNot(HaveOccurred())
		res := reflect.ValueOf(equalsFunc).Call([]reflect.Value{
			reflect.ValueOf(original),
			reflect.ValueOf(newRes),
		})
		return res[0].Interface().(bool)
	}, timeout, time.Second).Should(BeTrue())
}

func hasOwnerAnnotations(annotations map[string]string) bool {
	const typeName = "SSP.ssp.kubevirt.io"
	namespacedName := ssp.Namespace + "/" + ssp.Name

	if annotations == nil {
		return false
	}

	return annotations[handler.TypeAnnotation] == typeName &&
		annotations[handler.NamespacedNameAnnotation] == namespacedName
}
