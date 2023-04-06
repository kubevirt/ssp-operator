package vm_console_proxy

import (
	"context"
	"fmt"
	"strconv"

	routev1 "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
)

const (
	EnableAnnotation                  = "ssp.kubevirt.io/vm-console-proxy-enabled"
	VmConsoleProxyNamespaceAnnotation = "ssp.kubevirt.io/vm-console-proxy-namespace"

	operandName      = "vm-console-proxy"
	operandComponent = "vm-console-proxy"

	routeName = "vm-console-proxy"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=services;serviceaccounts;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func init() {
	utilruntime.Must(routev1.Install(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.ClusterRoleBinding{}},
		{Object: &core.ServiceAccount{}},
		{Object: &core.Service{}},
		{Object: &apps.Deployment{}, WatchFullObject: true},
		{Object: &core.ConfigMap{}},
		{Object: &routev1.Route{}},
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
		serviceAccount:     bundle.ServiceAccount,
		clusterRole:        bundle.ClusterRole,
		clusterRoleBinding: bundle.ClusterRoleBinding,
		service:            bundle.Service,
		deployment:         bundle.Deployment,
		configMap:          bundle.ConfigMap,
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
	if !isEnabled(request) {
		cleanupResults, err := v.Cleanup(request)
		if err != nil {
			return nil, err
		}
		var results []common.ReconcileResult
		for _, cleanupResult := range cleanupResults {
			if !cleanupResult.Deleted {
				results = append(results, common.ResourceDeletedResult(cleanupResult.Resource, common.OperationResultDeleted))
			}
		}
		return results, nil
	}

	return common.CollectResourceStatus(request,
		reconcileServiceAccount(*v.serviceAccount.DeepCopy()),
		reconcileClusterRole(*v.clusterRole.DeepCopy()),
		reconcileClusterRoleBinding(*v.clusterRoleBinding.DeepCopy()),
		reconcileConfigMap(*v.configMap.DeepCopy()),
		reconcileService(*v.service.DeepCopy()),
		reconcileDeployment(*v.deployment.DeepCopy()),
		reconcileRoute(v.service.GetName()))
}

func (v *vmConsoleProxy) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	// We need to use labels to find resources that were deployed by this operand,
	// because namespace annotation may not be present.

	var objectsToDelete []client.Object

	serviceAccounts, err := findResourcesUsingLabels(
		request.Context,
		v.serviceAccount.Name,
		request.Client,
		func(list *core.ServiceAccountList) []core.ServiceAccount { return list.Items },
	)
	if err != nil {
		return nil, err
	}
	objectsToDelete = append(objectsToDelete, serviceAccounts...)

	configMaps, err := findResourcesUsingLabels(
		request.Context,
		v.configMap.Name,
		request.Client,
		func(list *core.ConfigMapList) []core.ConfigMap { return list.Items },
	)
	if err != nil {
		return nil, err
	}
	objectsToDelete = append(objectsToDelete, configMaps...)

	services, err := findResourcesUsingLabels(
		request.Context,
		v.service.Name,
		request.Client,
		func(list *core.ServiceList) []core.Service { return list.Items },
	)
	if err != nil {
		return nil, err
	}
	objectsToDelete = append(objectsToDelete, services...)

	deployments, err := findResourcesUsingLabels(
		request.Context,
		v.deployment.Name,
		request.Client,
		func(list *apps.DeploymentList) []apps.Deployment { return list.Items },
	)
	if err != nil {
		return nil, err
	}
	objectsToDelete = append(objectsToDelete, deployments...)

	routes, err := findResourcesUsingLabels(
		request.Context,
		routeName,
		request.Client,
		func(list *routev1.RouteList) []routev1.Route { return list.Items },
	)
	if err != nil {
		return nil, err
	}
	objectsToDelete = append(objectsToDelete, routes...)

	objectsToDelete = append(objectsToDelete,
		v.clusterRole.DeepCopy(),
		v.clusterRoleBinding.DeepCopy())

	return common.DeleteAll(request, objectsToDelete...)
}

func reconcileServiceAccount(serviceAccount core.ServiceAccount) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		serviceAccount.Namespace = getVmConsoleProxyNamespace(request)
		return common.CreateOrUpdate(request).
			ClusterResource(&serviceAccount).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileClusterRole(clusterRole rbac.ClusterRole) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		return common.CreateOrUpdate(request).
			ClusterResource(&clusterRole).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				newClusterRole := newRes.(*rbac.ClusterRole)
				foundClusterRole := foundRes.(*rbac.ClusterRole)
				foundClusterRole.Rules = newClusterRole.Rules
			}).
			Reconcile()
	}
}

func reconcileClusterRoleBinding(clusterRoleBinding rbac.ClusterRoleBinding) common.ReconcileFunc {
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

func reconcileConfigMap(configMap core.ConfigMap) common.ReconcileFunc {
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

func reconcileService(service core.Service) common.ReconcileFunc {
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

func reconcileDeployment(deployment apps.Deployment) common.ReconcileFunc {
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
						"Not all vm-console-proxy pods are running. Expected: %d, running: %d",
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

func reconcileRoute(serviceName string) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		return common.CreateOrUpdate(request).
			ClusterResource(newRoute(getVmConsoleProxyNamespace(request), serviceName)).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(newRes, foundRes client.Object) {
				foundRes.(*routev1.Route).Spec = newRes.(*routev1.Route).Spec
			}).
			Reconcile()
	}
}

func isEnabled(request *common.Request) bool {
	if request.Instance.GetAnnotations() == nil {
		return false
	}
	if enable, isFound := request.Instance.GetAnnotations()[EnableAnnotation]; isFound {
		if isEnabled, err := strconv.ParseBool(enable); err == nil {
			return isEnabled
		}
	}
	return false
}

func getVmConsoleProxyNamespace(request *common.Request) string {
	const defaultNamespace = "kubevirt"
	if request.Instance.GetAnnotations() == nil {
		return defaultNamespace
	}
	if namespace, isFound := request.Instance.GetAnnotations()[VmConsoleProxyNamespaceAnnotation]; isFound {
		return namespace
	}
	return defaultNamespace
}

func getVmConsoleProxyImage() string {
	return common.EnvOrDefault("VM_CONSOLE_PROXY_IMAGE", "quay.io/kubevirt/vm-console-proxy:v0.1.0")
}

func findResourcesUsingLabels[PtrL interface {
	*L
	client.ObjectList
}, PtrT interface {
	*T
	client.Object
}, L any, T any](ctx context.Context, name string, cli client.Client, itemsFunc func(list PtrL) []T) ([]client.Object, error) {
	listObj := PtrL(new(L))
	err := cli.List(ctx, listObj,
		client.MatchingLabels{
			common.AppKubernetesNameLabel:      operandName,
			common.AppKubernetesComponentLabel: operandComponent,
			common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByLabelValue,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error listing objects: %w", err)
	}

	// Filtering in a loop instead of using a FieldSelector in the List() call.
	// It is only slightly inefficient, because all objects are already cached locally, so there is no API call.
	// Adding an Indexer to the cache for each object type that we want to list here would be a larger change.
	items := itemsFunc(listObj)
	var filteredItems []client.Object
	for i := range items {
		item := PtrT(&items[i])
		if item.GetName() == name {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems, nil
}

func newRoute(namespace string, serviceName string) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}
