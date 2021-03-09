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

func (nl *nodeLabeller) WatchTypes() []client.Object {
	return []client.Object{
		&v1.ServiceAccount{},
		&v1.ConfigMap{},
		&apps.DaemonSet{},
	}
}

func (nl *nodeLabeller) WatchClusterTypes() []client.Object {
	return []client.Object{
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
	for _, obj := range []client.Object{
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
	return common.CreateOrUpdate(request).
		ClusterResource(newClusterRole()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*rbac.ClusterRole).Rules = newRes.(*rbac.ClusterRole).Rules
		}).
		Reconcile()
}

func reconcileServiceAccount(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newServiceAccount(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileClusterRoleBinding(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newClusterRoleBinding(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newBinding := newRes.(*rbac.ClusterRoleBinding)
			foundBinding := foundRes.(*rbac.ClusterRoleBinding)
			foundBinding.RoleRef = newBinding.RoleRef
			foundBinding.Subjects = newBinding.Subjects
		}).
		Reconcile()
}

func reconcileConfigMap(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newConfigMap(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*v1.ConfigMap).Data = newRes.(*v1.ConfigMap).Data
		}).
		Reconcile()
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

func createOrUpdateDaemonSet(request *common.Request, daemonSet *apps.DaemonSet) (common.ResourceStatus, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(daemonSet).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*apps.DaemonSet).Spec = newRes.(*apps.DaemonSet).Spec
		}).
		StatusFunc(func(res client.Object) common.ResourceStatus {
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
		}).
		Reconcile()
}

func recreateDaemonSet(request *common.Request, daemonSet *apps.DaemonSet) (common.ResourceStatus, error) {
	if err := request.Client.Delete(request.Context, daemonSet, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
		return common.ResourceStatus{}, err
	}
	return createOrUpdateDaemonSet(request, daemonSet)
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
	return common.CreateOrUpdate(request).
		ClusterResource(newSecurityContextConstraint(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundScc := foundRes.(*secv1.SecurityContextConstraints)
			newScc := newRes.(*secv1.SecurityContextConstraints)
			foundScc.AllowPrivilegedContainer = newScc.AllowPrivilegedContainer
			foundScc.RunAsUser = newScc.RunAsUser
			foundScc.SELinuxContext = newScc.SELinuxContext
			foundScc.Users = newScc.Users
		}).
		Reconcile()
}
