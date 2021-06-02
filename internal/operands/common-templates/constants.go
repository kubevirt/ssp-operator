package common_templates

const (
	GoldenImagesNSname = "kubevirt-os-images"
	BundleDir          = "data/common-templates-bundle/"

	TemplateVersionLabel         = "template.kubevirt.io/version"
	TemplateTypeLabel            = "template.kubevirt.io/type"
	TemplateOsLabelPrefix        = "os.template.kubevirt.io/"
	TemplateFlavorLabelPrefix    = "flavor.template.kubevirt.io/"
	TemplateWorkloadLabelPrefix  = "workload.template.kubevirt.io/"
	TemplateDeprecatedAnnotation = "template.kubevirt.io/deprecated"

	CdiApiGroup   = "cdi.kubevirt.io"
	CdiApiVersion = "v1beta1"
)
