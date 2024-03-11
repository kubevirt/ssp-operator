package tekton_bundle

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	numberOfTasks           = 11
	numberOfServiceAccounts = 9
	numberOfRoleBindings    = 9
	numberOfClusterRoles    = 9
	numberOfPipelines       = 2
	numberOfConfigMaps      = 2
)

var _ = Describe("Tekton bundle", func() {
	It("should return tekton tasks path correctly", func() {
		kubernetesPath := GetTektonTasksBundlePath(false)
		okdPath := GetTektonTasksBundlePath(true)

		Expect(kubernetesPath).To(Equal("/data/tekton-tasks/kubernetes/kubevirt-tekton-tasks-kubernetes.yaml"))
		Expect(okdPath).To(Equal("/data/tekton-tasks/okd/kubevirt-tekton-tasks-okd.yaml"))
	})

	It("should read tekton tasks bundle correctly", func() {
		path, _ := os.Getwd()
		bundlePath := filepath.Join(path, "test-bundle-files/test-tasks/test-tasks.yaml")
		bundle, err := ReadBundle([]string{bundlePath})
		Expect(err).ToNot(HaveOccurred())
		Expect(bundle.Tasks).To(HaveLen(numberOfTasks), "number of tasks should equal")
	})

	It("should load tekton bundle correctly", func() {
		path, _ := os.Getwd()
		var paths []string
		paths = append(paths, filepath.Join(path, "test-bundle-files/test-tasks/test-tasks.yaml"))

		bundle, err := ReadBundle(paths)
		Expect(err).ToNot(HaveOccurred(), "it should not throw error")
		Expect(bundle.Tasks).To(HaveLen(numberOfTasks), "number of tasks should equal")
		Expect(bundle.ServiceAccounts).To(HaveLen(numberOfServiceAccounts), "number of service accounts should equal")
		Expect(bundle.RoleBindings).To(HaveLen(numberOfRoleBindings), "number of role bindings should equal")
		Expect(bundle.ClusterRoles).To(HaveLen(numberOfClusterRoles), "number of cluster roles should equal")
	})
})

func TestTektonBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tekton Bundle Suite")
}
