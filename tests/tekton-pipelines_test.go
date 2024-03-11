package tests

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/operator-framework/operator-lib/handler"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
)

var _ = Describe("Tekton Pipelines Operand", func() {
	Context("annotations on old resources", Ordered, func() {
		const (
			tektonDeprecatedAnnotation = "tekton.dev/deprecated"
			testResourceNamePrefix     = "ssp-pipelines-test-"
			operandName                = "tekton-pipelines"
		)

		var (
			serviceAccount *v1.ServiceAccount
			clusterRole    *rbac.ClusterRole
			roleBinding    *rbac.RoleBinding
			configMap      *v1.ConfigMap
			fakePipeline   *pipeline.Pipeline //nolint:staticcheck

			matchingLabels client.MatchingLabels
		)

		BeforeAll(func() {
			// Adding fake resources to simulate resources left from the previous version
			namespace := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testResourceNamePrefix,
				},
			}
			Expect(apiClient.Create(ctx, namespace)).To(Succeed())
			DeferCleanup(func() {
				Expect(apiClient.Delete(ctx, namespace)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
			})

			ssp := getSsp()

			commonAnnotations := map[string]string{
				handler.TypeAnnotation:           sspv1beta2.GroupVersion.WithKind("SSP").GroupKind().String(),
				handler.NamespacedNameAnnotation: ssp.GetNamespace() + "/" + ssp.GetName(),
			}

			commonLabels := map[string]string{
				common.AppKubernetesNameLabel:      operandName,
				common.AppKubernetesComponentLabel: common.AppComponentTektonPipelines.String(),
				common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
			}
			if sspPartOfLabel, exists := ssp.Labels[common.AppKubernetesPartOfLabel]; exists {
				commonLabels[common.AppKubernetesPartOfLabel] = sspPartOfLabel
			}

			serviceAccount = &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    namespace.Name,
					GenerateName: testResourceNamePrefix,
					Annotations:  commonAnnotations,
					Labels:       commonLabels,
				},
			}
			Expect(apiClient.Create(ctx, serviceAccount)).To(Succeed())
			DeferCleanup(func() {
				Expect(apiClient.Delete(ctx, serviceAccount)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
			})

			clusterRole = &rbac.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testResourceNamePrefix,
					Annotations:  commonAnnotations,
					Labels:       commonLabels,
				},
			}
			Expect(apiClient.Create(ctx, clusterRole)).To(Succeed())
			DeferCleanup(func() {
				Expect(apiClient.Delete(ctx, clusterRole)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
			})

			roleBinding = &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    namespace.Name,
					GenerateName: testResourceNamePrefix,
					Annotations:  commonAnnotations,
					Labels:       commonLabels,
				},
				Subjects: []rbac.Subject{{
					Kind:      rbac.ServiceAccountKind,
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				}},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     clusterRole.Name,
				},
			}
			Expect(apiClient.Create(ctx, roleBinding)).To(Succeed())
			DeferCleanup(func() {
				Expect(apiClient.Delete(ctx, roleBinding)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
			})

			configMap = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    namespace.Name,
					GenerateName: testResourceNamePrefix,
					Annotations:  commonAnnotations,
					Labels:       commonLabels,
				},
				Data: nil,
			}
			Expect(apiClient.Create(ctx, configMap)).To(Succeed())
			DeferCleanup(func() {
				Expect(apiClient.Delete(ctx, configMap)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
			})

			fakePipeline = &pipeline.Pipeline{ //nolint:staticcheck
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    namespace.Name,
					GenerateName: testResourceNamePrefix,
					Annotations:  commonAnnotations,
					Labels:       commonLabels,
				},
				Spec: pipeline.PipelineSpec{
					DisplayName: "test-pipeline",
					Description: "test-pipeline",
					Tasks:       nil,
				},
			}
			Expect(apiClient.Create(ctx, fakePipeline)).To(Succeed())
			DeferCleanup(func() {
				Expect(apiClient.Delete(ctx, fakePipeline)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
			})

			waitUntilDeployed()

			matchingLabels = client.MatchingLabels{
				common.AppKubernetesNameLabel:      operandName,
				common.AppKubernetesComponentLabel: common.AppComponentTektonPipelines.String(),
				common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
			}
		})

		It("[test_id:TODO] check ServiceAccounts annotations", func() {
			objList := &v1.ServiceAccountList{}
			Expect(apiClient.List(ctx, objList, matchingLabels)).To(Succeed())

			for _, obj := range objList.Items {
				Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecatedAnnotation, "true"), fmt.Sprintf("ServiceAccount %s/%s does not have deprecation annotation.", obj.Namespace, obj.Name))
			}
			Expect(objList.Items).To(ContainElement(Satisfy(func(obj v1.ServiceAccount) bool {
				return obj.Namespace == serviceAccount.Namespace && obj.Name == serviceAccount.Name
			})), "The fake ServiceAccount was not listed.")
		})

		It("[test_id:TODO] check ClusterRoles annotations", func() {
			objList := &rbac.ClusterRoleList{}
			Expect(apiClient.List(ctx, objList, matchingLabels)).To(Succeed())

			for _, obj := range objList.Items {
				Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecatedAnnotation, "true"), fmt.Sprintf("ClusterRole %s does not have deprecation annotation.", obj.Name))
			}
			Expect(objList.Items).To(ContainElement(Satisfy(func(obj rbac.ClusterRole) bool {
				return obj.Name == clusterRole.Name
			})), "The fake ClusterRole was not listed.")
		})

		It("[test_id:TODO] check RoleBindings annotations", func() {
			objList := &rbac.RoleBindingList{}
			Expect(apiClient.List(ctx, objList, matchingLabels)).To(Succeed())

			for _, obj := range objList.Items {
				Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecatedAnnotation, "true"), fmt.Sprintf("RoleBinding %s/%s does not have deprecation annotation.", obj.Namespace, obj.Name))
			}
			Expect(objList.Items).To(ContainElement(Satisfy(func(obj rbac.RoleBinding) bool {
				return obj.Namespace == roleBinding.Namespace && obj.Name == roleBinding.Name
			})), "The fake RoleBinding was not listed.")
		})

		It("[test_id:TODO] check ConfigMaps annotations", func() {
			objList := &v1.ConfigMapList{}
			Expect(apiClient.List(ctx, objList, matchingLabels)).To(Succeed())

			for _, obj := range objList.Items {
				Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecatedAnnotation, "true"), fmt.Sprintf("ConfigMap %s/%s does not have deprecation annotation.", obj.Namespace, obj.Name))
			}
			Expect(objList.Items).To(ContainElement(Satisfy(func(obj v1.ConfigMap) bool {
				return obj.Namespace == configMap.Namespace && obj.Name == configMap.Name
			})), "The fake ConfigMap was not listed.")
		})

		It("[test_id:TODO] check Pipelines annotations", func() {
			objList := &pipeline.PipelineList{}
			Expect(apiClient.List(ctx, objList, matchingLabels)).To(Succeed())

			for _, obj := range objList.Items {
				Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecatedAnnotation, "true"), fmt.Sprintf("Pipeline %s/%s does not have deprecation annotation.", obj.Namespace, obj.Name))
			}
			Expect(objList.Items).To(ContainElement(Satisfy(func(obj pipeline.Pipeline) bool { //nolint:staticcheck
				return obj.Namespace == fakePipeline.Namespace && obj.Name == fakePipeline.Name
			})), "The fake Pipeline was not listed.")
		})
	})
})
