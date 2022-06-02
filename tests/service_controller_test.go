package tests

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubevirt.io/ssp-operator/controllers"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getSspMetricsService() (*v1.Service, error) {
	service := controllers.ServiceObject(strategy.GetSSPDeploymentNameSpace())
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

	It("[test_id: 8807] Should create ssp-operator-metrics service", func() {
		_, serviceErr := getSspMetricsService()
		Expect(serviceErr).ToNot(HaveOccurred(), "Failed to get ssp-operator-metrics service")
	})

	It("[test_id: 8808] Should re-create ssp-operator-metrics service if deleted", func() {
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
		}, shortTimeout, time.Second).ShouldNot(Equal(oldUID), fmt.Sprintf("Did not recreate the %s service", controllers.MetricsServiceName))
	})

	It("[test_id: 8810] Should restore ssp-operator-metrics service after update", func() {
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
		}, shortTimeout, time.Second).Should(Succeed())

		Eventually(func() bool {
			Expect(apiClient.Get(ctx, client.ObjectKeyFromObject(changed), changed)).ToNot(HaveOccurred())
			return equalService(service, changed)
		}, shortTimeout, time.Second).Should(BeTrue())
	})
})
