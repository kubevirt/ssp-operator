package path

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/validation/test-utils"
)

var _ = Describe("Path", func() {
	Context("The JSONPATH filter", func() {
		It("Should detect non-jsonpaths", func() {
			testStrings := []string{
				"string-literal",
				"$.spec.domain.resources.requests.memory",
			}
			for _, s := range testStrings {
				p, err := NewJSONPathFromString(s)
				Expect(p).To(Equal(""))
				Expect(err).To(Equal(ErrInvalidJSONPath))
			}
		})

		It("Should detect non-jsonpaths on creation", func() {
			testStrings := []string{
				"string-literal",
				"$.spec.domain.resources.requests.memory",
			}
			for _, s := range testStrings {
				p, err := New(s)
				Expect(p).To(BeNil())
				Expect(err).To(Equal(ErrInvalidJSONPath))
			}
		})

		It("Should mangle valid JSONPaths", func() {
			expected := "{.spec.template.spec.domain.resources.requests.memory}"
			testStrings := []string{
				"jsonpath::$.spec.domain.resources.requests.memory",
				"jsonpath::.spec.domain.resources.requests.memory",
			}
			for _, s := range testStrings {
				p, err := NewJSONPathFromString(s)
				Expect(p).To(Equal(expected))
				Expect(err).To(BeNil())
			}
		})
	})

	Context("With invalid path", func() {

		var (
			vmCirros *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = test_utils.NewVMCirros()
		})

		It("Should return error", func() {
			p, err := New("jsonpath::.spec.this.path.does.not.exist")
			Expect(p).To(Not(BeNil()))
			Expect(err).To(BeNil())

			_, err = p.Find(vmCirros)
			Expect(err).To(Equal(ErrInvalidJSONPath))
		})

		It("Should detect malformed path", func() {
			p, err := New("jsonpath::random56junk%(*$%&*()")
			Expect(p).To(BeNil())
			Expect(err).To(Not(BeNil()))
		})
	})

	Context("With valid paths", func() {

		var (
			vmCirros *kubevirtv1.VirtualMachine
		)

		BeforeEach(func() {
			vmCirros = test_utils.NewVMCirros()
		})

		It("Should provide some integer results", func() {
			s := "jsonpath::.spec.domain.resources.requests.memory"
			p, err := New(s)
			Expect(p).To(Not(BeNil()))
			Expect(err).To(BeNil())

			results, err := p.Find(vmCirros)
			Expect(err).To(BeNil())
			Expect(results.Len()).To(BeNumerically(">=", 1))

			vals, err := results.AsInt64()
			Expect(err).To(BeNil())
			Expect(len(vals)).To(Equal(1))
			Expect(vals[0]).To(BeNumerically(">", 1024))
		})

		It("Should provide some string results", func() {
			s := "jsonpath::.spec.domain.machine.type"
			p, err := New(s)
			Expect(p).To(Not(BeNil()))
			Expect(err).To(BeNil())

			results, err := p.Find(vmCirros)
			Expect(err).To(BeNil())
			Expect(results.Len()).To(BeNumerically(">=", 1))

			vals, err := results.AsString()
			Expect(err).To(BeNil())
			Expect(len(vals)).To(Equal(1))
			Expect(vals[0]).To(Equal("q35"))
		})

		/* FIXME: the jsonpath package we use can't let us distinguish between:
		   - bogus paths (e.g. paths which don't make sense in a VM object) and
		   - uninitialized paths (e.g. legal paths but with a nil along the chain)
		It("Should handle uninitialized paths", func() {
			s := "jsonpath::.spec.domain.cpu.cores"
			p, err := New(s)
			Expect(p).To(Not(BeNil()))
			Expect(err).To(BeNil())

			err = p.Find(vmCirros)
			Expect(err).To(BeNil())
			Expect(p.Len()).To(BeNumerically("=", 0))
		})
		*/
	})
})
