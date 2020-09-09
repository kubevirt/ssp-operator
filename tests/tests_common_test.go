package tests

import (
	"reflect"

	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/operator-framework/operator-lib/handler"
	core "k8s.io/api/core/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/api/v1alpha1"
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

	// Watch status of the SSP resource
	watch, err := StartWatch(sspListerWatcher)
	Expect(err).ToNot(HaveOccurred())
	defer watch.Stop()

	Expect(apiClient.Delete(ctx, resource)).ToNot(HaveOccurred())

	err = WatchChangesUntil(watch, isStatusDeploying, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

	err = WatchChangesUntil(watch, isStatusDeployed, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")

	err = apiClient.Get(ctx, client.ObjectKey{
		Name: res.Name, Namespace: res.Namsespace,
	}, resource)
	Expect(err).ToNot(HaveOccurred())
}

func expectRestoreAfterUpdate(res *testResource, updateFunc interface{}, equalsFunc interface{}) {
	key := res.GetKey()
	original := res.NewResource()
	Expect(apiClient.Get(ctx, key, original)).ToNot(HaveOccurred())

	// Watch status of the SSP resource
	watch, err := StartWatch(sspListerWatcher)
	Expect(err).ToNot(HaveOccurred())
	defer watch.Stop()

	changed := original.DeepCopyObject()
	reflect.ValueOf(updateFunc).Call([]reflect.Value{reflect.ValueOf(changed)})
	Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

	err = WatchChangesUntil(watch, isStatusDeploying, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

	err = WatchChangesUntil(watch, isStatusDeployed, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")

	newRes := res.NewResource()
	Expect(apiClient.Get(ctx, key, newRes)).ToNot(HaveOccurred())
	result := reflect.ValueOf(equalsFunc).Call([]reflect.Value{
		reflect.ValueOf(original),
		reflect.ValueOf(newRes),
	})
	Expect(result[0].Interface().(bool)).To(BeTrue())
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

func isStatusDeploying(obj *v1alpha1.SSP) bool {
	available := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionAvailable)
	progressing := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionProgressing)
	degraded := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionDegraded)

	return obj.Status.Phase == api.PhaseDeploying &&
		available.Status == core.ConditionFalse &&
		progressing.Status == core.ConditionTrue &&
		degraded.Status == core.ConditionTrue
}

func isStatusDeployed(obj *v1alpha1.SSP) bool {
	available := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionAvailable)
	progressing := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionProgressing)
	degraded := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionDegraded)

	return obj.Status.Phase == api.PhaseDeployed &&
		available.Status == core.ConditionTrue &&
		progressing.Status == core.ConditionFalse &&
		degraded.Status == core.ConditionFalse
}
