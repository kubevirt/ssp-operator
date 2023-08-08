package vm_console_proxy_bundle

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VM Console Proxy Bundle", func() {
	It("should correctly return bundle path", func() {
		path := GetBundlePath()
		Expect(path).To(Equal("data/vm-console-proxy-bundle/vm-console-proxy.yaml"))
	})

	It("should correctly read bundle test yaml file", func() {
		bundle, err := ReadBundle("bundle-test.yaml")
		Expect(err).ToNot(HaveOccurred())
		Expect(bundle.ServiceAccount).ToNot(BeNil(), "service account should not be nil")
		Expect(bundle.ClusterRole).ToNot(BeNil(), "cluster role should not be nil")
		Expect(bundle.ClusterRoleBinding).ToNot(BeNil(), "cluster role binding should not be nil")
		Expect(bundle.RoleBinding).ToNot(BeNil(), "role binding should not be nil")
		Expect(bundle.Service).ToNot(BeNil(), "service should not be nil")
		Expect(bundle.Deployment).ToNot(BeNil(), "deployment should not be nil")
		Expect(bundle.ConfigMap).ToNot(BeNil(), "config map should not be nil")
		Expect(bundle.ApiService).ToNot(BeNil(), "api service should not be nil")
	})

	It("should fail if bundle does not contain needed resources", func() {
		_, err := ReadBundle("bundle-test-incorrect.yaml")
		Expect(err).To(MatchError(ContainSubstring("bundle is missing these objects")))
	})

	It("should fail if bundle contains duplicates", func() {
		_, err := ReadBundle("bundle-test-duplicate.yaml")
		Expect(err).To(MatchError(ContainSubstring("duplicate Kind found in vm-console-proxy bundle")))
	})
})

func TestVmConsoleProxyBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VM Console Proxy Bundle Suite")
}
