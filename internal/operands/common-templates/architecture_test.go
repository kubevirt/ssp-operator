package common_templates

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/ssp-operator/internal/architecture"
)

var _ = Describe("GetTemplateArch()", func() {
	It("should read architecture from label", func() {
		const testArch = architecture.ARM64
		template := &templatev1.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Labels: map[string]string{
					TemplateArchitectureLabel: string(testArch),
				},
			},
		}
		Expect(GetTemplateArch(template)).To(Equal(testArch))
	})

	It("should use default architecture if label is missing", func() {
		template := &templatev1.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		Expect(GetTemplateArch(template)).To(Equal(TemplateDefaultArchitecture))
	})

	It("should fail, if architecture value is not known", func() {
		template := &templatev1.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Labels: map[string]string{
					TemplateArchitectureLabel: "unknown-arch",
				},
			},
		}
		_, err := GetTemplateArch(template)
		Expect(err).To(MatchError(ContainSubstring("unknown architecture: unknown-arch")))
	})
})
