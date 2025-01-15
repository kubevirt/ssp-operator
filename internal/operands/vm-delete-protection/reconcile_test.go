package vm_delete_protection

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/google/cel-go/cel"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var _ = Describe("VM delete protection operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var (
		request common.Request
		operand = New()
		key     = client.ObjectKey{Name: virtualMachineDeleteProtectionPolicyName}
	)

	BeforeEach(func() {
		client := fake.NewClientBuilder().WithScheme(common.Scheme).Build()
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Context: context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			VersionCache: common.VersionCache{},
		}
	})

	It("should create VM deletion protection resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newValidatingAdmissionPolicy(), request)
		ExpectResourceExists(newValidatingAdmissionPolicyBinding(), request)
	})

	It("should update VAP spec if changed", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}

		Expect(request.Client.Get(request.Context, key, vap)).To(Succeed())

		vap.Spec.Variables = []admissionregistrationv1.Variable{
			{
				Name:       "test-variable",
				Expression: `test-expression`,
			},
		}

		Expect(request.Client.Update(request.Context, vap)).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		Expect(request.Client.Get(request.Context, key, vap)).To(Succeed())
		Expect(vap.Spec).To(Equal(newValidatingAdmissionPolicy().Spec))

	})

	It("should update VAPB spec if changed", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		vapb := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}

		Expect(request.Client.Get(request.Context, key, vapb)).To(Succeed())

		vapb.Spec.ValidationActions = []admissionregistrationv1.ValidationAction{
			admissionregistrationv1.Warn,
		}

		Expect(request.Client.Update(request.Context, vapb)).To(Succeed())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		Expect(request.Client.Get(request.Context, key, vapb)).To(Succeed())
		Expect(vapb.Spec).To(Equal(newValidatingAdmissionPolicyBinding().Spec))
	})

	It("should create one valid CEL expression", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}

		Expect(request.Client.Get(request.Context, key, vap)).To(Succeed())
		Expect(vap.Spec.Validations).To(HaveLen(1))

		celEnv, err := cel.NewEnv()
		Expect(err).ToNot(HaveOccurred())

		_, issues := celEnv.Parse(vap.Spec.Validations[0].Expression)
		Expect(issues.Err()).ToNot(HaveOccurred())
	})
})

func TestVMDeleteProtection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VM Delete Protection Suite")
}
