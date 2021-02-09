package metrics

import (
	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete

type metrics struct{}

func (m *metrics) Name() string {
	return operandName
}

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

const (
	operandName      = "metrics"
	operandComponent = common.AppComponentMonitoring
)

func reconcilePrometheusRule(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newPrometheusRule(request.Namespace)),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*promv1.PrometheusRule).Spec = newRes.(*promv1.PrometheusRule).Spec
		})
}
