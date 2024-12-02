package template_bundle

import (
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		testBundle Bundle
		nameSuffix string
	)

	BeforeAll(func() {
		var err error
		testBundle, err = ReadBundle("template-bundle-test.yaml")
		Expect(err).ToNot(HaveOccurred())
		nameSuffix = retrieveSuffix()
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
})

func TestTemplateBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Bundle Suite")
}
