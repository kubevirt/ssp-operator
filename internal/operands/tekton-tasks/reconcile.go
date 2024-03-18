package tekton_tasks

import (
	"fmt"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// +kubebuilder:rbac:groups=tekton.dev,resources=tasks,verbs=list;watch;update
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=list;watch;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;rolebindings,verbs=list;watch;update

// Below are RBAC for deployed ClusterRoles. We still need these permissions so we can update annotations on existing ClusterRoles

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;update;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;patch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances;virtualmachines,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines/finalizers,verbs=get
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datasources,verbs=get;create;delete
// +kubebuilder:rbac:groups=subresources.kubevirt.io,resources=virtualmachines/restart;virtualmachines/start;virtualmachines/stop,verbs=update
// +kubebuilder:rbac:groups=template.openshift.io,resources=processedtemplates,verbs=create
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch;create;update;delete

const (
	operandName      = "tekton-tasks"
	operandComponent = common.AppComponentTektonTasks
	tektonCrd        = "tasks.tekton.dev"

	tektonDeprecated = "tekton.dev/deprecated"
)

func init() {
	utilruntime.Must(pipeline.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.RoleBinding{}},
		{Object: &v1.ServiceAccount{}},
	}
}

type tektonTasks struct{}

var _ operands.Operand = &tektonTasks{}

func New() operands.Operand {
	return &tektonTasks{}
}

func (t *tektonTasks) Name() string {
	return operandName
}

func (t *tektonTasks) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (t *tektonTasks) WatchTypes() []operands.WatchType {
	return nil
}

func (t *tektonTasks) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	deprecateFuncs := []func(*common.Request) error{
		deprecateResource[rbac.ClusterRoleList, rbac.ClusterRole],
		deprecateResource[rbac.RoleBindingList, rbac.RoleBinding],
		deprecateResource[v1.ServiceAccountList, v1.ServiceAccount],
	}
	if request.CrdList.CrdExists(tektonCrd) {
		deprecateFuncs = append(deprecateFuncs, deprecateResource[pipeline.TaskList, pipeline.Task]) //nolint:staticcheck
	}

	for _, deprecate := range deprecateFuncs {
		if err := deprecate(request); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (t *tektonTasks) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	cleanupFuncs := []func(*common.Request) ([]common.CleanupResult, error){
		cleanupResource[rbac.ClusterRoleList, rbac.ClusterRole],
		cleanupResource[rbac.RoleBindingList, rbac.RoleBinding],
		cleanupResource[v1.ServiceAccountList, v1.ServiceAccount],
	}
	if request.CrdList.CrdExists(tektonCrd) {
		cleanupFuncs = append(cleanupFuncs, cleanupResource[pipeline.TaskList, pipeline.Task]) //nolint:staticcheck
	}

	var allResults []common.CleanupResult
	for _, cleanupFunc := range cleanupFuncs {
		cleanupResults, err := cleanupFunc(request)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, cleanupResults...)
	}

	return allResults, nil
}

func deprecateResource[L any, T any, PtrL interface {
	*L
	client.ObjectList
}, PtrT interface {
	*T
	client.Object
}](request *common.Request) error {
	resources, err := common.ListOwnedResources[L, T, PtrL, PtrT](request, matchingLabelsOption(request.Instance))
	if err != nil {
		return fmt.Errorf("failed to list owned resources: %w", err)
	}
	for i := range resources {
		resource := PtrT(&resources[i])
		existingVal := resource.GetAnnotations()[tektonDeprecated]
		if existingVal == "true" {
			continue
		}

		resource.GetAnnotations()[tektonDeprecated] = "true"
		if err := request.Client.Update(request.Context, resource); err != nil {
			return fmt.Errorf("failed to update %s: %w", resource.GetObjectKind().GroupVersionKind().Kind, err)
		}
	}
	return nil
}

func cleanupResource[L any, T any, PtrL interface {
	*L
	client.ObjectList
}, PtrT interface {
	*T
	client.Object
}](request *common.Request) ([]common.CleanupResult, error) {
	resources, err := common.ListOwnedResources[L, T, PtrL, PtrT](request, matchingLabelsOption(request.Instance))
	if err != nil {
		return nil, fmt.Errorf("failed to list owned resources: %w", err)
	}

	results := make([]common.CleanupResult, 0, len(resources))
	for i := range resources {
		resource := PtrT(&resources[i])
		cleanupResult, err := common.Cleanup(request, resource)
		if err != nil {
			return nil, fmt.Errorf("failed to cleanup resource: %w", err)
		}
		results = append(results, cleanupResult)
	}
	return results, nil
}

func matchingLabelsOption(ssp *ssp.SSP) client.MatchingLabels {
	result := client.MatchingLabels{
		common.AppKubernetesNameLabel:      operandName,
		common.AppKubernetesComponentLabel: operandComponent.String(),
		common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
	}
	if sspPartOf, exists := ssp.GetAnnotations()[common.AppKubernetesPartOfLabel]; exists {
		result[common.AppKubernetesPartOfLabel] = sspPartOf
	}
	return result
}
