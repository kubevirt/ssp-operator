package metrics

import (
	"context"
	"fmt"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	kvsspv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=kubevirtmetricsaggregations,verbs=get;list;watch;create;update;patch;delete

type metrics struct{}

func (m *metrics) AddWatchTypesToScheme(scheme *runtime.Scheme) error {
	return promv1.AddToScheme(scheme)
}

func (m *metrics) WatchTypes() []runtime.Object {
	return []runtime.Object{&promv1.PrometheusRule{}}
}

func (m *metrics) WatchClusterTypes() []runtime.Object {
	return nil
}

func (m *metrics) Reconcile(request *common.Request) ([]common.ResourceStatus, error) {
	pauseCRs(request)
	return common.CollectResourceStatus(request,
		reconcilePrometheusRule,
	)
}

func (m *metrics) Cleanup(*common.Request) error {
	return nil
}

var _ operands.Operand = &metrics{}

func GetOperand() operands.Operand {
	return &metrics{}
}

func pauseCRs(request *common.Request) {
	patch := []byte(`{"metadata":{"annotations":{"kubevirt.io/operator.paused": "true"}}}`)
	var kubevirtMetricsAggregations kvsspv1.KubevirtMetricsAggregationList
	err := request.Client.List(context.TODO(), &kubevirtMetricsAggregations, &client.ListOptions{})
	if err != nil {
		request.Logger.Error(err, fmt.Sprintf("Error listing metrics aggregations: %s", err))
		return
	}
	if err == nil && len(kubevirtMetricsAggregations.Items) > 0 {
		for _, kubevirtMetricsAggregation := range kubevirtMetricsAggregations.Items {
			err = request.Client.Patch(context.TODO(), &kvsspv1.KubevirtMetricsAggregation{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kubevirtMetricsAggregation.ObjectMeta.Namespace,
					Name:      kubevirtMetricsAggregation.ObjectMeta.Name,
				},
			}, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				request.Logger.Error(err, fmt.Sprintf("Error pausing %s from namespace %s: %s",
					kubevirtMetricsAggregation.ObjectMeta.Name,
					kubevirtMetricsAggregation.ObjectMeta.Namespace,
					err))
			}
		}
	}
}

func reconcilePrometheusRule(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateResource(request,
		newPrometheusRule(request.Namespace),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*promv1.PrometheusRule).Spec = newRes.(*promv1.PrometheusRule).Spec
		})
}
