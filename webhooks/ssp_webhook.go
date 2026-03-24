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
	"k8s.io/utils/ptr"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/architecture"
	"kubevirt.io/ssp-operator/webhooks/convert"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-ssp-kubevirt-io-v1beta2-ssp,mutating=false,failurePolicy=fail,groups=ssp.kubevirt.io,resources=ssps,versions=v1beta2,name=validation.v1beta2.ssp.kubevirt.io,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:webhook:verbs=create;update,path=/validate-ssp-kubevirt-io-v1beta3-ssp,mutating=false,failurePolicy=fail,groups=ssp.kubevirt.io,resources=ssps,versions=v1beta3,name=validation.v1beta3.ssp.kubevirt.io,admissionReviewVersions=v1,sideEffects=None

var ssplog = logf.Log.WithName("ssp-resource")

func Setup(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr, &sspv1beta2.SSP{}).
		WithValidator(newSspValidatorV1beta2(mgr.GetClient())).
		Complete()
	if err != nil {
		return fmt.Errorf("failed to create webhook for v1beta2.SSP")
	}

	err = ctrl.NewWebhookManagedBy(mgr, &sspv1beta3.SSP{}).
		WithValidator(newSspValidator(mgr.GetClient())).
		Complete()
	if err != nil {
		return fmt.Errorf("failed to create webhook for v1beta3.SSP")
	}

	return nil
}

type sspValidator struct {
	apiClient client.Client
}

var _ admission.Validator[*sspv1beta3.SSP] = &sspValidator{}

func (s *sspValidator) ValidateCreate(ctx context.Context, obj *sspv1beta3.SSP) (admission.Warnings, error) {
	var ssps sspv1beta3.SSPList

	// Check if no other SSP resources are present in the cluster
	ssplog.Info("validate create", "name", obj.Name)
	err := s.apiClient.List(ctx, &ssps, &client.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list SSPs for validation, please try again: %v", err)
	}
	if len(ssps.Items) > 0 {
		return nil, fmt.Errorf("creation failed, an SSP CR already exists in namespace %v: %v", ssps.Items[0].Namespace, ssps.Items[0].Name)
	}

	return s.validateSspObject(ctx, obj)
}

func (s *sspValidator) ValidateUpdate(ctx context.Context, _, newObj *sspv1beta3.SSP) (admission.Warnings, error) {
	ssplog.Info("validate update", "name", newObj.Name)

	return s.validateSspObject(ctx, newObj)
}

func (s *sspValidator) ValidateDelete(_ context.Context, _ *sspv1beta3.SSP) (admission.Warnings, error) {
	return nil, nil
}

func (s *sspValidator) validateSspObject(ctx context.Context, ssp *sspv1beta3.SSP) (admission.Warnings, error) {
	if err := validateCluster(ssp); err != nil {
		return nil, fmt.Errorf("cluster validation error: %w", err)
	}

	if err := validateDataImportCronTemplates(ssp); err != nil {
		return nil, fmt.Errorf("dataImportCronTemplates validation error: %w", err)
	}

	if err := s.validatePlacement(ctx, ssp); err != nil {
		return nil, fmt.Errorf("placement api validation error: %w", err)
	}

	return nil, nil
}

func validateCluster(ssp *sspv1beta3.SSP) error {
	if ptr.Deref(ssp.Spec.EnableMultipleArchitectures, false) && ssp.Spec.Cluster == nil {
		return fmt.Errorf(".spec.cluster needs to be non-nil, if multi-architecture is enabled")
	}

	cluster := ssp.Spec.Cluster
	if cluster != nil {
		if len(cluster.WorkloadArchitectures) == 0 && len(cluster.ControlPlaneArchitectures) == 0 {
			return fmt.Errorf("at least one architecture needs to be defined, if multi-architecture is enabled")
		}

		for _, archStr := range cluster.WorkloadArchitectures {
			if _, err := architecture.ToArch(archStr); err != nil {
				return fmt.Errorf("invalid workload architecture: %w", err)
			}
		}

		for _, archStr := range cluster.ControlPlaneArchitectures {
			if _, err := architecture.ToArch(archStr); err != nil {
				return fmt.Errorf("invalid control plane architecture: %w", err)
			}
		}
	}

	return nil
}

func (s *sspValidator) validatePlacement(ctx context.Context, ssp *sspv1beta3.SSP) error {
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
func validateDataImportCronTemplates(ssp *sspv1beta3.SSP) error {
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

func newSspValidatorV1beta2(clt client.Client) *sspValidatorV1beta2 {
	return &sspValidatorV1beta2{newSspValidator(clt)}
}

type sspValidatorV1beta2 struct {
	admission.Validator[*sspv1beta3.SSP]
}

func (s *sspValidatorV1beta2) ValidateCreate(ctx context.Context, obj *sspv1beta2.SSP) (warnings admission.Warnings, err error) {
	return s.Validator.ValidateCreate(ctx, convert.ConvertSSP(obj))
}

func (s *sspValidatorV1beta2) ValidateUpdate(ctx context.Context, oldObj, newObj *sspv1beta2.SSP) (warnings admission.Warnings, err error) {
	return s.Validator.ValidateUpdate(ctx, convert.ConvertSSP(oldObj), convert.ConvertSSP(newObj))
}

func (s *sspValidatorV1beta2) ValidateDelete(ctx context.Context, obj *sspv1beta2.SSP) (warnings admission.Warnings, err error) {
	return s.Validator.ValidateDelete(ctx, convert.ConvertSSP(obj))
}

var _ admission.Validator[*sspv1beta2.SSP] = &sspValidatorV1beta2{}
