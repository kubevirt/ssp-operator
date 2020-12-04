package node_labeller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	secv1 "github.com/openshift/api/security/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
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
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newClusterRole(), request)
		ExpectResourceExists(newServiceAccount(namespace), request)
		ExpectResourceExists(newClusterRoleBinding(namespace), request)
		ExpectResourceExists(newConfigMap(namespace), request)
		ExpectResourceExists(newDaemonSet(namespace), request)
		ExpectResourceExists(newSecurityContextConstraint(namespace), request)
	})

	It("should remove cluster resources on cleanup", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(newClusterRole(), request)
		ExpectResourceExists(newClusterRoleBinding(namespace), request)
		ExpectResourceExists(newSecurityContextConstraint(namespace), request)

		Expect(operand.Cleanup(&request)).ToNot(HaveOccurred())

		ExpectResourceNotExists(newClusterRole(), request)
		ExpectResourceNotExists(newClusterRoleBinding(namespace), request)
		ExpectResourceNotExists(newSecurityContextConstraint(namespace), request)
	})
})

func TestNodeLabeller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Node Labeller Suite")
}
