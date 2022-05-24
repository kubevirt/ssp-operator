package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules;servicemonitors;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io/v1,resources=role;rolebinding;serviceaccount,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;endpoints,verbs=get;list;watch

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
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.ClusterRoleBinding{}},
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
	return WatchClusterTypes()
}

func (m *metrics) RequiredCrds() []string {
	return []string{"prometheusrules.monitoring.coreos.com"}
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
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*promv1.ServiceMonitor).Spec = newRes.(*promv1.ServiceMonitor).Spec
		}).
		Reconcile()
}

func reconcileMonitoringRbacRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newMonitoringClusterRole()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*rbac.ClusterRole).Rules = newRes.(*rbac.ClusterRole).Rules
		}).
		Reconcile()
}

func reconcileMonitoringRbacRoleBinding(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newMonitoringClusterRoleBinding()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*rbac.ClusterRoleBinding).Subjects = newRes.(*rbac.ClusterRoleBinding).Subjects
			foundRes.(*rbac.ClusterRoleBinding).RoleRef = newRes.(*rbac.ClusterRoleBinding).RoleRef
		}).
		Reconcile()
}

func reconcilePrometheusRule(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newPrometheusRule(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*promv1.PrometheusRule).Spec = newRes.(*promv1.PrometheusRule).Spec
		}).
		Reconcile()
}
