package common_templates

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	namespace = "kubevirt"
	name      = "test-ssp"
)

func TestTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Templates Suite")
}

var _ = Describe("Common-Templates operand", func() {

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
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
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

	It("should remove labels only from old templates", func() {
		oldVersion := "v0.0.1"
		err := request.Client.Create(request.Context, getTestTemplate(oldVersion, "1", namespace))
		Expect(err).ToNot(HaveOccurred())
		err = request.Client.Create(request.Context, getTestTemplate(oldVersion, "2", "test-namespace"))
		Expect(err).ToNot(HaveOccurred())
		err = request.Client.Create(request.Context, getTestTemplate(Version, "3", namespace))
		Expect(err).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		var oldTemplates templatev1.TemplateList
		request.Client.List(request.Context, &oldTemplates)
		for _, template := range oldTemplates.Items {
			isTestTemplate := false
			if _, ok := template.Labels["test"]; ok {
				isTestTemplate = true
			}
			if value, ok := template.Labels["template.kubevirt.io/version"]; ok && value == oldVersion && isTestTemplate {
				Expect(template.Labels["os.template.kubevirt.io/some-os"]).To(Equal(""), "os.template.kubevirt.io should be empty")
				Expect(template.Labels["flavor.template.kubevirt.io/test"]).To(Equal(""), "flavor.template.kubevirt.io should be empty")
				Expect(template.Labels["workload.template.kubevirt.io/server"]).To(Equal(""), "workload.template.kubevirt.io should be empty")
				Expect(template.Labels["template.kubevirt.io/type"]).To(Equal("base"), "template.kubevirt.io/type should equal base")
				Expect(template.Labels["template.kubevirt.io/version"]).To(Equal(oldVersion), "template.kubevirt.io/version should equal "+oldVersion)
			}

			if value, ok := template.Labels["template.kubevirt.io/version"]; ok && value == Version && isTestTemplate {
				fmt.Printf("%#v\n", template)
				Expect(template.Labels["os.template.kubevirt.io/some-os"]).To(Equal("true"), "os.template.kubevirt.io should not be empty")
				Expect(template.Labels["flavor.template.kubevirt.io/test"]).To(Equal("true"), "flavor.template.kubevirt.io should not be empty")
				Expect(template.Labels["workload.template.kubevirt.io/server"]).To(Equal("true"), "workload.template.kubevirt.io should not be empty")
				Expect(template.Labels["template.kubevirt.io/type"]).To(Equal("base"), "template.kubevirt.io/type should equal base")
				Expect(template.Labels["template.kubevirt.io/version"]).To(Equal(Version), "template.kubevirt.io/version should equal "+Version)
			}
		}
	})
})

func getTestTemplate(version, indexStr string, namespace string) *templatev1.Template {
	return &templatev1.Template{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Template",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template" + indexStr,
			Namespace: namespace,
			Labels: map[string]string{
				"os.template.kubevirt.io/some-os":      "true",
				"flavor.template.kubevirt.io/test":     "true",
				"template.kubevirt.io/type":            "base",
				"template.kubevirt.io/version":         version,
				"workload.template.kubevirt.io/server": "true",
			},
		},
	}
}
