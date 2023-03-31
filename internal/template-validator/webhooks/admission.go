package validating

import (
	"bytes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/logger"
	"kubevirt.io/ssp-operator/internal/template-validator/validation"
)

func ValidateVm(rules []validation.Rule, vm *kubevirtv1.VirtualMachine) []metav1.StatusCause {
	if len(rules) == 0 {
		// no rules! everything is permitted, so let's bail out quickly
		logger.Log.V(8).Info("no admission rules", "vm", vm.Name)
		return nil
	}

	setDefaultValues(vm)

	buf := new(bytes.Buffer)
	ev := validation.Evaluator{Sink: buf}
	res := ev.Evaluate(rules, vm)
	logger.Log.V(2).Info("evaluation finished",
		"vm", vm.Name,
		"summary", buf.String(),
		"succeeded", res.Succeeded())

	return res.ToStatusCauses()
}

func setDefaultValues(vm *kubevirtv1.VirtualMachine) {
	vmSpec := vm.Spec.Template.Spec
	if vmSpec.Domain.CPU != nil {
		if vmSpec.Domain.CPU.Sockets == 0 {
			vmSpec.Domain.CPU.Sockets = 1
		}
		if vmSpec.Domain.CPU.Cores == 0 {
			vmSpec.Domain.CPU.Cores = 1
		}
		if vmSpec.Domain.CPU.Threads == 0 {
			vmSpec.Domain.CPU.Threads = 1
		}
	}
}
