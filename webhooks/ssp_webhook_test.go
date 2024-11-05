/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal"
)

var _ = Describe("SSP Validation", func() {
	var (
		client  client.Client
		objects = make([]runtime.Object, 0)

		validator admission.CustomValidator
		ctx       context.Context
	)

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		// add our own scheme
		Expect(sspv1beta2.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
		// add more schemes
		Expect(v1.AddToScheme(scheme)).To(Succeed())

		client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()

		validator = newSspValidator(client)
		ctx = context.Background()
	})

	Context("creating SSP CR", func() {
		const (
			templatesNamespace = "test-templates-ns"
		)

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})
		})

		AfterEach(func() {
			objects = make([]runtime.Object, 0)
		})

		Context("when one is already present", func() {
			BeforeEach(func() {
				// add an SSP CR to fake client
				objects = append(objects, &sspv1beta2.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-ssp",
						Namespace:       "test-ns",
						ResourceVersion: "1",
					},
					Spec: sspv1beta2.SSPSpec{
						CommonTemplates: sspv1beta2.CommonTemplates{
							Namespace: templatesNamespace,
						},
					},
				})
			})

			It("should be rejected", func() {
				ssp := &sspv1beta2.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ssp2",
						Namespace: "test-ns2",
					},
					Spec: sspv1beta2.SSPSpec{
						CommonTemplates: sspv1beta2.CommonTemplates{
							Namespace: templatesNamespace,
						},
					},
				}

				_, err := validator.ValidateCreate(ctx, ssp)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists in namespace test-ns: test-ssp"))
			})
		})
	})

	Context("DataImportCronTemplates", func() {
		const (
			templatesNamespace = "test-templates-ns"
		)

		var (
			oldSSP *sspv1beta2.SSP
			newSSP *sspv1beta2.SSP
		)

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})

			oldSSP = &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta2.SSPSpec{
					CommonTemplates: sspv1beta2.CommonTemplates{
						Namespace: templatesNamespace,
						DataImportCronTemplates: []sspv1beta2.DataImportCronTemplate{
							{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: internal.GoldenImagesNamespace,
								},
							},
						},
					},
				},
			}

			newSSP = oldSSP.DeepCopy()
		})

		AfterEach(func() {
			objects = make([]runtime.Object, 0)
		})

		It("should validate dataImportCronTemplates on create", func() {
			_, err := validator.ValidateCreate(ctx, newSSP)
			Expect(err).To(HaveOccurred())

			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"

			_, err = validator.ValidateCreate(ctx, newSSP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should validate dataImportCronTemplates on update", func() {
			_, err := validator.ValidateUpdate(ctx, oldSSP, newSSP)
			Expect(err).To(HaveOccurred())

			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"

			_, err = validator.ValidateUpdate(ctx, oldSSP, newSSP)
			Expect(err).ToNot(HaveOccurred())
		})
	})

})

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
