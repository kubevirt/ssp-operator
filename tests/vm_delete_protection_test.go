package tests

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/tests/decorators"
	"kubevirt.io/ssp-operator/tests/env"
)

const (
	deleteProtectionLabel = "kubevirt.io/vm-delete-protection"
	vmReadyTimeout        = 5 * time.Minute
)

var _ = Describe("VM delete protection", func() {

	var vm *kubevirtv1.VirtualMachine

	BeforeEach(func() {
		waitUntilDeployed()
	})

	AfterEach(func() {
		if vm != nil {
			err := apiClient.Get(ctx, client.ObjectKeyFromObject(vm), vm)
			Expect(err).To(Or(Not(HaveOccurred()), MatchError(errors.IsNotFound, "errors.IsNotFound")))

			if err == nil && vm.DeletionTimestamp == nil {
				Eventually(func() error {
					if err := apiClient.Get(ctx, client.ObjectKeyFromObject(vm), vm); err != nil {
						return err
					}

					vm.Labels[deleteProtectionLabel] = "False"
					return apiClient.Update(ctx, vm)
				}, env.ShortTimeout(), time.Second).Should(Succeed())

				Expect(apiClient.Delete(ctx, vm)).To(Succeed())
				waitForDeletion(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})
				vm = nil
			}
		}
	})

	DescribeTable("should not allow to delete a VM if the protection is enabled", decorators.Conformance, func(labelValue string) {
		vm = createVMWithDeleteProtection(labelValue, strategy.GetNamespace())

		Expect(apiClient.Delete(ctx, vm)).To(MatchError(ContainSubstring("VirtualMachine %v cannot be deleted, disable/remove label "+
			"'kubevirt.io/vm-delete-protection' from VirtualMachine before deleting it", vm.Name)))
	},
		Entry("[test_id:11926] using True as value", "True"),
		Entry("[test_id:11927] using true as value", "true"),
	)

	DescribeTable("should be able to delete a VM if the protection is disabled", decorators.Conformance, func(labelValue string) {
		vm = createVMWithDeleteProtection(labelValue, strategy.GetNamespace())

		Expect(apiClient.Delete(ctx, vm)).To(Succeed())
		waitForDeletion(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})
	},
		Entry("[test_id:11928] using False as value", "False"),
		Entry("[test_id:11929] using false as value", "false"),
		Entry("[test_id:11930] using value different from false or False", "niceValue"),
		Entry("[test_id:11931] using true in upper case", "TRUE"),
		Entry("[test_id:11932] using empty string as value", ""),
	)

	It("[test_id:11934] should be able to delete a VM if the VM does not have any label", decorators.Conformance, func() {
		vm = createVMWithLabels(nil, strategy.GetNamespace())

		Expect(apiClient.Delete(ctx, vm)).To(Succeed())
		waitForDeletion(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})
	})

	It("should not be able to delete the controller revision directly", func() {
		vm = createVMWithDeleteProtection("false", strategy.GetNamespace())

		startVM(vm.Name, vm.Namespace, vmReadyTimeout)

		Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Name, Namespace: vm.Namespace}, vm)).ToNot(HaveOccurred())

		controllerRevision := &appsv1.ControllerRevision{}
		Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Status.InstancetypeRef.ControllerRevisionRef.Name, Namespace: vm.Namespace}, controllerRevision)).ToNot(HaveOccurred())

		Expect(apiClient.Delete(ctx, controllerRevision)).To(MatchError(ContainSubstring("Instancetype controller revision deletion is blocked only GC/kubevirt-controller")))
	})

	It("should be able to clean up the controller revision when the VM is deleted", func() {
		vm = createVMWithDeleteProtection("false", strategy.GetNamespace())

		startVM(vm.Name, vm.Namespace, vmReadyTimeout)

		Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Name, Namespace: vm.Namespace}, vm)).ToNot(HaveOccurred())

		controllerRevisionName := vm.Status.InstancetypeRef.ControllerRevisionRef.Name

		Expect(apiClient.Delete(ctx, vm)).To(Succeed())
		waitForDeletion(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})

		Eventually(func() error {
			controllerRevision := &appsv1.ControllerRevision{}
			return apiClient.Get(ctx, client.ObjectKey{Name: controllerRevisionName, Namespace: vm.Namespace}, controllerRevision)
		}, env.ShortTimeout(), time.Second).Should(MatchError(errors.IsNotFound, "errors.IsNotFound"))
	})

	It("should not able to delete the controller revisions if the VM is protected", func() {
		vm = createVMWithDeleteProtection("true", strategy.GetNamespace())

		startVM(vm.Name, vm.Namespace, vmReadyTimeout)

		Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Name, Namespace: vm.Namespace}, vm)).ToNot(HaveOccurred())

		controllerRevisionName := vm.Status.InstancetypeRef.ControllerRevisionRef.Name

		Expect(apiClient.Delete(ctx, vm)).To(MatchError(ContainSubstring("VirtualMachine %v cannot be deleted, disable/remove label "+
			"'kubevirt.io/vm-delete-protection' from VirtualMachine before deleting it", vm.Name)))

		Consistently(func() error {
			controllerRevision := &appsv1.ControllerRevision{}
			return apiClient.Get(ctx, client.ObjectKey{Name: controllerRevisionName, Namespace: vm.Namespace}, controllerRevision)
		}, 30*time.Second, time.Second).Should(Succeed(), "controllerRevision should not be deleted")
	})

	It("should not be able to delete the controller revisions if the VM is protected when the namespace is deleted", func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-ns-%v", rand.String(5)),
			},
		}
		Expect(apiClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() {
			_ = apiClient.Delete(ctx, ns)
		})

		vm = createVMWithDeleteProtection("true", ns.Name)

		startVM(vm.Name, vm.Namespace, vmReadyTimeout)

		Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Name, Namespace: vm.Namespace}, vm)).ToNot(HaveOccurred())
		controllerRevisionName := vm.Status.InstancetypeRef.ControllerRevisionRef.Name

		_ = apiClient.Delete(ctx, ns)

		// Just check the namespace is in Terminating state (has DeletionTimestamp)
		Eventually(func(g Gomega) {
			updatedNs := &corev1.Namespace{}
			err := apiClient.Get(ctx, client.ObjectKey{Name: ns.Name}, updatedNs)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updatedNs.DeletionTimestamp).ToNot(BeNil())
		}, env.ShortTimeout(), time.Second).Should(Succeed())

		Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Name, Namespace: vm.Namespace}, vm)).ToNot(HaveOccurred())

		Consistently(func() error {
			controllerRevision := &appsv1.ControllerRevision{}
			return apiClient.Get(ctx, client.ObjectKey{Name: controllerRevisionName, Namespace: vm.Namespace}, controllerRevision)
		}, 30*time.Second, time.Second).Should(Succeed(), "controllerRevision should not be deleted")
	})

	It("should be able to delete the namespace if the VM delete protection is disabled", func() {
		namespaceName := fmt.Sprintf("test-ns-%v", rand.String(5))
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		Expect(apiClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() {
			_ = apiClient.Delete(ctx, ns)
		})

		vm = createVMWithDeleteProtection("true", ns.Name)

		startVM(vm.Name, vm.Namespace, vmReadyTimeout)

		_ = apiClient.Delete(ctx, ns)

		// Just check the namespace is in Terminating state (has DeletionTimestamp)
		Eventually(func(g Gomega) {
			updatedNs := &corev1.Namespace{}
			err := apiClient.Get(ctx, client.ObjectKey{Name: namespaceName}, updatedNs)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updatedNs.DeletionTimestamp).ToNot(BeNil())
		}, env.ShortTimeout(), time.Second).Should(Succeed())

		// Disable the delete protection
		Eventually(func() error {
			Expect(apiClient.Get(ctx, client.ObjectKey{Name: vm.Name, Namespace: vm.Namespace}, vm)).ToNot(HaveOccurred())
			vm.Labels[deleteProtectionLabel] = "False"
			return apiClient.Update(ctx, vm)
		}, env.ShortTimeout(), time.Second).ToNot(HaveOccurred())

		// Check the namespace is deleted
		Eventually(func() error {
			updatedNs := &corev1.Namespace{}
			return apiClient.Get(ctx, client.ObjectKey{Name: namespaceName}, updatedNs)
		}, env.ShortTimeout(), time.Second).Should(MatchError(errors.IsNotFound, "errors.IsNotFound"))
	})
})

func createVMWithDeleteProtection(protected string, namespace string) *kubevirtv1.VirtualMachine {
	return createVMWithLabels(map[string]string{deleteProtectionLabel: protected}, namespace)
}

func createVMWithLabels(labels map[string]string, namespace string) *kubevirtv1.VirtualMachine {
	vmName := fmt.Sprintf("testvmi-%v", rand.String(10))
	vmi := NewMinimalVMIWithNS(namespace, vmName)
	vmi.Spec.Volumes = []kubevirtv1.Volume{
		{
			Name: "containerdisk",
			VolumeSource: kubevirtv1.VolumeSource{
				ContainerDisk: &kubevirtv1.ContainerDiskSource{
					Image: "quay.io/containerdisks/debian:latest",
				},
			},
		},
	}

	//Get rid of the resources requirements, we want to use preferences and instancetypes
	vmi.Spec.Domain.Resources = kubevirtv1.ResourceRequirements{}

	vm := NewVirtualMachine(vmi)

	vm.Spec.Instancetype = &kubevirtv1.InstancetypeMatcher{
		Name: "u1.small",
	}

	vm.Spec.Preference = &kubevirtv1.PreferenceMatcher{
		Name: "debian",
	}

	vm.Labels = labels
	eventuallyCreateVm(vm)

	return vm
}

func startVM(vmName string, namespace string, vmReadyTimeout time.Duration) {
	vm := &kubevirtv1.VirtualMachine{}
	err := apiClient.Get(ctx, client.ObjectKey{Name: vmName, Namespace: namespace}, vm)
	Expect(err).ToNot(HaveOccurred())

	Eventually(func(g Gomega) {
		vm := &kubevirtv1.VirtualMachine{}
		g.Expect(apiClient.Get(ctx, client.ObjectKey{Name: vmName, Namespace: namespace}, vm)).ToNot(HaveOccurred())

		vm.Spec.RunStrategy = ptr.To(kubevirtv1.RunStrategyAlways)
		g.Expect(apiClient.Update(ctx, vm)).To(Succeed())
	}, env.ShortTimeout(), time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		vm := &kubevirtv1.VirtualMachine{}
		err := apiClient.Get(ctx, client.ObjectKey{Name: vmName, Namespace: namespace}, vm)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(vm.Status.Ready).To(BeTrue())
	}, vmReadyTimeout, time.Second).Should(Succeed())
}
