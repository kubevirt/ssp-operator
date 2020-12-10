package tests

import (
	"github.com/onsi/ginkgo"
	"reflect"

	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/operator-framework/operator-lib/handler"
	core "k8s.io/api/core/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/api/v1beta1"
)

type testResource struct {
	Name      string
	Namespace string
	Resource  controllerutil.Object

	UpdateFunc interface{}
	EqualsFunc interface{}
}

func (r *testResource) NewResource() controllerutil.Object {
	return r.Resource.DeepCopyObject().(controllerutil.Object)
}

func (r *testResource) GetKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

func (r *testResource) Update(obj controllerutil.Object) {
	reflect.ValueOf(r.UpdateFunc).Call([]reflect.Value{reflect.ValueOf(obj)})
}

func (r *testResource) Equals(a, b controllerutil.Object) bool {
	result := reflect.ValueOf(r.EqualsFunc).
		Call([]reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)})
	return result[0].Bool()
}

func expectRecreateAfterDelete(res *testResource) {
	resource := res.NewResource()
	resource.SetName(res.Name)
	resource.SetNamespace(res.Namespace)

	// Watch status of the SSP resource
	watch, err := StartWatch(sspListerWatcher)
	Expect(err).ToNot(HaveOccurred())
	defer watch.Stop()

	Expect(apiClient.Delete(ctx, resource)).ToNot(HaveOccurred())

	err = WatchChangesUntil(watch, isStatusDeploying, shortTimeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

	err = WatchChangesUntil(watch, isStatusDeployed, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")

	err = apiClient.Get(ctx, client.ObjectKey{
		Name: res.Name, Namespace: res.Namespace,
	}, resource)
	Expect(err).ToNot(HaveOccurred())
}

func expectRestoreAfterUpdate(res *testResource) {
	if res.UpdateFunc == nil || res.EqualsFunc == nil {
		ginkgo.Fail("Update or Equals functions are not defined.")
	}

	original := res.NewResource()
	Expect(apiClient.Get(ctx, res.GetKey(), original)).ToNot(HaveOccurred())

	// Watch status of the SSP resource
	watch, err := StartWatch(sspListerWatcher)
	Expect(err).ToNot(HaveOccurred())
	defer watch.Stop()

	changed := original.DeepCopyObject().(controllerutil.Object)
	res.Update(changed)
	Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

	err = WatchChangesUntil(watch, isStatusDeploying, shortTimeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

	err = WatchChangesUntil(watch, isStatusDeployed, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")

	found := res.NewResource()
	Expect(apiClient.Get(ctx, res.GetKey(), found)).ToNot(HaveOccurred())
	Expect(res.Equals(original, found)).To(BeTrue())
}

func hasOwnerAnnotations(annotations map[string]string) bool {
	const typeName = "SSP.ssp.kubevirt.io"
	namespacedName := strategy.GetNamespace() + "/" + strategy.GetName()

	if annotations == nil {
		return false
	}

	return annotations[handler.TypeAnnotation] == typeName &&
		annotations[handler.NamespacedNameAnnotation] == namespacedName
}

func isStatusDeploying(obj *v1beta1.SSP) bool {
	available := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionAvailable)
	progressing := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionProgressing)
	degraded := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionDegraded)

	return obj.Status.Phase == api.PhaseDeploying &&
		available.Status == core.ConditionFalse &&
		progressing.Status == core.ConditionTrue &&
		degraded.Status == core.ConditionTrue
}

func isStatusDeployed(obj *v1beta1.SSP) bool {
	available := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionAvailable)
	progressing := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionProgressing)
	degraded := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionDegraded)

	return obj.Status.Phase == api.PhaseDeployed &&
		available.Status == core.ConditionTrue &&
		progressing.Status == core.ConditionFalse &&
		degraded.Status == core.ConditionFalse
}
