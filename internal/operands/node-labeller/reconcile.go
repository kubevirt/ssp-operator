package node_labeller

import (
	"fmt"
	secv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

type nodeLabeller struct{}

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

func (nl *nodeLabeller) Reconcile(request *common.Request) error {
	for _, f := range []func(*common.Request) error{
		reconcileClusterRole,
		reconcileServiceAccount,
		reconcileClusterRoleBinding,
		reconcileConfigMap,
		reconcileDaemonSet,
		reconcileSecurityContextConstraint,
	} {
		if err := f(request); err != nil {
			return err
		}
	}
	return nil
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

func reconcileClusterRole(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request,
		newClusterRole(),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*rbac.ClusterRole).Rules = newRes.(*rbac.ClusterRole).Rules
		})
}

func reconcileServiceAccount(request *common.Request) error {
	return common.CreateOrUpdateResource(request,
		newServiceAccount(request.Namespace),
		func(_, _ controllerutil.Object) {})
}

func reconcileClusterRoleBinding(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request,
		newClusterRoleBinding(request.Namespace),
		func(newRes, foundRes controllerutil.Object) {
			newBinding := newRes.(*rbac.ClusterRoleBinding)
			foundBinding := foundRes.(*rbac.ClusterRoleBinding)
			foundBinding.RoleRef = newBinding.RoleRef
			foundBinding.Subjects = newBinding.Subjects
		})
}

func reconcileConfigMap(request *common.Request) error {
	return common.CreateOrUpdateResource(request,
		newConfigMap(request.Namespace),
		func(newRes, foundRes controllerutil.Object) {
			newConfigMap := newRes.(*v1.ConfigMap)
			foundConfigMap := foundRes.(*v1.ConfigMap)
			foundConfigMap.Data = newConfigMap.Data
		})
}

func reconcileDaemonSet(request *common.Request) error {
	nodeLabellerSpec := &request.Instance.Spec.NodeLabeller
	daemonSet := newDaemonSet(request.Namespace)
	addPlacementFields(daemonSet, &nodeLabellerSpec.Placement)
	return common.CreateOrUpdateResource(request,
		daemonSet,
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*apps.DaemonSet).Spec = newRes.(*apps.DaemonSet).Spec
		})
}

func addPlacementFields(daemonset *apps.DaemonSet, nodePlacement *lifecycleapi.NodePlacement) {
	podSpec := &daemonset.Spec.Template.Spec
	podSpec.Affinity = nodePlacement.Affinity
	podSpec.NodeSelector = nodePlacement.NodeSelector
	podSpec.Tolerations = nodePlacement.Tolerations
}

func reconcileSecurityContextConstraint(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request,
		newSecurityContextConstraint(request.Namespace),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*secv1.SecurityContextConstraints).AllowPrivilegedContainer = newRes.(*secv1.SecurityContextConstraints).AllowPrivilegedContainer
			foundRes.(*secv1.SecurityContextConstraints).RunAsUser = newRes.(*secv1.SecurityContextConstraints).RunAsUser
			foundRes.(*secv1.SecurityContextConstraints).SELinuxContext = newRes.(*secv1.SecurityContextConstraints).SELinuxContext
			foundRes.(*secv1.SecurityContextConstraints).Users = newRes.(*secv1.SecurityContextConstraints).Users
		})
}
