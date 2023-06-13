package common

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
)

var _ = Describe("Owner", func() {
	var owner *ssp.SSP

	BeforeEach(func() {
		owner = &ssp.SSP{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ssp.GroupVersion.String(),
				Kind:       sspResourceKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-ns",
			},
		}
	})

	It("should return false for object without annotations", func() {
		obj := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
		}
		Expect(CheckOwnerAnnotation(obj, owner)).To(BeFalse())
	})

	It("should return false for object with different type annotation", func() {
		obj := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					handler.TypeAnnotation:           "other.ssp.kubevirt.io",
					handler.NamespacedNameAnnotation: "test-ns/test-name",
				},
			},
		}
		Expect(CheckOwnerAnnotation(obj, owner)).To(BeFalse())
	})

	It("should return false for different namespacedname annotation", func() {
		obj := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					handler.TypeAnnotation:           "SSP.ssp.kubevirt.io",
					handler.NamespacedNameAnnotation: "test-ns/other-name",
				},
			},
		}
		Expect(CheckOwnerAnnotation(obj, owner)).To(BeFalse())
	})

	It("should return true for owned object", func() {
		obj := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					handler.TypeAnnotation:           "SSP.ssp.kubevirt.io",
					handler.NamespacedNameAnnotation: "test-ns/test-name",
				},
			},
		}
		Expect(CheckOwnerAnnotation(obj, owner)).To(BeTrue())
	})
})
