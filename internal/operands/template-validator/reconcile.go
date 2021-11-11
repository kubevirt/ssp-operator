package template_validator

import (
	"fmt"

	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch

func WatchTypes() []client.Object {
	return []client.Object{
		&v1.ServiceAccount{},
		&v1.Service{},
		&apps.Deployment{},
	}
}

func WatchClusterTypes() []client.Object {
	return []client.Object{
		&rbac.ClusterRole{},
		&rbac.ClusterRoleBinding{},
		&admission.ValidatingWebhookConfiguration{},
	}
}

type templateValidator struct{}

func (t *templateValidator) Name() string {
	return operandName
}

func (t *templateValidator) WatchTypes() []client.Object {
	return WatchTypes()
}

func (t *templateValidator) WatchClusterTypes() []client.Object {
	return WatchClusterTypes()
}

func (t *templateValidator) RequiredCrds() []string {
	return nil
}

func (t *templateValidator) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	return common.CollectResourceStatus(request,
		reconcileClusterRole,
		reconcileServiceAccount,
		reconcileClusterRoleBinding,
		reconcileService,
		reconcileDeployment,
		reconcileValidatingWebhook,
	)
}

func (t *templateValidator) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	return common.DeleteAll(request,
		newClusterRole(),
		newClusterRoleBinding(request.Namespace),
		newValidatingWebhook(request.Namespace),
	)
}

var _ operands.Operand = &templateValidator{}

func New() operands.Operand {
	return &templateValidator{}
}

const (
	operandName      = "template-validator"
	operandComponent = common.AppComponentTemplating
)

func reconcileClusterRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newClusterRole()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*rbac.ClusterRole).Rules = newRes.(*rbac.ClusterRole).Rules
		}).
		Reconcile()
}

func reconcileServiceAccount(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newServiceAccount(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileClusterRoleBinding(request *common.Request) (common.ReconcileResult, error) {
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

func reconcileService(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newService(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newService := newRes.(*v1.Service)
			foundService := foundRes.(*v1.Service)

			// ClusterIP should not be updated
			newService.Spec.ClusterIP = foundService.Spec.ClusterIP

			foundService.Spec = newService.Spec
		}).
		Reconcile()
}

func reconcileDeployment(request *common.Request) (common.ReconcileResult, error) {
	validatorSpec := request.Instance.Spec.TemplateValidator
	image := getTemplateValidatorImage()
	if image == "" {
		panic("Cannot reconcile without valid image name")
	}
	numberOfReplicas := *validatorSpec.Replicas
	if request.IsSingleReplicaTopologyMode() && (numberOfReplicas > 1) {
		numberOfReplicas = 1
	}
	deployment := newDeployment(request.Namespace, numberOfReplicas, image)
	addPlacementFields(deployment, validatorSpec.Placement)
	return common.CreateOrUpdate(request).
		NamespacedResource(deployment).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*apps.Deployment).Spec = newRes.(*apps.Deployment).Spec
		}).
		StatusFunc(func(res client.Object) common.ResourceStatus {
			dep := res.(*apps.Deployment)
			status := common.ResourceStatus{}
			if numberOfReplicas > 0 && dep.Status.AvailableReplicas == 0 {
				msg := fmt.Sprintf("No validator pods are running. Expected: %d", dep.Status.Replicas)
				status.NotAvailable = &msg
			}
			if dep.Status.AvailableReplicas != numberOfReplicas {
				msg := fmt.Sprintf(
					"Not all template validator pods are running. Expected: %d, running: %d",
					numberOfReplicas,
					dep.Status.AvailableReplicas,
				)
				status.Progressing = &msg
				status.Degraded = &msg
			}
			return status
		}).
		Reconcile()
}

func addPlacementFields(deployment *apps.Deployment, nodePlacement *lifecycleapi.NodePlacement) {
	if nodePlacement == nil {
		return
	}

	podSpec := &deployment.Spec.Template.Spec
	podSpec.Affinity = nodePlacement.Affinity
	podSpec.NodeSelector = nodePlacement.NodeSelector
	podSpec.Tolerations = nodePlacement.Tolerations
}

func reconcileValidatingWebhook(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newValidatingWebhook(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newWebhookConf := newRes.(*admission.ValidatingWebhookConfiguration)
			foundWebhookConf := foundRes.(*admission.ValidatingWebhookConfiguration)

			// Copy CA Bundle from the found webhook,
			// so it will not be overwritten
			copyFoundCaBundles(newWebhookConf.Webhooks, foundWebhookConf.Webhooks)

			foundWebhookConf.Webhooks = newWebhookConf.Webhooks
		}).
		Reconcile()
}

func copyFoundCaBundles(newWebhooks []admission.ValidatingWebhook, foundWebhooks []admission.ValidatingWebhook) {
	for i := range newWebhooks {
		newWebhook := &newWebhooks[i]
		for j := range foundWebhooks {
			foundWebhook := &foundWebhooks[j]
			if newWebhook.Name == foundWebhook.Name {
				newWebhook.ClientConfig.CABundle = foundWebhook.ClientConfig.CABundle
				break
			}
		}
	}
}
