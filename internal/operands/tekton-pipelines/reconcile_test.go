package tekton_pipelines

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/operands"
	tektonbundle "kubevirt.io/ssp-operator/internal/tekton-bundle"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace              = "kubevirt"
	name                   = "test-tekton"
	testNamespace          = "test-namespace"
	testDifferentNamespace = "different-namespace"
)

var _ = Describe("environments", func() {
	var (
		operand operands.Operand
		bundle  *tektonbundle.Bundle
		request *common.Request
	)

	BeforeEach(func() {
		bundle = getMockedTestBundle()
		operand = New(bundle)
		request = getMockedRequest()
	})

	It("Name function should return correct name", func() {
		name := operand.Name()
		Expect(name).To(Equal(operandName), "should return correct name")
	})

	Context("With feature gate enabled", func() {
		BeforeEach(func() {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = true
		})

		It("Reconcile function should return correct functions", func() {
			functions, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred(), "should not throw err")
			Expect(functions).To(HaveLen(6), "should return correct number of reconcile functions")
		})

		It("Should create tekton-pipelines resources", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceExists(&clusterRole, *request)
			}

			for _, pipeline := range bundle.ClusterRoles {
				ExpectResourceExists(&pipeline, *request)
			}

			for _, configMap := range bundle.ConfigMaps {
				ExpectResourceExists(&configMap, *request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceExists(&roleBinding, *request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceExists(&serviceAccount, *request)
			}
		})

		It("should remove tekton-pipelines resources on cleanup", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceExists(&clusterRole, *request)
			}

			for _, pipeline := range bundle.ClusterRoles {
				ExpectResourceExists(&pipeline, *request)
			}

			for _, configMap := range bundle.ConfigMaps {
				ExpectResourceExists(&configMap, *request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceExists(&roleBinding, *request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceExists(&serviceAccount, *request)
			}

			_, err = operand.Cleanup(request)
			Expect(err).ToNot(HaveOccurred())

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceNotExists(&clusterRole, *request)
			}

			for _, pipeline := range bundle.ClusterRoles {
				ExpectResourceNotExists(&pipeline, *request)
			}

			for _, configMap := range bundle.ConfigMaps {
				ExpectResourceNotExists(&configMap, *request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceNotExists(&roleBinding, *request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceNotExists(&serviceAccount, *request)
			}
		})
	})

	Context("With feature gate disabled", func() {
		BeforeEach(func() {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = false
		})

		It("Should not create tekton-pipelines resources", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceNotExists(&clusterRole, *request)
			}

			for _, pipeline := range bundle.ClusterRoles {
				ExpectResourceNotExists(&pipeline, *request)
			}

			for _, configMap := range bundle.ConfigMaps {
				ExpectResourceNotExists(&configMap, *request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceNotExists(&roleBinding, *request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceNotExists(&serviceAccount, *request)
			}
		})
	})

	Context("With user defined namespace in ssp CR for pipelines", func() {
		BeforeEach(func() {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = true
			request.Instance.Spec.TektonPipelines = &ssp.TektonPipelines{
				Namespace: testNamespace,
			}
		})

		It("kubevirt.io/tekton-piplines-deploy-namespace annotation in configMaps should be replaced by user defined namespace", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, configMap := range bundle.ConfigMaps {
				configMap.Namespace = testNamespace
				key := client.ObjectKeyFromObject(&configMap)
				cm := &v1.ConfigMap{}
				ExpectWithOffset(1, request.Client.Get(request.Context, key, cm)).ToNot(HaveOccurred())
				Expect(cm.Namespace).To(Equal(testNamespace), "configMap namespace should equal")
			}
		})

		It("kubevirt.io/tekton-piplines-deploy-namespace annotation in roleBindings should be replaced by user defined namespace", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, roleBinding := range bundle.RoleBindings {
				roleBinding.Namespace = testNamespace
				key := client.ObjectKeyFromObject(&roleBinding)
				rb := &rbac.RoleBinding{}
				ExpectWithOffset(1, request.Client.Get(request.Context, key, rb)).ToNot(HaveOccurred())
				Expect(rb.Namespace).To(Equal(testNamespace), rb.Name+" roleBinding namespace should equal")
			}
		})
	})

	Context("With kubevirt namespace in ssp CR for pipelines", func() {
		BeforeEach(func() {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = true
			request.Instance.Spec.TektonPipelines = &ssp.TektonPipelines{
				Namespace: kubevirtNamespace,
			}
		})

		It("kubevirt.io/tekton-piplines-deploy-namespace annotation in configMaps should not be replaced", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, configMap := range bundle.ConfigMaps {
				expectedNamespace := namespace
				if annotationNamespace, ok := configMap.Annotations[deployNamespaceAnnotation]; ok {
					expectedNamespace = annotationNamespace
				}

				key := client.ObjectKeyFromObject(&configMap)
				cm := &v1.ConfigMap{}
				Expect(request.Client.Get(request.Context, key, cm)).ToNot(HaveOccurred())
				Expect(cm.Namespace).To(Equal(expectedNamespace), cm.Name+" configMap namespace should equal")
			}
		})

		It("kubevirt.io/tekton-piplines-deploy-namespace annotation in roleBindings should not be replaced", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, roleBinding := range bundle.RoleBindings {
				expectedNamespace := namespace
				if annotationNamespace, ok := roleBinding.Annotations[deployNamespaceAnnotation]; ok {
					expectedNamespace = annotationNamespace
				}

				key := client.ObjectKeyFromObject(&roleBinding)
				rb := &rbac.RoleBinding{}
				Expect(request.Client.Get(request.Context, key, rb)).ToNot(HaveOccurred())
				Expect(rb.Namespace).To(Equal(expectedNamespace), rb.Name+" roleBinding namespace should equal")
			}
		})
	})

	Context("Without user defined namespace in ssp CR for pipelines", func() {
		BeforeEach(func() {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = true
			request.Instance.Spec.TektonPipelines = nil
		})

		It("kubevirt.io/tekton-piplines-deploy-namespace annotation in configMaps should replace default namespace", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, configMap := range bundle.ConfigMaps {
				objNamespace := namespace

				if configMap.Name == "test-cm" {
					configMap.Namespace = testDifferentNamespace
					objNamespace = testDifferentNamespace
				}

				key := client.ObjectKeyFromObject(&configMap)
				cm := &v1.ConfigMap{}
				ExpectWithOffset(1, request.Client.Get(request.Context, key, cm)).ToNot(HaveOccurred())
				Expect(cm.Namespace).To(Equal(objNamespace), cm.Name+" configMap namespace should equal")
			}
		})

		It("kubevirt.io/tekton-piplines-deploy-namespace annotation in roleBindings should replace default namespace", func() {
			_, err := operand.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			for _, roleBinding := range bundle.RoleBindings {
				objNamespace := namespace

				if roleBinding.Name == "test-rb" {
					roleBinding.Namespace = testDifferentNamespace
					objNamespace = testDifferentNamespace
				}

				key := client.ObjectKeyFromObject(&roleBinding)
				rb := &rbac.RoleBinding{}
				ExpectWithOffset(1, request.Client.Get(request.Context, key, rb)).ToNot(HaveOccurred())
				Expect(rb.Namespace).To(Equal(objNamespace), rb.Name+" roleBinding namespace should equal")
			}
		})
	})
})

func TestTektonPipelines(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tekton Pipelines Suite")
}

func getMockedRequest() *common.Request {
	log := logf.Log.WithName("tekton-pipelines-operand")

	Expect(internalmeta.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(apiextensions.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(common.AddConversionFunctions(scheme.Scheme)).To(Succeed())
	Expect(pipeline.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(ssp.AddToScheme(scheme.Scheme)).To(Succeed())

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	tektonCrdObj := &apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextensions.SchemeGroupVersion.String(),
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
				Name:      name,
			},
		},
		Client:  client,
		Context: context.Background(),
		Instance: &ssp.SSP{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TektonPipelines",
				APIVersion: ssp.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: ssp.SSPSpec{
				FeatureGates: &ssp.FeatureGates{
					DeployTektonTaskResources: false,
				},
				TektonPipelines: &ssp.TektonPipelines{
					Namespace: namespace,
				},
			},
		},
		Logger:       log,
		VersionCache: common.VersionCache{},
		CrdList:      crdWatch,
	}
}

func getMockedTestBundle() *tektonbundle.Bundle {
	return &tektonbundle.Bundle{
		Pipelines: []pipeline.Pipeline{ //nolint:staticcheck
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pipeline",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pipeline2",
				},
			},
		},
		ConfigMaps: []v1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cm",
					Annotations: map[string]string{
						deployNamespaceAnnotation: testDifferentNamespace,
					},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cm2",
				},
			},
		},
		RoleBindings: []rbac.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb",
					Annotations: map[string]string{
						deployNamespaceAnnotation: testDifferentNamespace,
					},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb2",
				},
			},
		},
	}
}
