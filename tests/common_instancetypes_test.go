package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"

	"github.com/operator-framework/operator-lib/handler"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/tests/env"
)

var _ = Describe("Common Instance Types", func() {
	var (
		commonAnnotations map[string]string
		commonLabels      map[string]string
	)

	BeforeEach(func() {
		commonAnnotations = map[string]string{
			handler.TypeAnnotation: ssp.GroupVersion.WithKind("SSP").GroupKind().String(),
			handler.NamespacedNameAnnotation: types.NamespacedName{
				Namespace: strategy.GetNamespace(),
				Name:      strategy.GetName(),
			}.String(),
		}
		commonLabels = map[string]string{
			common.AppKubernetesNameLabel:      "common-instancetypes",
			common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
			common.AppKubernetesComponentLabel: string(common.AppComponentTemplating),
		}

		waitUntilDeployed()
	})

	It("should remove old instance types", func() {
		instanceType := &instancetypev1beta1.VirtualMachineClusterInstancetype{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-instance-type-",
				Annotations:  commonAnnotations,
				Labels:       commonLabels,
			},
			Spec: instancetypev1beta1.VirtualMachineInstancetypeSpec{
				Memory: instancetypev1beta1.MemoryInstancetype{
					Guest: resource.MustParse("1G"),
				},
			},
		}

		Expect(apiClient.Create(ctx, instanceType)).To(Succeed())
		DeferCleanup(func() {
			Expect(apiClient.Delete(ctx, instanceType)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
		})

		triggerReconciliation()

		// Eventually the instance type should be removed by SSP
		Eventually(func() error {
			return apiClient.Get(ctx, client.ObjectKeyFromObject(instanceType),
				&instancetypev1beta1.VirtualMachineClusterInstancetype{})
		}, env.ShortTimeout(), time.Second).
			Should(MatchError(errors.IsNotFound, "errors.IsNotFound"))
	})

	It("should remove old preferences", func() {
		preference := &instancetypev1beta1.VirtualMachineClusterPreference{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-preference-",
				Annotations:  commonAnnotations,
				Labels:       commonLabels,
			},
			Spec: instancetypev1beta1.VirtualMachinePreferenceSpec{
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: ptr.To(instancetypev1beta1.DeprecatedPreferCores),
				},
			},
		}

		Expect(apiClient.Create(ctx, preference)).To(Succeed())
		DeferCleanup(func() {
			Expect(apiClient.Delete(ctx, preference)).To(Or(Succeed(), MatchError(errors.IsNotFound, "errors.IsNotFound")))
		})

		triggerReconciliation()

		// Eventually the preference should be removed by SSP
		Eventually(func() error {
			return apiClient.Get(ctx, client.ObjectKeyFromObject(preference),
				&instancetypev1beta1.VirtualMachineClusterPreference{})
		}, env.ShortTimeout(), time.Second).
			Should(MatchError(errors.IsNotFound, "errors.IsNotFound"))
	})
})
