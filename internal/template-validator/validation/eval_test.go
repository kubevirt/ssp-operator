package validation

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/validation/path"
	test_utils "kubevirt.io/ssp-operator/internal/template-validator/validation/test-utils"
)

var _ = Describe("Eval", func() {
	Context("With invalid rule set", func() {
		It("Should detect duplicate names", func() {
			rules := []Rule{
				{
					Name: "rule-1",
					Rule: IntegerRule,
					// any legal path is fine
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
					Message: "testing",
				}, {
					Name: "rule-1",
					Rule: StringRule,
					// any legal path is fine
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
					Message: "testing",
				},
			}
			vm := kubevirtv1.VirtualMachine{}

			res := NewEvaluator().Evaluate(rules, &vm)
			Expect(res.Succeeded()).To(BeFalse())
			Expect(res.Status).To(HaveLen(2))
			Expect(res.Status[0].Error).ToNot(HaveOccurred())
			Expect(res.Status[1].Error).To(Equal(ErrDuplicateRuleName))

		})

		It("Should detect missing keys", func() {
			rules := []Rule{{
				Name: "rule-1",
				Rule: IntegerRule,
				// any legal path is fine
				Path: *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
			}}
			vm := kubevirtv1.VirtualMachine{}

			res := NewEvaluator().Evaluate(rules, &vm)
			Expect(res.Succeeded()).To(BeFalse())
			Expect(res.Status).To(HaveLen(1))
			Expect(res.Status[0].Error).To(Equal(ErrMissingRequiredKey))
		})

		It("Should detect invalid rules", func() {
			rules := []Rule{{
				Name: "rule-1",
				Rule: "foobar",
				// any legal path is fine
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
				Message: "testing",
			}}
			vm := kubevirtv1.VirtualMachine{}

			res := NewEvaluator().Evaluate(rules, &vm)
			Expect(res.Succeeded()).To(BeFalse())
			Expect(res.Status).To(HaveLen(1))
			Expect(res.Status[0].Error).To(Equal(ErrUnrecognizedRuleType))
		})
		It("Should detect unappliable rules", func() {
			rules := []Rule{{
				Name: "rule-1",
				Rule: IntegerRule,
				// any legal path is fine
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
				Message: "testing",
				Valid:   path.NewOrPanic("jsonpath::.spec.domain.some.inexistent.path"),
			}}
			vm := kubevirtv1.VirtualMachine{}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, &vm)

			Expect(res.Succeeded()).To(BeTrue())
			Expect(res.Status).To(HaveLen(1))
			Expect(res.Status[0].Skipped).To(BeTrue())
			Expect(res.Status[0].Satisfied).To(BeFalse())
			Expect(res.Status[0].Error).ToNot(HaveOccurred())
		})

		It("Should not fail, when justWarning is set", func() {
			rules := []Rule{
				{
					Name: "rule-1",
					Rule: IntegerRule,
					Min:  &path.IntOrPath{Int: 8},
					// any legal path is fine
					Path:        *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
					Message:     "testing",
					JustWarning: true,
				},
			}
			vm := kubevirtv1.VirtualMachine{}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, &vm)

			Expect(res.Succeeded()).To(BeTrue(), "succeeded")
			Expect(res.Status).To(HaveLen(1), "status length")
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).ToNot(HaveOccurred(), "error")
		})
	})

	Context("With an initialized VM object", func() {
		var (
			vmCirros *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = test_utils.NewVMCirros()
		})

		It("should skip uninitialized paths if requested", func() {
			rules := []Rule{{
				Name:    "LimitCores",
				Rule:    IntegerRule,
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
				Valid:   path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
				Message: "testing",
				Min:     &path.IntOrPath{Int: 1},
				Max:     &path.IntOrPath{Int: 8},
			}}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeTrue())
			Expect(res.Status).To(HaveLen(1))
			Expect(res.Status[0].Skipped).To(BeTrue())
			Expect(res.Status[0].Satisfied).To(BeFalse())
			Expect(res.Status[0].Error).ToNot(HaveOccurred())

		})

		It("should handle uninitialized paths", func() {
			rules := []Rule{{
				Name:    "LimitCores",
				Rule:    IntegerRule,
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
				Message: "testing",
				Min:     &path.IntOrPath{Int: 1},
				Max:     &path.IntOrPath{Int: 8},
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
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.resources.requests.memory"),
					Message: "Memory size not specified",
					Min:     &path.IntOrPath{Int: 64 * 1024 * 1024},
					Max:     &path.IntOrPath{Int: 512 * 1024 * 1024},
				}, {
					Rule:    IntegerRule,
					Name:    "LimitCores",
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
					Message: "Core amount not within range",
					Min:     &path.IntOrPath{Int: 1},
					Max:     &path.IntOrPath{Int: 4},
				}, {
					Rule:    EnumRule,
					Name:    "SupportedChipset",
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.machine.type"),
					Message: "machine type must be a supported value",
					Values:  []path.StringOrPath{{Str: "q35"}},
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)
			Expect(res.Succeeded()).To(BeFalse())

			causes := res.ToStatusCauses()
			Expect(causes).To(HaveLen(1))
		})

		It("should not fail, when justWarning is set", func() {
			rules := []Rule{
				{
					Name:        "disk bus",
					Rule:        EnumRule,
					Path:        *path.NewOrPanic("jsonpath::.spec.domain.devices.disks[*].disk.bus"),
					Message:     "testing",
					Values:      []path.StringOrPath{{Str: "sata"}},
					JustWarning: true,
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeTrue(), "succeeded")
			Expect(res.Status).To(HaveLen(1), "status length")
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).ToNot(HaveOccurred(), "error")
		})

		It("should fail, when one rule does not have justWarning set", func() {
			rules := []Rule{
				{
					Name:        "disk bus",
					Rule:        EnumRule,
					Path:        *path.NewOrPanic("jsonpath::.spec.domain.devices.disks[*].disk.bus"),
					Message:     "testing",
					Values:      []path.StringOrPath{{Str: "sata"}},
					JustWarning: true,
				}, {
					Name: "rule-2",
					Rule: IntegerRule,
					Min:  &path.IntOrPath{Int: 6},
					Max:  &path.IntOrPath{Int: 8},
					// any legal path is fine
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
					Message: "enough cores",
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			Expect(res.Succeeded()).To(BeFalse(), "succeeded")
			Expect(res.Status).To(HaveLen(2), "status length")
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).ToNot(HaveOccurred(), "error")
			Expect(res.Status[1].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[1].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[1].Error).ToNot(HaveOccurred(), "error")
		})
	})

	Context("With valid rule set", func() {

		var (
			vmCirros *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = test_utils.NewVMCirros()
		})

		It("Should succeed applying a ruleset", func() {
			rules := []Rule{
				{
					Rule:    IntegerRule,
					Name:    "EnoughMemory",
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.resources.requests.memory"),
					Message: "Memory size not specified",
					Min:     &path.IntOrPath{Int: 64 * 1024 * 1024},
					Max:     &path.IntOrPath{Int: 512 * 1024 * 1024},
				}, {
					Rule:    EnumRule,
					Name:    "SupportedChipset",
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.machine.type"),
					Message: "machine type must be a supported value",
					Values:  []path.StringOrPath{{Str: "q35"}},
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)
			Expect(res.Succeeded()).To(BeTrue())

			for ix := range res.Status {
				_, err := fmt.Fprintf(GinkgoWriter, "%+#v", res.Status[ix])
				Expect(err).ToNot(HaveOccurred())
			}

			Expect(res.Status).To(HaveLen(len(rules)))
			for ix := range res.Status {
				Expect(res.Status[ix].Satisfied).To(BeTrue())
				Expect(res.Status[ix].Error).ToNot(HaveOccurred())
			}
		})

		It("Should fail applying a ruleset with at least one malformed rule", func() {
			rules := []Rule{
				{
					Rule:    IntegerRule,
					Name:    "EnoughMemory",
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.resources.requests.memory"),
					Message: "Memory size not specified",
					Min:     &path.IntOrPath{Int: 64 * 1024 * 1024},
					Max:     &path.IntOrPath{Int: 512 * 1024 * 1024},
				}, {
					Rule:    "value-set",
					Name:    "SupportedChipset",
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.machine.type"),
					Message: "machine type must be a supported value",
					Values:  []path.StringOrPath{{Str: "q35"}},
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
					Path:        *path.NewOrPanic("jsonpath::.spec.domain.devices.some.non.existing.path"),
					Message:     "testing",
					Values:      []path.StringOrPath{{Str: "sata"}},
					JustWarning: true,
				}, {
					Name: "rule-2",
					Rule: IntegerRule,
					Min:  &path.IntOrPath{Int: 0},
					Max:  &path.IntOrPath{Int: 8},
					// any legal path is fine
					Path:    *path.NewOrPanic("jsonpath::.spec.domain.cpu.cores"),
					Message: "enough cores",
				},
			}

			ev := Evaluator{Sink: GinkgoWriter}
			res := ev.Evaluate(rules, vmCirros)

			for ix := range res.Status {
				_, err := fmt.Fprintf(GinkgoWriter, "%+#v", res.Status[ix])
				Expect(err).ToNot(HaveOccurred())
			}

			Expect(res.Succeeded()).To(BeFalse(), "succeeded")
			Expect(res.Status).To(HaveLen(2), "status length")
			//status for second rule which should pass
			Expect(res.Status[0].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[0].Satisfied).To(BeFalse(), "satisfied")
			Expect(res.Status[0].Error).To(HaveOccurred(), "error") // expects invalid JSONPath

			Expect(res.Status[1].Skipped).To(BeFalse(), "skipped")
			Expect(res.Status[1].Satisfied).To(BeTrue(), "satisfied")
			Expect(res.Status[1].Error).ToNot(HaveOccurred(), "error")
		})
	})
})

// TODO:
// test with 2+ rules failed
// test to exercise the translation logic
