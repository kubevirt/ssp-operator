package tests

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/tests/env"
)

const deleteProtectionLabel = "kubevirt.io/vm-delete-protection"

var _ = Describe("VM delete protection", func() {

	var vm *kubevirtv1.VirtualMachine

	BeforeEach(func() {
		waitUntilDeployed()
	})

	AfterEach(func() {
		if vm != nil {
			err := apiClient.Get(ctx, client.ObjectKeyFromObject(vm), vm)
			Expect(err).To(Or(Not(HaveOccurred()), MatchError(errors.IsNotFound, "errors.IsNotFound")))

			if err == nil {
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

	DescribeTable("should not allow to delete a VM if the protection is enabled", func(labelValue string) {
		vm = createVMWithDeleteProtection(labelValue)

		Expect(apiClient.Delete(ctx, vm)).To(
			MatchError(ContainSubstring("VirtualMachine %v cannot be deleted, remove delete protection", vm.Name)))
	},
		Entry("using True as value", "True"),
		Entry("using true as value", "true"),
	)

	DescribeTable("should be able to delete a VM if the protection is disabled", func(labelValue string) {
		vm = createVMWithDeleteProtection(labelValue)

		Expect(apiClient.Delete(ctx, vm)).To(Succeed())
	},
		Entry("using False as value", "False"),
		Entry("using false as value", "false"),
		Entry("using value different from false or False", "niceValue"),
	)

	It("should be able to delete a VM if the VM does not have any label", func() {
		vm = createVMWithLabels(nil)

		Expect(apiClient.Delete(ctx, vm)).To(Succeed())
	})
})

func createVMWithDeleteProtection(protected string) *kubevirtv1.VirtualMachine {
	return createVMWithLabels(map[string]string{deleteProtectionLabel: protected})
}

func createVMWithLabels(labels map[string]string) *kubevirtv1.VirtualMachine {
	vmName := fmt.Sprintf("testvmi-%v", rand.String(10))
	vmi := NewMinimalVMIWithNS(strategy.GetNamespace(), vmName)
	vm := NewVirtualMachine(vmi)

	vm.Labels = labels
	eventuallyCreateVm(vm)

	return vm
}
