package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"
)

func ConvertSSP(src *sspv1beta3.SSP) *sspv1beta2.SSP {
	return &sspv1beta2.SSP{
		TypeMeta: metav1.TypeMeta{
			Kind:       src.Kind,
			APIVersion: sspv1beta2.GroupVersion.String(),
		},
		ObjectMeta: src.ObjectMeta,
		Spec: sspv1beta2.SSPSpec{
			TemplateValidator: convertTemplateValidator(src.Spec.TemplateValidator),
			CommonTemplates: sspv1beta2.CommonTemplates{
				Namespace:               src.Spec.CommonTemplates.Namespace,
				DataImportCronTemplates: convertDataImportCronTemplates(src.Spec.CommonTemplates.DataImportCronTemplates),
			},
			TLSSecurityProfile:     src.Spec.TLSSecurityProfile,
			TokenGenerationService: convertTokenGenerationService(src.Spec.TokenGenerationService),
		},
		Status: sspv1beta2.SSPStatus{
			Status:             src.Status.Status,
			Paused:             src.Status.Paused,
			ObservedGeneration: src.Status.ObservedGeneration,
		},
	}
}

func convertTemplateValidator(src *sspv1beta3.TemplateValidator) *sspv1beta2.TemplateValidator {
	if src == nil {
		return nil
	}

	return &sspv1beta2.TemplateValidator{
		Replicas:  src.Replicas,
		Placement: src.Placement,
	}
}

func convertDataImportCronTemplates(src []sspv1beta3.DataImportCronTemplate) []sspv1beta2.DataImportCronTemplate {
	if len(src) == 0 {
		return nil
	}

	result := make([]sspv1beta2.DataImportCronTemplate, 0, len(src))
	for i := range src {
		oldTemplate := &src[i]
		result = append(result, sspv1beta2.DataImportCronTemplate{
			ObjectMeta: oldTemplate.ObjectMeta,
			Spec:       oldTemplate.Spec,
		})
	}

	return result
}

func convertTokenGenerationService(src *sspv1beta3.TokenGenerationService) *sspv1beta2.TokenGenerationService {
	if src == nil {
		return nil
	}

	return &sspv1beta2.TokenGenerationService{
		Enabled: src.Enabled,
	}
}
