package tests

import (
	"fmt"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"reflect"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/operator-framework/operator-lib/handler"
	authv1 "k8s.io/api/authorization/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/api/v1beta1"
)

const pauseDuration = 10 * time.Second

type testResource struct {
	Name      string
	Namespace string
	Resource  client.Object

	ExpectedLabels map[string]string

	UpdateFunc interface{}
	EqualsFunc interface{}
}

func (r *testResource) NewResource() client.Object {
	return r.Resource.DeepCopyObject().(client.Object)
}

func (r *testResource) GetKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

func (r *testResource) Update(obj client.Object) {
	reflect.ValueOf(r.UpdateFunc).Call([]reflect.Value{reflect.ValueOf(obj)})
}

func (r *testResource) Equals(a, b client.Object) bool {
	result := reflect.ValueOf(r.EqualsFunc).
		Call([]reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)})
	return result[0].Bool()
}

type resourceEqualsMatcher struct {
	res      *testResource
	expected client.Object
}

func (r *resourceEqualsMatcher) Match(actual interface{}) (success bool, err error) {
	actualObj, ok := actual.(client.Object)
	if !ok {
		return false, fmt.Errorf("EqualResource matcher expects client.Object. Got:\n%s", format.Object(actual, 1))
	}
	return r.res.Equals(r.expected, actualObj), nil
}

func (r *resourceEqualsMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be equal resource as", r.expected)
}

func (r *resourceEqualsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to not be equal resource as", r.expected)
}

func EqualResource(testRes *testResource, expected client.Object) types.GomegaMatcher {
	return &resourceEqualsMatcher{
		res:      testRes,
		expected: expected,
	}
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

	changed := original.DeepCopyObject().(client.Object)
	res.Update(changed)
	Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

	err = WatchChangesUntil(watch, isStatusDeploying, shortTimeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

	err = WatchChangesUntil(watch, isStatusDeployed, timeout)
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")

	found := res.NewResource()
	Expect(apiClient.Get(ctx, res.GetKey(), found)).ToNot(HaveOccurred())
	Expect(found).To(EqualResource(res, original))
}

func expectRestoreAfterUpdateWithPause(res *testResource) {
	if res.UpdateFunc == nil || res.EqualsFunc == nil {
		ginkgo.Fail("Update or Equals functions are not defined.")
	}

	original := res.NewResource()
	Expect(apiClient.Get(ctx, res.GetKey(), original)).ToNot(HaveOccurred())

	pauseSsp()

	changed := original.DeepCopyObject().(client.Object)
	res.Update(changed)
	Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

	Consistently(func() (client.Object, error) {
		found := res.NewResource()
		err := apiClient.Get(ctx, res.GetKey(), found)
		return found, err
	}, pauseDuration, time.Second).Should(EqualResource(res, changed))

	unpauseSsp()

	Eventually(func() (client.Object, error) {
		found := res.NewResource()
		err := apiClient.Get(ctx, res.GetKey(), found)
		return found, err
	}, timeout, time.Second).Should(EqualResource(res, original))
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

func getResourceKey(obj client.Object) client.ObjectKey {
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

func GetPodLogs(name, namespace string) (string, error) {
	RawLogs, err := coreClient.CoreV1().Pods(namespace).
		GetLogs(name, &core.PodLogOptions{}).DoRaw(ctx)
	return string(RawLogs), err
}

func GetRunningPodsByLabel(label, labelType, namespace string) (*core.PodList, error) {
	pods := &core.PodList{}
	err := apiClient.List(ctx, pods,
		client.InNamespace(namespace),
		client.MatchingLabels{labelType: label},
		client.MatchingFields{"status.phase": string(core.PodRunning)})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("failed to find pod with the label %s", label)
	}
	return pods, nil
}

func GetCertFromSecret(secretName, namespace string) (string, error) {
	secret := core.Secret{}
	err := apiClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, &secret)
	if err != nil {
		return "", err
	}
	if rawBundle, ok := secret.Data["tls.crt"]; ok {
		return string(rawBundle), nil
	}
	return "", nil
}

func NewRandomVMIWithBridgeInterface(namespace string) *kubevirtv1.VirtualMachineInstance {
	vmi := kubevirtv1.NewMinimalVMIWithNS(namespace, fmt.Sprintf("testvmi-%v", rand.String(10)))
	vmi.Spec.Domain.Resources.Requests = core.ResourceList{
		core.ResourceMemory: resource.MustParse("64M"),
	}
	t := int64(0)
	vmi.Spec.TerminationGracePeriodSeconds = &t
	vmi.Spec.Domain.Devices = kubevirtv1.Devices{
		Interfaces: []kubevirtv1.Interface{
			{
				Name: "default",
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Bridge: &kubevirtv1.InterfaceBridge{},
				},
			},
		},
	}
	vmi.Spec.Networks = []kubevirtv1.Network{*kubevirtv1.DefaultPodNetwork()}
	return vmi
}

func NewVirtualMachine(vmi *kubevirtv1.VirtualMachineInstance) *kubevirtv1.VirtualMachine {
	running := false
	name := vmi.Name
	namespace := vmi.Namespace
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Running: &running,
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    map[string]string{"kubevirt.io/vm": name},
					Name:      name + "makeitinteresting", // this name should have no effect
					Namespace: namespace,
				},
				Spec: vmi.Spec,
			},
		},
	}
	vm.SetGroupVersionKind(schema.GroupVersionKind{Group: kubevirtv1.GroupVersion.Group, Kind: "VirtualMachine", Version: kubevirtv1.GroupVersion.Version})
	return vm
}

func addDomainResourcesToVMI(vmi *kubevirtv1.VirtualMachineInstance, cpuCores uint32, machineType string, memory string) *kubevirtv1.VirtualMachineInstance {
	vmi.Spec.Domain.CPU = &kubevirtv1.CPU{
		Cores: cpuCores,
	}
	vmi.Spec.Domain.Machine = &kubevirtv1.Machine{Type: machineType}
	vmi.Spec.Domain.Resources.Requests = core.ResourceList{
		core.ResourceMemory: resource.MustParse(memory),
	}
	return vmi
}
