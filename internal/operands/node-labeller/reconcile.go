package node_labeller

/*
*
* This package is deprecated! Do not add any new code here.
*
 */

import (
	"fmt"

	secv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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

//Reconsile deletes all node-labeller component, because labeller is migrated into kubevirt core.
func (nl *nodeLabeller) Reconcile(request *common.Request) ([]common.ResourceStatus, error) {
	returnResults := make([]common.ResourceStatus, 0)
	for _, obj := range []controllerutil.Object{
		newClusterRole(),
		newServiceAccount(request.Namespace),
		newConfigMap(request.Namespace),
		newClusterRoleBinding(request.Namespace),
		newSecurityContextConstraint(request.Namespace),
		newDaemonSet(request.Namespace),
	} {
		err := request.Client.Delete(request.Context, obj)

		if err != nil && !errors.IsNotFound(err) {
			request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", obj.GetName(), err))
			return []common.ResourceStatus{}, err
		}
		returnResults = append(returnResults, common.ResourceStatus{})
	}
	return returnResults, nil
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
	daemonSet := newDaemonSet(request.Namespace)
	status, err := createOrUpdateDaemonSet(request, daemonSet)

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
