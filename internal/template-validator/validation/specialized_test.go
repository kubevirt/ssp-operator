package validation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	k6tobjs "kubevirt.io/ssp-operator/internal/template-validator/kubevirtjobs"
)

var _ = Describe("Specialized", func() {
	Context("With valid data", func() {

		var (
			vmCirros *kubevirtv1.VirtualMachine
			vmRef    *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = NewVMCirros()
			vmRef = k6tobjs.NewDefaultVirtualMachine()
		})

		It("Should apply simple integer rules", func() {
			r := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Min:     64 * 1024 * 1024,
				Max:     512 * 1024 * 1024,
			}
			expectRuleApplicationSuccess(&r, vmCirros, vmRef)
		})

		It("Should apply simple string rules", func() {
			r := Rule{
				Rule:      "string",
				Name:      "HasChipset",
				Path:      "jsonpath::.spec.domain.machine.type",
				Message:   "machine type must be specified",
				MinLength: 1,
				MaxLength: 32,
			}
			expectRuleApplicationSuccess(&r, vmCirros, vmRef)
		})

		It("Should apply simple enum rules", func() {
			r := Rule{
				Rule:    "enum",
				Name:    "SupportedChipset",
				Path:    "jsonpath::.spec.domain.machine.type",
				Message: "machine type must be a supported value",
				Values:  []string{"q35", "440fx"},
			}
			expectRuleApplicationSuccess(&r, vmCirros, vmRef)
		})

		It("Should apply enum rule to multiple values", func() {
			r := Rule{
				Rule:    "enum",
				Name:    "SupportedDiskBus",
				Path:    "jsonpath::.spec.domain.devices.disks[*].disk.bus",
				Message: "disk bus must be a supported value",
				Values:  []string{"virtio", "sata"},
			}
			expectRuleApplicationSuccess(&r, vmCirros, vmRef)
		})

		It("Should apply simple regex rules", func() {
			r := Rule{
				Rule:    "regex",
				Name:    "SupportedChipset",
				Path:    "jsonpath::.spec.domain.machine.type",
				Message: "machine type must be a supported value",
				Regex:   "q35|440fx",
			}
			expectRuleApplicationSuccess(&r, vmCirros, vmRef)
		})

		It("Should apply regex rule to multiple values", func() {
			r := Rule{
				Rule:    "regex",
				Name:    "SupportedDiskBus",
				Path:    "jsonpath::.spec.domain.devices.disks[*].disk.bus",
				Message: "disk bus must be a supported value",
				Regex:   "virtio|sata",
			}
			expectRuleApplicationSuccess(&r, vmCirros, vmRef)
		})
	})

	Context("With invalid data", func() {

		var (
			vmCirros *kubevirtv1.VirtualMachine
			vmRef    *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = NewVMCirros()
			vmRef = k6tobjs.NewDefaultVirtualMachine()
		})

		It("Should detect bogus rules", func() {
			r := Rule{
				Rule:    "integer-value",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Valid:   "jsonpath::.spec.domain.this.path.does.not.exist",
				Min:     64 * 1024 * 1024,
				Max:     512 * 1024 * 1024,
			}

			ra, err := r.Specialize(vmCirros, vmRef)
			Expect(err).To(Not(BeNil()))
			Expect(ra).To(BeNil())
		})

		It("Should fail simple integer rules", func() {
			r1 := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Valid:   "jsonpath::.spec.domain.this.path.does.not.exist",
				Min:     512 * 1024 * 1024,
			}
			expectRuleApplicationFailure(&r1, vmCirros, vmRef)

			r2 := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Valid:   "jsonpath::.spec.domain.this.path.does.not.exist",
				Max:     64 * 1024 * 1024,
			}
			expectRuleApplicationFailure(&r2, vmCirros, vmRef)
		})

		It("Should apply simple string rules", func() {
			r1 := Rule{
				Rule:      "string",
				Name:      "HasChipset",
				Path:      "jsonpath::.spec.domain.machine.type",
				Message:   "machine type must be specified",
				MinLength: 64,
			}
			expectRuleApplicationFailure(&r1, vmCirros, vmRef)

			r2 := Rule{
				Rule:      "string",
				Name:      "HasChipset",
				Path:      "jsonpath::.spec.domain.machine.type",
				Message:   "machine type must be specified",
				MaxLength: 1,
			}
			expectRuleApplicationFailure(&r2, vmCirros, vmRef)
		})

		It("Should apply simple enum rules", func() {
			r := Rule{
				Rule:    "enum",
				Name:    "SupportedChipset",
				Path:    "jsonpath::.spec.domain.machine.type",
				Message: "machine type must be a supported value",
				Values:  []string{"foo", "bar"},
			}
			expectRuleApplicationFailure(&r, vmCirros, vmRef)
		})

		It("Should apply enum rule to multiple values", func() {
			r := Rule{
				Rule:    "enum",
				Name:    "SupportedDiskBus",
				Path:    "jsonpath::.spec.domain.devices.disks[*].disk.bus",
				Message: "disk bus must be a supported value",
				Values:  []string{"foo", "bar"},
			}
			expectRuleApplicationFailure(&r, vmCirros, vmRef)
		})

		It("Should error enum rule if values do not exist", func() {
			vmCirros.Spec.Template.Spec.Domain.Devices.Disks = nil
			r := Rule{
				Rule:    "enum",
				Name:    "SupportedDiskBus",
				Path:    "jsonpath::.spec.domain.devices.disks[*].disk.bus",
				Message: "disk bus must be a supported value",
				Values:  []string{"virtio"},
			}
			expectRuleApplicationError(&r, vmCirros, vmRef)
		})

		It("Should apply simple regex rules", func() {
			r := Rule{
				Rule:    "regex",
				Name:    "SupportedChipset",
				Path:    "jsonpath::.spec.domain.machine.type",
				Message: "machine type must be a supported value",
				Regex:   "\\d[a-z]+\\d\\d",
			}
			expectRuleApplicationFailure(&r, vmCirros, vmRef)
		})

		It("Should apply regex rule to multiple values", func() {
			r := Rule{
				Rule:    "regex",
				Name:    "SupportedDiskBus",
				Path:    "jsonpath::.spec.domain.devices.disks[*].disk.bus",
				Message: "disk bus must be a supported value",
				Regex:   "foo|bar",
			}
			expectRuleApplicationFailure(&r, vmCirros, vmRef)
		})

		It("Should error regex rule if values do not exist", func() {
			vmCirros.Spec.Template.Spec.Domain.Devices.Disks = nil
			r := Rule{
				Rule:    "regex",
				Name:    "SupportedDiskBus",
				Path:    "jsonpath::.spec.domain.devices.disks[*].disk.bus",
				Message: "disk bus must be a supported value",
				Regex:   "virtio|sata",
			}
			expectRuleApplicationError(&r, vmCirros, vmRef)
		})

		It("Should post message when value is lower", func() {
			vmCirros.Spec.Template.Spec.Domain.Resources.Requests = k8sv1.ResourceList{
				k8sv1.ResourceMemory: resource.MustParse("1M"),
			}

			r := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Min:     64 * 1024 * 1024,
				Max:     512 * 1024 * 1024,
			}
			ra, err := r.Specialize(vmCirros, vmRef)
			Expect(err).To(BeNil())
			Expect(ra).To(Not(BeNil()))

			ok, err := ra.Apply(vmCirros, vmRef)
			Expect(err).To(BeNil())
			Expect(ok).To(Equal(false))

			result := ra.String()
			Expect(result).To(Equal("value 1000000 is lower than minimum [67108864]"))
		})

		It("Should post message when value is higher", func() {
			vmCirros.Spec.Template.Spec.Domain.Resources.Requests = k8sv1.ResourceList{
				k8sv1.ResourceMemory: resource.MustParse("10G"),
			}

			r := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Min:     64 * 1024 * 1024,
				Max:     512 * 1024 * 1024,
			}
			ra, err := r.Specialize(vmCirros, vmRef)
			Expect(err).To(BeNil())
			Expect(ra).To(Not(BeNil()))

			ok, err := ra.Apply(vmCirros, vmRef)
			Expect(err).To(BeNil())
			Expect(ok).To(Equal(false))

			result := ra.String()
			Expect(result).To(Equal("value 10000000000 is higher than maximum [536870912]"))
		})

		It("Should post message when value is winthin limits", func() {
			vmCirros.Spec.Template.Spec.Domain.Resources.Requests = k8sv1.ResourceList{
				k8sv1.ResourceMemory: resource.MustParse("68M"),
			}

			r := Rule{
				Rule:    "integer",
				Name:    "EnoughMemory",
				Path:    "jsonpath::.spec.domain.resources.requests.memory",
				Message: "Memory size not specified",
				Min:     64 * 1024 * 1024,
				Max:     512 * 1024 * 1024,
			}
			ra, err := r.Specialize(vmCirros, vmRef)
			Expect(err).To(BeNil())
			Expect(ra).To(Not(BeNil()))

			ok, err := ra.Apply(vmCirros, vmRef)
			Expect(err).To(BeNil())
			Expect(ok).To(Equal(true))

			result := ra.String()
			Expect(result).To(Equal("All values [68000000] are in interval [67108864, 536870912]"))
		})
	})

})

func expectRuleApplicationSuccess(r *Rule, vm, ref *kubevirtv1.VirtualMachine) {
	checkRuleApplication(r, vm, ref, true)
}

func expectRuleApplicationFailure(r *Rule, vm, ref *kubevirtv1.VirtualMachine) {
	checkRuleApplication(r, vm, ref, false)
}

func expectRuleApplicationError(r *Rule, vm, ref *kubevirtv1.VirtualMachine) {
	ra, err := r.Specialize(vm, ref)
	Expect(err).To(BeNil())
	Expect(ra).To(Not(BeNil()))

	_, err = ra.Apply(vm, ref)
	Expect(err).To(HaveOccurred())
}

func checkRuleApplication(r *Rule, vm, ref *kubevirtv1.VirtualMachine, expected bool) {
	ra, err := r.Specialize(vm, ref)
	Expect(err).To(BeNil())
	Expect(ra).To(Not(BeNil()))

	ok, err := ra.Apply(vm, ref)
	Expect(err).To(BeNil())
	Expect(ok).To(Equal(expected))
}
