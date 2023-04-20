package tekton_bundle

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	numberOfPipelines  = 2
	numberOfConfigMaps = 2
)

var _ = Describe("Tekton bundle", func() {

	It("should return correct pipeline folder path on okd", func() {
		path := getPipelineBundlePath(true)
		Expect(path).To(Equal("/data/tekton-pipelines/okd/"))
	})

	It("should return correct pipeline folder path on kubernetes", func() {
		path := getPipelineBundlePath(false)
		Expect(path).To(Equal("/data/tekton-pipelines/kubernetes/"))
	})

	It("should load correct files and convert them", func() {
		path, _ := os.Getwd()

		pipelinePath := filepath.Join(path, "test-bundle-files/test-pipelines/")

		pipelineFiles, err := readFolder(pipelinePath)
		Expect(err).ToNot(HaveOccurred())
		files := [][]byte{}
		files = append(files, pipelineFiles...)

		tektonObjs, err := decodeObjectsFromFiles(files)
		Expect(err).ToNot(HaveOccurred(), "it should not throw error")
		Expect(tektonObjs.Pipelines).To(HaveLen(numberOfPipelines), "number of pipelines should equal")
		Expect(tektonObjs.ConfigMaps).To(HaveLen(numberOfConfigMaps), "number of config maps should equal")
	})
})

func TestTektonBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tekton Bundle Suite")
}
