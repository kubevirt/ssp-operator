package validation

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("Eval", func() {
	Context("With invalid rule set", func() {
		It("Should detect duplicate names", func() {
			rules := []Rule{
				{
					Name: "rule-1",
					Rule: IntegerRule,
					// any legal path is fine
					Path:    "jsonpath::.spec.domain.cpu.cores",
					Message: "testing",
				}, {
					Name: "rule-1",
					Rule: StringRule,
					// any legal path is fine
					Path:    "jsonpath::.spec.domain.cpu.cores",
					Message: "testing",
				},
			}
			vm := kubevirtv1.VirtualMachine{}

			res := NewEvaluator().Evaluate(rules, &vm)
			Expect(res.Succeeded()).To(BeFalse())
			Expect(len(res.Status)).To(Equal(2))
			Expect(res.Status[0].Error).To(BeNil())
			Expect(res.Status[1].Error).To(Equal(ErrDuplicateRuleName))

		})

		It("Should detect missing keys", func() {
			rules := []Rule{
				{
					Name: "rule-1",
					Rule: IntegerRule,
					// any legal path is fine
					Path: "jsonpath::.spec.domain.cpu.cores",
				}, {
					Name:    "rule-2",
					Rule:    StringRule,
					Message: "testing",
				},
			}
			vm := kubevirtv1.VirtualMachine{}

			res := NewEvaluator().Evaluate(rules, &vm)
			Expect(res.Succeeded()).To(BeFalse())
			Expect(len(res.Status)).To(Equal(2))
			Expect(res.Status[0].Error).To(Equal(ErrMissingRequiredKey))
			Expect(res.Status[1].Error).To(Equal(ErrMissingRequiredKey))
		})

		It("Should detect invalid rules", func() {
			rules := []Rule{{
				Name: "rule-1",
				Rule: "foobar",
				// any legal path is fine
				Path:    "jsonpath::.spec.domain.cpu.cores",
				Message: "testing",
			}}
			vm := kubevirtv1.VirtualMachine{}

			res := NewEvaluator().Evaluate(rules, &vm)
			Expect(res.Succeeded()).To(BeFalse())
			Expect(len(res.Status)).To(Equal(1))
			Expect(res.Status[0].Error).To(Equal(ErrUnrecognizedRuleType))
		})
		It("Should detect unappliable rules", func() {
			rules := []Rule{{
				Name: "rule-1",
				Rule: IntegerRule,
				// any legal path is fine
				Path:    "jsonpath::.spec.domain.cpu.cores",
				Message: "testing",
				Valid:   "jsonpath::.spec.domain.some.inexistent.path",
			}}
			vm := kubevirtv1.VirtualMachine{}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, &vm)

			Expect(res.Succeeded()).To(BeTrue())
			Expect(len(res.Status)).To(Equal(1))
			Expect(res.Status[0].Skipped).To(BeTrue())
			Expect(res.Status[0].Satisfied).To(BeFalse())
			Expect(res.Status[0].Error).To(BeNil())
		})

		It("Should not fail, when justWarning is set", func() {
			rules := []Rule{
				{
					Name: "rule-1",
					Rule: IntegerRule,
					Min:  8,
					// any legal path is fine
					Path:        "jsonpath::.spec.domain.cpu.cores",
					Message:     "testing",
					JustWarning: true,
				},
			}
			vm := kubevirtv1.VirtualMachine{}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, &vm)

			Expect(res.Succeeded()).To(BeTrue(), "succeeded")
			Expect(len(res.Status)).To(Equal(1), "status length")
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).To(BeNil(), "error")
		})
	})

	Context("With an initialized VM object", func() {
		var (
			vmCirros *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = NewVMCirros()
		})

		It("should skip uninitialized paths if requested", func() {
			rules := []Rule{{
				Name:    "LimitCores",
				Rule:    IntegerRule,
				Path:    "jsonpath::.spec.domain.cpu.cores",
				Valid:   "jsonpath::.spec.domain.cpu.cores",
				Message: "testing",
				Min:     1,
				Max:     8,
			}}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeTrue())
			Expect(len(res.Status)).To(Equal(1))
			Expect(res.Status[0].Skipped).To(BeTrue())
			Expect(res.Status[0].Satisfied).To(BeFalse())
			Expect(res.Status[0].Error).To(BeNil())

		})

		It("should handle uninitialized paths", func() {
			rules := []Rule{{
				Name:    "LimitCores",
				Rule:    IntegerRule,
				Path:    "jsonpath::.spec.domain.cpu.cores",
				Message: "testing",
				Min:     1,
				Max:     8,
			}}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeFalse())
		})

		It("should handle uninitialized paths intermixed with valid paths", func() {
			rules := []Rule{
				{
					Rule:    IntegerRule,
					Name:    "EnoughMemory",
					Path:    "jsonpath::.spec.domain.resources.requests.memory",
					Message: "Memory size not specified",
					Min:     64 * 1024 * 1024,
					Max:     512 * 1024 * 1024,
				}, {
					Rule:    IntegerRule,
					Name:    "LimitCores",
					Path:    "jsonpath::.spec.domain.cpu.cores",
					Message: "Core amount not within range",
					Min:     1,
					Max:     4,
				}, {
					Rule:    EnumRule,
					Name:    "SupportedChipset",
					Path:    "jsonpath::.spec.domain.machine.type",
					Message: "machine type must be a supported value",
					Values:  []string{"q35"},
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)
			Expect(res.Succeeded()).To(BeFalse())

			causes := res.ToStatusCauses()
			Expect(len(causes)).To(Equal(1))
		})

		It("should not fail, when justWarning is set", func() {
			rules := []Rule{
				{
					Name:        "disk bus",
					Rule:        EnumRule,
					Path:        "jsonpath::.spec.domain.devices.disks[*].disk.bus",
					Message:     "testing",
					Values:      []string{"sata"},
					JustWarning: true,
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeTrue(), "succeeded")
			Expect(len(res.Status)).To(Equal(1), "status length")
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).To(BeNil(), "error")
		})

		It("should fail, when one rule does not have justWarning set", func() {
			rules := []Rule{
				{
					Name:        "disk bus",
					Rule:        EnumRule,
					Path:        "jsonpath::.spec.domain.devices.disks[*].disk.bus",
					Message:     "testing",
					Values:      []string{"sata"},
					JustWarning: true,
				}, {
					Name: "rule-2",
					Rule: IntegerRule,
					Min:  6,
					Max:  8,
					// any legal path is fine
					Path:    "jsonpath::.spec.domain.cpu.cores",
					Message: "enough cores",
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeFalse(), "succeeded")
			Expect(len(res.Status)).To(Equal(2), "status length")
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).To(BeNil(), "error")
			Expect(res.Status[1].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[1].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[1].Error).To(BeNil(), "error")
		})
	})

	Context("With valid rule set", func() {

		var (
			vmCirros *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = NewVMCirros()
		})

		It("Should succeed applying a ruleset", func() {
			rules := []Rule{
				{
					Rule:    IntegerRule,
					Name:    "EnoughMemory",
					Path:    "jsonpath::.spec.domain.resources.requests.memory",
					Message: "Memory size not specified",
					Min:     64 * 1024 * 1024,
					Max:     512 * 1024 * 1024,
				}, {
					Rule:    EnumRule,
					Name:    "SupportedChipset",
					Path:    "jsonpath::.spec.domain.machine.type",
					Message: "machine type must be a supported value",
					Values:  []string{"q35"},
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)
			Expect(res.Succeeded()).To(BeTrue())

			for ix := range res.Status {
				fmt.Fprintf(GinkgoWriter, "%+#v", res.Status[ix])
			}

			Expect(len(res.Status)).To(Equal(len(rules)))
			for ix := range res.Status {
				Expect(res.Status[ix].Satisfied).To(BeTrue())
				Expect(res.Status[ix].Error).To(BeNil())
			}
		})

		It("Should fail applying a ruleset with at least one malformed rule", func() {
			rules := []Rule{
				{
					Rule:    IntegerRule,
					Name:    "EnoughMemory",
					Path:    "jsonpath::.spec.domain.resources.requests.memory",
					Message: "Memory size not specified",
					Min:     64 * 1024 * 1024,
					Max:     512 * 1024 * 1024,
				}, {
					Rule:    "value-set",
					Name:    "SupportedChipset",
					Path:    "jsonpath::.spec.domain.machine.type",
					Message: "machine type must be a supported value",
					Values:  []string{"q35"},
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)
			Expect(res.Succeeded()).To(BeFalse())
		})

		It("Should fail, when rule with justWarning has incorrect path and another rule is correct", func() {
			rules := []Rule{
				{
					Name:        "disk bus",
					Rule:        EnumRule,
					Path:        "jsonpath::.spec.domain.devices.some.non.existing.path",
					Message:     "testing",
					Values:      []string{"sata"},
					JustWarning: true,
				}, {
					Name: "rule-2",
					Rule: IntegerRule,
					Min:  0,
					Max:  8,
					// any legal path is fine
					Path:    "jsonpath::.spec.domain.cpu.cores",
					Message: "enough cores",
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			for ix := range res.Status {
				fmt.Fprintf(GinkgoWriter, "%+#v", res.Status[ix])
			}

			Expect(res.Succeeded()).To(BeFalse(), "succeeded")
			Expect(len(res.Status)).To(Equal(2), "status length")
			//status for second rule which should pass
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).NotTo(BeNil(), "error") // expects invalid JSONPath

			Expect(res.Status[1].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[1].Satisfied).To(BeTrue(), "satisfied")
			Expect(res.Status[1].Error).To(BeNil(), "error")
		})
	})
})

// TODO:
// test with 2+ rules failed
// test to exercise the translation logic
