package template_bundle

import (
	"github.com/onsi/ginkgo/extensions/table"
	osconfv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/ssp-operator/internal/common"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template bundle", func() {
	var (
		testBundle Bundle
	)

	readBundle := func(topology osconfv1.TopologyMode) {
		var err error
		testBundle, err = ReadBundle("template-bundle-test.yaml", topology, common.Scheme)
		Expect(err).ToNot(HaveOccurred())
	}

	table.DescribeTable("should correctly read templates", func(topology osconfv1.TopologyMode) {
		readBundle(topology)
		templates := testBundle.Templates
		Expect(templates).To(HaveLen(4))

		evictionStrategyExpectFun := ExpectEvictionStrategyLiveMigrate
		if topology == osconfv1.SingleReplicaTopologyMode {
			evictionStrategyExpectFun = ExpectEvictionStrategyNotPresent
		}
		{
			templ := templates[0]
			Expect(templ.Name).To(Equal("centos-stream8-server-medium"))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/centos-stream8"))
			Expect(templ.Objects).To(HaveLen(1))
			evictionStrategyExpectFun(&templ.Objects[0])
		}
		{
			templ := templates[1]
			Expect(templ.Name).To(Equal("centos-stream8-desktop-large"))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/centos-stream8"))
			Expect(templ.Objects).To(HaveLen(1))
			evictionStrategyExpectFun(&templ.Objects[0])
		}
		{
			templ := templates[2]
			Expect(templ.Name).To(Equal("windows10-desktop-medium"))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/win10"))
			Expect(templ.Objects).To(HaveLen(1))
			evictionStrategyExpectFun(&templ.Objects[0])
		}
		{
			templ := templates[3]
			Expect(templ.Name).To(Equal("rhel8-saphana-tiny"))
			Expect(templ.Annotations).To(HaveKey("name.os.template.kubevirt.io/rhel8.4"))
			Expect(templ.Objects).To(HaveLen(1))
			evictionStrategyExpectFun(&templ.Objects[0])
		}
	},
		table.Entry("in HighlyAvailableTopologyMode infrastructure", osconfv1.HighlyAvailableTopologyMode),
		table.Entry("in SingleReplicaTopologyMode infrastructure", osconfv1.SingleReplicaTopologyMode),
	)

	It("should create DataSources", func() {
		readBundle(osconfv1.HighlyAvailableTopologyMode)
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

func ExpectEvictionStrategyNotPresent(obj *runtime.RawExtension) {
	vmUnstructured := decodeToVMUnstructuredAndExpectSuccess(obj)
	_, found, err := unstructured.NestedFieldNoCopy(vmUnstructured.Object, "spec", "template", "spec", "evictionStrategy")
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeFalse())
}

func ExpectEvictionStrategyLiveMigrate(obj *runtime.RawExtension) {
	vmUnstructured := decodeToVMUnstructuredAndExpectSuccess(obj)
	val, found, err := unstructured.NestedFieldNoCopy(vmUnstructured.Object, "spec", "template", "spec", "evictionStrategy")
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())
	Expect(val).To(Equal(string(kubevirtv1.EvictionStrategyLiveMigrate)))
}

func decodeToVMUnstructuredAndExpectSuccess(obj *runtime.RawExtension) *unstructured.Unstructured {
	vmUnstructured, isVm, err := decodeToVMUnstructured(obj)
	Expect(err).ToNot(HaveOccurred())
	Expect(isVm).To(BeTrue())
	return vmUnstructured
}

func TestTemplateBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Bundle Suite")
}
