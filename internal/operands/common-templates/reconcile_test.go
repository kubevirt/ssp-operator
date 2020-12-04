package common_templates

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
)

var (
	log     = logf.Log.WithName("common-templates-operand")
	operand = GetOperand()
)

func TestTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Templates Suite")
}

var _ = Describe("Common-Templates operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var request common.Request

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(operand.AddWatchTypesToScheme(s)).ToNot(HaveOccurred())

		client := fake.NewFakeClientWithScheme(s)
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Scheme:  s,
			Context: context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: meta.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: meta.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ssp.SSPSpec{
					CommonTemplates: ssp.CommonTemplates{
						Namespace: namespace,
					},
				},
			},
			Logger: log,
		}
	})

	It("should create golden-images namespace", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newGoldenImagesNS(GoldenImagesNSname), request)
	})
	It("should create common-template resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		Expect(templatesBundle).ToNot(BeNil())
		for _, template := range templatesBundle {
			template.Namespace = namespace
			ExpectResourceExists(&template, request)
		}
	})
	It("should create view role", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newViewRole(GoldenImagesNSname), request)
	})
	It("should create view role binding", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newViewRoleBinding(GoldenImagesNSname), request)
	})
	It("should create view role binding", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newEditRole(), request)
	})
})
