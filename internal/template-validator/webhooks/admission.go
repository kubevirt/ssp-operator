package validating

import (
	"bytes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/log"

	"kubevirt.io/ssp-operator/internal/template-validator/validation"
)

func ValidateVMTemplate(rules []validation.Rule, newVM, oldVM *kubevirtv1.VirtualMachine) []metav1.StatusCause {
	var causes []metav1.StatusCause
	if len(rules) == 0 {
		// no rules! everything is permitted, so let's bail out quickly
		log.Log.V(8).Infof("no admission rules for: %s", newVM.Name)
		return causes
	}

	setDefaultValues(newVM)

	buf := new(bytes.Buffer)
	ev := validation.Evaluator{Sink: buf}
	res := ev.Evaluate(rules, newVM)
	log.Log.V(2).Infof("evalution summary for %s:\n%s\nsucceeded=%v", newVM.Name, buf.String(), res.Succeeded())

	if res.Succeeded() {
		return causes
	}
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
