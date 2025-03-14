package env

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/ssp-operator/internal"
)

var _ = Describe("environments", func() {
	It("should return correct value for OPERATOR_VERSION when variable is set", func() {
		Expect(os.Setenv(OperatorVersionKey, "v0.0.1")).To(Succeed())
		defer func() { Expect(os.Unsetenv(OperatorVersionKey)).To(Succeed()) }()

		res := GetOperatorVersion()
		Expect(res).To(Equal("v0.0.1"), "OPERATOR_VERSION should equal")
	})

	It("should return correct value for OPERATOR_VERSION when variable is not set", func() {
		res := GetOperatorVersion()
		Expect(res).To(Equal(internal.DefaultOperatorVersion), "OPERATOR_VERSION should equal")
	})

	It("should return namespace from POD_NAMESAPCE env", func() {
		const namespaceValue = "test-namespace"
		Expect(os.Setenv(podNamespaceKey, namespaceValue)).To(Succeed())
		defer func() {
			Expect(os.Unsetenv(podNamespaceKey)).To(Succeed())
		}()

		namespace, err := GetOperatorNamespace()
		Expect(err).ToNot(HaveOccurred())

		Expect(namespace).To(Equal(namespaceValue))
	})

	It("should fail if POD_NAMESPACE is not defined", func() {
		_, err := GetOperatorNamespace()
		Expect(err).To(MatchError(ContainSubstring("environment variable")))
	})
})

func TestEnv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Env Suite")
}
