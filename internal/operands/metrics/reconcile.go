package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules;servicemonitors,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=pods;endpoints,verbs=get;list;watch

const prometheusRulesCrd = "prometheusrules.monitoring.coreos.com"

func init() {
	utilruntime.Must(promv1.AddToScheme(common.Scheme))
}

func WatchTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &promv1.PrometheusRule{}},
		{Object: &promv1.ServiceMonitor{}},
	}
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{{
		Object:             &rbac.ClusterRole{},
		Crd:                prometheusRulesCrd,
		WatchOnlyWithLabel: true,
	}, {
		Object:             &rbac.ClusterRoleBinding{},
		Crd:                prometheusRulesCrd,
		WatchOnlyWithLabel: true,
	}}
}

type metrics struct{}

func (m *metrics) Name() string {
	return operandName
}

func (m *metrics) WatchTypes() []operands.WatchType {
	return WatchTypes()
}

func (m *metrics) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (m *metrics) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	return common.CollectResourceStatus(request,
		reconcilePrometheusMonitor,
		reconcilePrometheusRule,
		reconcileMonitoringRbacRole,
		reconcileMonitoringRbacRoleBinding,
	)
}

func (m *metrics) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	return common.DeleteAll(request,
		newMonitoringClusterRole(),
		newMonitoringClusterRoleBinding(),
	)
}

var _ operands.Operand = &metrics{}

func New() operands.Operand {
	return &metrics{}
}

const (
	operandName      = "metrics"
	operandComponent = common.AppComponentMonitoring
)

func reconcilePrometheusMonitor(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newServiceMonitorCR(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileMonitoringRbacRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newMonitoringClusterRole()).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileMonitoringRbacRoleBinding(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newMonitoringClusterRoleBinding()).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcilePrometheusRule(request *common.Request) (common.ReconcileResult, error) {
	if err := rules.SetupRules(); err != nil {
		return common.ReconcileResult{}, err
	}

	prometheusRule, err := rules.BuildPrometheusRule(request.Namespace)
	if err != nil {
		return common.ReconcileResult{}, err
	}

	return common.CreateOrUpdate(request).
		NamespacedResource(prometheusRule).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}
