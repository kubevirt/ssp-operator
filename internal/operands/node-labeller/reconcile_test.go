package node_labeller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
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
		operand = New()
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(secv1.Install(s)).ToNot(HaveOccurred())

		client := fake.NewClientBuilder().WithScheme(s).Build()
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
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

	It("should delete node-labeller during reconcile", func() {
		_, err := reconcileClusterRole(&request)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconcileServiceAccount(&request)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconcileClusterRoleBinding(&request)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconcileConfigMap(&request)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconcileDaemonSet(&request)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconcileSecurityContextConstraint(&request)
		Expect(err).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceNotExists(newClusterRole(), request)
		ExpectResourceNotExists(newServiceAccount(namespace), request)
		ExpectResourceNotExists(newClusterRoleBinding(namespace), request)
		ExpectResourceNotExists(newConfigMap(namespace), request)
		ExpectResourceNotExists(newDaemonSet(namespace), request)
		ExpectResourceNotExists(newSecurityContextConstraint(namespace), request)
	})
})

func TestNodeLabeller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Node Labeller Suite")
}
