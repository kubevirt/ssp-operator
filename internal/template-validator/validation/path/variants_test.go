package path

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Variant tests", func() {
	Context("IntOrPath", func() {
		It("parses int from json", func() {
			jsonData := []byte("{\"data\": 1234}")
			data := &struct {
				Data *IntOrPath `json:"data"`
			}{}

			Expect(json.Unmarshal(jsonData, data)).To(Succeed())
			Expect(data.Data.IsInt()).To(BeTrue(), "Expected int value")
			Expect(data.Data.Int).To(Equal(int64(1234)))
		})

		It("parses path form json", func() {
			jsonData := []byte("{\"data\": \"jsonpath::.test.path\"}")
			data := &struct {
				Data *IntOrPath `json:"data"`
			}{}

			Expect(json.Unmarshal(jsonData, data)).To(Succeed())
			Expect(data.Data.IsInt()).To(BeFalse(), "Expected path value")
			Expect(data.Data.Path.Expr()).To(Equal(".test.path"))
		})
	})

	Context("StringOrPath", func() {
		It("parses string from json", func() {
			jsonData := []byte("{\"data\": \"test string\"}")
			data := &struct {
				Data *StringOrPath `json:"data"`
			}{}

			Expect(json.Unmarshal(jsonData, data)).To(Succeed())
			Expect(data.Data.IsString()).To(BeTrue(), "Expected string value")
			Expect(data.Data.Str).To(Equal("test string"))
		})

		It("parses path form json", func() {
			jsonData := []byte("{\"data\": \"jsonpath::.test.path\"}")
			data := &struct {
				Data *StringOrPath `json:"data"`
			}{}

			Expect(json.Unmarshal(jsonData, data)).To(Succeed())
			Expect(data.Data.IsString()).To(BeFalse(), "Expected path value")
			Expect(data.Data.Path.Expr()).To(Equal(".test.path"))
		})
	})
})
