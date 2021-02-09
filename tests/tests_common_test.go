package tests

import (
	"fmt"
	"reflect"
	"time"

	"github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/operator-framework/operator-lib/handler"
	authv1 "k8s.io/api/authorization/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/api/v1beta1"
)

const pauseDuration = 10 * time.Second

type testResource struct {
	Name      string
	Namespace string
	Resource  controllerutil.Object

	ExpectedLabels map[string]string

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

func expectRestoreAfterUpdateWithPause(res *testResource) {
	if res.UpdateFunc == nil || res.EqualsFunc == nil {
		ginkgo.Fail("Update or Equals functions are not defined.")
	}

	original := res.NewResource()
	Expect(apiClient.Get(ctx, res.GetKey(), original)).ToNot(HaveOccurred())

	pauseSsp()

	changed := original.DeepCopyObject().(controllerutil.Object)
	res.Update(changed)
	Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

	Consistently(func() (bool, error) {
		found := res.NewResource()
		err := apiClient.Get(ctx, res.GetKey(), found)
		if err != nil {
			return false, err
		}
		return res.Equals(changed, found), nil
	}, pauseDuration, time.Second).Should(BeTrue())

	unpauseSsp()

	Eventually(func() (bool, error) {
		found := res.NewResource()
		err := apiClient.Get(ctx, res.GetKey(), found)
		if err != nil {
			return false, err
		}
		return res.Equals(original, found), nil
	}, timeout, time.Second).Should(BeTrue())
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

func updateSsp(updateFunc func(foundSsp *v1beta1.SSP)) {
	Eventually(func() error {
		foundSsp := getSsp()
		updateFunc(foundSsp)
		return apiClient.Update(ctx, foundSsp)
	}, timeout, time.Second).ShouldNot(HaveOccurred())
}

func pauseSsp() {
	updateSsp(func(foundSsp *v1beta1.SSP) {
		if foundSsp.Annotations == nil {
			foundSsp.Annotations = map[string]string{}
		}
		foundSsp.Annotations[v1beta1.OperatorPausedAnnotation] = "true"
	})
	Eventually(func() bool {
		return getSsp().Status.Paused
	}, shortTimeout, time.Second).Should(BeTrue())
}

func unpauseSsp() {
	updateSsp(func(foundSsp *v1beta1.SSP) {
		delete(foundSsp.Annotations, v1beta1.OperatorPausedAnnotation)
	})
	Eventually(func() bool {
		return getSsp().Status.Paused
	}, shortTimeout, time.Second).Should(BeFalse())
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

func getResourceKey(obj controllerutil.Object) client.ObjectKey {
	return client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func expectUserCan(sars *authv1.SubjectAccessReviewSpec) {
	sar, err := coreClient.AuthorizationV1().SubjectAccessReviews().Create(ctx, &authv1.SubjectAccessReview{
		Spec: *sars,
	}, metav1.CreateOptions{})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, sar.Status.Allowed).To(BeTrue(),
		fmt.Sprintf("user [%s] with groups %v cannot [%s] resource: [%s], subresource: [%s], name: [%s] in group [%s/%s] in namespace [%s]",
			sars.User, sars.Groups, sars.ResourceAttributes.Verb, sars.ResourceAttributes.Resource,
			sars.ResourceAttributes.Subresource, sars.ResourceAttributes.Name, sars.ResourceAttributes.Group,
			sars.ResourceAttributes.Version, sars.ResourceAttributes.Namespace))
}

func expectUserCannot(sars *authv1.SubjectAccessReviewSpec) {
	sar, err := coreClient.AuthorizationV1().SubjectAccessReviews().Create(ctx, &authv1.SubjectAccessReview{
		Spec: *sars,
	}, metav1.CreateOptions{})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	// Cannot assert Status.Denied here because it is optional and can be flaky even if Allowed is false
	ExpectWithOffset(1, sar.Status.Allowed).To(BeFalse(),
		fmt.Sprintf("user [%s] with groups %v should not be able to [%s] resource: [%s], subresource: [%s], name: [%s] in group [%s/%s] in namespace [%s]",
			sars.User, sars.Groups, sars.ResourceAttributes.Verb, sars.ResourceAttributes.Resource,
			sars.ResourceAttributes.Subresource, sars.ResourceAttributes.Name, sars.ResourceAttributes.Group,
			sars.ResourceAttributes.Version, sars.ResourceAttributes.Namespace))
}
