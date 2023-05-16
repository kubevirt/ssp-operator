package tekton_tasks

import (
	"fmt"
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

// +kubebuilder:rbac:groups=tekton.dev,resources=clustertasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tekton.dev,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances;virtualmachines,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=subresources.kubevirt.io,resources=virtualmachines/restart;virtualmachines/start;virtualmachines/stop,verbs=update
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups=template.openshift.io,resources=processedtemplates,verbs=create
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes,verbs=*
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datasources,verbs=get;create;delete
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines/finalizers,verbs=*
// +kubebuilder:rbac:groups=*,resources=persistentvolumeclaims,verbs=*
// +kubebuilder:rbac:groups=*,resources=pods,verbs=create
// +kubebuilder:rbac:groups=*,resources=secrets,verbs=*

const (
	operandName      = "tekton-tasks"
	operandComponent = common.AppComponentTektonTasks
	tektonCrd        = "tasks.tekton.dev"

	cleanVMTaskName              = "cleanup-vm"
	copyTemplateTaskName         = "copy-template"
	modifyDataObjectTaskName     = "modify-data-object"
	createVMFromTemplateTaskName = "create-vm-from-template"
	diskVirtCustomizeTaskName    = "disk-virt-customize"
	diskVirtSysprepTaskName      = "disk-virt-sysprep"
	modifyTemplateTaskName       = "modify-vm-template"
	waitForVMITaskName           = "wait-for-vmi-status"
	createVMFromManifestTaskName = "create-vm-from-manifest"
	generateSSHKeysTaskName      = "generate-ssh-keys"
	executeInVMTaskName          = "execute-in-vm"
	modifyWindowsVMIsoFileName   = "modify-windows-iso-file"
)

var AllowedTasks = map[string]func() string{
	createVMFromManifestTaskName: common.GetTektonTasksImage,
	cleanVMTaskName:              common.GetTektonTasksImage,
	copyTemplateTaskName:         common.GetTektonTasksImage,
	modifyDataObjectTaskName:     common.GetTektonTasksImage,
	createVMFromTemplateTaskName: common.GetTektonTasksImage,
	diskVirtCustomizeTaskName:    common.GetTektonTasksDiskVirtImage,
	diskVirtSysprepTaskName:      common.GetTektonTasksDiskVirtImage,
	modifyTemplateTaskName:       common.GetTektonTasksImage,
	waitForVMITaskName:           common.GetTektonTasksImage,
	generateSSHKeysTaskName:      common.GetTektonTasksImage,
	executeInVMTaskName:          common.GetTektonTasksImage,
	modifyWindowsVMIsoFileName:   common.GetTektonTasksImage,
}

func init() {
	utilruntime.Must(pipeline.AddToScheme(common.Scheme))
}

type tektonTasks struct {
	tasks           []pipeline.Task
	serviceAccounts []v1.ServiceAccount
	roleBindings    []rbac.RoleBinding
	clusterRoles    []rbac.ClusterRole
}

var _ operands.Operand = &tektonTasks{}

func New(bundle *tektonbundle.Bundle) operands.Operand {
	newTasks := []pipeline.Task{}
	for _, task := range bundle.Tasks {
		if _, ok := AllowedTasks[task.Name]; ok {
			newTasks = append(newTasks, task)
		}
	}
	bundle.Tasks = newTasks

	newServiceAccounts := []v1.ServiceAccount{}
	for _, serviceAccount := range bundle.ServiceAccounts {
		if _, ok := AllowedTasks[strings.TrimSuffix(serviceAccount.Name, "-task")]; ok {
			newServiceAccounts = append(newServiceAccounts, serviceAccount)
		}
	}
	bundle.ServiceAccounts = newServiceAccounts

	newRoleBinding := []rbac.RoleBinding{}
	for _, roleBinding := range bundle.RoleBindings {
		if _, ok := AllowedTasks[strings.TrimSuffix(roleBinding.Name, "-task")]; ok {
			newRoleBinding = append(newRoleBinding, roleBinding)
		}
	}
	bundle.RoleBindings = newRoleBinding

	newClusterRole := []rbac.ClusterRole{}
	for _, clusterRole := range bundle.ClusterRoles {
		if _, ok := AllowedTasks[strings.TrimSuffix(clusterRole.Name, "-task")]; ok {
			newClusterRole = append(newClusterRole, clusterRole)
		}
	}
	bundle.ClusterRoles = newClusterRole

	return &tektonTasks{
		tasks:           bundle.Tasks,
		serviceAccounts: bundle.ServiceAccounts,
		roleBindings:    bundle.RoleBindings,
		clusterRoles:    bundle.ClusterRoles,
	}
}

func (t *tektonTasks) Name() string {
	return operandName
}

func (t *tektonTasks) WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &pipeline.ClusterTask{}, Crd: tektonCrd, WatchFullObject: true},
		{Object: &rbac.RoleBinding{}},
		{Object: &v1.ServiceAccount{}},
	}
}

func (t *tektonTasks) WatchTypes() []operands.WatchType {
	return nil
}

func (t *tektonTasks) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	if !request.Instance.Spec.FeatureGates.DeployTektonTaskResources {
		request.Logger.V(1).Info("Tekton Tasks resources were not deployed, because spec.featureGates.deployTektonTaskResources is set to false")
		return nil, nil
	}
	if !request.CrdList.CrdExists(tektonCrd) {
		return nil, fmt.Errorf("Tekton CRD %s does not exist", tektonCrd)
	}

	var reconcileFunc []common.ReconcileFunc
	reconcileFunc = append(reconcileFunc, reconcileTektonTasksFuncs(t.tasks)...)
	reconcileFunc = append(reconcileFunc, reconcileClusterRoleFuncs(t.clusterRoles)...)
	reconcileFunc = append(reconcileFunc, reconcileServiceAccountsFuncs(t.serviceAccounts)...)
	reconcileFunc = append(reconcileFunc, reconcileRoleBindingFuncs(t.roleBindings)...)

	reconcileTektonBundleResults, err := common.CollectResourceStatus(request, reconcileFunc...)
	if err != nil {
		return nil, err
	}

	upgradingNow := isUpgradingNow(request)
	for _, r := range reconcileTektonBundleResults {
		if !upgradingNow && (r.OperationResult == common.OperationResultUpdated) {
			request.Logger.Info(fmt.Sprintf("Changes reverted in tekton tasks: %s", r.Resource.GetName()))
		}
	}
	return reconcileTektonBundleResults, nil
}

func (t *tektonTasks) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object
	for _, t := range t.tasks {
		o := t.DeepCopy()
		objects = append(objects, o)
	}
	for _, cr := range t.clusterRoles {
		o := cr.DeepCopy()
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
	clusterTasks, err := listDeprecatedClusterTasks(request)
	if err != nil {
		return nil, err
	}
	for _, ct := range clusterTasks {
		o := ct.DeepCopy()
		objects = append(objects, o)
	}
	return common.DeleteAll(request, objects...)
}

// Note: ClusterTasks are deprecated and replaced by Tasks [1].
// [1] https://tekton.dev/docs/pipelines/tasks/#task-vs-clustertask
func listDeprecatedClusterTasks(request *common.Request) ([]pipeline.ClusterTask, error) {
	deprecatedClusterTasks := &pipeline.ClusterTaskList{}
	err := request.Client.List(request.Context, deprecatedClusterTasks, &client.MatchingLabels{
		common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
	})
	if err != nil {
		return nil, err
	}
	return deprecatedClusterTasks.Items, nil
}

func isUpgradingNow(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != common.GetOperatorVersion()
}

func reconcileTektonTasksFuncs(tasks []pipeline.Task) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(tasks))
	for i := range tasks {
		task := &tasks[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			task.Namespace = request.Instance.Namespace
			if request.Instance.Spec.TektonTasks != nil && request.Instance.Spec.TektonTasks.Namespace != "" {
				task.Namespace = request.Instance.Spec.TektonTasks.Namespace
			}
			if task.Name == modifyWindowsVMIsoFileName {
				for i, step := range task.Spec.Steps {
					if step.Name == "create-iso-file" {
						task.Spec.Steps[i].Image = AllowedTasks[modifyDataObjectTaskName]()
					}
					if step.Name == "convert-iso-file" || step.Name == "modify-iso-file" {
						task.Spec.Steps[i].Image = AllowedTasks[diskVirtCustomizeTaskName]()
					}
				}
			} else {
				task.Spec.Steps[0].Image = AllowedTasks[task.Name]()
			}
			task.Labels[TektonTasksVersionLabel] = common.TektonTasksVersion
			return common.CreateOrUpdate(request).
				ClusterResource(task).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func reconcileClusterRoleFuncs(crs []rbac.ClusterRole) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(crs))
	for i := range crs {
		cr := &crs[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(cr).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func reconcileServiceAccountsFuncs(sas []v1.ServiceAccount) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(sas))
	for i := range sas {
		sa := &sas[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Namespace
			sa.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(sa).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}

func reconcileRoleBindingFuncs(rbs []rbac.RoleBinding) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(rbs))
	for i := range rbs {
		rb := &rbs[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Namespace
			rb.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(rb).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}
