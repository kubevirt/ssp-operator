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

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal"
	"kubevirt.io/ssp-operator/internal/common"
)

var _ = Describe("SSP Validation", func() {

	var (
		apiClient       client.Client
		createIntercept func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error

		validator admission.CustomValidator
		ctx       context.Context
	)

	BeforeEach(func() {
		createIntercept = func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			return client.Create(ctx, obj, opts...)
		}

		apiClient = fake.NewClientBuilder().
			WithScheme(common.Scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					return createIntercept(ctx, client, obj, opts...)
				},
			}).
			Build()

		validator = newSspValidator(apiClient)
	})

	Context("creating SSP CR", func() {
		BeforeEach(func() {
			err := apiClient.Create(ctx, &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta2.SSPSpec{},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject SSP when one is already present", func() {
			ssp := &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp2",
					Namespace: "test-ns2",
				},
				Spec: sspv1beta2.SSPSpec{},
			}

			_, err := validator.ValidateCreate(ctx, ssp)
			Expect(err).To(MatchError(ContainSubstring("creation failed, an SSP CR already exists in namespace test-ns: test-ssp")))
		})
	})

	Context("DataImportCronTemplates", func() {
		var (
			oldSSP *sspv1beta2.SSP
			newSSP *sspv1beta2.SSP
		)

		BeforeEach(func() {
			oldSSP = &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta2.SSPSpec{
					CommonTemplates: sspv1beta2.CommonTemplates{
						Namespace: "test-templates-ns",
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

		It("should validate dataImportCronTemplates on create", func() {
			_, err := validator.ValidateCreate(ctx, newSSP)
			Expect(err).To(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))

			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"

			_, err = validator.ValidateCreate(ctx, newSSP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should validate dataImportCronTemplates on update", func() {
			_, err := validator.ValidateUpdate(ctx, oldSSP, newSSP)
			Expect(err).To(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))

			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"

			_, err = validator.ValidateUpdate(ctx, oldSSP, newSSP)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("validate placement", func() {
		It("should not call create API, if placement is nil", func() {
			createIntercept = func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
				Fail("Called create API")
				return nil
			}

			ssp := &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta2.SSPSpec{
					TemplateValidator: &sspv1beta2.TemplateValidator{
						Replicas:  ptr.To(int32(2)),
						Placement: nil,
					},
				},
			}

			_, err := validator.ValidateCreate(ctx, ssp)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call create API dry run", func() {
			placement := &api.NodePlacement{
				NodeSelector: map[string]string{
					"test-label": "test-value",
				},
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						}},
					},
				},
				Tolerations: []v1.Toleration{{
					Key:   "key",
					Value: "value",
				}},
			}

			var createWasCalled bool
			createIntercept = func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				deployment, ok := obj.(*apps.Deployment)
				if !ok {
					Fail("Expected created object to be Deployment.")
				}

				createOptions := &client.CreateOptions{}
				for _, opt := range opts {
					opt.ApplyToCreate(createOptions)
				}

				if len(createOptions.DryRun) != 1 || createOptions.DryRun[0] != metav1.DryRunAll {
					Fail("Create call should be dry run.")
				}

				Expect(deployment.Spec.Template.Spec.NodeSelector).To(Equal(placement.NodeSelector))
				Expect(deployment.Spec.Template.Spec.Affinity).To(Equal(placement.Affinity))
				Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(placement.Tolerations))
				createWasCalled = true
				return nil
			}

			ssp := &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta2.SSPSpec{
					TemplateValidator: &sspv1beta2.TemplateValidator{
						Replicas:  ptr.To(int32(2)),
						Placement: placement,
					},
				},
			}

			_, err := validator.ValidateCreate(ctx, ssp)
			Expect(err).ToNot(HaveOccurred())

			Expect(createWasCalled).To(BeTrue())
		})
	})
})

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Suite")
}
