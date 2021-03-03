package common_templates

import (
	"context"
	"strings"
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

	libhandler "github.com/operator-framework/operator-lib/handler"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log     = logf.Log.WithName("common-templates-operand")
	operand = GetOperand()
)

const (
	namespace = "kubevirt"
	name      = "test-ssp"

	testOsLabel       = TemplateOsLabelPrefix + "some-os"
	testFlavorLabel   = TemplateFlavorLabelPrefix + "test"
	testWorkflowLabel = TemplateWorkloadLabelPrefix + "server"
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
			Logger:       log,
			VersionCache: common.VersionCache{},
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

	Context("old templates", func() {
		var (
			parentTpl, oldTpl *templatev1.Template
		)

		BeforeEach(func() {
			// Create a dummy template to act as an owner for the test template
			// we can't use the SSP CR as an owner for these tests because the tempaltes
			// might be deployed in a different namespace than the CR, and will be immediately
			// removed by the GC, the choice to use a template as an owner object was arbitrary
			parentTpl = &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-test-template",
					Namespace: request.Instance.Spec.CommonTemplates.Namespace,
				},
			}
			Expect(request.Client.Create(request.Context, parentTpl)).ToNot(HaveOccurred(), "creation of parent template failed")
			key, err := client.ObjectKeyFromObject(parentTpl)
			Expect(err).ToNot(HaveOccurred())
			Expect(request.Client.Get(request.Context, key, parentTpl)).ToNot(HaveOccurred())

			oldTpl = &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tpl",
					Namespace: request.Instance.Spec.CommonTemplates.Namespace,
					Labels: map[string]string{
						TemplateVersionLabel: "not-latest",
						TemplateTypeLabel:    "base",
						testOsLabel:          "true",
						testFlavorLabel:      "true",
						testWorkflowLabel:    "true",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: parentTpl.APIVersion,
						Kind:       parentTpl.Kind,
						UID:        parentTpl.UID,
						Name:       parentTpl.Name,
					}},
				},
			}

			err = request.Client.Create(request.Context, oldTpl)
			Expect(err).ToNot(HaveOccurred(), "creation of old template failed")
		})

		AfterEach(func() {
			Expect(request.Client.Delete(request.Context, oldTpl)).ToNot(HaveOccurred(), "deletion of parent tempalte failed")
			Expect(request.Client.Delete(request.Context, parentTpl)).ToNot(HaveOccurred(), "deletion of parent tempalte failed")
		})

		It("should replace ownerReferences with owner annotations for older templates", func() {

			Expect(oldTpl.GetOwnerReferences()).ToNot(BeNil(), "template should have owner reference before reconciliation")

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "reconciliation in order to update old template failed")

			key, err := client.ObjectKeyFromObject(oldTpl)
			Expect(err).ToNot(HaveOccurred(), "getting template object key failed")

			updatedTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, updatedTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching updated template")

			Expect(len(updatedTpl.GetOwnerReferences())).To(Equal(0), "ownerReferences exist for an older template")
			Expect(updatedTpl.GetAnnotations()[libhandler.NamespacedNameAnnotation]).ToNot(Equal(""), "owner name annotation is empty for an older template")
			Expect(updatedTpl.GetAnnotations()[libhandler.TypeAnnotation]).ToNot(Equal(""), "owner type annotation is empty for an older template")
		})
		It("should remove labels from old templates", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "reconciliation in order to update old template failed")

			key, err := client.ObjectKeyFromObject(oldTpl)
			Expect(err).ToNot(HaveOccurred(), "getting template object key failed")

			updatedTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, updatedTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching updated template")

			Expect(updatedTpl.Labels[testOsLabel]).To(Equal(""), TemplateOsLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels[testFlavorLabel]).To(Equal(""), TemplateFlavorLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels[testWorkflowLabel]).To(Equal(""), TemplateWorkloadLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels[TemplateTypeLabel]).To(Equal("base"), TemplateTypeLabel+" should equal base")
			Expect(updatedTpl.Labels[TemplateVersionLabel]).To(Equal("not-latest"), TemplateVersionLabel+" should equal not-latest")
			Expect(updatedTpl.Labels[TemplateDeprecatedLabel]).To(Equal("true"), TemplateDeprecatedLabel+" should not be empty")
		})
		It("should not remove labels from latest templates", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "reconciliation in order to update old template failed")

			var latestTemplates templatev1.TemplateList
			err = request.Client.List(request.Context, &latestTemplates, client.MatchingLabels{TemplateVersionLabel: Version})
			Expect(err).ToNot(HaveOccurred())
			for _, template := range latestTemplates.Items {
				for _, label := range template.Labels {
					if strings.HasPrefix(label, TemplateOsLabelPrefix) {
						Expect(template.Labels[label]).To(Equal("true"), TemplateOsLabelPrefix+" should not be empty")
					}
					if strings.HasPrefix(label, TemplateFlavorLabelPrefix) {
						Expect(template.Labels[label]).To(Equal("true"), TemplateFlavorLabelPrefix+" should not be empty")
					}
					if strings.HasPrefix(label, TemplateWorkloadLabelPrefix) {
						Expect(template.Labels[label]).To(Equal("true"), TemplateWorkloadLabelPrefix+" should not be empty")
					}
					Expect(template.Labels[TemplateTypeLabel]).To(Equal("base"), TemplateTypeLabel+" should equal base")
					Expect(template.Labels[TemplateVersionLabel]).To(Equal(Version), TemplateVersionLabel+" should equal "+Version)
				}
			}
		})
	})
})
