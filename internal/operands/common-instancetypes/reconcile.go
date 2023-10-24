package common_instancetypes

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	virtv1 "kubevirt.io/api/core/v1"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=instancetype.kubevirt.io,resources=virtualmachineclusterinstancetypes,verbs=list;watch;create;update;delete
// +kubebuilder:rbac:groups=instancetype.kubevirt.io,resources=virtualmachineclusterpreferences,verbs=list;watch;create;update;delete

const (
	operandName                          = "common-instancetypes"
	operandComponent                     = common.AppComponentTemplating
	BundleDir                            = "data/common-instancetypes-bundle/"
	ClusterInstancetypesBundle           = "common-clusterinstancetypes-bundle.yaml"
	ClusterPreferencesBundle             = "common-clusterpreferences-bundle.yaml"
	virtualMachineClusterInstancetypeCrd = instancetypeapi.ClusterPluralResourceName + "." + instancetypeapi.GroupName
	virtualMachineClusterPreferenceCrd   = instancetypeapi.ClusterPluralPreferenceResourceName + "." + instancetypeapi.GroupName
)

type CommonInstancetypes struct {
	resourceURL                             string
	virtualMachineClusterInstancetypeBundle string
	virtualMachineClusterPreferenceBundle   string
	virtualMachineClusterInstancetypes      []instancetypev1beta1.VirtualMachineClusterInstancetype
	virtualMachineClusterPreferences        []instancetypev1beta1.VirtualMachineClusterPreference
	KustomizeRunFunc                        func(filesys.FileSystem, string) (resmap.ResMap, error)
}

var _ operands.Operand = &CommonInstancetypes{}

type clusterType interface {
	instancetypev1beta1.VirtualMachineClusterInstancetype | instancetypev1beta1.VirtualMachineClusterPreference
}

func (c *CommonInstancetypes) Name() string {
	return operandName
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &instancetypev1beta1.VirtualMachineClusterInstancetype{}, Crd: virtualMachineClusterInstancetypeCrd, WatchFullObject: true},
		{Object: &instancetypev1beta1.VirtualMachineClusterPreference{}, Crd: virtualMachineClusterPreferenceCrd, WatchFullObject: true},
	}
}

func (c *CommonInstancetypes) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (c *CommonInstancetypes) WatchTypes() []operands.WatchType {
	return nil
}

func New(virtualMachineClusterInstancetypeBundlePath, virtualMachineClusterPreferenceBundlePath string) *CommonInstancetypes {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	return &CommonInstancetypes{
		virtualMachineClusterInstancetypeBundle: virtualMachineClusterInstancetypeBundlePath,
		virtualMachineClusterPreferenceBundle:   virtualMachineClusterPreferenceBundlePath,
		KustomizeRunFunc:                        k.Run,
	}
}

func decodeResources[C clusterType](b []byte) ([]C, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(b), 1024)
	var bundle []C
	for {
		bundleResource := new(C)
		err := decoder.Decode(bundleResource)
		if err == io.EOF {
			return bundle, nil
		}
		if err != nil {
			return nil, err
		}
		bundle = append(bundle, *bundleResource)
	}
}

func FetchBundleResource[C clusterType](path string) ([]C, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeResources[C](file)
}

func (c *CommonInstancetypes) fetchResourcesFromBundle() ([]instancetypev1beta1.VirtualMachineClusterInstancetype, []instancetypev1beta1.VirtualMachineClusterPreference, error) {
	virtualMachineClusterInstancetypes, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterInstancetype](c.virtualMachineClusterInstancetypeBundle)
	if err != nil {
		return nil, nil, err
	}
	virtualMachineClusterPreferences, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterPreference](c.virtualMachineClusterPreferenceBundle)
	if err != nil {
		return nil, nil, err
	}
	return virtualMachineClusterInstancetypes, virtualMachineClusterPreferences, err
}

func (c *CommonInstancetypes) generateResourcesFromURL(URL string) (resmap.ResMap, error) {
	fSys := filesys.MakeFsOnDisk()
	tmpDir, err := filesys.NewTmpConfirmedDir()
	if err != nil {
		return nil, err
	}
	tmpDirPath := tmpDir.String()
	defer os.RemoveAll(tmpDir.String())
	if err = fSys.WriteFile(filepath.Join(tmpDirPath, "kustomization.yaml"), []byte(fmt.Sprintf("\nresources:\n  - %s", URL))); err != nil {
		return nil, err
	}
	return c.KustomizeRunFunc(fSys, tmpDirPath)
}

func decodeResMapResources[C clusterType](r *resource.Resource) ([]C, error) {
	b, err := r.MarshalJSON()
	if err != nil {
		return nil, err
	}
	bundle, err := decodeResources[C](b)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}

func (c *CommonInstancetypes) FetchResourcesFromURL(URL string) ([]instancetypev1beta1.VirtualMachineClusterInstancetype, []instancetypev1beta1.VirtualMachineClusterPreference, error) {
	resmapFromURL, err := c.generateResourcesFromURL(URL)
	if err != nil {
		return nil, nil, err
	}

	var (
		virtualMachineClusterInstancetypes []instancetypev1beta1.VirtualMachineClusterInstancetype
		virtualMachineClusterPreferences   []instancetypev1beta1.VirtualMachineClusterPreference
	)

	for _, r := range resmapFromURL.Resources() {
		switch strings.ToLower(r.GetKind()) {
		case instancetypeapi.ClusterSingularResourceName:
			resources, err := decodeResMapResources[instancetypev1beta1.VirtualMachineClusterInstancetype](r)
			if err != nil {
				return nil, nil, err
			}
			virtualMachineClusterInstancetypes = append(virtualMachineClusterInstancetypes, resources...)
		case instancetypeapi.ClusterSingularPreferenceResourceName:
			resources, err := decodeResMapResources[instancetypev1beta1.VirtualMachineClusterPreference](r)
			if err != nil {
				return nil, nil, err
			}
			virtualMachineClusterPreferences = append(virtualMachineClusterPreferences, resources...)
		}
	}
	return virtualMachineClusterInstancetypes, virtualMachineClusterPreferences, nil
}

func (c *CommonInstancetypes) fetchExistingResources(request *common.Request) ([]instancetypev1beta1.VirtualMachineClusterInstancetype, []instancetypev1beta1.VirtualMachineClusterPreference, error) {
	selector, err := common.GetAppNameSelector(c.Name())
	if err != nil {
		return nil, nil, err
	}
	listOpts := &client.ListOptions{
		LabelSelector: selector,
	}
	existingClusterInstancetypes := &instancetypev1beta1.VirtualMachineClusterInstancetypeList{}
	if err := request.Client.List(request.Context, existingClusterInstancetypes, listOpts); err != nil {
		return nil, nil, err
	}
	existingClusterPreferences := &instancetypev1beta1.VirtualMachineClusterPreferenceList{}
	if err := request.Client.List(request.Context, existingClusterPreferences, listOpts); err != nil {
		return nil, nil, err
	}
	return existingClusterInstancetypes.Items, existingClusterPreferences.Items, nil
}

func reconcileRemovedInstancetypes(request *common.Request, existingResources []instancetypev1beta1.VirtualMachineClusterInstancetype, resourcesFromURL []instancetypev1beta1.VirtualMachineClusterInstancetype) error {
	resourceFromURLByName := make(map[string]instancetypev1beta1.VirtualMachineClusterInstancetype)
	for _, resourceFromURL := range resourcesFromURL {
		resourceFromURLByName[resourceFromURL.Name] = resourceFromURL
	}
	for _, existingResource := range existingResources {
		if _, resourceProvided := resourceFromURLByName[existingResource.Name]; !resourceProvided {
			request.Logger.Info(fmt.Sprintf("removing the no longer provided %s VirtualMachineClusterInstancetype", existingResource.Name))
			if err := request.Client.Delete(request.Context, &existingResource); err != nil {
				return err
			}
		}
	}
	return nil
}

func reconcileRemovedPreferences(request *common.Request, existingResources []instancetypev1beta1.VirtualMachineClusterPreference, resourcesFromURL []instancetypev1beta1.VirtualMachineClusterPreference) error {
	resourceFromURLByName := make(map[string]instancetypev1beta1.VirtualMachineClusterPreference)
	for _, resourceFromURL := range resourcesFromURL {
		resourceFromURLByName[resourceFromURL.Name] = resourceFromURL
	}
	for _, existingResource := range existingResources {
		if _, resourceProvided := resourceFromURLByName[existingResource.Name]; !resourceProvided {
			request.Logger.Info(fmt.Sprintf("removing the no longer provided %s VirtualMachineClusterPreference", existingResource.Name))
			if err := request.Client.Delete(request.Context, &existingResource); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CommonInstancetypes) reconcileRemovedResources(request *common.Request, newInstancetypes []instancetypev1beta1.VirtualMachineClusterInstancetype, newPreferences []instancetypev1beta1.VirtualMachineClusterPreference) error {
	existingClusterInstancetypes, existingClusterPreferences, err := c.fetchExistingResources(request)
	if err != nil {
		return err
	}

	if err = reconcileRemovedInstancetypes(request, existingClusterInstancetypes, newInstancetypes); err != nil {
		return err
	}

	if err = reconcileRemovedPreferences(request, existingClusterPreferences, newPreferences); err != nil {
		return err
	}
	return nil
}

func (c *CommonInstancetypes) reconcileFromURL(request *common.Request) ([]common.ReconcileResult, error) {
	// TODO - In the future we should handle cases where the URL remains the same but the provided resources change.
	if c.resourceURL != "" && c.resourceURL == *request.Instance.Spec.CommonInstancetypes.URL {
		request.Logger.Info(fmt.Sprintf("Skipping reconcile of common-instancetypes from URL %s, force with a restart of the service.", *request.Instance.Spec.CommonInstancetypes.URL))
		return nil, nil
	}

	// Cache the URL so we can check if it changes with future reconcile attempts above
	c.resourceURL = *request.Instance.Spec.CommonInstancetypes.URL
	request.Logger.Info(fmt.Sprintf("Reconciling common-instancetypes from URL %s", c.resourceURL))
	clusterInstancetypesFromURL, clusterPreferencesFromURL, err := c.FetchResourcesFromURL(c.resourceURL)
	if err != nil {
		return nil, err
	}

	// Remove any resources no longer provided by the URL, this should only happen when switching from the internal bundle to external URL for now.
	if err = c.reconcileRemovedResources(request, clusterInstancetypesFromURL, clusterPreferencesFromURL); err != nil {
		return nil, err
	}

	// Generate the normal set of reconcile funcs to create or update the provided resources
	c.virtualMachineClusterInstancetypes = clusterInstancetypesFromURL
	c.virtualMachineClusterPreferences = clusterPreferencesFromURL
	reconcileFuncs, err := c.reconcileFuncs(request)
	if err != nil {
		return nil, err
	}
	return common.CollectResourceStatus(request, reconcileFuncs...)
}

func (c *CommonInstancetypes) reconcileFromBundle(request *common.Request) ([]common.ReconcileResult, error) {
	request.Logger.Info("Reconciling common-instancetypes from internal bundle")
	clusterInstancetypesFromBundle, clusterPreferencesFromBundle, err := c.fetchResourcesFromBundle()
	if err != nil {
		return nil, err
	}

	// Remove any resources no longer provided by the bundle
	if err = c.reconcileRemovedResources(request, clusterInstancetypesFromBundle, clusterPreferencesFromBundle); err != nil {
		return nil, err
	}

	c.virtualMachineClusterInstancetypes = clusterInstancetypesFromBundle
	c.virtualMachineClusterPreferences = clusterPreferencesFromBundle
	reconcileFuncs, err := c.reconcileFuncs(request)
	if err != nil {
		return nil, err
	}
	return common.CollectResourceStatus(request, reconcileFuncs...)
}

func (c *CommonInstancetypes) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	if request.Instance.Spec.CommonInstancetypes != nil && request.Instance.Spec.CommonInstancetypes.URL != nil {
		return c.reconcileFromURL(request)
	}
	return c.reconcileFromBundle(request)
}

func (c *CommonInstancetypes) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object

	// Before collecting resources to clean up ensure the corresponding CRD is available
	if request.CrdList.CrdExists(virtualMachineClusterInstancetypeCrd) {
		for i := range c.virtualMachineClusterInstancetypes {
			objects = append(objects, &c.virtualMachineClusterInstancetypes[i])
		}
	}
	if request.CrdList.CrdExists(virtualMachineClusterPreferenceCrd) {
		for i := range c.virtualMachineClusterPreferences {
			objects = append(objects, &c.virtualMachineClusterPreferences[i])
		}
	}

	if len(objects) > 0 {
		return common.DeleteAll(request, objects...)
	}

	return nil, nil
}

func (c *CommonInstancetypes) reconcileFuncs(request *common.Request) ([]common.ReconcileFunc, error) {
	instancetypeFuncs, err := c.reconcileVirtualMachineClusterInstancetypesFuncs(request)
	if err != nil {
		return nil, err
	}
	preferenceFuncs, err := c.reconcileVirtualMachineClusterPreferencesFuncs(request)
	if err != nil {
		return nil, err
	}

	funcs := []common.ReconcileFunc{}
	funcs = append(funcs, instancetypeFuncs...)
	funcs = append(funcs, preferenceFuncs...)
	return funcs, nil
}

func (c *CommonInstancetypes) reconcileVirtualMachineClusterInstancetypesFuncs(request *common.Request) ([]common.ReconcileFunc, error) {
	funcs := make([]common.ReconcileFunc, 0, len(c.virtualMachineClusterInstancetypes))
	for i := range c.virtualMachineClusterInstancetypes {
		clusterInstancetype := &c.virtualMachineClusterInstancetypes[i]
		if ignore, err := ignoreObjectOwnedByVirtOperator(request, clusterInstancetype); err != nil {
			return nil, err
		} else if ignore {
			continue
		}
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(clusterInstancetype).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs, nil
}

func (c *CommonInstancetypes) reconcileVirtualMachineClusterPreferencesFuncs(request *common.Request) ([]common.ReconcileFunc, error) {
	funcs := make([]common.ReconcileFunc, 0, len(c.virtualMachineClusterPreferences))
	for i := range c.virtualMachineClusterPreferences {
		clusterPreference := &c.virtualMachineClusterPreferences[i]
		if ignore, err := ignoreObjectOwnedByVirtOperator(request, clusterPreference); err != nil {
			return nil, err
		} else if ignore {
			continue
		}
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(clusterPreference).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs, nil
}

func ignoreObjectOwnedByVirtOperator(request *common.Request, obj client.Object) (bool, error) {
	// During an upgrade to v0.19.0 we might encounter virt-operator attempting
	// to reconcile the same set of common-instancetype resources. Ignore these
	// requests to reconcile such objects once owned by virt-operator.
	existing := obj.DeepCopyObject().(client.Object)
	if err := request.Client.Get(request.Context, client.ObjectKeyFromObject(existing), existing); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}

	return isOwnedByVirtOperator(existing), nil
}

func isOwnedByVirtOperator(obj client.Object) bool {
	if obj.GetLabels() == nil {
		return false
	}
	managedValue, managedOk := obj.GetLabels()[virtv1.ManagedByLabel]
	return managedOk && managedValue == virtv1.ManagedByLabelOperatorValue
}
