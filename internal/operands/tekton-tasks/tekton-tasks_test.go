package tekton_tasks

import (
	"context"
	"strings"
	"testing"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	tektonbundle "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-bundle"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace = "kubevirt"
	name      = "test-tekton"
)

var _ = Describe("environments", func() {
	var tt *tektonTasks
	var mockedRequest *common.Request
	BeforeEach(func() {
		tt = getMockedTektonTasksOperand()
		mockedRequest = getMockedRequest()
	})

	It("New function should return object with correct tasks", func() {
		res := New(getMockedTestBundle())
		Expect(len(res.tasks)).To(Equal(11), "should return correct number of tasks")
		Expect(len(res.serviceAccounts)).To(Equal(11), "should return correct number of service accounts")
		Expect(len(res.roleBindings)).To(Equal(11), "should return correct number of role bindings")
		Expect(len(res.clusterRoles)).To(Equal(11), "should return correct number of cluster roles")
		for _, task := range res.tasks {
			if _, ok := AllowedTasks[task.Name]; !ok {
				Expect(ok).To(BeTrue(), "only allowed task is deployed - "+task.Name)
			}
		}
		for _, sa := range res.serviceAccounts {
			if _, ok := AllowedTasks[strings.TrimSuffix(sa.Name, "-task")]; !ok {
				Expect(ok).To(BeTrue(), "only allowed service accounts is deployed - "+sa.Name)
			}
		}
		for _, rb := range res.roleBindings {
			if _, ok := AllowedTasks[strings.TrimSuffix(rb.Name, "-task")]; !ok {
				Expect(ok).To(BeTrue(), "only allowed role bindings is deployed - "+rb.Name)
			}
		}
		for _, cr := range res.clusterRoles {
			if _, ok := AllowedTasks[strings.TrimSuffix(cr.Name, "-task")]; !ok {
				Expect(ok).To(BeTrue(), "only allowed role bindings is deployed - "+cr.Name)
			}
		}
	})

	It("Name function should return correct name", func() {
		name := tt.Name()
		Expect(name).To(Equal(operandName), "should return correct name")
	})

	It("Reconcile function should return correct functions", func() {
		functions, err := tt.Reconcile(mockedRequest)
		Expect(err).ToNot(HaveOccurred(), "should not throw err")
		Expect(len(functions)).To(Equal(8), "should return correct number of reconcile functions")
	})

	It("RequiredCrds function should return required crds", func() {
		tt := getMockedTektonTasksOperand()
		crds := tt.RequiredCrds()

		Expect(len(crds) > 0).To(BeTrue(), "should return required crds")

		for _, crd := range crds {
			found := false
			for _, c := range requiredCRDs {
				if crd == c {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "should return correct required crd")
		}
	})

})

func TestTektonBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "tekton tasks Suite")
}

func getMockedRequest() *common.Request {
	log := logf.Log.WithName("tekton-tasks-operand")
	clientBuilder := fake.NewClientBuilder().WithScheme(common.Scheme)
	return &common.Request{
		Request: reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
		},
		Client:  clientBuilder.Build(),
		Context: context.Background(),
		Instance: &tekton.TektonTasks{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TetktonTasks",
				APIVersion: tekton.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		Logger:       log,
		VersionCache: common.VersionCache{},
	}
}

func getMockedTektonTasksOperand() *tektonTasks {
	return &tektonTasks{
		tasks: []pipeline.Task{
			{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
					Name:   diskVirtSysprepTaskName,
				},
				Spec: pipeline.TaskSpec{
					Steps: []pipeline.Step{
						{
							Container: v1.Container{
								Name: "test",
							},
						},
					},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
					Name:   modifyTemplateTaskName,
				},
				Spec: pipeline.TaskSpec{
					Steps: []pipeline.Step{
						{
							Container: v1.Container{
								Name: "test",
							},
						},
					},
				},
			},
		},
		serviceAccounts: []v1.ServiceAccount{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName + "-task",
				},
			},
		},
		roleBindings: []rbac.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName + "-task",
				},
			},
		},
		clusterRoles: []rbac.ClusterRole{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName + "-task",
				},
			},
		},
	}
}

func getMockedTestBundle() *tektonbundle.Bundle {
	return &tektonbundle.Bundle{
		Tasks: []pipeline.Task{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-task",
				},
				Spec: pipeline.TaskSpec{
					Steps: []pipeline.Step{
						{
							Container: v1.Container{
								Name: "test",
							},
						},
					},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: cleanVMTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: copyTemplateTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyDataObjectTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: waitForVMITaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtCustomizeTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: generateSSHKeysTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: executeInVMTaskName,
				},
			},
		},
		ServiceAccounts: []v1.ServiceAccount{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: cleanVMTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: copyTemplateTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyDataObjectTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: waitForVMITaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtCustomizeTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: generateSSHKeysTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: executeInVMTaskName + "-task",
				},
			},
		},
		RoleBindings: []rbac.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: cleanVMTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: copyTemplateTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyDataObjectTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: waitForVMITaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtCustomizeTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: generateSSHKeysTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: executeInVMTaskName + "-task",
				},
			},
		},
		ClusterRoles: []rbac.ClusterRole{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: cleanVMTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: copyTemplateTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyDataObjectTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: waitForVMITaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtCustomizeTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: diskVirtSysprepTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: modifyTemplateTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: generateSSHKeysTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: createVMFromManifestTaskName + "-task",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: executeInVMTaskName + "-task",
				},
			},
		},
	}
}
