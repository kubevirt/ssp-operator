package template_bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
)

func retrieveSuffix() string {
	switch runtime.GOARCH {
	case "s390x":
		return "-s390x"
	default:
		return ""
	}
}

var _ = Describe("Template bundle", Ordered, func() {
	var (
		testBundle          Bundle
		nameSuffix          string
		tmpDir              string
		archIndependentFile string
		archDependentFile   string
	)

	BeforeAll(func() {
		var err error
		testBundle, err = ReadBundle("template-bundle-test.yaml")
		Expect(err).ToNot(HaveOccurred())
		nameSuffix = retrieveSuffix()
	})

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		archIndependentFile = filepath.Join(tmpDir, fmt.Sprintf("common-templates-%s.yaml", common_templates.Version))
		archDependentFile = filepath.Join(tmpDir, fmt.Sprintf("common-templates-%s-%s.yaml", runtime.GOARCH, common_templates.Version))
	})

	It("should correctly read templates", func() {
		templates := testBundle.Templates
		Expect(templates).To(HaveLen(4))
		{
			templ := templates[0]
			Expect(templ.Name).To(Equal("centos-stream8-server-medium" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/centos-stream8"))
			Expect(templ.Objects).To(HaveLen(1))
		}
		{
			templ := templates[1]
			Expect(templ.Name).To(Equal("centos-stream8-desktop-large" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/centos-stream8"))
			Expect(templ.Objects).To(HaveLen(1))
		}
		{
			templ := templates[2]
			Expect(templ.Name).To(Equal("windows10-desktop-medium" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/win10"))
			Expect(templ.Objects).To(HaveLen(1))
		}
		{
			templ := templates[3]
			Expect(templ.Name).To(Equal("rhel8-saphana-tiny" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/rhel8.4"))
			Expect(templ.Objects).To(HaveLen(1))
		}
	})

	It("should create DataSources", func() {
		dataSources := testBundle.DataSources
		Expect(dataSources).To(HaveLen(2))

		ds1 := dataSources[0]
		Expect(ds1.Name).To(Equal("centos-stream8"))
		Expect(ds1.Namespace).To(Equal("kubevirt-os-images"))
		Expect(ds1.Spec.Source.PVC.Name).To(Equal("centos-stream8"))
		Expect(ds1.Spec.Source.PVC.Namespace).To(Equal("kubevirt-os-images"))

		ds2 := dataSources[1]
		Expect(ds2.Name).To(Equal("win10"))
		Expect(ds2.Namespace).To(Equal("kubevirt-os-images"))
		Expect(ds2.Spec.Source.PVC.Name).To(Equal("win10"))
		Expect(ds2.Spec.Source.PVC.Namespace).To(Equal("kubevirt-os-images"))
	})

	It("should throw an error retrieving the bundle file", func() {
		_, err := RetrieveCommonTemplatesBundleFile(tmpDir)
		Expect(err).To(MatchError(ContainSubstring("failed to find common-templates bundles, none of the files were found")))
	})

	It("should retrieve the bundle arch independent file", func() {
		err := os.WriteFile(archIndependentFile, []byte(""), 0644)
		Expect(err).ToNot(HaveOccurred())
		commonTemplatesBundleFile, err := RetrieveCommonTemplatesBundleFile(tmpDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(commonTemplatesBundleFile).To(Equal(archIndependentFile))
	})

	It("should retrieve the bundle arch dependent file", func() {
		err := os.WriteFile(archDependentFile, []byte(""), 0644)
		Expect(err).ToNot(HaveOccurred())
		commonTemplatesBundleFile, err := RetrieveCommonTemplatesBundleFile(tmpDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(commonTemplatesBundleFile).To(Equal(archDependentFile))
	})

	It("should retrieve the bundle arch dependent file when the generic one also exists", func() {
		err := os.WriteFile(archIndependentFile, []byte(""), 0644)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(archDependentFile, []byte(""), 0644)
		Expect(err).ToNot(HaveOccurred())
		commonTemplatesBundleFile, err := RetrieveCommonTemplatesBundleFile(tmpDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(commonTemplatesBundleFile).To(Equal(archDependentFile))
	})
})

func TestTemplateBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Bundle Suite")
}
