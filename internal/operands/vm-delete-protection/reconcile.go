package vm_delete_protection

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingadmissionpolicies;validatingadmissionpolicybindings,verbs=get;list;create;watch;update;delete

const (
	operandName                              = "vm-delete-protection"
	operandComponent                         = common.AppComponentVMDeletionProtection
	virtualMachineDeleteProtectionPolicyName = "kubevirt-vm-deletion-protection"
)

func init() {
	utilruntime.Must(admissionregistrationv1.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &admissionregistrationv1.ValidatingAdmissionPolicy{}},
		{Object: &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}},
	}
}

type VMDeleteProtection struct{}

var _ operands.Operand = &VMDeleteProtection{}

func New() operands.Operand { return &VMDeleteProtection{} }

func (v *VMDeleteProtection) WatchTypes() []operands.WatchType { return nil }

func (v *VMDeleteProtection) WatchClusterTypes() []operands.WatchType { return WatchClusterTypes() }

func (v *VMDeleteProtection) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	return common.CollectResourceStatus(request,
		reconcileVAP,
		reconcileVAPB,
	)
}

func (v *VMDeleteProtection) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	return common.DeleteAll(request,
		newValidatingAdmissionPolicy(),
		newValidatingAdmissionPolicyBinding(),
	)
}

func (v *VMDeleteProtection) Name() string { return operandName }

func reconcileVAP(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newValidatingAdmissionPolicy()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(expected, found client.Object) {
			foundVAP := found.(*admissionregistrationv1.ValidatingAdmissionPolicy)
			expectedVAP := expected.(*admissionregistrationv1.ValidatingAdmissionPolicy)

			foundVAP.Spec = expectedVAP.Spec
		}).
		StatusFunc(func(resource client.Object) common.ResourceStatus {
			vap := resource.(*admissionregistrationv1.ValidatingAdmissionPolicy)
			if vap.Status.TypeChecking == nil {
				msg := fmt.Sprintf("Delete protection VAP type checking in progress")
				return common.ResourceStatus{
					Progressing:  &msg,
					NotAvailable: &msg,
					Degraded:     &msg,
				}
			}

			if len(vap.Status.TypeChecking.ExpressionWarnings) != 0 {
				msg := fmt.Sprintf("Incorrect VM delete protection VAP CEL expression %v",
					vap.Status.TypeChecking)
				return common.ResourceStatus{
					NotAvailable: &msg,
					Degraded:     &msg,
				}
			}
			return common.ResourceStatus{}
		}).
		Reconcile()
}

func reconcileVAPB(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newValidatingAdmissionPolicyBinding()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(expected, found client.Object) {
			foundVAPB := found.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding)
			expectedVAPB := expected.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding)

			foundVAPB.Spec = expectedVAPB.Spec
		}).
		Reconcile()
}
