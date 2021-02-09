package template_validator

import (
	"fmt"

	admission "k8s.io/api/admissionregistration/v1"
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

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch

type templateValidator struct{}

func (t *templateValidator) Name() string {
	return operandName
}

func (t *templateValidator) AddWatchTypesToScheme(*runtime.Scheme) error {
	return nil
}

func (t *templateValidator) WatchTypes() []runtime.Object {
	return []runtime.Object{
		&v1.ServiceAccount{},
		&v1.Service{},
		&apps.Deployment{},
	}
}

func (t *templateValidator) WatchClusterTypes() []runtime.Object {
	return []runtime.Object{
		&rbac.ClusterRole{},
		&rbac.ClusterRoleBinding{},
		&admission.ValidatingWebhookConfiguration{},
	}
}

func (t *templateValidator) Reconcile(request *common.Request) ([]common.ResourceStatus, error) {
	return common.CollectResourceStatus(request,
		reconcileClusterRole,
		reconcileServiceAccount,
		reconcileClusterRoleBinding,
		reconcileService,
		reconcileDeployment,
		reconcileValidatingWebhook,
	)
}

func (t *templateValidator) Cleanup(request *common.Request) error {
	for _, obj := range []controllerutil.Object{
		newClusterRole(),
		newClusterRoleBinding(request.Namespace),
		newValidatingWebhook(request.Namespace),
	} {
		err := request.Client.Delete(request.Context, obj)
		if err != nil && !errors.IsNotFound(err) {
			request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", obj.GetName(), err))
			return err
		}
	}
	return nil
}

var _ operands.Operand = &templateValidator{}

func GetOperand() operands.Operand {
	return &templateValidator{}
}

const (
	operandName      = "template-validator"
	operandComponent = common.AppComponentTemplating
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

func reconcileService(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newService(request.Namespace)),
		func(newRes, foundRes controllerutil.Object) {
			newService := newRes.(*v1.Service)
			foundService := foundRes.(*v1.Service)

			// ClusterIP should not be updated
			newService.Spec.ClusterIP = foundService.Spec.ClusterIP

			foundService.Spec = newService.Spec
		})
}

func reconcileDeployment(request *common.Request) (common.ResourceStatus, error) {
	validatorSpec := request.Instance.Spec.TemplateValidator
	image := getTemplateValidatorImage()
	deployment := newDeployment(request.Namespace, *validatorSpec.Replicas, image)
	addPlacementFields(deployment, validatorSpec.Placement)
	return common.CreateOrUpdateResourceWithStatus(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, deployment),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*apps.Deployment).Spec = newRes.(*apps.Deployment).Spec
		},
		func(res controllerutil.Object) common.ResourceStatus {
			dep := res.(*apps.Deployment)
			status := common.ResourceStatus{}
			if *validatorSpec.Replicas > 0 && dep.Status.AvailableReplicas == 0 {
				msg := fmt.Sprintf("No validator pods are running. Expected: %d", dep.Status.Replicas)
				status.NotAvailable = &msg
			}
			if dep.Status.AvailableReplicas != *validatorSpec.Replicas {
				msg := fmt.Sprintf(
					"Not all template validator pods are running. Expected: %d, running: %d",
					*validatorSpec.Replicas,
					dep.Status.AvailableReplicas,
				)
				status.Progressing = &msg
				status.Degraded = &msg
			}
			return status
		})
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

func reconcileValidatingWebhook(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request,
		common.AddAppLabels(request.Instance, operandName, operandComponent, newValidatingWebhook(request.Namespace)),
		func(newRes, foundRes controllerutil.Object) {
			newWebhookConf := newRes.(*admission.ValidatingWebhookConfiguration)
			foundWebhookConf := foundRes.(*admission.ValidatingWebhookConfiguration)

			// Copy CA Bundle from the found webhook,
			// so it will not be overwritten
			copyFoundCaBundles(newWebhookConf.Webhooks, foundWebhookConf.Webhooks)

			foundWebhookConf.Webhooks = newWebhookConf.Webhooks
		})
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
