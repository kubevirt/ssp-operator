package validation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

var _ = Describe("Rules", func() {
	Context("Without validation text", func() {
		It("Should return no rules", func() {
			rules, err := ParseRules([]byte(""))

			Expect(err).To(Not(HaveOccurred()))
			Expect(len(rules)).To(Equal(0))
		})
	})

	Context("With validation text", func() {
		It("Should parse a single rule", func() {
			text := `[{
            "name": "core-limits",
            "valid": "spec.domain.cpu.cores",
            "path": "spec.domain.cpu.cores",
            "rule": "integer",
            "message": "cpu cores must be limited",
            "min": 1,
            "max": 8
          }]`
			rules, err := ParseRules([]byte(text))

			Expect(err).To(Not(HaveOccurred()))
			Expect(len(rules)).To(Equal(1))
		})

		It("Should parse multiple rules", func() {
			text := `[{
            "name": "core-limits",
            "valid": "spec.domain.cpu.cores",
            "path": "spec.domain.cpu.cores",
            "rule": "integer",
            "message": "cpu cores must be limited",
            "min": 1,
            "max": 8
	  }, {
            "name": "supported-bus",
            "path": "spec.devices.disks[*].type",
            "rule": "enum",
            "message": "the disk bus type must be one of the supported values",
            "values": ["virtio", "scsi"]
          }]`
			rules, err := ParseRules([]byte(text))

			Expect(err).To(Not(HaveOccurred()))
			Expect(len(rules)).To(Equal(2))
		})
		It("Should apply on a relevant VM", func() {
			vm := NewVMCirros()
			r := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Valid:   "jsonpath::.spec.domain.resources.requests.memory",
				Min:     64 * 1024 * 1024,
			}
			ok, err := r.IsAppliableOn(vm)

			Expect(err).To(Not(HaveOccurred()))
			Expect(ok).To(BeTrue())
		})
		It("Should NOT apply on a NOT relevant VM", func() {
			vm := NewVMCirros()
			r := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Valid:   "jsonpath::.spec.domain.this.path.does.not.exist",
				Min:     64 * 1024 * 1024,
			}
			ok, err := r.IsAppliableOn(vm)

			Expect(err).To(Not(HaveOccurred()))
			Expect(ok).To(BeFalse())
		})

	})
})

func TestValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation Suite")
}
