package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete

func init() {
	utilruntime.Must(promv1.AddToScheme(common.Scheme))
}

func WatchTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &promv1.PrometheusRule{}},
	}
}

type metrics struct{}

func (m *metrics) Name() string {
	return operandName
}

func (m *metrics) WatchTypes() []operands.WatchType {
	return WatchTypes()
}

func (m *metrics) WatchClusterTypes() []operands.WatchType {
	return nil
}

func (m *metrics) RequiredCrds() []string {
	return []string{"prometheusrules.monitoring.coreos.com"}
}

func (m *metrics) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	return common.CollectResourceStatus(request,
		reconcilePrometheusRule,
	)
}

func (m *metrics) Cleanup(*common.Request) ([]common.CleanupResult, error) {
	return nil, nil
}

var _ operands.Operand = &metrics{}

func New() operands.Operand {
	return &metrics{}
}

const (
	operandName      = "metrics"
	operandComponent = common.AppComponentMonitoring
)

func reconcilePrometheusRule(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newPrometheusRule(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*promv1.PrometheusRule).Spec = newRes.(*promv1.PrometheusRule).Spec
		}).
		Reconcile()
}
