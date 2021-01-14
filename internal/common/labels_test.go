package common

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
)

var _ = Describe("AddAppLabels", func() {
	var (
		request Request
	)

	BeforeEach(func() {
		request = Request{
			Instance: &ssp.SSP{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						AppKubernetesPartOfLabel:  "tests",
						AppKubernetesVersionLabel: "v0.0.0-tests",
					},
				},
			},
		}
	})

	When("SSP CR has app labels", func() {
		It("adds app labels from request", func() {
			obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

			labels := obj.GetLabels()
			Expect(labels[AppKubernetesPartOfLabel]).To(Equal("tests"))
			Expect(labels[AppKubernetesVersionLabel]).To(Equal("v0.0.0-tests"))
		})
	})
	When("SSP CR does not have app labels", func() {
		It("does not add app labels on nil", func() {
			request.Instance.Labels = nil
			obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

			labels := obj.GetLabels()
			Expect(labels[AppKubernetesPartOfLabel]).To(Equal(""))
			Expect(labels[AppKubernetesVersionLabel]).To(Equal(""))
		})
		It("does not add app labels empty map", func() {
			request.Instance.Labels = map[string]string{}
			obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

			labels := obj.GetLabels()
			Expect(labels[AppKubernetesPartOfLabel]).To(Equal(""))
			Expect(labels[AppKubernetesVersionLabel]).To(Equal(""))
		})
	})

	It("adds dynamic app labels", func() {
		obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

		labels := obj.GetLabels()
		Expect(labels[AppKubernetesComponentLabel]).To(Equal("testing"))
		Expect(labels[AppKubernetesNameLabel]).To(Equal("test"))
	})

	It("adds managed-by label", func() {
		obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

		labels := obj.GetLabels()
		Expect(labels[AppKubernetesManagedByLabel]).To(Equal("ssp-operator"))
	})
})
