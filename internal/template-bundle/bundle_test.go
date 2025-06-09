package template_bundle

import (
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func retrieveSuffix() string {
	switch runtime.GOARCH {
	case "s390x":
		return "-s390x"
	default:
		return ""
	}
}

var _ = Describe("Template bundle", func() {
	It("ReadTemplates() should correctly read templates", func() {
		testTemplates, err := ReadTemplates("template-bundle-test.yaml")
		Expect(err).ToNot(HaveOccurred())

		nameSuffix := retrieveSuffix()

		Expect(testTemplates).To(HaveLen(4))
		{
			templ := testTemplates[0]
			Expect(templ.Name).To(Equal("centos-stream8-server-medium" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/centos-stream8"))
			Expect(templ.Objects).To(HaveLen(1))
		}
		{
			templ := testTemplates[1]
			Expect(templ.Name).To(Equal("centos-stream8-desktop-large" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/centos-stream8"))
			Expect(templ.Objects).To(HaveLen(1))
		}
		{
			templ := testTemplates[2]
			Expect(templ.Name).To(Equal("windows10-desktop-medium" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/win10"))
			Expect(templ.Objects).To(HaveLen(1))
		}
		{
			templ := testTemplates[3]
			Expect(templ.Name).To(Equal("rhel8-saphana-tiny" + nameSuffix))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/rhel8.4"))
			Expect(templ.Objects).To(HaveLen(1))
		}
	})

	Context("CollectDataSourceNames", func() {
		It("should collect DataSource names", func() {
			// The template object is not strictly a VirtualMachine, because it can contain
			// string variables in fields that don't have string type. But for this test code,
			// we don't use any such variables.
			testVmTemplate := &kubevirtv1.VirtualMachine{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualMachine",
					APIVersion: kubevirtv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: kubevirtv1.VirtualMachineSpec{
					DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{{
						Spec: cdiv1beta1.DataVolumeSpec{
							SourceRef: &cdiv1beta1.DataVolumeSourceRef{},
						},
					}},
				},
			}

			testTemplates := []templatev1.Template{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "centos-stream8-server-medium",
				},
				Objects: []k8sruntime.RawExtension{{
					Object: testVmTemplate,
				}},
				Parameters: []templatev1.Parameter{{
					Name:  "DATA_SOURCE_NAME",
					Value: "centos-stream8",
				}, {
					Name:  "DATA_SOURCE_NAMESPACE",
					Value: "kubevirt-os-images",
				}},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "windows10-desktop-medium",
				},
				Objects: []k8sruntime.RawExtension{{
					Object: testVmTemplate,
				}},
				Parameters: []templatev1.Parameter{{
					Name:  "DATA_SOURCE_NAME",
					Value: "win10",
				}, {
					Name:  "DATA_SOURCE_NAMESPACE",
					Value: "kubevirt-os-images",
				}},
			}}

			for i := range testTemplates {
				for j := range testTemplates[i].Objects {
					object := &testTemplates[i].Objects[j]
					var err error
					object.Raw, err = json.Marshal(object.Object)
					Expect(err).ToNot(HaveOccurred())
				}
			}

			dataSourceNames, err := CollectDataSourceNames(testTemplates)
			Expect(err).ToNot(HaveOccurred())

			Expect(dataSourceNames).To(ConsistOf("centos-stream8", "win10"))
		})
	})
})

func TestTemplateBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Bundle Suite")
}
