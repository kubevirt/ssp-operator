package tekton_pipelines

import (
	"fmt"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// +kubebuilder:rbac:groups=tekton.dev,resources=pipelines,verbs=list;watch;create;update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=list;watch;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=list;watch;create;update

const (
	operandName      = "tekton-pipelines"
	operandComponent = common.AppComponentTektonPipelines
	tektonCrd        = "tasks.tekton.dev"

	tektonDeprecated = "tekton.dev/deprecated"
)

func init() {
	utilruntime.Must(pipeline.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &v1.ConfigMap{}},
		{Object: &rbac.RoleBinding{}},
		{Object: &v1.ServiceAccount{}},
		{Object: &rbac.ClusterRole{}},
	}
}

type tektonPipelines struct {
}

var _ operands.Operand = &tektonPipelines{}

func New() operands.Operand {
	return &tektonPipelines{}
}

func (t *tektonPipelines) Name() string {
	return operandName
}

func (t *tektonPipelines) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (t *tektonPipelines) WatchTypes() []operands.WatchType {
	return nil
}

func (t *tektonPipelines) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	deprecateFuncs := []func(*common.Request) error{
		deprecateResource[rbac.ClusterRoleList, rbac.ClusterRole],
		deprecateResource[rbac.RoleBindingList, rbac.RoleBinding],
		deprecateResource[v1.ServiceAccountList, v1.ServiceAccount],
		deprecateResource[v1.ConfigMapList, v1.ConfigMap],
	}
	if request.CrdList.CrdExists(tektonCrd) {
		deprecateFuncs = append(deprecateFuncs, deprecateResource[pipeline.PipelineList, pipeline.Pipeline]) //nolint:staticcheck
	}

	for _, deprecateFunc := range deprecateFuncs {
		if err := deprecateFunc(request); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (t *tektonPipelines) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	cleanupFuncs := []func(*common.Request) ([]common.CleanupResult, error){
		cleanupResource[rbac.ClusterRoleList, rbac.ClusterRole],
		cleanupResource[rbac.RoleBindingList, rbac.RoleBinding],
		cleanupResource[v1.ServiceAccountList, v1.ServiceAccount],
		cleanupResource[v1.ConfigMapList, v1.ConfigMap],
	}
	if request.CrdList.CrdExists(tektonCrd) {
		cleanupFuncs = append(cleanupFuncs, cleanupResource[pipeline.PipelineList, pipeline.Pipeline]) //nolint:staticcheck
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
	if sspPartOf, exists := ssp.Labels[common.AppKubernetesPartOfLabel]; exists {
		result[common.AppKubernetesPartOfLabel] = sspPartOf
	}
	return result
}
