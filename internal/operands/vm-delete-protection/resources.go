package vm_delete_protection

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	kubevirt "kubevirt.io/api/core"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const vmDeleteProtectionCELExpression = `(!has(oldObject.metadata.labels) || !(variables.label in oldObject.metadata.labels) || !oldObject.metadata.labels[variables.label].matches('^(true|True)$'))`

func newValidatingAdmissionPolicy() *admissionregistrationv1.ValidatingAdmissionPolicy {
	var apiVersions []string
	for _, version := range kubevirtv1.ApiSupportedVersions {
		apiVersions = append(apiVersions, version.Name)
	}

	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualMachineDeleteProtectionPolicyName,
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Delete,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{kubevirt.GroupName},
								APIVersions: apiVersions,
								Resources:   []string{"virtualmachines"},
							},
						},
					},
				},
			},
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "label",
					Expression: `string('kubevirt.io/vm-delete-protection')`,
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression:        vmDeleteProtectionCELExpression,
					MessageExpression: `'VirtualMachine ' + string(oldObject.metadata.name) + ' cannot be deleted, remove delete protection'`,
				},
			},
		},
	}
}

func newValidatingAdmissionPolicyBinding() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualMachineDeleteProtectionPolicyName,
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: virtualMachineDeleteProtectionPolicyName,
			ValidationActions: []admissionregistrationv1.ValidationAction{
				admissionregistrationv1.Deny,
			},
		},
	}
}
