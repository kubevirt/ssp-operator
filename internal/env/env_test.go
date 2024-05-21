package env

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("environments", func() {
	It("should return correct value for OPERATOR_VERSION when variable is set", func() {
		os.Setenv(OperatorVersionKey, "v0.0.1")
		res := GetOperatorVersion()
		Expect(res).To(Equal("v0.0.1"), "OPERATOR_VERSION should equal")
		os.Unsetenv(OperatorVersionKey)
	})

	It("should return correct value for OPERATOR_VERSION when variable is not set", func() {
		res := GetOperatorVersion()
		Expect(res).To(Equal(defaultOperatorVersion), "OPERATOR_VERSION should equal")
	})
})

func TestEnv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Env Suite")
}
