package common_templates

import templatev1 "github.com/openshift/api/template/v1"

func GetTemplateArch(template *templatev1.Template) string {
	templateArch := template.Labels[TemplateArchitectureLabel]
	if templateArch == "" {
		templateArch = TemplateDefaultArchitecture
	}
	return templateArch
}
