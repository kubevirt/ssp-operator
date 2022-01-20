package common_templates

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var log = logf.Log.WithName("common-templates-operand")

const (
	namespace = "kubevirt"
	name      = "test-ssp"

	testOsLabel       = TemplateOsLabelPrefix + "some-os"
	testFlavorLabel   = TemplateFlavorLabelPrefix + "test"
	testWorkflowLabel = TemplateWorkloadLabelPrefix + "server"
	futureVersion     = "v999.999.999"
)

func TestTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Templates Suite")
}

var _ = Describe("Common-Templates operand", func() {

	var (
		testTemplates []templatev1.Template
		operand       operands.Operand
		request       common.Request
	)

	BeforeEach(func() {
		testTemplates = getTestTemplates()
		operand = New(testTemplates)

		client := fake.NewFakeClientWithScheme(common.Scheme)
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

	It("should create common-template resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		for _, template := range testTemplates {
			template.Namespace = namespace
			ExpectResourceExists(&template, request)
		}
	})

	Context("old templates", func() {
		var (
			parentTpl, oldTpl, newerTemplate *templatev1.Template
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
			key := client.ObjectKeyFromObject(parentTpl)
			Expect(request.Client.Get(request.Context, key, parentTpl)).ToNot(HaveOccurred())

			oldTpl = &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tpl",
					Namespace: request.Instance.Spec.CommonTemplates.Namespace,
					Labels: map[string]string{
						TemplateVersionLabel: "not-latest",
						TemplateTypeLabel:    TemplateTypeLabelBaseValue,
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

			err := request.Client.Create(request.Context, oldTpl)
			Expect(err).ToNot(HaveOccurred(), "creation of old template failed")

			newerTemplate = &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tpl-newer",
					Namespace: request.Instance.Spec.CommonTemplates.Namespace,
					Labels: map[string]string{
						TemplateVersionLabel: futureVersion,
						TemplateTypeLabel:    TemplateTypeLabelBaseValue,
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

			err = request.Client.Create(request.Context, newerTemplate)
			Expect(err).ToNot(HaveOccurred(), "creation of newer template failed")
		})

		AfterEach(func() {
			Expect(request.Client.Delete(request.Context, oldTpl)).ToNot(HaveOccurred(), "deletion of old tempalte failed")
			Expect(request.Client.Delete(request.Context, parentTpl)).ToNot(HaveOccurred(), "deletion of parent tempalte failed")
			Expect(request.Client.Delete(request.Context, newerTemplate)).ToNot(HaveOccurred(), "deletion of newer tempalte failed")
		})

		It("should replace ownerReferences with owner annotations for older templates", func() {

			Expect(oldTpl.GetOwnerReferences()).ToNot(BeNil(), "template should have owner reference before reconciliation")

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "reconciliation in order to update old template failed")

			key := client.ObjectKeyFromObject(oldTpl)
			updatedTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, updatedTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching updated template")

			Expect(len(updatedTpl.GetOwnerReferences())).To(Equal(0), "ownerReferences exist for an older template")
			Expect(updatedTpl.GetAnnotations()[libhandler.NamespacedNameAnnotation]).ToNot(Equal(""), "owner name annotation is empty for an older template")
			Expect(updatedTpl.GetAnnotations()[libhandler.TypeAnnotation]).ToNot(Equal(""), "owner type annotation is empty for an older template")
		})
		It("should remove labels from old templates but keep future template untouched", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "reconciliation in order to update old template failed")

			key := client.ObjectKeyFromObject(oldTpl)
			updatedTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, updatedTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching updated template")

			Expect(updatedTpl.Labels[testOsLabel]).To(Equal(""), TemplateOsLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels[testFlavorLabel]).To(Equal(""), TemplateFlavorLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels[testWorkflowLabel]).To(Equal(""), TemplateWorkloadLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels[TemplateTypeLabel]).To(Equal(TemplateTypeLabelBaseValue), TemplateTypeLabel+" should equal base")
			Expect(updatedTpl.Labels[TemplateVersionLabel]).To(Equal("not-latest"), TemplateVersionLabel+" should equal not-latest")
			Expect(updatedTpl.Annotations[TemplateDeprecatedAnnotation]).To(Equal("true"), TemplateDeprecatedAnnotation+" should not be empty")

			key = client.ObjectKeyFromObject(newerTemplate)
			newerTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, newerTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching newer template")

			Expect(newerTpl.Labels[testOsLabel]).To(Equal("true"), TemplateOsLabelPrefix+" should not be empty")
			Expect(newerTpl.Labels[testFlavorLabel]).To(Equal("true"), TemplateFlavorLabelPrefix+" should not be empty")
			Expect(newerTpl.Labels[testWorkflowLabel]).To(Equal("true"), TemplateWorkloadLabelPrefix+" should not be empty")
			Expect(newerTpl.Labels[TemplateTypeLabel]).To(Equal("base"), TemplateTypeLabel+" should equal base")
			Expect(newerTpl.Labels[TemplateVersionLabel]).To(Equal(futureVersion), TemplateVersionLabel+" should equal "+futureVersion)
			Expect(newerTpl.Annotations[TemplateDeprecatedAnnotation]).To(Equal(""), TemplateDeprecatedAnnotation+" should be empty")
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
					Expect(template.Labels[TemplateTypeLabel]).To(Equal(TemplateTypeLabelBaseValue), TemplateTypeLabel+" should equal base")
					Expect(template.Labels[TemplateVersionLabel]).To(Equal(Version), TemplateVersionLabel+" should equal "+Version)
				}
				Expect(template.Annotations[TemplateDeprecatedAnnotation]).To(Equal(""), TemplateDeprecatedAnnotation+" should be empty")

			}
		})
	})
})

func getTestTemplates() []templatev1.Template {
	return []templatev1.Template{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "centos-stream8-server-medium",
			Labels: map[string]string{
				TemplateTypeLabel:                      TemplateTypeLabelBaseValue,
				TemplateVersionLabel:                   "v0.16.2",
				TemplateOsLabelPrefix + "centos8":      "true",
				TemplateFlavorLabelPrefix + "medium":   "true",
				TemplateWorkloadLabelPrefix + "server": "true",
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "windows10-desktop-medium",
			Labels: map[string]string{
				TemplateTypeLabel:                       TemplateTypeLabelBaseValue,
				TemplateVersionLabel:                    "v0.16.2",
				TemplateOsLabelPrefix + "win10":         "true",
				TemplateFlavorLabelPrefix + "medium":    "true",
				TemplateWorkloadLabelPrefix + "desktop": "true",
			},
		},
	}}
}
