package node_labeller

import (
	"context"
	secv1 "github.com/openshift/api/security/v1"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1alpha1"
	"kubevirt.io/ssp-operator/internal/common"
)

var log = logf.Log.WithName("node_labeller_operand")

var _ = Describe("Node Labeller operand", func() {
	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	var (
		request common.Request
		operand = GetOperand()
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(secv1.Install(s)).ToNot(HaveOccurred())

		client := fake.NewFakeClientWithScheme(s)
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Scheme:  s,
			Context: context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: meta.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: meta.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger: log,
		}
	})

	It("should create node labeller resources", func() {
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())

		expectResourceExists(newClusterRole(), request)
		expectResourceExists(newServiceAccount(namespace), request)
		expectResourceExists(newClusterRoleBinding(namespace), request)
		expectResourceExists(newConfigMap(namespace), request)
		expectResourceExists(newDaemonSet(namespace), request)
		expectResourceExists(newSecurityContextConstraint(namespace), request)
	})

	It("should remove cluster resources on cleanup", func() {
		Expect(operand.Reconcile(&request)).ToNot(HaveOccurred())

		expectResourceExists(newClusterRole(), request)
		expectResourceExists(newClusterRoleBinding(namespace), request)
		expectResourceExists(newSecurityContextConstraint(namespace), request)

		Expect(operand.Cleanup(&request)).ToNot(HaveOccurred())

		expectResourceNotExists(newClusterRole(), request)
		expectResourceNotExists(newClusterRoleBinding(namespace), request)
		expectResourceNotExists(newSecurityContextConstraint(namespace), request)
	})
})

func expectResourceExists(resource controllerutil.Object, request common.Request) {
	key, err := client.ObjectKeyFromObject(resource)
	Expect(err).ToNot(HaveOccurred())
	Expect(request.Client.Get(request.Context, key, resource)).ToNot(HaveOccurred())
}

func expectResourceNotExists(resource controllerutil.Object, request common.Request) {
	key, err := client.ObjectKeyFromObject(resource)
	Expect(err).ToNot(HaveOccurred())

	err = request.Client.Get(request.Context, key, resource)
	Expect(err).To(HaveOccurred())
	Expect(errors.IsNotFound(err)).To(BeTrue())
}

func TestNodeLabeller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Node Labeller Suite")
}
