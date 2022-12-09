package common_instancetypes

import (
	"bytes"
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/util/yaml"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=instancetype.kubevirt.io,resources=virtualmachineclusterinstancetypes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=instancetype.kubevirt.io,resources=virtualmachineclusterpreferences,verbs=get;list;watch;create;update;patch;delete

const (
	operandName                      = "common-instancetypes"
	operandComponent                 = common.AppComponentTemplating
	BundleDir                        = "data/common-instancetypes-bundle/"
	ClusterInstancetypesBundlePrefix = "common-clusterinstancetypes-bundle"
	ClusterPreferencesBundlePrefix   = "common-clusterpreferences-bundle"
)

type commonInstancetypes struct {
	virtualMachineClusterInstancetypes []instancetypev1alpha2.VirtualMachineClusterInstancetype
	virtualMachineClusterPreferences   []instancetypev1alpha2.VirtualMachineClusterPreference
}

var _ operands.Operand = &commonInstancetypes{}

type clusterType interface {
	instancetypev1alpha2.VirtualMachineClusterInstancetype | instancetypev1alpha2.VirtualMachineClusterPreference
}

func fetchClusterResources[C clusterType](bundlePath string) ([]C, error) {
	file, err := ioutil.ReadFile(bundlePath)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(file), 1024)
	var bundle []C
	for {
		resource := new(C)
		err = decoder.Decode(resource)
		if err == io.EOF {
			return bundle, nil
		}
		if err != nil {
			return nil, err
		}
		bundle = append(bundle, *resource)
	}
}

func New(instancetypeBundlePath, preferenceBundlePath string) (operands.Operand, error) {
	virtualMachineClusterInstancetypes, err := fetchClusterResources[instancetypev1alpha2.VirtualMachineClusterInstancetype](instancetypeBundlePath)
	if err != nil {
		return nil, err
	}
	virtualMachineClusterPreferences, err := fetchClusterResources[instancetypev1alpha2.VirtualMachineClusterPreference](preferenceBundlePath)
	if err != nil {
		return nil, err
	}
	return &commonInstancetypes{
		virtualMachineClusterInstancetypes: virtualMachineClusterInstancetypes,
		virtualMachineClusterPreferences:   virtualMachineClusterPreferences,
	}, nil
}

func (c *commonInstancetypes) Name() string {
	return operandName
}

func (c *commonInstancetypes) WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &instancetypev1alpha2.VirtualMachineClusterInstancetype{}, Crd: instancetypeapi.ClusterPluralResourceName, WatchFullObject: true},
		{Object: &instancetypev1alpha2.VirtualMachineClusterPreference{}, Crd: instancetypeapi.ClusterPluralPreferenceResourceName, WatchFullObject: true},
	}
}

func (c *commonInstancetypes) WatchTypes() []operands.WatchType {
	return nil
}

func (c *commonInstancetypes) RequiredCrds() []string {
	return nil
}

func (c *commonInstancetypes) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	request.Logger.Info("Reconciling common-instancetypes")
	return common.CollectResourceStatus(request, c.reconcileFuncs()...)
}

func (c *commonInstancetypes) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	return nil, nil
}

func (c *commonInstancetypes) reconcileFuncs() []common.ReconcileFunc {
	funcs := []common.ReconcileFunc{}
	funcs = append(funcs, c.reconcileVirtualMachineClusterInstancetypesFuncs()...)
	funcs = append(funcs, c.reconcileVirtualMachineClusterPreferencesFuncs()...)
	return funcs
}

func (c *commonInstancetypes) reconcileVirtualMachineClusterInstancetypesFuncs() []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(c.virtualMachineClusterInstancetypes))
	for i := range c.virtualMachineClusterInstancetypes {
		clusterInstancetype := &c.virtualMachineClusterInstancetypes[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(clusterInstancetype).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					foundRes.(*instancetypev1alpha2.VirtualMachineClusterInstancetype).Spec = newRes.(*instancetypev1alpha2.VirtualMachineClusterInstancetype).Spec
				}).
				ImmutableSpec(func(resource client.Object) interface{} {
					return resource.(*instancetypev1alpha2.VirtualMachineClusterInstancetype).Spec
				}).
				Reconcile()
		})
	}
	return funcs
}

func (c *commonInstancetypes) reconcileVirtualMachineClusterPreferencesFuncs() []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(c.virtualMachineClusterPreferences))
	for i := range c.virtualMachineClusterPreferences {
		clusterPreference := &c.virtualMachineClusterPreferences[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(clusterPreference).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					foundRes.(*instancetypev1alpha2.VirtualMachineClusterPreference).Spec = newRes.(*instancetypev1alpha2.VirtualMachineClusterPreference).Spec
				}).
				ImmutableSpec(func(resource client.Object) interface{} {
					return resource.(*instancetypev1alpha2.VirtualMachineClusterPreference).Spec
				}).
				Reconcile()
		})
	}
	return funcs
}
