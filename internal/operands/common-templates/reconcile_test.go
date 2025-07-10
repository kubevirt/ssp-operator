package common_templates

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templatev1 "github.com/openshift/api/template/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/architecture"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	metrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
)

var log = logf.Log.WithName("common-templates-operand")

const (
	namespace = "kubevirt"
	name      = "test-ssp"

	futureVersion = "v999.999.999"
)

var _ = Describe("Common-Templates operand", func() {

	var (
		testTemplates []templatev1.Template
		operand       operands.Operand
		request       common.Request
	)

	BeforeEach(func() {
		defaultArchitecture = architecture.AMD64

		var err error
		operand, err = New(getTestTemplatesMultiArch())
		Expect(err).ToNot(HaveOccurred())

		testTemplates = getTestTemplates()

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
					Labels: map[string]string{
						common.AppKubernetesPartOfLabel:  "template-unit-tests",
						common.AppKubernetesVersionLabel: "v1.0.0",
					},
				},
				Spec: ssp.SSPSpec{
					CommonTemplates: ssp.CommonTemplates{
						Namespace: namespace,
					},
				},
				Status: ssp.SSPStatus{
					Status: lifecycleapi.Status{
						ObservedVersion: env.GetOperatorVersion(),
					},
				},
			},
			InstanceChanged: false,
			Logger:          log,
			VersionCache:    common.VersionCache{},
		}
	})

	It("should create common-template resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		for _, template := range testTemplates {
			template.Namespace = namespace
			ExpectResourceExists(&template, request)
		}

		value, err := metrics.GetCommonTemplatesRestored()
		Expect(err).ToNot(HaveOccurred())
		Expect(value).To(BeZero())
	})

	It("should reconcile predefined labels", func() {
		const (
			defaultOsLabel = "template.kubevirt.io/default-os-variant"
			testLabel      = "some.test.label"
		)

		for _, template := range getTestTemplates() {
			template.Namespace = namespace
			template.Labels[defaultOsLabel] = "true"
			template.Labels[testLabel] = "test"
			Expect(request.Client.Create(request.Context, &template)).To(Succeed())
		}

		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		for i := range testTemplates {
			key := client.ObjectKey{
				Name:      testTemplates[i].Name,
				Namespace: namespace,
			}
			template := &templatev1.Template{}
			Expect(request.Client.Get(request.Context, key, template)).To(Succeed())

			Expect(template.Labels).ToNot(HaveKey(defaultOsLabel))
			Expect(template.Labels).To(HaveKey(testLabel))
		}

		value, err := metrics.GetCommonTemplatesRestored()
		Expect(err).ToNot(HaveOccurred())
		Expect(value).To(Equal(float64(len(testTemplates))))
	})

	Context("old templates", func() {
		var (
			oldTpl        templatev1.Template
			newerTemplate templatev1.Template
		)

		BeforeEach(func() {
			oldTpl = createTestTemplate("test-tpl", "some-os", "test", "server", architecture.AMD64)
			oldTpl.Namespace = namespace
			oldTpl.Labels[TemplateVersionLabel] = "not-latest"

			Expect(libhandler.SetOwnerAnnotations(request.Instance, &oldTpl)).To(Succeed())

			err := request.Client.Create(request.Context, &oldTpl)
			Expect(err).ToNot(HaveOccurred(), "creation of old template failed")

			newerTemplate = createTestTemplate("test-tpl-newer", "some-os", "test", "server", architecture.AMD64)
			newerTemplate.Namespace = namespace
			newerTemplate.Labels[TemplateVersionLabel] = futureVersion

			Expect(libhandler.SetOwnerAnnotations(request.Instance, &newerTemplate)).To(Succeed())

			err = request.Client.Create(request.Context, &newerTemplate)
			Expect(err).ToNot(HaveOccurred(), "creation of newer template failed")
		})

		It("should remove labels from old templates but keep future template untouched", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "reconciliation in order to update old template failed")

			key := client.ObjectKeyFromObject(&oldTpl)
			updatedTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, updatedTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching updated template")

			Expect(updatedTpl.Labels).ToNot(HaveKey(HavePrefix(TemplateOsLabelPrefix)), TemplateOsLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels).ToNot(HaveKey(HavePrefix(TemplateFlavorLabelPrefix)), TemplateFlavorLabelPrefix+" should be empty")
			Expect(updatedTpl.Labels).ToNot(HaveKey(HavePrefix(TemplateWorkloadLabelPrefix)), TemplateWorkloadLabelPrefix+" should be empty")

			Expect(updatedTpl.Labels[TemplateTypeLabel]).To(Equal(TemplateTypeLabelBaseValue), TemplateTypeLabel+" should equal base")
			Expect(updatedTpl.Labels[TemplateVersionLabel]).To(Equal("not-latest"), TemplateVersionLabel+" should equal not-latest")
			Expect(updatedTpl.Annotations[TemplateDeprecatedAnnotation]).To(Equal("true"), TemplateDeprecatedAnnotation+" should not be empty")

			key = client.ObjectKeyFromObject(&newerTemplate)
			newerTpl := &templatev1.Template{}
			err = request.Client.Get(request.Context, key, newerTpl)
			Expect(err).ToNot(HaveOccurred(), "failed fetching newer template")

			Expect(newerTpl.Labels).To(HaveKeyWithValue(HavePrefix(TemplateOsLabelPrefix), "true"), TemplateOsLabelPrefix+" should not be empty")
			Expect(newerTpl.Labels).To(HaveKeyWithValue(HavePrefix(TemplateFlavorLabelPrefix), "true"), TemplateFlavorLabelPrefix+" should not be empty")
			Expect(newerTpl.Labels).To(HaveKeyWithValue(HavePrefix(TemplateWorkloadLabelPrefix), "true"), TemplateWorkloadLabelPrefix+" should not be empty")

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

	Context("kubevirt_ssp_common_templates_restored_total metric", func() {
		var (
			originalTemplate   *templatev1.Template
			template           *templatev1.Template
			initialMetricValue float64
		)

		BeforeEach(func() {
			originalTemplate = &testTemplates[0]
			originalTemplate.Namespace = namespace

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			template = &templatev1.Template{}
			Expect(request.Client.Get(
				request.Context, client.ObjectKeyFromObject(originalTemplate), template,
			)).To(Succeed())

			value, err := metrics.GetCommonTemplatesRestored()
			Expect(err).ToNot(HaveOccurred())
			initialMetricValue = value
		})

		It("should increase by 1 when one template is restored", func() {
			template.Labels[TemplateTypeLabel] = "rand"
			err := request.Client.Update(request.Context, template)
			Expect(err).ToNot(HaveOccurred())

			_, err = operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			updatedTpl := &templatev1.Template{}
			Expect(request.Client.Get(
				request.Context, client.ObjectKeyFromObject(template), updatedTpl,
			)).To(Succeed())

			Expect(updatedTpl.Labels[TemplateTypeLabel]).To(Equal(originalTemplate.Labels[TemplateTypeLabel]))

			value, err := metrics.GetCommonTemplatesRestored()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(initialMetricValue + 1))
		})

		It("should not increase when template restored is from previous version", func() {
			template.Labels[TemplateTypeLabel] = "rand"
			template.Labels[TemplateVersionLabel] = "rand"
			err := request.Client.Update(request.Context, template)
			Expect(err).ToNot(HaveOccurred())

			_, err = operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			updatedTpl := &templatev1.Template{}
			Expect(request.Client.Get(
				request.Context, client.ObjectKeyFromObject(template), updatedTpl,
			)).To(Succeed())

			Expect(updatedTpl.Labels[TemplateTypeLabel]).To(Equal(originalTemplate.Labels[TemplateTypeLabel]))

			value, err := metrics.GetCommonTemplatesRestored()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(initialMetricValue))
		})

		It("should not increase when SSP CR is changed", func() {
			const updatedPartOf = "updated-part-of"
			const updatedVersion = "v2.0.0"

			request.Instance.Labels[common.AppKubernetesPartOfLabel] = updatedPartOf
			request.Instance.Labels[common.AppKubernetesVersionLabel] = updatedVersion
			request.InstanceChanged = true

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			updatedTemplate := &templatev1.Template{}
			Expect(request.Client.Get(
				request.Context, client.ObjectKeyFromObject(template), updatedTemplate,
			)).To(Succeed())

			Expect(updatedTemplate.Labels).To(HaveKeyWithValue(common.AppKubernetesPartOfLabel, updatedPartOf))
			Expect(updatedTemplate.Labels).To(HaveKeyWithValue(common.AppKubernetesVersionLabel, updatedVersion))

			value, err := metrics.GetCommonTemplatesRestored()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(initialMetricValue))
		})
	})
})

func getTestTemplates() []templatev1.Template {
	return []templatev1.Template{
		createTestTemplate("centos-stream8-server-medium", "centos8", "medium", "server", architecture.AMD64),
		createTestTemplate("windows10-desktop-medium", "win10", "medium", "desktop", architecture.AMD64),
	}
}

func getTestTemplatesMultiArch() []templatev1.Template {
	return []templatev1.Template{
		createTestTemplate("centos-stream8-server-medium", "centos8", "medium", "server", architecture.AMD64),
		createTestTemplate("windows10-desktop-medium", "win10", "medium", "desktop", architecture.AMD64),
		createTestTemplate("centos-stream8-server-medium-"+string(architecture.ARM64), "centos8", "medium", "server", architecture.ARM64),
		createTestTemplate("windows10-desktop-medium-"+string(architecture.ARM64), "win10", "medium", "desktop", architecture.ARM64),
		createTestTemplate("centos-stream8-server-medium-"+string(architecture.S390X), "centos8", "medium", "server", architecture.S390X),
		createTestTemplate("windows10-desktop-medium-"+string(architecture.S390X), "win10", "medium", "desktop", architecture.S390X),
	}
}

func createTestTemplate(name, os, flavor, workload string, arch architecture.Arch) templatev1.Template {
	return templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				TemplateTypeLabel:                      TemplateTypeLabelBaseValue,
				TemplateVersionLabel:                   Version,
				TemplateOsLabelPrefix + os:             "true",
				TemplateFlavorLabelPrefix + flavor:     "true",
				TemplateWorkloadLabelPrefix + workload: "true",
				TemplateArchitectureLabel:              string(arch),
			},
		},
	}
}
