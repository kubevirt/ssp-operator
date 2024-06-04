package vm_console_proxy

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands"
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
)

const (
	operandName      = "vm-console-proxy"
	operandComponent = "vm-console-proxy"

	routeName = "vm-console-proxy"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;serviceaccounts,verbs=list;watch;create;update;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;rolebindings,verbs=list;watch;create;update;delete
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=get;list;watch;create;update;delete

// Deprecated:
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=list;watch;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances;virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=subresources.kubevirt.io,resources=virtualmachineinstances/vnc,verbs=get
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=token.kubevirt.io,resources=virtualmachines/vnc,verbs=get

func init() {
	utilruntime.Must(routev1.Install(common.Scheme))
	utilruntime.Must(apiregv1.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.ClusterRoleBinding{}},
		{Object: &apiregv1.APIService{}},
	}
}

func WatchTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &core.ServiceAccount{}},
		{Object: &core.Service{}},
		{Object: &apps.Deployment{}, WatchFullObject: true},
		{Object: &core.ConfigMap{}},
		{Object: &routev1.Route{}},
	}
}

type vmConsoleProxy struct {
	serviceAccount     *core.ServiceAccount
	clusterRoles       []rbac.ClusterRole
	clusterRoleBinding *rbac.ClusterRoleBinding
	roleBinding        *rbac.RoleBinding
	service            *core.Service
	deployment         *apps.Deployment
	configMap          *core.ConfigMap
	apiService         *apiregv1.APIService
}

var _ operands.Operand = &vmConsoleProxy{}

func New(bundle *vm_console_proxy_bundle.Bundle) operands.Operand {
	return &vmConsoleProxy{
		serviceAccount:     bundle.ServiceAccount,
		clusterRoles:       bundle.ClusterRoles,
		clusterRoleBinding: bundle.ClusterRoleBinding,
		roleBinding:        bundle.RoleBinding,
		service:            bundle.Service,
		deployment:         bundle.Deployment,
		configMap:          bundle.ConfigMap,
		apiService:         bundle.ApiService,
	}
}

func (v *vmConsoleProxy) Name() string {
	return operandName
}

func (v *vmConsoleProxy) WatchTypes() []operands.WatchType {
	return WatchTypes()
}

func (v *vmConsoleProxy) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (v *vmConsoleProxy) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	if request.Instance.Spec.FeatureGates == nil || !request.Instance.Spec.FeatureGates.DeployVmConsoleProxy {
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

	reconcileFuncs := []common.ReconcileFunc{
		reconcileServiceAccount(*v.serviceAccount.DeepCopy()),
	}
	for i := range v.clusterRoles {
		reconcileFuncs = append(reconcileFuncs, reconcileClusterRole(*v.clusterRoles[i].DeepCopy()))
	}
	reconcileFuncs = append(reconcileFuncs,
		reconcileClusterRoleBinding(*v.clusterRoleBinding.DeepCopy()),
		reconcileRoleBinding(v.roleBinding.DeepCopy()),
		reconcileConfigMap(*v.configMap.DeepCopy()),
		reconcileService(*v.service.DeepCopy()),
		reconcileDeployment(*v.deployment.DeepCopy()),
		reconcileApiService(v.apiService.DeepCopy()))

	reconcileResults, err := common.CollectResourceStatus(request, reconcileFuncs...)
	if err != nil {
		return nil, err
	}

	// Route is no longer needed.
	routeCleanupResults, err := v.deleteRoute(request)
	if err != nil {
		return nil, err
	}
	for _, cleanupResult := range routeCleanupResults {
		if !cleanupResult.Deleted {
			reconcileResults = append(reconcileResults, common.ResourceDeletedResult(cleanupResult.Resource, common.OperationResultDeleted))
		}
	}

	return reconcileResults, nil
}

func (v *vmConsoleProxy) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	// We need to use labels to find resources that were deployed by this operand,
	// because namespace annotation may not be present.

	routeCleanupResults, err := v.deleteRoute(request)
	if err != nil {
		return nil, err
	}

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

	for i := range v.clusterRoles {
		objectsToDelete = append(objectsToDelete, v.clusterRoles[i].DeepCopy())
	}

	objectsToDelete = append(objectsToDelete,
		v.clusterRoleBinding.DeepCopy(),
		v.roleBinding.DeepCopy(),
		v.apiService.DeepCopy())

	cleanupResults, err := common.DeleteAll(request, objectsToDelete...)
	if err != nil {
		return nil, err
	}

	return append(cleanupResults, routeCleanupResults...), nil
}

func (v *vmConsoleProxy) deleteRoute(request *common.Request) ([]common.CleanupResult, error) {
	routes, err := findResourcesUsingLabels(
		request.Context,
		routeName,
		request.Client,
		func(list *routev1.RouteList) []routev1.Route { return list.Items },
	)
	if err != nil {
		return nil, err
	}

	return common.DeleteAll(request, routes...)
}

func reconcileServiceAccount(serviceAccount core.ServiceAccount) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		serviceAccount.Namespace = request.Instance.Namespace
		return common.CreateOrUpdate(request).
			NamespacedResource(&serviceAccount).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileClusterRole(clusterRole rbac.ClusterRole) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		return common.CreateOrUpdate(request).
			ClusterResource(&clusterRole).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileClusterRoleBinding(clusterRoleBinding rbac.ClusterRoleBinding) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		clusterRoleBinding.Subjects[0].Namespace = request.Instance.Namespace
		return common.CreateOrUpdate(request).
			ClusterResource(&clusterRoleBinding).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileRoleBinding(roleBinding *rbac.RoleBinding) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		roleBinding.Subjects[0].Namespace = request.Instance.Namespace
		return common.CreateOrUpdate(request).
			ClusterResource(roleBinding).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileConfigMap(configMap core.ConfigMap) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		configMap.Namespace = request.Instance.Namespace
		return common.CreateOrUpdate(request).
			NamespacedResource(&configMap).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileService(service core.Service) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		service.Namespace = request.Instance.Namespace
		return common.CreateOrUpdate(request).
			NamespacedResource(&service).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileDeployment(deployment apps.Deployment) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		deployment.Namespace = request.Instance.Namespace
		deployment.Spec.Template.Spec.Containers[0].Image = getVmConsoleProxyImage()
		common.AddAppLabels(request.Instance, operandName, operandComponent, &deployment.Spec.Template.ObjectMeta)
		return common.CreateOrUpdate(request).
			NamespacedResource(&deployment).
			WithAppLabels(operandName, operandComponent).
			Reconcile()
	}
}

func reconcileApiService(apiService *apiregv1.APIService) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		apiService.Spec.Service.Namespace = request.Instance.Namespace
		return common.CreateOrUpdate(request).
			ClusterResource(apiService).
			WithAppLabels(operandName, operandComponent).
			UpdateFunc(func(expected, found client.Object) {
				foundApiService := found.(*apiregv1.APIService)
				expectedApiService := expected.(*apiregv1.APIService)

				// Keep CA bundle the same in the found object
				expectedApiService.Spec.CABundle = foundApiService.Spec.CABundle

				foundApiService.Spec = expectedApiService.Spec
			}).
			Reconcile()
	}
}

func getVmConsoleProxyImage() string {
	return env.EnvOrDefault(env.VmConsoleProxyImageKey, defaultVmConsoleProxyImage)
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
			common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
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
