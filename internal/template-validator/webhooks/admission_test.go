package validating

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k6tv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/labels"
	"kubevirt.io/ssp-operator/internal/template-validator/validation"
	"kubevirt.io/ssp-operator/internal/template-validator/validation/path"
)

var _ = Describe("Admission", func() {
	Context("Without some data", func() {
		It("should admit without template", func() {
			newVM := k6tv1.VirtualMachine{}
			var rules []validation.Rule

			causes := ValidateVm(rules, &newVM)

			Expect(len(causes)).To(Equal(0))
		})
	})

	Context("Default values", func() {
		var vm k6tv1.VirtualMachine

		BeforeEach(func() {
			vm = k6tv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: k6tv1.VirtualMachineSpec{
					Template: &k6tv1.VirtualMachineInstanceTemplateSpec{
						Spec: k6tv1.VirtualMachineInstanceSpec{
							Domain: k6tv1.DomainSpec{
								CPU: &k6tv1.CPU{},
							},
						},
					},
				},
			}
		})

		It("should set default sockets", func() {
			rules := []validation.Rule{{
				Name:    "test-sockets-default",
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.sockets"),
				Rule:    "integer",
				Message: "invalid number of sockets",
				Min:     &path.IntOrPath{Int: 1},
			}}

			causes := ValidateVm(rules, &vm)
			Expect(len(causes)).To(Equal(0))
		})

		It("should set default cores", func() {
			rules := []validation.Rule{{
				Name:    "test-cores-default",
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
				Rule:    "integer",
				Message: "invalid number of cores",
				Min:     &path.IntOrPath{Int: 1},
			}}

			causes := ValidateVm(rules, &vm)
			Expect(len(causes)).To(Equal(0))
		})

		It("should set default threads", func() {
			rules := []validation.Rule{{
				Name:    "test-threads-default",
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.threads"),
				Rule:    "integer",
				Message: "invalid number of threads",
				Min:     &path.IntOrPath{Int: 1},
			}}

			causes := ValidateVm(rules, &vm)
			Expect(len(causes)).To(Equal(0))
		})
	})

	Context("vm validation annotation", func() {
		It("validation annotation on a VM should be used if it exists", func() {
			ruleName := "vmRule"
			rules, err := json.Marshal([]validation.Rule{{
				Name: ruleName,
				Path: *path.NewOrPanic("jsonpath::.test.path"),
			}})
			Expect(err).ToNot(HaveOccurred())

			vm := &k6tv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
					Annotations: map[string]string{
						labels.VmValidationAnnotationKey: string(rules),
					},
				},
				Spec: k6tv1.VirtualMachineSpec{},
			}

			vmRules, err := getValidationRulesForVM(vm, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(vmRules)).To(Equal(1))
			Expect(vmRules[0].Name).To(Equal(ruleName))
			Expect(vmRules[0].Path.Expr()).To(Equal(".test.path"))
		})
	})
})

func TestValidating(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validating Suite")
}
