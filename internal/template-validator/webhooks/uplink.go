package validating

import (
	"fmt"

	templatev1 "github.com/openshift/api/template/v1"
	k6tv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/log"
	"kubevirt.io/ssp-operator/internal/template-validator/validation"
	"kubevirt.io/ssp-operator/internal/template-validator/virtinformers"
)

const (
	annotationTemplateNameKey         string = "vm.kubevirt.io/template"
	annotationTemplateNamespaceKey    string = "vm.kubevirt.io/template.namespace"
	annotationTemplateNamespaceOldKey string = "vm.kubevirt.io/template-namespace"
	annotationValidationKey           string = "validations"

	// This is the new annotation we will be using for VirtualMachines that carry their own validation rules
	vmValidationAnnotationKey string = "vm.kubevirt.io/validations"

	// If this annotation exists on a VM, it means that validation should be skipped.
	// This annotation is used for troubleshooting, debugging and experimenting with templated VMs.
	vmSkipValidationAnnotationKey string = "vm.kubevirt.io/skip-validations"
)

func getTemplateKeyFromMap(vmName, targetName string, targetMap map[string]string) (string, bool) {
	if targetMap == nil {
		log.Log.V(4).Infof("VM %s missing %s entirely", vmName, targetName)
		return "", false
	}

	templateNamespace := targetMap[annotationTemplateNamespaceKey]
	if templateNamespace == "" {
		templateNamespace = targetMap[annotationTemplateNamespaceOldKey]
		if templateNamespace != "" {
			log.Log.V(5).Warningf("VM %s has old-style template namespace %s '%s', should be updated to '%s'", vmName, targetName, annotationTemplateNamespaceOldKey, annotationTemplateNamespaceKey)
		}
	}

	if templateNamespace == "" {
		log.Log.V(4).Infof("VM %s missing template namespace %s", vmName, targetName)
		return "", false
	}

	templateName := targetMap[annotationTemplateNameKey]
	if templateName == "" {
		log.Log.V(4).Infof("VM %s missing template %s", vmName, targetName)
		return "", false
	}

	return fmt.Sprintf("%s/%s", templateNamespace, templateName), true
}

func getTemplateKey(vm *k6tv1.VirtualMachine) (string, bool) {
	var cacheKey string
	var ok bool

	cacheKey, ok = getTemplateKeyFromMap(vm.Name, "labels", vm.Labels)
	if !ok {
		cacheKey, ok = getTemplateKeyFromMap(vm.Name, "annotations", vm.Annotations)
	}
	return cacheKey, ok
}

func getParentTemplateForVM(vm *k6tv1.VirtualMachine) (*templatev1.Template, error) {
	informers := virtinformers.GetInformers()

	if !informers.Available() {
		log.Log.V(8).Infof("no informer available (deployed on K8S?)")
		return nil, nil
	}

	cacheKey, ok := getTemplateKey(vm)
	if !ok {
		log.Log.V(8).Infof("detected %s as baked (no parent template)", vm.Name)
		return nil, nil
	}

	obj, exists, err := informers.TemplateInformer.GetStore().GetByKey(cacheKey)
	if err != nil {
		log.Log.V(8).Infof("parent template (key=%s) not found for %s: %v", cacheKey, vm.Name, err)
		return nil, err
	}

	if !exists {
		msg := fmt.Sprintf("missing parent template (key=%s) for %s", cacheKey, vm.Name)
		log.Log.V(4).Warning(msg)
		return nil, fmt.Errorf("%s", msg)
	}

	log.Log.V(8).Infof("found parent template for %s", vm.Name)
	tmpl := obj.(*templatev1.Template)
	// TODO explain deepcopy
	return tmpl.DeepCopy(), nil
}

func getValidationRulesFromTemplate(tmpl *templatev1.Template) ([]validation.Rule, error) {
	return validation.ParseRules([]byte(tmpl.Annotations[annotationValidationKey]))
}

func getValidationRulesFromVM(vm *k6tv1.VirtualMachine) ([]validation.Rule, error) {
	return validation.ParseRules([]byte(vm.Annotations[vmValidationAnnotationKey]))
}

func getValidationRulesForVM(vm *k6tv1.VirtualMachine) ([]validation.Rule, error) {
	// If the VM has the 'vm.kubevirt.io/skip-validations' annotations, skip validation
	if _, skip := vm.Annotations[vmSkipValidationAnnotationKey]; skip {
		log.Log.V(8).Infof("skipped validation for VM [%s] in namespace [%s]", vm.Name, vm.Namespace)
		return []validation.Rule{}, nil
	}

	// If the VM has the 'vm.kubevirt.io/validations' annotation applied, we will use the validation rules
	// it contains instead of the validation rules from the template.
	if vm.Annotations[vmValidationAnnotationKey] != "" {
		return getValidationRulesFromVM(vm)
	}

	tmpl, err := getParentTemplateForVM(vm)
	if tmpl == nil || err != nil {
		// no template resources (kubevirt deployed on kubernetes, not OKD/OCP) or
		// no parent template for this VM. In either case, we have nothing to do,
		// and err is automatically correct
		return []validation.Rule{}, err
	}
	return getValidationRulesFromTemplate(tmpl)
}
