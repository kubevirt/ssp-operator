package tekton_pipelines

import (
	"context"
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
	var tp *tektonPipelines
	var mockedRequest *common.Request
	BeforeEach(func() {
		tp = getMockedTektonPipelinesOperand()
		mockedRequest = getMockedRequest()
	})

	It("New function should return object", func() {
		res := New(getMockedTestBundle())
		Expect(len(res.pipelines)).To(Equal(2), "should return correct number of pipelines")
		Expect(len(res.configMaps)).To(Equal(2), "should return correct number of config maps")
		Expect(len(res.roleBindings)).To(Equal(2), "should return correct number of rolebindings")
	})

	It("Name function should return correct name", func() {
		name := tp.Name()
		Expect(name).To(Equal(operandName), "should return correct name")
	})

	It("Reconcile function should return correct functions", func() {
		functions, err := tp.Reconcile(mockedRequest)
		Expect(err).ToNot(HaveOccurred(), "should not throw err")
		Expect(len(functions)).To(Equal(6), "should return correct number of reconcile functions")
	})

	It("RequiredCrds function should return required crds", func() {
		tp := getMockedTektonPipelinesOperand()
		crds := tp.RequiredCrds()

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

func TestTektonPipelines(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "tekton pipelines Suite")
}

func getMockedRequest() *common.Request {
	log := logf.Log.WithName("tekton-pipelines-operand")
	client := fake.NewFakeClientWithScheme(common.Scheme)
	return &common.Request{
		Request: reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
		},
		Client:  client,
		Context: context.Background(),
		Instance: &tekton.TektonTasks{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TektonTasks",
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

func getMockedTektonPipelinesOperand() *tektonPipelines {
	return &tektonPipelines{
		pipelines: []pipeline.Pipeline{
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
		configMaps: []v1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cm",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cm2",
				},
			},
		},
		roleBindings: []rbac.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb",
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb2",
				},
			},
		}}
}

func getMockedTestBundle() *tektonbundle.Bundle {
	return &tektonbundle.Bundle{
		Pipelines: []pipeline.Pipeline{
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
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb2",
				},
			},
		},
	}
}
