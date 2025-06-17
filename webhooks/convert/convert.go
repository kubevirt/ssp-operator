package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"
)

func ConvertSSP(src *sspv1beta2.SSP) *sspv1beta3.SSP {
	return &sspv1beta3.SSP{
		TypeMeta: metav1.TypeMeta{
			Kind:       src.Kind,
			APIVersion: sspv1beta3.GroupVersion.String(),
		},
		ObjectMeta: src.ObjectMeta,
		Spec: sspv1beta3.SSPSpec{
			TemplateValidator: convertTemplateValidator(src.Spec.TemplateValidator),
			CommonTemplates: sspv1beta3.CommonTemplates{
				Namespace:               src.Spec.CommonTemplates.Namespace,
				DataImportCronTemplates: convertDataImportCronTemplates(src.Spec.CommonTemplates.DataImportCronTemplates),
			},
			TLSSecurityProfile:     src.Spec.TLSSecurityProfile,
			TokenGenerationService: convertTokenGenerationService(src.Spec.TokenGenerationService),
		},
		Status: sspv1beta3.SSPStatus{
			Status:             src.Status.Status,
			Paused:             src.Status.Paused,
			ObservedGeneration: src.Status.ObservedGeneration,
		},
	}
}

func convertTemplateValidator(src *sspv1beta2.TemplateValidator) *sspv1beta3.TemplateValidator {
	if src == nil {
		return nil
	}

	return &sspv1beta3.TemplateValidator{
		Replicas:  src.Replicas,
		Placement: src.Placement,
	}
}

func convertDataImportCronTemplates(src []sspv1beta2.DataImportCronTemplate) []sspv1beta3.DataImportCronTemplate {
	if len(src) == 0 {
		return nil
	}

	result := make([]sspv1beta3.DataImportCronTemplate, 0, len(src))
	for i := range src {
		oldTemplate := &src[i]
		result = append(result, sspv1beta3.DataImportCronTemplate{
			ObjectMeta: oldTemplate.ObjectMeta,
			Spec:       oldTemplate.Spec,
		})
	}

	return result
}

func convertTokenGenerationService(src *sspv1beta2.TokenGenerationService) *sspv1beta3.TokenGenerationService {
	if src == nil {
		return nil
	}

	return &sspv1beta3.TokenGenerationService{
		Enabled: src.Enabled,
	}
}
