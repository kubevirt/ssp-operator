package predicates

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
)

var _ = Describe("SpecChangedPredicate", func() {
	It("Should be false if spec is the same", func() {
		obj := &ssp.SSP{
			Spec: ssp.SSPSpec{
				TemplateValidator: &ssp.TemplateValidator{
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
		obj := &ssp.SSP{
			Spec: ssp.SSPSpec{
				TemplateValidator: &ssp.TemplateValidator{
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
