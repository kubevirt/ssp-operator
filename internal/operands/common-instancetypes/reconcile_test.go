package common_instancetypes

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var log = logf.Log.WithName("common-instancetypes-operand")

func TestInstancetypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Instance types Suite")
}

var _ = Describe("Common-Instancetypes operand", func() {

	var (
		err     error
		operand operands.Operand
		request common.Request
	)

	const (
		namespace        = "kubevirt"
		name             = "test-ssp"
		instancetypePath = "../../../" + BundleDir + ClusterInstancetypesBundlePrefix + ".yaml"
		preferencePath   = "../../../" + BundleDir + ClusterPreferencesBundlePrefix + ".yaml"
	)

	BeforeEach(func() {
		operand, err = New(instancetypePath, preferencePath)
		Expect(err).ToNot(HaveOccurred())

		client := fake.NewFakeClientWithScheme(common.Scheme)
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
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger:       log,
			VersionCache: common.VersionCache{},
		}
	})

	It("should create common-instancetypes resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		virtualMachineClusterInstancetypes, err := fetchClusterResources[instancetypev1alpha2.VirtualMachineClusterInstancetype](instancetypePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterInstancetypes).ToNot(BeEmpty())

		virtualMachineClusterPreferences, err := fetchClusterResources[instancetypev1alpha2.VirtualMachineClusterPreference](preferencePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterPreferences).ToNot(BeEmpty())

		for _, instancetype := range virtualMachineClusterInstancetypes {
			ExpectResourceExists(&instancetype, request)
		}

		for _, preference := range virtualMachineClusterPreferences {
			ExpectResourceExists(&preference, request)
		}
	})
})
