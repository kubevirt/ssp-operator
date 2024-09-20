package common_instancetypes

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=instancetype.kubevirt.io,resources=virtualmachineclusterinstancetypes,verbs=list;watch;delete
// +kubebuilder:rbac:groups=instancetype.kubevirt.io,resources=virtualmachineclusterpreferences,verbs=list;watch;delete

const (
	operandName                          = "common-instancetypes"
	operandComponent                     = common.AppComponentTemplating
	virtualMachineClusterInstancetypeCrd = instancetypeapi.ClusterPluralResourceName + "." + instancetypeapi.GroupName
	virtualMachineClusterPreferenceCrd   = instancetypeapi.ClusterPluralPreferenceResourceName + "." + instancetypeapi.GroupName
)

type CommonInstancetypes struct {
}

var _ operands.Operand = &CommonInstancetypes{}

func (c *CommonInstancetypes) Name() string {
	return operandName
}

func (c *CommonInstancetypes) WatchClusterTypes() []operands.WatchType {
	return nil
}

func (c *CommonInstancetypes) WatchTypes() []operands.WatchType {
	return nil
}

func New() *CommonInstancetypes {
	return &CommonInstancetypes{}
}

func (c *CommonInstancetypes) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	return c.cleanupReconcile(request)
}

func (c *CommonInstancetypes) cleanupReconcile(request *common.Request) ([]common.ReconcileResult, error) {
	cleanupResults, err := c.Cleanup(request)
	if err != nil {
		return nil, err
	}
	var results []common.ReconcileResult
	for _, cleanupResult := range cleanupResults {
		if !cleanupResult.Deleted {
			results = append(results, common.ResourceDeletedResult(cleanupResult.Resource, common.OperationResultDeleted))
		}
	}
	return results, nil
}

func (c *CommonInstancetypes) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	selector, err := appNameSelector(c.Name())
	if err != nil {
		return nil, err
	}
	listOpts := &client.ListOptions{
		LabelSelector: selector,
	}

	var allResults []common.CleanupResult

	if request.CrdList.CrdExists(virtualMachineClusterInstancetypeCrd) {
		results, err := common.CleanupResources[
			instancetypev1beta1.VirtualMachineClusterInstancetypeList,
			instancetypev1beta1.VirtualMachineClusterInstancetype,
		](request, listOpts)
		if err != nil {
			return nil, err
		}

		allResults = append(allResults, results...)
	}

	if request.CrdList.CrdExists(virtualMachineClusterPreferenceCrd) {
		results, err := common.CleanupResources[
			instancetypev1beta1.VirtualMachineClusterPreferenceList,
			instancetypev1beta1.VirtualMachineClusterPreference,
		](request, listOpts)
		if err != nil {
			return nil, err
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}

func appNameSelector(name string) (labels.Selector, error) {
	appNameRequirement, err := labels.NewRequirement(common.AppKubernetesNameLabel, selection.Equals, []string{name})
	if err != nil {
		return nil, err
	}
	return labels.NewSelector().Add(*appNameRequirement), nil
}
