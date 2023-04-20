package tekton_pipelines

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	"github.com/kubevirt/tekton-tasks-operator/pkg/environment"
	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
	tektonbundle "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-bundle"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=tekton.dev,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=configmaps,verbs=list;watch;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

const (
	namespacePattern = "^(openshift|kube)-"
	operandName      = "tekton-pipelines"
	operandComponent = common.AppComponentTektonPipelines
)

var namespaceRegex = regexp.MustCompile(namespacePattern)

var requiredCRDs = []string{"tasks.tekton.dev"}

func init() {
	utilruntime.Must(pipeline.AddToScheme(common.Scheme))
}

type tektonPipelines struct {
	pipelines       []pipeline.Pipeline
	configMaps      []v1.ConfigMap
	roleBindings    []rbac.RoleBinding
	serviceAccounts []v1.ServiceAccount
	clusterRoles    []rbac.ClusterRole
}

var _ operands.Operand = &tektonPipelines{}

func New(bundle *tektonbundle.Bundle) *tektonPipelines {
	tp := &tektonPipelines{
		pipelines:       bundle.Pipelines,
		configMaps:      bundle.ConfigMaps,
		roleBindings:    bundle.RoleBindings,
		serviceAccounts: bundle.ServiceAccounts,
		clusterRoles:    bundle.ClusterRoles,
	}
	return tp
}

func (t *tektonPipelines) Name() string {
	return operandName
}

func (t *tektonPipelines) WatchClusterTypes() []client.Object {
	return []client.Object{
		&pipeline.Pipeline{},
		&v1.ConfigMap{},
		&rbac.RoleBinding{},
		&v1.ServiceAccount{},
		&rbac.ClusterRole{},
	}
}

func (t *tektonPipelines) WatchTypes() []client.Object {
	return nil
}

func (t *tektonPipelines) RequiredCrds() []string {
	return requiredCRDs
}

func (t *tektonPipelines) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	var results []common.ReconcileResult
	var reconcileFunc []common.ReconcileFunc
	reconcileFunc = append(reconcileFunc, reconcileClusterRolesFuncs(request, t.clusterRoles)...)
	reconcileFunc = append(reconcileFunc, reconcileTektonPipelinesFuncs(t.pipelines)...)
	reconcileFunc = append(reconcileFunc, reconcileConfigMapsFuncs(t.configMaps)...)
	reconcileFunc = append(reconcileFunc, reconcileRoleBindingsFuncs(request, t.roleBindings)...)
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
	return append(results, reconcileTektonBundleResults...), nil
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
	for _, cr := range t.clusterRoles {
		o := cr.DeepCopy()
		objects = append(objects, o)
	}

	return common.DeleteAll(request, objects...)
}

func isUpgradingNow(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != environment.GetOperatorVersion()
}

func reconcileTektonPipelinesFuncs(pipelines []pipeline.Pipeline) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(pipelines))
	for i := range pipelines {
		p := &pipelines[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Spec.Pipelines.Namespace
			p.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(p).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newPipeline := newRes.(*pipeline.Pipeline)
					foundPipeline := foundRes.(*pipeline.Pipeline)
					foundPipeline.Spec = newPipeline.Spec
					for i, param := range foundPipeline.Spec.Params {
						if strings.HasPrefix(param.Name, "virtioContainer") {
							foundPipeline.Spec.Params[i].Default = &v1beta1.ArrayOrString{
								Type:      v1beta1.ParamTypeString,
								StringVal: environment.GetVirtioImage(),
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
		cm := &configMaps[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Spec.Pipelines.Namespace
			cm.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(cm).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newCM := newRes.(*v1.ConfigMap)
					foundCM := foundRes.(*v1.ConfigMap)
					foundCM.Data = newCM.Data
				}).
				Reconcile()
		})
	}
	return funcs
}

func reconcileServiceAccountsFuncs(r *common.Request, sas []v1.ServiceAccount) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(sas))
	for i := range sas {
		sa := &sas[i]
		// deploy pipeline SA only in `^(openshift|kube)-` namespaces
		if !namespaceRegex.MatchString(r.Instance.Spec.Pipelines.Namespace) && sa.Name == "pipeline" {
			continue
		}

		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Namespace
			if sa.Namespace == "" {
				sa.Namespace = namespace
			}
			return common.CreateOrUpdate(request).
				ClusterResource(sa).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newSA := newRes.(*v1.ServiceAccount)
					foundSA := foundRes.(*v1.ServiceAccount)
					foundSA.Labels = newSA.Labels
					foundSA.Annotations = newSA.Annotations
				}).
				Reconcile()
		})
	}
	return funcs
}

func reconcileClusterRolesFuncs(r *common.Request, crs []rbac.ClusterRole) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(crs))
	for i := range crs {
		cr := &crs[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(cr).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newCR := newRes.(*rbac.ClusterRole)
					foundCR := foundRes.(*rbac.ClusterRole)
					foundCR.Labels = newCR.Labels
					foundCR.Annotations = newCR.Annotations
					foundCR.Rules = newCR.Rules
				}).
				Reconcile()
		})
	}
	return funcs
}

func reconcileRoleBindingsFuncs(r *common.Request, rbs []rbac.RoleBinding) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(rbs))
	for i := range rbs {
		rb := &rbs[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Namespace
			if rb.Namespace == "" {
				rb.Namespace = namespace
			}
			for j := range rb.Subjects {
				subject := &rb.Subjects[j]
				subject.Namespace = namespace
			}
			return common.CreateOrUpdate(request).
				ClusterResource(rb).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newTask := newRes.(*rbac.RoleBinding)
					foundTask := foundRes.(*rbac.RoleBinding)
					foundTask.RoleRef = newTask.RoleRef
					foundTask.Subjects = newTask.Subjects
				}).
				Reconcile()
		})
	}
	return funcs
}
