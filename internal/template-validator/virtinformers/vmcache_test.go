package virtinformers

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubevirt.io/ssp-operator/internal/template-validator/labels"
)

const (
	testVmNamespace       = "test-vm-ns"
	testTemplateNamespace = "test-template-ns"
)

var _ = Describe("VM Cache", func() {
	var (
		vmCache    VmCache
		filterFunc Predicate
	)
	BeforeEach(func() {
		filterFunc = func(_ metav1.Object) bool {
			return true
		}

		vmCache = NewVmCache(func(vm metav1.Object) bool {
			return filterFunc(vm)
		})
	})

	Context("store", func() {
		It("should add value", func() {
			vm := newObject("test-vm", "test-template")
			Expect(vmCache.Add(vm)).To(Succeed())

			cacheObj, exists, err := vmCache.Get(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "Expected to find cache value")

			cacheVal, ok := cacheObj.(VmCacheValue)
			Expect(ok).To(BeTrue(), "Expected cacheObj to be VmCacheValue")

			templateKeys := labels.GetTemplateKeys(vm)
			Expect(cacheVal.Template).To(Equal(templateKeys.Get().String()))
		})

		It("should not add value if fails filter", func() {
			filterFunc = func(_ metav1.Object) bool {
				return false
			}

			vm := newObject("test-vm", "test-template")
			Expect(vmCache.Add(vm)).To(Succeed())

			_, exists, err := vmCache.Get(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "Expected vm to not exist")
		})

		It("should update existing value", func() {
			vm := newObject("test-vm", "test-template")
			Expect(vmCache.Add(vm)).To(Succeed())

			vm.GetLabels()[labels.AnnotationTemplateNameKey] = "updated-template"
			Expect(vmCache.Update(vm)).To(Succeed())

			cacheObj, exists, err := vmCache.Get(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "Expected to find cache value")

			cacheVal, ok := cacheObj.(VmCacheValue)
			Expect(ok).To(BeTrue(), "Expected cacheObj to be VmCacheValue")

			templateKeys := labels.GetTemplateKeys(vm)
			Expect(cacheVal.Template).To(Equal(templateKeys.Get().String()))
		})

		It("should remove value on update, if filter fails", func() {
			vm := newObject("test-vm", "test-template")
			Expect(vmCache.Add(vm)).To(Succeed())

			filterFunc = func(_ metav1.Object) bool {
				return false
			}

			vm.GetLabels()[labels.AnnotationTemplateNameKey] = "updated-template"
			Expect(vmCache.Update(vm)).To(Succeed())

			_, exists, err := vmCache.Get(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "Expected vm to not exist")
		})

		It("should remove value on delete", func() {
			vm := newObject("test-vm", "test-template")
			Expect(vmCache.Add(vm)).To(Succeed())

			_, exists, err := vmCache.Get(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "Expected to find cache value")

			Expect(vmCache.Delete(vm)).To(Succeed())

			_, exists, err = vmCache.Get(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "Expected vm to not exist")
		})

		It("should list existing values", func() {
			vm1 := newObject("test-vm-1", "test-template")
			vm2 := newObject("test-vm-2", "test-template")

			Expect(vmCache.Add(vm1)).To(Succeed())
			Expect(vmCache.Add(vm2)).To(Succeed())

			list := vmCache.List()
			Expect(list).To(HaveLen(2))

			for _, cacheObj := range list {
				cacheVal, ok := cacheObj.(VmCacheValue)
				Expect(ok).To(BeTrue(), "Expected cacheObj to be VmCacheValue")

				switch cacheVal.Vm {
				case keyFromObject(vm1):
					templateKeys := labels.GetTemplateKeys(vm1)
					Expect(cacheVal.Template).To(Equal(templateKeys.Get().String()))
				case keyFromObject(vm2):
					templateKeys := labels.GetTemplateKeys(vm2)
					Expect(cacheVal.Template).To(Equal(templateKeys.Get().String()))
				default:
					Fail(fmt.Sprintf("Unexpected value in cache: %v", cacheVal))
				}
			}
		})

		It("should list existing keys", func() {
			vm1 := newObject("test-vm-1", "test-template")
			vm2 := newObject("test-vm-2", "test-template")

			Expect(vmCache.Add(vm1)).To(Succeed())
			Expect(vmCache.Add(vm2)).To(Succeed())

			keys := vmCache.ListKeys()
			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ConsistOf(
				keyFromObject(vm1),
				keyFromObject(vm2),
			))
		})
	})

	Context("template map", func() {
		const (
			templateName1 = "test-template"
			templateName2 = "different-template"

			templateKey1 = testTemplateNamespace + "/" + templateName1
			templateKey2 = testTemplateNamespace + "/" + templateName2
		)

		var (
			vm1 metav1.Object
			vm2 metav1.Object
			vm3 metav1.Object
		)

		BeforeEach(func() {
			vm1 = newObject("vm1", templateName1)
			vm2 = newObject("vm2", templateName2)
			vm3 = newObject("vm3", templateName1)

			Expect(vmCache.Add(vm1)).To(Succeed())
			Expect(vmCache.Add(vm2)).To(Succeed())
			Expect(vmCache.Add(vm3)).To(Succeed())
		})

		It("should get multiple values", func() {
			vmNames := vmCache.GetVmsForTemplate(templateKey1)
			Expect(vmNames).To(ConsistOf(
				keyFromObject(vm1),
				keyFromObject(vm3),
			))

			vmNames = vmCache.GetVmsForTemplate(templateKey2)
			Expect(vmNames).To(ConsistOf(
				keyFromObject(vm2),
			))
		})

		It("should clear map on delete", func() {
			Expect(vmCache.Delete(vm2)).To(Succeed())

			vmNames := vmCache.GetVmsForTemplate(templateKey2)
			Expect(vmNames).To(BeEmpty())
		})

		It("should change map on update", func() {
			updatedVm := newObject(vm2.GetName(), templateName1)
			Expect(vmCache.Update(updatedVm)).To(Succeed())

			vmNames := vmCache.GetVmsForTemplate(templateKey1)
			Expect(vmNames).To(ConsistOf(
				keyFromObject(vm1),
				keyFromObject(vm2),
				keyFromObject(vm3),
			))

			vmNames = vmCache.GetVmsForTemplate(templateKey2)
			Expect(vmNames).To(BeEmpty())
		})
	})
})

func keyFromObject(obj metav1.Object) string {
	return types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}.String()
}

func newObject(name, template string) metav1.Object {

	return &metav1.ObjectMeta{
		Name:      name,
		Namespace: testVmNamespace,
		Labels: map[string]string{
			labels.AnnotationTemplateNameKey:      template,
			labels.AnnotationTemplateNamespaceKey: testTemplateNamespace,
		},
	}
}

func TestInformers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Informers Suite")
}
