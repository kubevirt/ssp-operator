/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhooks

import (
	"context"
	"fmt"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-ssp-kubevirt-io-v1beta2-ssp,mutating=false,failurePolicy=fail,groups=ssp.kubevirt.io,resources=ssps,versions=v1beta2,name=validation.ssp.kubevirt.io,admissionReviewVersions=v1,sideEffects=None

var ssplog = logf.Log.WithName("ssp-resource")

func Setup(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&sspv1beta2.SSP{}).
		WithValidator(newSspValidator(mgr.GetClient())).
		Complete()
}

type sspValidator struct {
	apiClient client.Client
}

var _ admission.CustomValidator = &sspValidator{}

func (s *sspValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sspObj, ok := obj.(*sspv1beta2.SSP)
	if !ok {
		return nil, fmt.Errorf("expected v1beta2.SSP object, got %T", obj)
	}

	var ssps sspv1beta2.SSPList

	// Check if no other SSP resources are present in the cluster
	ssplog.Info("validate create", "name", sspObj.Name)
	err := s.apiClient.List(ctx, &ssps, &client.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list SSPs for validation, please try again: %v", err)
	}
	if len(ssps.Items) > 0 {
		return nil, fmt.Errorf("creation failed, an SSP CR already exists in namespace %v: %v", ssps.Items[0].ObjectMeta.Namespace, ssps.Items[0].ObjectMeta.Name)
	}

	return s.validateSspObject(ctx, sspObj)
}

func (s *sspValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	newSsp, ok := newObj.(*sspv1beta2.SSP)
	if !ok {
		return nil, fmt.Errorf("expected v1beta2.SSP object, got %T", newObj)
	}

	ssplog.Info("validate update", "name", newSsp.Name)

	return s.validateSspObject(ctx, newSsp)
}

func (s *sspValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (s *sspValidator) validateSspObject(ctx context.Context, ssp *sspv1beta2.SSP) (admission.Warnings, error) {
	if err := s.validatePlacement(ctx, ssp); err != nil {
		return nil, fmt.Errorf("placement api validation error: %w", err)
	}

	if err := validateDataImportCronTemplates(ssp); err != nil {
		return nil, fmt.Errorf("dataImportCronTemplates validation error: %w", err)
	}

	return nil, nil
}

func (s *sspValidator) validatePlacement(ctx context.Context, ssp *sspv1beta2.SSP) error {
	if ssp.Spec.TemplateValidator == nil {
		return nil
	}
	return s.validateOperandPlacement(ctx, ssp.Namespace, ssp.Spec.TemplateValidator.Placement)
}

func (s *sspValidator) validateOperandPlacement(ctx context.Context, namespace string, placement *api.NodePlacement) error {
	if placement == nil {
		return nil
	}

	const (
		dplName          = "ssp-webhook-placement-verification-deployment"
		webhookTestLabel = "webhook.ssp.kubevirt.io/placement-verification-pod"
		podName          = "ssp-webhook-placement-verification-pod"
		naImage          = "ssp.kubevirt.io/not-available"
	)

	// Does a dry-run on a deployment creation to verify that placement fields are correct
	deployment := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dplName,
			Namespace: namespace,
		},
		Spec: apps.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					webhookTestLabel: "",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: podName,
					Labels: map[string]string{
						webhookTestLabel: "",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  podName,
							Image: naImage,
						},
					},
					// Inject placement fields here
					NodeSelector: placement.NodeSelector,
					Affinity:     placement.Affinity,
					Tolerations:  placement.Tolerations,
				},
			},
		},
	}

	return s.apiClient.Create(ctx, deployment, &client.CreateOptions{DryRun: []string{metav1.DryRunAll}})
}

// TODO: also validate DataImportCronTemplates in general once CDI exposes its own validation
func validateDataImportCronTemplates(ssp *sspv1beta2.SSP) error {
	for _, cron := range ssp.Spec.CommonTemplates.DataImportCronTemplates {
		if cron.Name == "" {
			return fmt.Errorf("missing name in DataImportCronTemplate")
		}
	}
	return nil
}

func newSspValidator(clt client.Client) *sspValidator {
	return &sspValidator{apiClient: clt}
}
