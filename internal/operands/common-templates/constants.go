package common_templates

import "kubevirt.io/ssp-operator/internal/architecture"

const (
	TemplateVersionLabel         = "template.kubevirt.io/version"
	TemplateTypeLabel            = "template.kubevirt.io/type"
	TemplateTypeLabelBaseValue   = "base"
	TemplateArchitectureLabel    = "template.kubevirt.io/architecture"
	TemplateDefaultArchitecture  = architecture.AMD64
	TemplateOsLabelPrefix        = "os.template.kubevirt.io/"
	TemplateFlavorLabelPrefix    = "flavor.template.kubevirt.io/"
	TemplateWorkloadLabelPrefix  = "workload.template.kubevirt.io/"
	TemplateDeprecatedAnnotation = "template.kubevirt.io/deprecated"

	TemplateDataSourceParameterName = "DATA_SOURCE_NAME"
)
