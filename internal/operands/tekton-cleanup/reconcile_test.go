package tekton_cleanup

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"

	libhandler "github.com/operator-framework/operator-lib/handler"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/operands"
)

const (
	namespace = "kubevirt"
	sspName   = "test-ssp"

	sspPartOfValue = "ssp-unit-tests"
)

var _ = Describe("tekton-cleanup operand", func() {
	var (
		operand operands.Operand
		request *common.Request
	)

	BeforeEach(func() {
		operand = New()
		request = getMockedRequest()
	})

	It("Name function should return correct name", func() {
		name := operand.Name()
		Expect(name).To(Equal(operandName), "should return correct name")
	})

	Context("with old Pipelines resources in cluster", func() {
		const (
			resourceName = "test-tekton"
		)

		BeforeEach(func() {
			commonPipelinesMeta := metav1.ObjectMeta{
				Namespace: namespace,
				Name:      resourceName,
				Annotations: map[string]string{
					libhandler.NamespacedNameAnnotation: types.NamespacedName{
						Namespace: request.Instance.Namespace,
						Name:      request.Instance.Name,
					}.String(),
					libhandler.TypeAnnotation: request.Instance.GroupVersionKind().GroupKind().String(),
				},
				Labels: map[string]string{
					common.AppKubernetesNameLabel:      operandPipelinesName,
					common.AppKubernetesComponentLabel: common.AppComponentTektonPipelines.String(),
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					common.AppKubernetesPartOfLabel:    sspPartOfValue,
				},
			}

			for _, resource := range []client.Object{
				&rbac.ClusterRole{ObjectMeta: commonPipelinesMeta},
				&rbac.RoleBinding{ObjectMeta: commonPipelinesMeta},
				&v1.ServiceAccount{ObjectMeta: commonPipelinesMeta},
				&v1.ConfigMap{ObjectMeta: commonPipelinesMeta},
				&pipeline.Pipeline{ObjectMeta: commonPipelinesMeta}, //nolint:staticcheck
			} {
				Expect(request.Client.Create(request.Context, resource)).To(Succeed())
			}
		})

		DescribeTable("should add deprecated annotation", func(obj client.Object) {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(request.Client.Get(request.Context, client.ObjectKey{Namespace: namespace, Name: resourceName}, obj)).To(Succeed())

			Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecated, "true"))
		},
			Entry("ClusterRoles", &rbac.ClusterRole{}),
			Entry("RoleBindings", &rbac.RoleBinding{}),
			Entry("ServiceAccounts", &v1.ServiceAccount{}),
			Entry("ConfigMaps", &v1.ConfigMap{}),
			Entry("Pipelines", &pipeline.Pipeline{}), //nolint:staticcheck
		)

		DescribeTable("should delete resource on Cleanup", func(obj client.Object) {
			_, err := operand.Cleanup(request)
			Expect(err).ToNot(HaveOccurred())

			err = request.Client.Get(request.Context, client.ObjectKey{Namespace: namespace, Name: resourceName}, obj)
			Expect(err).To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
		},
			Entry("ClusterRoles", &rbac.ClusterRole{}),
			Entry("RoleBindings", &rbac.RoleBinding{}),
			Entry("ServiceAccounts", &v1.ServiceAccount{}),
			Entry("ConfigMaps", &v1.ConfigMap{}),
			Entry("Pipelines", &pipeline.Pipeline{}), //nolint:staticcheck
		)
	})

	Context("with old Tasks resources in cluster", func() {
		const (
			resourceName = "test-tekton"
		)

		BeforeEach(func() {
			commonObjectMeta := metav1.ObjectMeta{
				Namespace: namespace,
				Name:      resourceName,
				Annotations: map[string]string{
					libhandler.NamespacedNameAnnotation: types.NamespacedName{
						Namespace: request.Instance.Namespace,
						Name:      request.Instance.Name,
					}.String(),
					libhandler.TypeAnnotation: request.Instance.GroupVersionKind().GroupKind().String(),
				},
				Labels: map[string]string{
					common.AppKubernetesNameLabel:      operandTasksName,
					common.AppKubernetesComponentLabel: common.AppComponentTektonTasks.String(),
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					common.AppKubernetesPartOfLabel:    sspPartOfValue,
				},
			}

			for _, resource := range []client.Object{
				&rbac.ClusterRole{ObjectMeta: commonObjectMeta},
				&rbac.RoleBinding{ObjectMeta: commonObjectMeta},
				&v1.ServiceAccount{ObjectMeta: commonObjectMeta},
				&pipeline.Task{ObjectMeta: commonObjectMeta}, //nolint:staticcheck
			} {
				Expect(request.Client.Create(request.Context, resource)).To(Succeed())
			}
		})

		DescribeTable("should add deprecated annotation", func(obj client.Object) {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(request.Client.Get(request.Context, client.ObjectKey{Namespace: namespace, Name: resourceName}, obj)).To(Succeed())

			Expect(obj.GetAnnotations()).To(HaveKeyWithValue(tektonDeprecated, "true"))
		},
			Entry("ClusterRoles", &rbac.ClusterRole{}),
			Entry("RoleBindings", &rbac.RoleBinding{}),
			Entry("ServiceAccounts", &v1.ServiceAccount{}),
			Entry("Tasks", &pipeline.Task{}), //nolint:staticcheck
		)

		DescribeTable("should delete resource if feature gate is disabled", func(obj client.Object) {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = false

			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			err = request.Client.Get(request.Context, client.ObjectKey{Namespace: namespace, Name: resourceName}, obj)
			Expect(err).To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
		},
			Entry("ClusterRoles", &rbac.ClusterRole{}),
			Entry("RoleBindings", &rbac.RoleBinding{}),
			Entry("ServiceAccounts", &v1.ServiceAccount{}),
			Entry("Tasks", &pipeline.Task{}), //nolint:staticcheck
		)

		DescribeTable("should delete resource on Cleanup", func(obj client.Object) {
			_, err := operand.Cleanup(request)
			Expect(err).ToNot(HaveOccurred())

			err = request.Client.Get(request.Context, client.ObjectKey{Namespace: namespace, Name: resourceName}, obj)
			Expect(err).To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
		},
			Entry("ClusterRoles", &rbac.ClusterRole{}),
			Entry("RoleBindings", &rbac.RoleBinding{}),
			Entry("ServiceAccounts", &v1.ServiceAccount{}),
			Entry("Tasks", &pipeline.Task{}), //nolint:staticcheck
		)
	})
})

func TestTektonCleanup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tekton Cleanup Suite")
}

func getMockedRequest() *common.Request {
	log := logf.Log.WithName("tekton-pipelines-operand")

	Expect(internalmeta.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(extv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(common.AddConversionFunctions(scheme.Scheme)).To(Succeed())
	Expect(pipeline.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(ssp.AddToScheme(scheme.Scheme)).To(Succeed())

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	tektonCrdObj := &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: extv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: tektonCrd,
		},
	}
	Expect(client.Create(context.Background(), tektonCrdObj)).To(Succeed())

	crdWatch := crd_watch.New(nil, tektonCrd)
	Expect(crdWatch.Init(context.Background(), client)).To(Succeed())

	return &common.Request{
		Request: reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      sspName,
			},
		},
		Client:  client,
		Context: context.Background(),
		Instance: &ssp.SSP{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ssp.GroupVersion.String(),
				Kind:       "SSP",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sspName,
				Namespace: namespace,
				Labels: map[string]string{
					common.AppKubernetesPartOfLabel: sspPartOfValue,
				},
			},
			Spec: ssp.SSPSpec{
				FeatureGates: &ssp.FeatureGates{
					DeployTektonTaskResources: true,
				},
			},
		},
		Logger:       log,
		VersionCache: common.VersionCache{},
		CrdList:      crdWatch,
	}
}
