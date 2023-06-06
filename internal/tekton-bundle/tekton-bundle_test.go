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
	It("should return correct pipeline folder path ", func() {
		path := getPipelineBundlePath()
		Expect(path).To(Equal("/data/tekton-pipelines/"))
	})

	It("should return correct task path on okd", func() {
		path := getTasksBundlePath(true)
		Expect(path).To(Equal("/data/tekton-tasks/okd/kubevirt-tekton-tasks-okd.yaml"))
	})

	It("should return correct task path on kubernetes", func() {
		path := getTasksBundlePath(false)
		Expect(path).To(Equal("/data/tekton-tasks/kubernetes/kubevirt-tekton-tasks-kubernetes.yaml"))
	})

	It("should load correct files and convert them", func() {
		path, _ := os.Getwd()

		taskPath := filepath.Join(path, "test-bundle-files/test-tasks/test-tasks.yaml")
		pipelinePath := filepath.Join(path, "test-bundle-files/test-pipelines/")

		taskFile, err := os.ReadFile(taskPath)
		Expect(err).ToNot(HaveOccurred())
		pipelineFiles, err := readFolder(pipelinePath)
		Expect(err).ToNot(HaveOccurred())
		files := [][]byte{}
		files = append(files, taskFile)
		files = append(files, pipelineFiles...)

		tektonObjs, err := decodeObjectsFromFiles(files)
		Expect(err).ToNot(HaveOccurred(), "it should not throw error")
		Expect(tektonObjs.Tasks).To(HaveLen(numberOfTasks), "number of tasks should equal")
		Expect(tektonObjs.ServiceAccounts).To(HaveLen(numberOfServiceAccounts), "number of service accounts should equal")
		Expect(tektonObjs.RoleBindings).To(HaveLen(numberOfRoleBindings), "number of role bindings should equal")
		Expect(tektonObjs.ClusterRoles).To(HaveLen(numberOfClusterRoles), "number of cluster roles should equal")
		Expect(tektonObjs.Pipelines).To(HaveLen(numberOfPipelines), "number of pipelines should equal")
		Expect(tektonObjs.ConfigMaps).To(HaveLen(numberOfConfigMaps), "number of config maps should equal")
	})
})

func TestTektonBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tekton Bundle Suite")
}
