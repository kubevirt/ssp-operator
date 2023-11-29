package tekton_tasks

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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace = "kubevirt"
	name      = "test-tekton"
)

var _ = Describe("environments", func() {
	var (
		bundle  *tektonbundle.Bundle
		operand operands.Operand
		request common.Request
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
			functions, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred(), "should not throw err")
			Expect(functions).To(HaveLen(8), "should return correct number of reconcile functions")
		})

		It("Should create tekton-tasks resources", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for _, task := range bundle.Tasks {
				ExpectResourceExists(&task, request)
			}

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceExists(&clusterRole, request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceExists(&serviceAccount, request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceExists(&roleBinding, request)
			}
		})

		It("Should remove tekton-tasks resources on cleanup", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for _, task := range bundle.Tasks {
				ExpectResourceExists(&task, request)
			}

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceExists(&clusterRole, request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceExists(&serviceAccount, request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceExists(&roleBinding, request)
			}

			_, err = operand.Cleanup(&request)
			Expect(err).ToNot(HaveOccurred())

			for _, task := range bundle.Tasks {
				ExpectResourceNotExists(&task, request)
			}

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceNotExists(&clusterRole, request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceNotExists(&serviceAccount, request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceNotExists(&roleBinding, request)
			}
		})
	})

	Context("With feature gate disabled", func() {
		BeforeEach(func() {
			request.Instance.Spec.FeatureGates.DeployTektonTaskResources = false
		})

		It("Should not create tekton-tasks resources", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for _, task := range bundle.Tasks {
				ExpectResourceNotExists(&task, request)
			}

			for _, clusterRole := range bundle.ClusterRoles {
				ExpectResourceNotExists(&clusterRole, request)
			}

			for _, serviceAccount := range bundle.ServiceAccounts {
				ExpectResourceNotExists(&serviceAccount, request)
			}

			for _, roleBinding := range bundle.RoleBindings {
				ExpectResourceNotExists(&roleBinding, request)
			}
		})
	})
})

func TestTektonTasks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tekton Tasks Suite")
}

func getMockedRequest() common.Request {
	log := logf.Log.WithName("tekton-tasks-operand")

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

	return common.Request{
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
				Kind:       "TetktonTasks",
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
				TektonTasks: &ssp.TektonTasks{
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
		Tasks: []pipeline.Task{
			{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    map[string]string{},
					Name:      diskVirtSysprepTaskName,
					Namespace: namespace,
				},
				Spec: pipeline.TaskSpec{
					Steps: []pipeline.Step{
						{
							Image: "test",
						},
					},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Labels:    map[string]string{},
					Name:      modifyTemplateTaskName,
					Namespace: namespace,
				},
				Spec: pipeline.TaskSpec{
					Steps: []pipeline.Step{
						{
							Image: "test",
						},
					},
				},
			},
		},
		ServiceAccounts: []v1.ServiceAccount{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      diskVirtSysprepTaskName + "-task",
					Namespace: namespace,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name:      modifyTemplateTaskName + "-task",
					Namespace: namespace,
				},
			},
		},
		RoleBindings: []rbac.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      diskVirtSysprepTaskName + "-task",
					Namespace: namespace,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name:      modifyTemplateTaskName + "-task",
					Namespace: namespace,
				},
			},
		},
		ClusterRoles: []rbac.ClusterRole{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      diskVirtSysprepTaskName + "-task",
					Namespace: namespace,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name:      modifyTemplateTaskName + "-task",
					Namespace: namespace,
				},
			},
		},
	}
}
