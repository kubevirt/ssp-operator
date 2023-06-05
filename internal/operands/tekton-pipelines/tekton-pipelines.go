package tekton_pipelines

import (
	"fmt"
	"regexp"
	"strings"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	tektonbundle "kubevirt.io/ssp-operator/internal/tekton-bundle"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=tekton.dev,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=configmaps,verbs=list;watch;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

const (
	namespacePattern = "^(openshift|kube)-"
	operandName      = "tekton-pipelines"
	operandComponent = common.AppComponentTektonPipelines
	tektonCrd        = "tasks.tekton.dev"
)

var namespaceRegex = regexp.MustCompile(namespacePattern)

func init() {
	utilruntime.Must(pipeline.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		// Solution to optional Tekton CRD is not implemented yet.
		// Until then, do not watch to Tekton CRD.
		// {Object: &pipeline.Pipeline{}, Crd: tektonCrd, WatchFullObject: true},
		{Object: &v1.ConfigMap{}},
		{Object: &rbac.RoleBinding{}},
		{Object: &v1.ServiceAccount{}},
		{Object: &rbac.ClusterRole{}},
	}
}

type tektonPipelines struct {
	pipelines       []pipeline.Pipeline
	configMaps      []v1.ConfigMap
	roleBindings    []rbac.RoleBinding
	serviceAccounts []v1.ServiceAccount
	clusterRoles    []rbac.ClusterRole
}

var _ operands.Operand = &tektonPipelines{}

func New(bundle *tektonbundle.Bundle) operands.Operand {
	return &tektonPipelines{
		pipelines:       bundle.Pipelines,
		configMaps:      bundle.ConfigMaps,
		roleBindings:    bundle.RoleBindings,
		serviceAccounts: bundle.ServiceAccounts,
		clusterRoles:    bundle.ClusterRoles,
	}
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
	if request.Instance.Spec.FeatureGates == nil {
		request.Logger.V(1).Info("Tekton Pipelines resources were not deployed, because spec.featureGates is nil")
		return nil, nil
	}
	if !request.Instance.Spec.FeatureGates.DeployTektonTaskResources {
		request.Logger.V(1).Info("Tekton Pipelines resources were not deployed, because spec.featureGates.deployTektonTaskResources is set to false")
		return nil, nil
	}
	// Solution to optional Tekton CRD is not implemented yet.
	// Until then, do not check if Tekton CRD exists.
	// if !request.CrdList.CrdExists(tektonCrd) {
	// 	return nil, fmt.Errorf("Tekton CRD %s does not exist", tektonCrd)
	// }

	var reconcileFunc []common.ReconcileFunc
	reconcileFunc = append(reconcileFunc, reconcileClusterRolesFuncs(t.clusterRoles)...)
	reconcileFunc = append(reconcileFunc, reconcileTektonPipelinesFuncs(t.pipelines)...)
	reconcileFunc = append(reconcileFunc, reconcileConfigMapsFuncs(t.configMaps)...)
	reconcileFunc = append(reconcileFunc, reconcileRoleBindingsFuncs(t.roleBindings)...)
	reconcileFunc = append(reconcileFunc, reconcileServiceAccountsFuncs(request, t.serviceAccounts)...)

	reconcileTektonBundleResults, err := common.CollectResourceStatus(request, reconcileFunc...)
	if err != nil {
		return nil, err
	}

	upgradingNow := isUpgradingNow(request)
	for _, r := range reconcileTektonBundleResults {
		if !upgradingNow && (r.OperationResult == common.OperationResultUpdated) {
			request.Logger.Info(fmt.Sprintf("Changes reverted in tekton pipeline: %s", r.Resource.GetName()))
		}
	}
	return reconcileTektonBundleResults, nil
}

func (t *tektonPipelines) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object
	for _, p := range t.pipelines {
		o := p.DeepCopy()
		objects = append(objects, o)
	}
	for _, cm := range t.configMaps {
		o := cm.DeepCopy()
		objects = append(objects, o)
	}
	for _, rb := range t.roleBindings {
		o := rb.DeepCopy()
		objects = append(objects, o)
	}
	for _, sa := range t.serviceAccounts {
		o := sa.DeepCopy()
		objects = append(objects, o)
	}

	for i := range objects {
		objects[i].SetNamespace(getTektonPipelinesNamespace(request))
	}

	for _, cr := range t.clusterRoles {
		o := cr.DeepCopy()
		objects = append(objects, o)
	}

	return common.DeleteAll(request, objects...)
}

func isUpgradingNow(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != common.GetOperatorVersion()
}

func reconcileTektonPipelinesFuncs(pipelines []pipeline.Pipeline) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(pipelines))
	for i := range pipelines {
		tektonPipeline := &pipelines[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			tektonPipeline.Namespace = getTektonPipelinesNamespace(request)
			return common.CreateOrUpdate(request).
				ClusterResource(tektonPipeline).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newPipeline := newRes.(*pipeline.Pipeline)
					foundPipeline := foundRes.(*pipeline.Pipeline)
					foundPipeline.Spec = newPipeline.Spec
					for i, param := range foundPipeline.Spec.Params {
						if strings.HasPrefix(param.Name, "virtioContainer") {
							foundPipeline.Spec.Params[i].Default = &pipeline.ParamValue{
								Type:      pipeline.ParamTypeString,
								StringVal: common.GetVirtioImage(),
							}
						}
					}
				}).
				Reconcile()
		})
	}
	return funcs
}

func reconcileConfigMapsFuncs(configMaps []v1.ConfigMap) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(configMaps))
	for i := range configMaps {
		configMap := &configMaps[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			configMap.Namespace = getTektonPipelinesNamespace(request)
			return common.CreateOrUpdate(request).
				ClusterResource(configMap).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func reconcileServiceAccountsFuncs(request *common.Request, serviceAccounts []v1.ServiceAccount) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(serviceAccounts))
	for i := range serviceAccounts {
		serviceAccount := &serviceAccounts[i]

		if request.Instance.Spec.TektonPipelines != nil && request.Instance.Spec.TektonPipelines.Namespace != "" {
			// deploy pipeline SA only in `^(openshift|kube)-` namespaces
			if !namespaceRegex.MatchString(request.Instance.Spec.TektonPipelines.Namespace) && serviceAccount.Name == "pipeline" {
				continue
			}
		}

		funcs = append(funcs, func(r *common.Request) (common.ReconcileResult, error) {
			serviceAccount.Namespace = getTektonPipelinesNamespace(r)
			return common.CreateOrUpdate(r).
				ClusterResource(serviceAccount).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func reconcileClusterRolesFuncs(clusterRoles []rbac.ClusterRole) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(clusterRoles))
	for i := range clusterRoles {
		clusterRole := &clusterRoles[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(clusterRole).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func reconcileRoleBindingsFuncs(rolebindings []rbac.RoleBinding) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(rolebindings))
	for i := range rolebindings {
		roleBinding := &rolebindings[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := getTektonPipelinesNamespace(request)
			if roleBinding.Namespace == "" {
				roleBinding.Namespace = namespace
			}
			for j := range roleBinding.Subjects {
				subject := &roleBinding.Subjects[j]
				subject.Namespace = namespace
			}
			return common.CreateOrUpdate(request).
				ClusterResource(roleBinding).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func getTektonPipelinesNamespace(request *common.Request) string {
	if request.Instance.Spec.TektonPipelines != nil && request.Instance.Spec.TektonPipelines.Namespace != "" {
		return request.Instance.Spec.TektonPipelines.Namespace
	}
	return request.Instance.Namespace
}
