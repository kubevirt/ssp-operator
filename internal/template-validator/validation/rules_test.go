package validation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/ssp-operator/internal/template-validator/validation/path"
	"kubevirt.io/ssp-operator/internal/template-validator/validation/test-utils"
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
            "valid": "jsonpath::spec.domain.cpu.cores",
            "path": "jsonpath::spec.domain.cpu.cores",
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
            "valid": "jsonpath::spec.domain.cpu.cores",
            "path": "jsonpath::spec.domain.cpu.cores",
            "rule": "integer",
            "message": "cpu cores must be limited",
            "min": 1,
            "max": 8
	  }, {
            "name": "supported-bus",
            "path": "jsonpath::spec.devices.disks[*].type",
            "rule": "enum",
            "message": "the disk bus type must be one of the supported values",
            "values": ["virtio", "scsi"]
          }]`
			rules, err := ParseRules([]byte(text))

			Expect(err).To(Not(HaveOccurred()))
			Expect(len(rules)).To(Equal(2))
		})
		It("Should apply on a relevant VM", func() {
			vm := test_utils.NewVMCirros()
			r := Rule{
				Rule:    IntegerRule,
				Name:    "EnoughMemory",
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.resources.requests.memory"),
				Message: "Memory size not specified",
				Valid:   path.NewOrPanic("jsonpath::.spec.domain.resources.requests.memory"),
				Min:     &path.IntOrPath{Int: 64 * 1024 * 1024},
			}
			Expect(r.IsAppliableOn(vm)).To(BeTrue())
		})
		It("Should NOT apply on a NOT relevant VM", func() {
			vm := test_utils.NewVMCirros()
			r := Rule{
				Rule:    IntegerRule,
				Name:    "EnoughMemory",
				Path:    *path.NewOrPanic("jsonpath::.spec.domain.resources.requests.memory"),
				Message: "Memory size not specified",
				Valid:   path.NewOrPanic("jsonpath::.spec.domain.this.path.does.not.exist"),
				Min:     &path.IntOrPath{Int: 64 * 1024 * 1024},
			}
			Expect(r.IsAppliableOn(vm)).To(BeFalse())
		})

	})
})
