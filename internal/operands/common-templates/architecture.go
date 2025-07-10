package common_templates

import (
	templatev1 "github.com/openshift/api/template/v1"

	"kubevirt.io/ssp-operator/internal/architecture"
)

func GetTemplateArch(template *templatev1.Template) (architecture.Arch, error) {
	templateArchLabel := template.Labels[TemplateArchitectureLabel]
	if templateArchLabel == "" {
		return TemplateDefaultArchitecture, nil
	}

	arch, err := architecture.ToArch(templateArchLabel)
	if err != nil {
		return "", err
	}

	return arch, nil
}
