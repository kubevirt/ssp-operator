package predicates

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
)

var _ = Describe("SpecChangedPredicate", func() {
	It("Should be false if spec is the same", func() {
		obj := &sspv1beta1.SSP{
			Spec: sspv1beta1.SSPSpec{
				TemplateValidator: &sspv1beta1.TemplateValidator{
					Replicas: pointer.Int32(1),
				},
			},
		}

		updateEvent := event.UpdateEvent{
			ObjectOld: obj,
			ObjectNew: obj.DeepCopy(),
		}

		Expect(SpecChangedPredicate{}.Update(updateEvent)).To(BeFalse())
	})

	It("Shuld be true if spec is different", func() {
		obj := &sspv1beta1.SSP{
			Spec: sspv1beta1.SSPSpec{
				TemplateValidator: &sspv1beta1.TemplateValidator{
					Replicas: pointer.Int32(1),
				},
			},
		}

		newObj := obj.DeepCopy()
		newObj.Spec.TemplateValidator.Replicas = pointer.Int32(2)

		updateEvent := event.UpdateEvent{
			ObjectOld: obj,
			ObjectNew: newObj,
		}

		Expect(SpecChangedPredicate{}.Update(updateEvent)).To(BeTrue())
	})

	It("Should be true is spec does not exist", func() {
		obj := &v1.Role{}

		updateEvent := event.UpdateEvent{
			ObjectOld: obj,
			ObjectNew: obj.DeepCopy(),
		}

		Expect(SpecChangedPredicate{}.Update(updateEvent)).To(BeTrue())
	})
})

func TestPredicates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Predicates Suite")
}
