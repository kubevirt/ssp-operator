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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
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
		sspv1beta1.SchemeBuilder.AddToScheme(scheme)
		// add more schemes
		v1.AddToScheme(scheme)

		client = fake.NewFakeClientWithScheme(scheme, objects...)

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
				objects = append(objects, &sspv1beta1.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-ssp",
						Namespace:       "test-ns",
						ResourceVersion: "1",
					},
					Spec: sspv1beta1.SSPSpec{
						CommonTemplates: &sspv1beta1.CommonTemplates{
							Namespace: templatesNamespace,
						},
					},
				})
			})

			It("should be rejected", func() {
				ssp := &sspv1beta1.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ssp2",
						Namespace: "test-ns2",
					},
					Spec: sspv1beta1.SSPSpec{
						CommonTemplates: &sspv1beta1.CommonTemplates{
							Namespace: templatesNamespace,
						},
					},
				}
				err := validator.ValidateCreate(ctx, ssp)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists in namespace test-ns: test-ssp"))
			})
		})

		It("should fail if template namespace does not exist", func() {
			const nonexistingNamespace = "nonexisting-namespace"
			ssp := &sspv1beta1.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta1.SSPSpec{
					CommonTemplates: &sspv1beta1.CommonTemplates{
						Namespace: nonexistingNamespace,
					},
				},
			}
			err := validator.ValidateCreate(ctx, ssp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creation failed, the configured namespace for common templates does not exist: " + nonexistingNamespace))
		})
	})

	It("should allow update of commonTemplates.namespace", func() {
		oldSsp := &sspv1beta1.SSP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ssp",
				Namespace: "test-ns",
			},
			Spec: sspv1beta1.SSPSpec{
				CommonTemplates: &sspv1beta1.CommonTemplates{
					Namespace: "old-ns",
				},
			},
		}

		newSsp := oldSsp.DeepCopy()
		newSsp.Spec.CommonTemplates.Namespace = "new-ns"

		err := validator.ValidateUpdate(ctx, oldSsp, newSsp)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("DataImportCronTemplates", func() {
		const (
			templatesNamespace = "test-templates-ns"
		)

		var (
			oldSSP *sspv1beta1.SSP
			newSSP *sspv1beta1.SSP
		)

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})

			oldSSP = &sspv1beta1.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta1.SSPSpec{
					CommonTemplates: &sspv1beta1.CommonTemplates{
						Namespace: templatesNamespace,
						DataImportCronTemplates: []sspv1beta1.DataImportCronTemplate{
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
			Expect(validator.ValidateCreate(ctx, newSSP)).To(HaveOccurred())
			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"
			Expect(validator.ValidateCreate(ctx, newSSP)).ToNot(HaveOccurred())
		})

		It("should validate dataImportCronTemplates on update", func() {
			Expect(validator.ValidateUpdate(ctx, oldSSP, newSSP)).To(HaveOccurred())
			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"
			Expect(validator.ValidateUpdate(ctx, oldSSP, newSSP)).ToNot(HaveOccurred())
		})
	})
})

func checkExpectedError(err error, shouldFail bool) {
	if shouldFail {
		ExpectWithOffset(1, err).To(HaveOccurred())
	} else {
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	}
}

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
