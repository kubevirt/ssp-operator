package vm_console_proxy

import (
	"fmt"
	"strconv"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EnableAnnotation                  = "ssp.kubevirt.io/vm-console-proxy-enabled"
	VmConsoleProxyNamespaceAnnotation = "ssp.kubevirt.io/vm-console-proxy-namespace"

	operandName      = "vm-console-proxy"
	operandComponent = "vm-console-proxy"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=services;serviceaccounts;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.ClusterRoleBinding{}},
		{Object: &core.ServiceAccount{}},
		{Object: &core.Service{}},
		{Object: &apps.Deployment{}, WatchFullObject: true},
		{Object: &core.ConfigMap{}},
	}
}

type vmConsoleProxy struct {
	serviceAccount     *core.ServiceAccount
	clusterRole        *rbac.ClusterRole
	clusterRoleBinding *rbac.ClusterRoleBinding
	service            *core.Service
	deployment         *apps.Deployment
	configMap          *core.ConfigMap
}

var _ operands.Operand = &vmConsoleProxy{}

func New(bundle *vm_console_proxy_bundle.Bundle) *vmConsoleProxy {
	return &vmConsoleProxy{
		serviceAccount:     &bundle.ServiceAccount,
		clusterRole:        &bundle.ClusterRole,
		clusterRoleBinding: &bundle.ClusterRoleBinding,
		service:            &bundle.Service,
		deployment:         &bundle.Deployment,
		configMap:          &bundle.ConfigMap,
	}
}

func (v *vmConsoleProxy) Name() string {
	return operandName
}

func (v *vmConsoleProxy) WatchTypes() []operands.WatchType {
	return nil
}

func (v *vmConsoleProxy) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (v *vmConsoleProxy) RequiredCrds() []string {
	return nil
}

func (v *vmConsoleProxy) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	var results []common.ReconcileResult
	var reconcileFunc []common.ReconcileFunc

	if !isEnabled(request) {
		_, err := v.Cleanup(request)
		if err != nil {
			return []common.ReconcileResult{}, err
		}

		return []common.ReconcileResult{}, nil
	}

	reconcileFunc = append(reconcileFunc, reconcileServiceAccountsFuncs(*v.serviceAccount.DeepCopy()))
	reconcileFunc = append(reconcileFunc, reconcileClusterRoleFuncs(*v.clusterRole.DeepCopy()))
	reconcileFunc = append(reconcileFunc, reconcileClusterRoleBindingFuncs(*v.clusterRoleBinding.DeepCopy()))
	reconcileFunc = append(reconcileFunc, reconcileConfigMapFuncs(*v.configMap.DeepCopy()))
	reconcileFunc = append(reconcileFunc, reconcileServiceFuncs(*v.service.DeepCopy()))
	reconcileFunc = append(reconcileFunc, reconcileDeploymentFuncs(*v.deployment.DeepCopy()))

	reconcileBundleResults, err := common.CollectResourceStatus(request, reconcileFunc...)
	if err != nil {
		return nil, err
	}

	return append(results, reconcileBundleResults...), nil
}

func (v *vmConsoleProxy) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object

	objects = append(objects, v.serviceAccount.DeepCopy())
	objects = append(objects, v.clusterRole.DeepCopy())
	objects = append(objects, v.clusterRoleBinding.DeepCopy())
	objects = append(objects, v.configMap.DeepCopy())
	objects = append(objects, v.service.DeepCopy())
	objects = append(objects, v.deployment.DeepCopy())

	return common.DeleteAll(request, objects...)
}

func reconcileServiceAccountsFuncs(serviceAccount core.ServiceAccount) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		serviceAccount.Namespace = getVmConsoleProxyNamespace(request)
		return common.CreateOrUpdate(request).
			ClusterResource(&serviceAccount).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileClusterRoleFuncs(clusterRole rbac.ClusterRole) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		return common.CreateOrUpdate(request).
			ClusterResource(&clusterRole).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				newTask := newRes.(*rbac.ClusterRole)
				foundTask := foundRes.(*rbac.ClusterRole)
				foundTask.Rules = newTask.Rules
			}).
			Reconcile()
	}
}

func reconcileClusterRoleBindingFuncs(clusterRoleBinding rbac.ClusterRoleBinding) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		return common.CreateOrUpdate(request).
			ClusterResource(&clusterRoleBinding).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				newBinding := newRes.(*rbac.ClusterRoleBinding)
				foundBinding := foundRes.(*rbac.ClusterRoleBinding)
				foundBinding.RoleRef = newBinding.RoleRef
				foundBinding.Subjects = newBinding.Subjects
			}).
			Reconcile()
	}
}

func reconcileConfigMapFuncs(configMap core.ConfigMap) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		configMap.Namespace = getVmConsoleProxyNamespace(request)
		return common.CreateOrUpdate(request).
			ClusterResource(&configMap).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				newConfigMap := newRes.(*core.ConfigMap)
				foundConfigMap := foundRes.(*core.ConfigMap)
				foundConfigMap.Immutable = newConfigMap.Immutable
				foundConfigMap.Data = newConfigMap.Data
				foundConfigMap.BinaryData = newConfigMap.BinaryData
			}).
			Reconcile()
	}
}

func reconcileServiceFuncs(service core.Service) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		service.Namespace = getVmConsoleProxyNamespace(request)
		return common.CreateOrUpdate(request).
			ClusterResource(&service).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				newService := newRes.(*core.Service)
				foundService := foundRes.(*core.Service)
				// ClusterIP should not be updated
				newService.Spec.ClusterIP = foundService.Spec.ClusterIP
				foundService.Spec = newService.Spec
			}).
			Reconcile()
	}
}

func reconcileDeploymentFuncs(deployment apps.Deployment) common.ReconcileFunc {
	numberOfReplicas := *deployment.Spec.Replicas
	return func(request *common.Request) (common.ReconcileResult, error) {
		deployment.Namespace = getVmConsoleProxyNamespace(request)
		deployment.Spec.Template.Spec.Containers[0].Image = getVmConsoleProxyImage()
		return common.CreateOrUpdate(request).
			ClusterResource(&deployment).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				foundRes.(*apps.Deployment).Spec = newRes.(*apps.Deployment).Spec
			}).
			StatusFunc(func(res client.Object) common.ResourceStatus {
				dep := res.(*apps.Deployment)
				status := common.ResourceStatus{}
				if numberOfReplicas > 0 && dep.Status.AvailableReplicas == 0 {
					msg := fmt.Sprintf("No vm-console-proxy pods are running. Expected: %d", dep.Status.Replicas)
					status.NotAvailable = &msg
				}
				if dep.Status.AvailableReplicas != numberOfReplicas {
					msg := fmt.Sprintf(
						"Not all template vm-console-proxy pods are running. Expected: %d, running: %d",
						numberOfReplicas,
						dep.Status.AvailableReplicas,
					)
					status.Progressing = &msg
					status.Degraded = &msg
				}
				return status
			}).
			Reconcile()
	}
}

func isEnabled(request *common.Request) bool {
	if request.Instance.GetAnnotations() == nil {
		return false
	}
	enabledStr, ok := request.Instance.GetAnnotations()[EnableAnnotation]
	if !ok {
		return false
	}
	isEnabled, err := strconv.ParseBool(enabledStr)
	if err != nil {
		return false
	}
	return isEnabled
}

func getVmConsoleProxyNamespace(request *common.Request) string {
	const defaultNamespace = "kubevirt"
	if request.Instance.GetAnnotations() == nil {
		return defaultNamespace
	}
	namespaceStr, ok := request.Instance.GetAnnotations()[VmConsoleProxyNamespaceAnnotation]
	if !ok {
		return defaultNamespace
	}
	return namespaceStr
}

func getVmConsoleProxyImage() string {
	return common.EnvOrDefault("VM_CONSOLE_PROXY_IMAGE", "quay.io/kubevirt/vm-console-proxy:v0.1.0")
}
