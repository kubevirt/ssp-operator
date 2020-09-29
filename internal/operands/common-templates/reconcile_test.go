package common_templates

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"

	ssp "kubevirt.io/ssp-operator/api/v1alpha1"
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
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())
		expectResourceExists(newGoldenImagesNS(GoldenImagesNSname), request)
	})
	It("should create common-template resources", func() {
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())
		Expect(templatesBundle).ToNot(BeNil())
		for _, template := range templatesBundle {
			template.Namespace = namespace
			expectResourceExists(&template, request)
		}
	})
	It("should create view role", func() {
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())
		expectResourceExists(newViewRole(GoldenImagesNSname), request)
	})
	It("should create view role binding", func() {
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())
		expectResourceExists(newViewRoleBinding(GoldenImagesNSname), request)
	})
	It("should create view role binding", func() {
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())
		expectResourceExists(newEditRole(), request)
	})
})

//TODO: Move this to common test-util for all the unit tests
func expectResourceExists(resource controllerutil.Object, request common.Request) {
	key, err := client.ObjectKeyFromObject(resource)
	Expect(err).ToNot(HaveOccurred())
	Expect(request.Client.Get(request.Context, key, resource)).ToNot(HaveOccurred())
}
