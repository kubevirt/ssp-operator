package labels

import (
	"fmt"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnotationTemplateNameKey         string = "vm.kubevirt.io/template"
	AnnotationTemplateNamespaceKey    string = "vm.kubevirt.io/template.namespace"
	AnnotationTemplateNamespaceOldKey string = "vm.kubevirt.io/template-namespace"
	AnnotationValidationKey           string = "validations"

	// This is the new annotation we will be using for VirtualMachines that carry their own validation rules
	VmValidationAnnotationKey string = "vm.kubevirt.io/validations"

	// If this annotation exists on a VM, it means that validation should be skipped.
	// This annotation is used for troubleshooting, debugging and experimenting with templated VMs.
	VmSkipValidationAnnotationKey string = "vm.kubevirt.io/skip-validations"
)

type TemplateKey struct {
	Name         string
	Namespace    string
	OldNamespace string
}

func (t *TemplateKey) String() string {
	if !t.IsValid() {
		return ""
	}
	return fmt.Sprintf("%s/%s", t.AnyNamespace(), t.Name)
}

func (t *TemplateKey) IsValid() bool {
	return t.Name != "" && t.AnyNamespace() != ""
}

func (t *TemplateKey) AnyNamespace() string {
	if t.Namespace != "" {
		return t.Namespace
	}
	return t.OldNamespace
}

type TemplateKeys struct {
	LabelKey      TemplateKey
	AnnotationKey TemplateKey
}

func (t *TemplateKeys) Get() *TemplateKey {
	if t.LabelKey.IsValid() {
		return &t.LabelKey
	}
	return &t.AnnotationKey
}

func (t *TemplateKeys) IsValid() bool {
	return t.LabelKey.IsValid() || t.AnnotationKey.IsValid()
}

func getTemplateKeyFromMap(targetMap map[string]string) TemplateKey {
	if len(targetMap) == 0 {
		return TemplateKey{}
	}

	return TemplateKey{
		Name:         targetMap[AnnotationTemplateNameKey],
		Namespace:    targetMap[AnnotationTemplateNamespaceKey],
		OldNamespace: targetMap[AnnotationTemplateNamespaceOldKey],
	}
}

func GetTemplateKeys(obj meta.Object) TemplateKeys {
	return TemplateKeys{
		LabelKey:      getTemplateKeyFromMap(obj.GetLabels()),
		AnnotationKey: getTemplateKeyFromMap(obj.GetAnnotations()),
	}
}
