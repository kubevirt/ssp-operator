package template_validator

import (
	"fmt"
	"reflect"

	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/ssp-operator/internal/common"
	ssp "kubevirt.io/ssp-operator/pkg/apis/ssp/v1"
)

func WatchTypes() []runtime.Object {
	return []runtime.Object{
		&v1.ServiceAccount{},
		&v1.Service{},
		&apps.Deployment{},
	}
}

func WatchClusterTypes() []runtime.Object {
	return []runtime.Object{
		&rbac.ClusterRole{},
		&rbac.ClusterRoleBinding{},
		&admission.ValidatingWebhookConfiguration{},
	}
}

func Reconcile(request *common.Request) error {
	for _, f := range []func(*common.Request) error{
		reconcileClusterRole,
		reconcileServiceAccount,
		reconcileClusterRoleBinding,
		reconcileService,
		reconcileDeployment,
		reconcileValidatingWebhook,
	} {
		if err := f(request); err != nil {
			return err
		}
	}
	return nil
}

func Cleanup(request *common.Request) error {
	for _, obj := range []common.Resource{
		newClusterRole(request.Namespace),
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

func reconcileClusterRole(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request,
		newClusterRole(request.Namespace),
		&rbac.ClusterRole{},
		func(newRes common.Resource, foundRes common.Resource) bool {
			newRole := newRes.(*rbac.ClusterRole)
			foundRole := foundRes.(*rbac.ClusterRole)
			if !reflect.DeepEqual(newRole.Rules, foundRole.Rules) {
				foundRole.Rules = newRole.Rules
				return true
			}
			return false
		})
}

func reconcileServiceAccount(request *common.Request) error {
	return common.CreateOrUpdateResource(request,
		newServiceAccount(request.Namespace),
		&v1.ServiceAccount{},
		common.NoUpdate)
}

func reconcileClusterRoleBinding(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request,
		newClusterRoleBinding(request.Namespace),
		&rbac.ClusterRoleBinding{},
		func(newRes common.Resource, foundRes common.Resource) bool {
			newBinding := newRes.(*rbac.ClusterRoleBinding)
			foundBinding := foundRes.(*rbac.ClusterRoleBinding)
			if !reflect.DeepEqual(newBinding.RoleRef, foundBinding.RoleRef) ||
				!reflect.DeepEqual(newBinding.Subjects, foundBinding.Subjects) {
				foundBinding.RoleRef = newBinding.RoleRef
				foundBinding.Subjects = newBinding.Subjects
				return true
			}
			return false
		})
}

func reconcileService(request *common.Request) error {
	return common.CreateOrUpdateResource(request,
		newService(request.Namespace),
		&v1.Service{},
		func(newRes common.Resource, foundRes common.Resource) bool {
			newService := newRes.(*v1.Service)
			foundService := foundRes.(*v1.Service)

			// ClusterIP should not be updated
			newService.Spec.ClusterIP = foundService.Spec.ClusterIP

			if !reflect.DeepEqual(newService.Spec, foundService.Spec) {
				foundService.Spec = newService.Spec
				return true
			}
			return false
		})
}

func reconcileDeployment(request *common.Request) error {
	validatorSpec := &request.Instance.Spec.TemplateValidator
	image := common.GetTemplateValidatorImage()
	deployment := newDeployment(request.Namespace, validatorSpec.Replicas, image)
	addPlacementFields(deployment, validatorSpec)
	return common.CreateOrUpdateResource(request,
		deployment,
		&apps.Deployment{},
		func(newRes common.Resource, foundRes common.Resource) bool {
			newDep := newRes.(*apps.Deployment)
			foundDep := foundRes.(*apps.Deployment)
			if !reflect.DeepEqual(newDep.Spec, foundDep.Spec) {
				foundDep.Spec = newDep.Spec
				return true
			}
			return false
		})
}

func addPlacementFields(deployment *apps.Deployment, validatorSpec *ssp.TemplateValidator) {
	podSpec := &deployment.Spec.Template.Spec
	podSpec.Affinity = validatorSpec.Affinity
	podSpec.NodeSelector = validatorSpec.NodeSelector
	podSpec.Tolerations = validatorSpec.Tolerations
}

func reconcileValidatingWebhook(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request,
		newValidatingWebhook(request.Namespace),
		&admission.ValidatingWebhookConfiguration{},
		func(newRes common.Resource, foundRes common.Resource) bool {
			newWebhookConf := newRes.(*admission.ValidatingWebhookConfiguration)
			foundWebhookConf := foundRes.(*admission.ValidatingWebhookConfiguration)

			// Copy CA Bundle from the found webhook,
			// so it will not be overwritten
			copyFoundCaBundles(newWebhookConf.Webhooks, foundWebhookConf.Webhooks)

			if !reflect.DeepEqual(newWebhookConf.Webhooks, foundWebhookConf.Webhooks) {
				foundWebhookConf.Webhooks = newWebhookConf.Webhooks
				return true
			}

			return false
		})
}

func copyFoundCaBundles(newWebhooks []admission.ValidatingWebhook, foundWebhooks []admission.ValidatingWebhook) {
	for i, _ := range newWebhooks {
		newWebhook := &newWebhooks[i]
		for j, _ := range foundWebhooks {
			foundWebhook := &foundWebhooks[j]
			if newWebhook.Name == foundWebhook.Name {
				newWebhook.ClientConfig.CABundle = foundWebhook.ClientConfig.CABundle
				break
			}
		}
	}
}
