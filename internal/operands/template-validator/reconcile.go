package template_validator

import (
	"fmt"

	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"kubevirt.io/ssp-operator/api/v1beta1"
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

func WatchTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &v1.ServiceAccount{}},
		{Object: &v1.Service{}},
		{Object: &apps.Deployment{}, WatchFullObject: true},
	}
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.ClusterRoleBinding{}},
		{Object: &admission.ValidatingWebhookConfiguration{}},
	}
}

type templateValidator struct{}

func (t *templateValidator) Name() string {
	return operandName
}

func (t *templateValidator) WatchTypes() []operands.WatchType {
	return WatchTypes()
}

func (t *templateValidator) WatchClusterTypes() []operands.WatchType {
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
		reconcilePrometheusService,
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
		UpdateFunc(updateServiceSpec).
		Reconcile()
}

func updateServiceSpec(newRes, foundRes client.Object) {
	newService := newRes.(*v1.Service)
	foundService := foundRes.(*v1.Service)

	// ClusterIP should not be updated
	newService.Spec.ClusterIP = foundService.Spec.ClusterIP

	foundService.Spec = newService.Spec
}

func reconcilePrometheusService(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		NamespacedResource(newPrometheusService(request.Namespace)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(updateServiceSpec).
		Reconcile()
}

func reconcileDeployment(request *common.Request) (common.ReconcileResult, error) {
	image := getTemplateValidatorImage()
	if image == "" {
		panic("Cannot reconcile without valid image name")
	}
	numberOfReplicas := int32(1)
	validatorSpec := request.Instance.Spec.TemplateValidator
	if validatorSpec != nil && validatorSpec.Replicas != nil {
		numberOfReplicas = *validatorSpec.Replicas
		if request.IsSingleReplicaTopologyMode() && (numberOfReplicas > 1) {
			numberOfReplicas = 1
		}
	}

	sspTLSOptions, err := common.NewSSPTLSOptions(request.Instance.Spec.TLSSecurityProfile, nil)
	if err != nil {
		return common.ReconcileResult{}, err
	}

	deployment := newDeployment(request.Namespace, numberOfReplicas, image, sspTLSOptions)
	injectPlacementMetadata(&deployment.Spec.Template.Spec, validatorSpec)
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

// Merge all Tolerations, Affinity and NodeSelectors from NodePlacement into pod spec
func injectPlacementMetadata(podSpec *v1.PodSpec, componentConfig *v1beta1.TemplateValidator) {
	if componentConfig == nil || componentConfig.Placement == nil {
		return
	}
	if podSpec == nil {
		podSpec = &v1.PodSpec{}
	}
	nodePlacement := componentConfig.Placement
	if podSpec.NodeSelector == nil {
		podSpec.NodeSelector = make(map[string]string, len(nodePlacement.NodeSelector))
	}
	for nsKey, nsVal := range nodePlacement.NodeSelector {
		// Favor podSpec over NodePlacement. This prevents cluster admin from clobbering
		// node selectors that operator intentionally set.
		if _, ok := podSpec.NodeSelector[nsKey]; !ok {
			podSpec.NodeSelector[nsKey] = nsVal
		}
	}

	if nodePlacement.Affinity != nil {
		if podSpec.Affinity == nil {
			podSpec.Affinity = nodePlacement.Affinity.DeepCopy()
		} else {
			mergeNodeAffinity(podSpec.Affinity, nodePlacement.Affinity.NodeAffinity)
			mergePodAffinity(podSpec.Affinity, nodePlacement.Affinity.PodAffinity)
			mergePodAntiAffinity(podSpec.Affinity, nodePlacement.Affinity.PodAntiAffinity)
		}
	}
	podSpec.Tolerations = append(podSpec.Tolerations, nodePlacement.Tolerations...)
}

func mergeNodeAffinity(currentAffinity *v1.Affinity, configNodeAffinity *v1.NodeAffinity) {
	if configNodeAffinity != nil {
		if currentAffinity.NodeAffinity == nil {
			currentAffinity.NodeAffinity = configNodeAffinity.DeepCopy()
			return
		}
		if configNodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			if currentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				currentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = configNodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.DeepCopy()
			} else {
				currentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(currentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, configNodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms...)
			}
		}
		currentAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(currentAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, configNodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}
}

func mergePodAffinity(currentAffinity *v1.Affinity, configPodAffinity *v1.PodAffinity) {
	if configPodAffinity != nil {
		if currentAffinity.PodAffinity == nil {
			currentAffinity.PodAffinity = configPodAffinity.DeepCopy()
			return
		}
		currentAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(currentAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, configPodAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
		currentAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(currentAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, configPodAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}
}

func mergePodAntiAffinity(currentAffinity *v1.Affinity, configPodAntiAffinity *v1.PodAntiAffinity) {
	if configPodAntiAffinity != nil {
		if currentAffinity.PodAntiAffinity == nil {
			currentAffinity.PodAntiAffinity = configPodAntiAffinity.DeepCopy()
			return
		}
		currentAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(currentAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, configPodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
		currentAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(currentAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, configPodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}
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
