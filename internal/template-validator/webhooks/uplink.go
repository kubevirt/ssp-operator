package validating

import (
	"fmt"

	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/client-go/tools/cache"
	k6tv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/labels"
	"kubevirt.io/ssp-operator/internal/template-validator/logger"
	"kubevirt.io/ssp-operator/internal/template-validator/validation"
)

func getParentTemplateForVM(vm *k6tv1.VirtualMachine, templateGetter cache.KeyGetter) (*templatev1.Template, error) {
	templateKeys := labels.GetTemplateKeys(vm)
	logVmTemplateKeys(vm.Name, templateKeys)
	if !templateKeys.IsValid() {
		logger.Log.V(8).Info("detected VM as baked (no parent template)", "vm", vm.Name)
		return nil, nil
	}

	cacheKey := templateKeys.Get().String()
	obj, exists, err := templateGetter.GetByKey(cacheKey)
	if err != nil {
		logger.Log.V(8).Info("parent template not found",
			"key", cacheKey,
			"vm", vm.Name,
			"error", err)
		return nil, err
	}

	if !exists {
		logger.Log.V(4).Info("Missing parent template", "key", cacheKey, "vm", vm.Name)
		return nil, nil
	}

	logger.Log.V(8).Info("found parent template for VM", "vm", vm.Name)
	tmpl := obj.(*templatev1.Template)
	// We must copy what is retrieved from the cache to allow modifying it.
	// Modifying tmpl without DeepCopy would break the cache on modification.
	// Ref: vendor/k8s.io/client-go/tools/cache.ThreadSafeStore
	return tmpl.DeepCopy(), nil
}

func logVmTemplateKeys(vmName string, templateKeys labels.TemplateKeys) {
	logVmTemplateKey(vmName, "labels", templateKeys.LabelKey)
	if !templateKeys.LabelKey.IsValid() {
		logVmTemplateKey(vmName, "annotations", templateKeys.AnnotationKey)
	}
}

func logVmTemplateKey(vmName string, targetName string, templateKey labels.TemplateKey) {
	if templateKey.OldNamespace != "" {
		logger.Log.V(5).Info(fmt.Sprintf("VM %s has old-style template namespace %s '%s', should be updated to '%s'", vmName, targetName, labels.AnnotationTemplateNamespaceOldKey, labels.AnnotationTemplateNamespaceKey))
	}
	if templateKey.AnyNamespace() == "" {
		logger.Log.V(4).Info(fmt.Sprintf("VM %s missing template namespace %s", vmName, targetName))
	}
	if templateKey.Name == "" {
		logger.Log.V(4).Info(fmt.Sprintf("VM %s missing template %s", vmName, targetName))
	}
}

func getValidationRulesFromTemplate(tmpl *templatev1.Template) ([]validation.Rule, error) {
	return validation.ParseRules([]byte(tmpl.Annotations[labels.AnnotationValidationKey]))
}

func getValidationRulesFromVM(vm *k6tv1.VirtualMachine) ([]validation.Rule, error) {
	return validation.ParseRules([]byte(vm.Annotations[labels.VmValidationAnnotationKey]))
}

func getValidationRulesForVM(vm *k6tv1.VirtualMachine, templateGetter cache.KeyGetter) ([]validation.Rule, error) {
	// If the VM has the 'vm.kubevirt.io/skip-validations' annotations, skip validation
	if _, skip := vm.Annotations[labels.VmSkipValidationAnnotationKey]; skip {
		logger.Log.V(8).Info(fmt.Sprintf("skipped validation for VM [%s] in namespace [%s]", vm.Name, vm.Namespace))
		return []validation.Rule{}, nil
	}

	// If the VM has the 'vm.kubevirt.io/validations' annotation applied, we will use the validation rules
	// it contains instead of the validation rules from the template.
	if vm.Annotations[labels.VmValidationAnnotationKey] != "" {
		return getValidationRulesFromVM(vm)
	}

	tmpl, err := getParentTemplateForVM(vm, templateGetter)
	if tmpl == nil || err != nil {
		// no template resources (kubevirt deployed on kubernetes, not OKD/OCP) or
		// no parent template for this VM. In either case, we have nothing to do,
		// and err is automatically correct
		return nil, err
	}
	return getValidationRulesFromTemplate(tmpl)
}
