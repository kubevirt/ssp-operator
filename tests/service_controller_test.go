package tests

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/controllers"
	sspenv "kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	"kubevirt.io/ssp-operator/tests/decorators"
	"kubevirt.io/ssp-operator/tests/env"
)

func getSspMetricsService() (*v1.Service, error) {
	service := controllers.ServiceObject(strategy.GetSSPDeploymentNameSpace(), "")
	err := apiClient.Get(ctx, client.ObjectKeyFromObject(service), service)
	return service, err
}

func equalService(serviceA, serviceB *v1.Service) bool {
	return reflect.DeepEqual(serviceA.Labels, serviceB.Labels) && reflect.DeepEqual(serviceA.Spec, serviceB.Spec)
}

var _ = Describe("Service Controller", func() {
	BeforeEach(func() {
		waitUntilDeployed()
	})

	It("[test_id:8807] Should create ssp-operator-metrics service", decorators.Conformance, func() {
		_, serviceErr := getSspMetricsService()
		Expect(serviceErr).ToNot(HaveOccurred(), "Failed to get ssp-operator-metrics service")
	})

	It("[test_id: TODO] Service ssp-operator-metrics should contain required labels", func() {
		service, serviceErr := getSspMetricsService()
		Expect(serviceErr).ToNot(HaveOccurred(), "Failed to get ssp-operator-metrics service")

		deployment := &apps.Deployment{}
		Expect(apiClient.Get(ctx, types.NamespacedName{
			Name:      strategy.GetSSPDeploymentName(),
			Namespace: strategy.GetSSPDeploymentNameSpace(),
		}, deployment)).To(Succeed())

		containers := deployment.Spec.Template.Spec.Containers
		id := slices.IndexFunc(containers, func(container v1.Container) bool {
			return container.Name == "manager"
		})
		Expect(id).ToNot(Equal(-1), "SSP deployment does not contain manager container")

		envId := slices.IndexFunc(containers[id].Env, func(envVar v1.EnvVar) bool {
			return envVar.Name == sspenv.OperatorVersionKey
		})
		Expect(envId).ToNot(Equal(-1), "SSP manager container does not have the operator version variable")

		version := containers[id].Env[envId].Value
		if version != "" {
			Expect(service.GetLabels()).To(HaveKeyWithValue(common.AppKubernetesVersionLabel, version))
		} else {
			// If the version env variable is not defined, then it is not simple to get the version from test code.
			Expect(service.GetLabels()).To(HaveKey(common.AppKubernetesVersionLabel))
		}

		Expect(service.GetLabels()).To(HaveKeyWithValue(common.AppKubernetesManagedByLabel, controllers.ServiceManagedByLabelValue))
		Expect(service.GetLabels()).To(HaveKeyWithValue(common.AppKubernetesComponentLabel, controllers.ServiceControllerName))
		// Not using HaveKeyWithValue, because the label does not need to exist.
		Expect(service.GetLabels()[common.AppKubernetesPartOfLabel]).To(Equal(deployment.Labels[common.AppKubernetesPartOfLabel]))
	})

	It("[test_id:8808] Should re-create ssp-operator-metrics service if deleted", decorators.Conformance, func() {
		service, serviceErr := getSspMetricsService()
		Expect(serviceErr).ToNot(HaveOccurred(), "Failed to get ssp-operator-metrics service")
		oldUID := service.UID
		Expect(apiClient.Delete(ctx, service)).To(Succeed())
		Eventually(func() (types.UID, error) {
			var foundService v1.Service
			err := apiClient.Get(ctx, client.ObjectKeyFromObject(service), &foundService)
			if err != nil {
				return "", err
			}
			return foundService.UID, nil
		}, env.ShortTimeout(), time.Second).ShouldNot(Equal(oldUID), fmt.Sprintf("Did not recreate the %s service", common.SspOperatorMetricsServiceName))
	})

	It("[test_id:8810] Should restore ssp-operator-metrics service after update", decorators.Conformance, func() {
		service, serviceErr := getSspMetricsService()
		Expect(serviceErr).ToNot(HaveOccurred(), "Failed to get ssp-operator-metrics service")
		changed := service.DeepCopy()
		changed.Labels = nil
		changed.Spec.Ports = []v1.ServicePort{
			{
				Name:       metrics.MetricsPortName,
				Port:       755,
				Protocol:   v1.ProtocolTCP,
				TargetPort: intstr.FromString(metrics.MetricsPortName),
			},
		}

		Eventually(func() error {
			return apiClient.Update(ctx, changed)
		}, env.ShortTimeout(), time.Second).Should(Succeed())

		Eventually(func() bool {
			Expect(apiClient.Get(ctx, client.ObjectKeyFromObject(changed), changed)).ToNot(HaveOccurred())
			return equalService(service, changed)
		}, env.ShortTimeout(), time.Second).Should(BeTrue())
	})
})
