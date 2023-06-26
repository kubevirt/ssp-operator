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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
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
		Expect(sspv1beta1.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
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

				_, err := validator.ValidateCreate(ctx, toUnstructured(ssp))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists in namespace test-ns: test-ssp"))
			})
		})

		It("should fail if template namespace does not exist", func() {
			const nonexistingNamespace = "nonexisting-namespace"
			ssp := &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta2.SSPSpec{
					CommonTemplates: sspv1beta2.CommonTemplates{
						Namespace: nonexistingNamespace,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, toUnstructured(ssp))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creation failed, the configured namespace for common templates does not exist: " + nonexistingNamespace))
		})

		It("should accept old v1beta1 SSP CR", func() {
			ssp := &sspv1beta1.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: sspv1beta1.SSPSpec{
					TemplateValidator: &sspv1beta1.TemplateValidator{
						Replicas: pointer.Int32(2),
					},
					CommonTemplates: sspv1beta1.CommonTemplates{
						Namespace: templatesNamespace,
					},
					NodeLabeller: &sspv1beta1.NodeLabeller{},
					CommonInstancetypes: &sspv1beta1.CommonInstancetypes{
						URL: pointer.String("https://foo.com/bar?ref=1234"),
					},
					TektonPipelines: &sspv1beta1.TektonPipelines{
						Namespace: "test-pipelines-ns",
					},
					TektonTasks: &sspv1beta1.TektonTasks{
						Namespace: "test-tasks-ns",
					},
					FeatureGates: &sspv1beta1.FeatureGates{
						DeployTektonTaskResources: true,
					},
				},
			}

			_, err := validator.ValidateCreate(ctx, toUnstructured(ssp))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("should allow update of commonTemplates.namespace", func() {
		oldSsp := &sspv1beta2.SSP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ssp",
				Namespace: "test-ns",
			},
			Spec: sspv1beta2.SSPSpec{
				CommonTemplates: sspv1beta2.CommonTemplates{
					Namespace: "old-ns",
				},
			},
		}

		newSsp := oldSsp.DeepCopy()
		newSsp.Spec.CommonTemplates.Namespace = "new-ns"

		_, err := validator.ValidateUpdate(ctx, toUnstructured(oldSsp), toUnstructured(newSsp))
		Expect(err).ToNot(HaveOccurred())
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
									Name:      "foo",
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
			_, err := validator.ValidateCreate(ctx, toUnstructured(newSSP))
			Expect(err).ToNot(HaveOccurred())
			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"
			_, err = validator.ValidateCreate(ctx, toUnstructured(newSSP))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should validate dataImportCronTemplates on update", func() {
			_, err := validator.ValidateUpdate(ctx, toUnstructured(oldSSP), toUnstructured(newSSP))
			Expect(err).ToNot(HaveOccurred())
			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"
			_, err = validator.ValidateUpdate(ctx, toUnstructured(oldSSP), toUnstructured(newSSP))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("CommonInstancetypes", func() {

		const (
			templatesNamespace = "test-templates-ns"
		)

		var sspObj *sspv1beta2.SSP

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})
			sspObj = &sspv1beta2.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ssp",
				},
				Spec: sspv1beta2.SSPSpec{
					CommonTemplates: sspv1beta2.CommonTemplates{
						Namespace: templatesNamespace,
					},
					CommonInstancetypes: &sspv1beta2.CommonInstancetypes{},
				},
			}
		})

		AfterEach(func() {
			objects = make([]runtime.Object, 0)
		})

		It("should reject URL without https:// or ssh://", func() {
			sspObj.Spec.CommonInstancetypes.URL = pointer.String("file://foo/bar")
			_, err := validator.ValidateCreate(ctx, toUnstructured(sspObj))
			Expect(err).To(HaveOccurred())
		})

		It("should reject URL without ?ref= or ?version=", func() {
			sspObj.Spec.CommonInstancetypes.URL = pointer.String("https://foo.com/bar")
			_, err := validator.ValidateCreate(ctx, toUnstructured(sspObj))
			Expect(err).To(HaveOccurred())
		})

		DescribeTable("should accept a valid remote kustomize target URL", func(url string) {
			sspObj.Spec.CommonInstancetypes.URL = pointer.String(url)
			_, err := validator.ValidateCreate(ctx, toUnstructured(sspObj))
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("https:// with ?ref=", "https://foo.com/bar?ref=1234"),
			Entry("https:// with ?target=", "https://foo.com/bar?version=1234"),
			Entry("ssh:// with ?ref=", "ssh://foo.com/bar?ref=1234"),
			Entry("ssh:// with ?target=", "ssh://foo.com/bar?version=1234"),
		)

		It("should accept when no URL is provided", func() {
			_, err := validator.ValidateCreate(ctx, toUnstructured(sspObj))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func toUnstructured(obj runtime.Object) *unstructured.Unstructured {
	switch t := obj.(type) {
	case *sspv1beta1.SSP:
		t.APIVersion = sspv1beta1.GroupVersion.String()
		t.Kind = "SSP"
	case *sspv1beta2.SSP:
		t.APIVersion = sspv1beta2.GroupVersion.String()
		t.Kind = "SSP"
	}

	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(fmt.Sprintf("cannot convert object to unstructured: %s", err))
	}
	return &unstructured.Unstructured{Object: data}
}

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
