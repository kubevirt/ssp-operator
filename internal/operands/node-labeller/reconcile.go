package node_labeller

import (
	"fmt"

	secv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=serviceaccounts;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use,resourceNames=privileged

// RBAC for created roles
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;update;patch

type nodeLabeller struct{}

func (nl *nodeLabeller) Name() string {
	return operandName
}

func (nl *nodeLabeller) AddWatchTypesToScheme(s *runtime.Scheme) error {
	return secv1.Install(s)
}

func (nl *nodeLabeller) WatchTypes() []runtime.Object {
	return []runtime.Object{
		&v1.ServiceAccount{},
		&v1.ConfigMap{},
		&apps.DaemonSet{},
	}
}

func (nl *nodeLabeller) WatchClusterTypes() []runtime.Object {
	return []runtime.Object{
		&rbac.ClusterRole{},
		&rbac.ClusterRoleBinding{},
		&secv1.SecurityContextConstraints{},
	}
}

func (nl *nodeLabeller) Reconcile(request *common.Request) ([]common.ResourceStatus, error) {
	return common.CollectResourceStatus(request,
		reconcileClusterRole,
		reconcileServiceAccount,
		reconcileClusterRoleBinding,
		reconcileConfigMap,
		reconcileDaemonSet,
		reconcileSecurityContextConstraint,
	)
}

func (nl *nodeLabeller) Cleanup(request *common.Request) error {
	for _, obj := range []controllerutil.Object{
		newClusterRole(),
		newClusterRoleBinding(request.Namespace),
		newSecurityContextConstraint(request.Namespace),
	} {
		err := request.Client.Delete(request.Context, obj)
		if err != nil && !errors.IsNotFound(err) {
			request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", obj.GetName(), err))
			return err
		}
	}
	return nil
}

var _ operands.Operand = &nodeLabeller{}

func GetOperand() operands.Operand {
	return &nodeLabeller{}
}

const (
	operandName      = "node-labeler"
	operandComponent = common.AppComponentSchedule
)

func reconcileClusterRole(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newClusterRole()),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*rbac.ClusterRole).Rules = newRes.(*rbac.ClusterRole).Rules
		})
}

func reconcileServiceAccount(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newServiceAccount(request.Namespace)),
		func(_, _ controllerutil.Object) {})
}

func reconcileClusterRoleBinding(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newClusterRoleBinding(request.Namespace)),
		func(newRes, foundRes controllerutil.Object) {
			newBinding := newRes.(*rbac.ClusterRoleBinding)
			foundBinding := foundRes.(*rbac.ClusterRoleBinding)
			foundBinding.RoleRef = newBinding.RoleRef
			foundBinding.Subjects = newBinding.Subjects
		})
}

func reconcileConfigMap(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newConfigMap(request.Namespace)),
		func(newRes, foundRes controllerutil.Object) {
			newConfigMap := newRes.(*v1.ConfigMap)
			foundConfigMap := foundRes.(*v1.ConfigMap)
			foundConfigMap.Data = newConfigMap.Data
		})
}

func reconcileDaemonSet(request *common.Request) (common.ResourceStatus, error) {
	nodeLabellerSpec := request.Instance.Spec.NodeLabeller
	daemonSet := newDaemonSet(request.Namespace)
	addPlacementFields(daemonSet, nodeLabellerSpec.Placement)
	status, err := createOrUpdateDaemonSet(request, daemonSet)
	if errors.IsInvalid(err) {
		return recreateDaemonSet(request, daemonSet)
	}
	return status, err
}

func createOrUpdateDaemonSet(request *common.Request, resource controllerutil.Object) (common.ResourceStatus, error) {
	return common.CreateOrUpdateResourceWithStatus(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, resource),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*apps.DaemonSet).Spec = newRes.(*apps.DaemonSet).Spec
		},
		func(res controllerutil.Object) common.ResourceStatus {
			ds := res.(*apps.DaemonSet)
			status := common.ResourceStatus{}
			if ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
				msg := fmt.Sprintf("Not all node-labeler pods are ready. (ready pods: %d, desired pods: %d)",
					ds.Status.NumberReady,
					ds.Status.DesiredNumberScheduled)
				status.NotAvailable = &msg
				status.Progressing = &msg
				status.Degraded = &msg
			}
			return status
		})
}

func recreateDaemonSet(request *common.Request, resource controllerutil.Object) (common.ResourceStatus, error) {
	if err := request.Client.Delete(request.Context, resource, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
		return common.ResourceStatus{}, err
	}
	return createOrUpdateDaemonSet(request, resource)
}

func addPlacementFields(daemonset *apps.DaemonSet, nodePlacement *lifecycleapi.NodePlacement) {
	if nodePlacement == nil {
		return
	}

	podSpec := &daemonset.Spec.Template.Spec
	podSpec.Affinity = nodePlacement.Affinity
	podSpec.NodeSelector = nodePlacement.NodeSelector
	podSpec.Tolerations = nodePlacement.Tolerations
}

func reconcileSecurityContextConstraint(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newSecurityContextConstraint(request.Namespace)),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*secv1.SecurityContextConstraints).AllowPrivilegedContainer = newRes.(*secv1.SecurityContextConstraints).AllowPrivilegedContainer
			foundRes.(*secv1.SecurityContextConstraints).RunAsUser = newRes.(*secv1.SecurityContextConstraints).RunAsUser
			foundRes.(*secv1.SecurityContextConstraints).SELinuxContext = newRes.(*secv1.SecurityContextConstraints).SELinuxContext
			foundRes.(*secv1.SecurityContextConstraints).Users = newRes.(*secv1.SecurityContextConstraints).Users
		})
}
