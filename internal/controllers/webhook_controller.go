package controllers

import (
	"context"
	"slices"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
)

const (
	OlmNameLabel      = "olm.webhook-description-generate-name"
	OlmNameLabelValue = "validation.ssp.kubevirt.io"
)

// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;update

// CreateWebhookConfigurationController creates a controller
// that watches ValidatingWebhookConfiguration created by OLM,
// and removes any namespaceSelector defined in it.
//
// The OLM limits the webhook scope to the namespaces that are defined in the OperatorGroup
// by setting namespaceSelector in the ValidatingWebhookConfiguration.
// We would like our webhook to intercept requests from all namespaces.
//
// The SSP operator already watches all ValidatingWebhookConfigurations, because
// of template validator operand, so this controller is not a performance issue.
func NewWebhookConfigurationController(apiClient client.Client) ControllerReconciler {
	return &webhookCtrl{
		apiClient: apiClient,
	}
}

type webhookCtrl struct {
	apiClient client.Client
}

var _ ControllerReconciler = &webhookCtrl{}

var _ reconcile.Reconciler = &webhookCtrl{}

func (w *webhookCtrl) Start(_ context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(w.Name()).
		For(&admissionv1.ValidatingWebhookConfiguration{}, builder.WithPredicates(
			predicate.NewPredicateFuncs(hasExpectedLabel),
		)).
		Complete(w)
}

func (w *webhookCtrl) Name() string {
	return "validating-webhook-controller"
}

func (w *webhookCtrl) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	if err := w.apiClient.Get(ctx, request.NamespacedName, webhookConfig); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !hasExpectedLabel(webhookConfig) {
		return reconcile.Result{}, nil
	}

	if changed := updateWebhookConfiguration(webhookConfig); !changed {
		return reconcile.Result{}, nil
	}

	err := w.apiClient.Update(ctx, webhookConfig)
	return reconcile.Result{}, err
}

func hasExpectedLabel(obj client.Object) bool {
	return obj.GetLabels()[OlmNameLabel] == OlmNameLabelValue
}

func updateWebhookConfiguration(webhookConfig *admissionv1.ValidatingWebhookConfiguration) bool {
	var changed bool
	for i := range webhookConfig.Webhooks {
		webhook := &webhookConfig.Webhooks[i]
		if webhook.NamespaceSelector == nil {
			continue
		}
		// Check if the webhook reacts to SSP resource.
		var hasSspRule bool
		for _, rule := range webhook.Rules {
			if slices.Contains(rule.APIGroups, sspv1beta2.GroupVersion.Group) {
				hasSspRule = true
				break
			}
		}
		if !hasSspRule {
			continue
		}

		webhook.NamespaceSelector = nil
		changed = true
	}
	return changed
}
