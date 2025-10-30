package resources

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubevirt.io/ssp-operator/internal"
	"kubevirt.io/ssp-operator/internal/common"
)

const (
	MetricsServiceName = "ssp-operator-metrics"
	MetricsPortName    = "http-metrics"
)

func PrometheusServiceLabels() map[string]string {
	return common.PrometheusServiceLabels(MetricsServiceName)
}

func SspMetricsService(namespace string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetricsServiceName,
			Namespace: namespace,
			Labels:    PrometheusServiceLabels(),
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       MetricsPortName,
					Port:       443,
					Protocol:   v1.ProtocolTCP,
					TargetPort: intstr.FromString(MetricsPortName),
				},
			},
			Selector: map[string]string{
				"name": internal.SspDeploymentName,
			},
		},
	}
}
