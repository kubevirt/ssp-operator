package data_sources

import (
	"fmt"

	core "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;roles;rolebindings,verbs=list;watch;create;update;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=dataimportcrons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=list;watch;create;update;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes/source,verbs=create
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=dataimportcrons,verbs=get;list;watch;create;update;patch;delete

const (
	operandName      = "data-sources"
	operandComponent = common.AppComponentTemplating
)

const (
	dataSourceCrd     = "datasources.cdi.kubevirt.io"
	dataImportCronCrd = "dataimportcrons.cdi.kubevirt.io"
)

func init() {
	utilruntime.Must(cdiv1beta1.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &rbac.ClusterRole{}},
		{Object: &rbac.Role{}},
		{Object: &rbac.RoleBinding{}},
		{Object: &core.Namespace{}},
		// Need to watch status of DataSource to notice if referenced PVC was deleted.
		{Object: &cdiv1beta1.DataSource{}, Crd: dataSourceCrd, WatchFullObject: true},
		{Object: &cdiv1beta1.DataImportCron{}, Crd: dataImportCronCrd},
		{Object: &networkv1.NetworkPolicy{}},
	}
}

type dataSources struct {
	sources            []cdiv1beta1.DataSource
	runningOnOpenShift bool
}

var _ operands.Operand = &dataSources{}

func New(sourceNames []string, runningOnOpenShift bool) operands.Operand {
	sources := make([]cdiv1beta1.DataSource, 0, len(sourceNames))
	for _, name := range sourceNames {
		sources = append(sources, newDataSource(name))
	}

	return &dataSources{
		sources:            sources,
		runningOnOpenShift: runningOnOpenShift,
	}
}

func (d *dataSources) Name() string {
	return operandName
}

func (d *dataSources) WatchTypes() []operands.WatchType {
	return nil
}

func (d *dataSources) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

type dataSourceInfo struct {
	dataSource         *cdiv1beta1.DataSource
	autoUpdateEnabled  bool
	dataImportCronName string
}

func (d *dataSources) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	funcs := []common.ReconcileFunc{
		reconcileGoldenImagesNS,
		reconcileViewRole,
		reconcileViewRoleBinding,
		reconcileEditRole,
	}

	dsAndCrons, err := d.getDataSourcesAndCrons(request)
	if err != nil {
		return nil, err
	}

	dsFuncs, err := reconcileDataSources(dsAndCrons.dataSourceInfos, request)
	if err != nil {
		return nil, err
	}
	funcs = append(funcs, dsFuncs...)
	funcs = append(funcs, d.reconcileNetworkPolicies()...)

	results, err := common.CollectResourceStatus(request, funcs...)
	if err != nil {
		return nil, err
	}

	var allSucceeded = true
	for i := range results {
		if !results[i].IsSuccess() {
			allSucceeded = false
			break
		}
	}

	// DataImportCrons can be reconciled only after all resources successfully reconciled.
	if !allSucceeded {
		return results, nil
	}

	dicFuncs, err := reconcileDataImportCrons(dsAndCrons.dataImportCrons, request)
	if err != nil {
		return nil, err
	}

	return common.CollectResourceStatus(request, dicFuncs...)
}

func (d *dataSources) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	if request.CrdList.CrdExists(dataImportCronCrd) {
		ownedCrons, err := listAllOwnedDataImportCrons(request)
		if err != nil {
			return nil, err
		}

		var results []common.CleanupResult
		allDataImportCronsDeleted := true
		for i := range ownedCrons {
			result, err := common.Cleanup(request, &ownedCrons[i])
			if err != nil {
				return nil, err
			}
			results = append(results, result)
			if !result.Deleted {
				allDataImportCronsDeleted = false
			}
		}

		// The rest of the objects will be deleted when all DataImportCrons are deleted.
		if !allDataImportCronsDeleted {
			return results, nil
		}
	}

	var objects []client.Object
	if request.CrdList.CrdExists(dataSourceCrd) {
		for i := range d.sources {
			ds := d.sources[i]
			ds.Namespace = internal.GoldenImagesNamespace
			objects = append(objects, &ds)
		}
	}

	objects = append(objects,
		newGoldenImagesNS(internal.GoldenImagesNamespace),
		newViewRole(internal.GoldenImagesNamespace),
		newViewRoleBinding(internal.GoldenImagesNamespace),
		newEditRole())
	for _, policy := range newNetworkPolicies(internal.GoldenImagesNamespace, d.runningOnOpenShift) {
		objects = append(objects, policy)
	}

	return common.DeleteAll(request, objects...)
}

func reconcileGoldenImagesNS(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newGoldenImagesNS(internal.GoldenImagesNamespace)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileViewRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newViewRole(internal.GoldenImagesNamespace)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileViewRoleBinding(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newViewRoleBinding(internal.GoldenImagesNamespace)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileEditRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newEditRole()).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

type dataSourcesAndCrons struct {
	dataSourceInfos []dataSourceInfo
	dataImportCrons []cdiv1beta1.DataImportCron
}

func (d *dataSources) getDataSourcesAndCrons(request *common.Request) (dataSourcesAndCrons, error) {
	cronTemplates := request.Instance.Spec.CommonTemplates.DataImportCronTemplates
	cronByDataSource := make(map[client.ObjectKey]*ssp.DataImportCronTemplate, len(cronTemplates))
	for i := range cronTemplates {
		cron := &cronTemplates[i]
		if cron.Namespace == "" {
			cron.Namespace = internal.GoldenImagesNamespace
		}
		cronByDataSource[client.ObjectKey{
			Name:      cron.Spec.ManagedDataSource,
			Namespace: cron.Namespace,
		}] = cron
	}

	var dataSourceInfos []dataSourceInfo
	for i := range d.sources {
		dataSource := d.sources[i] // Make a copy
		dataSource.Namespace = internal.GoldenImagesNamespace
		autoUpdateEnabled, err := dataSourceAutoUpdateEnabled(&dataSource, cronByDataSource, request)
		if err != nil {
			return dataSourcesAndCrons{}, err
		}

		var dicName string
		if dic, ok := cronByDataSource[client.ObjectKeyFromObject(&dataSource)]; ok {
			dicName = dic.GetName()
		}

		dataSourceInfos = append(dataSourceInfos, dataSourceInfo{
			dataSource:         &dataSource,
			autoUpdateEnabled:  autoUpdateEnabled,
			dataImportCronName: dicName,
		})
	}

	for i := range dataSourceInfos {
		if !dataSourceInfos[i].autoUpdateEnabled {
			delete(cronByDataSource, client.ObjectKeyFromObject(dataSourceInfos[i].dataSource))
		}
	}

	dataImportCrons := make([]cdiv1beta1.DataImportCron, 0, len(cronByDataSource))
	for _, cronTemplate := range cronByDataSource {
		dataImportCrons = append(dataImportCrons, cronTemplate.AsDataImportCron())
	}

	return dataSourcesAndCrons{
		dataSourceInfos: dataSourceInfos,
		dataImportCrons: dataImportCrons,
	}, nil
}

const dataImportCronLabel = "cdi.kubevirt.io/dataImportCron"

func dataSourceAutoUpdateEnabled(dataSource *cdiv1beta1.DataSource, cronByDataSource map[client.ObjectKey]*ssp.DataImportCronTemplate, request *common.Request) (bool, error) {
	objectKey := client.ObjectKeyFromObject(dataSource)
	_, cronExists := cronByDataSource[objectKey]
	if !cronExists {
		// If DataImportCron does not exist for this DataSource, auto-update is disabled.
		return false, nil
	}

	// Check existing data source. The Get call uses cache.
	foundDataSource := &cdiv1beta1.DataSource{}
	err := request.Client.Get(request.Context, objectKey, foundDataSource)
	if errors.IsNotFound(err) {
		pvcExists, err := checkIfPvcExists(dataSource, request)
		if err != nil {
			return false, err
		}

		// If PVC exists, DataSource does not use auto-update.
		// Otherwise, DataSource uses auto-update.
		return !pvcExists, nil
	}
	if err != nil {
		return false, err
	}

	if _, foundDsUsesAutoUpdate := foundDataSource.GetLabels()[dataImportCronLabel]; foundDsUsesAutoUpdate {
		// Found DS is labeled to use auto-update.
		return true, nil
	}

	dsReadyCondition := getDataSourceReadyCondition(foundDataSource)
	// It makes sense to check the ready condition only if the found DataSource spec
	// points to the golden image PVC, not to auto-update PVC.
	if dsReadyCondition != nil && foundDataSource.Spec.Source.PVC == dataSource.Spec.Source.PVC {
		// Auto-update will ony be enabled if the DataSource does not refer to an existing PVC.
		return dsReadyCondition.Status != core.ConditionTrue, nil
	}

	// In case found DataSource spec is different from expected spec, we need to check if PVC exists.
	pvcExists, err := checkIfPvcExists(dataSource, request)
	if err != nil {
		return false, err
	}
	// If PVC exists, DataSource does not use auto-update. Otherwise, DataSource uses auto-update.
	return !pvcExists, nil
}

func checkIfPvcExists(dataSource *cdiv1beta1.DataSource, request *common.Request) (bool, error) {
	if dataSource.Spec.Source.PVC == nil {
		return false, nil
	}

	// This is an unchanged API call, but it is only called when DataSource does not exist,
	// and there is a small number of DataSources reconciled by this operator.
	err := request.UncachedReader.Get(request.Context, client.ObjectKey{
		Name:      dataSource.Spec.Source.PVC.Name,
		Namespace: dataSource.Spec.Source.PVC.Namespace,
	}, &core.PersistentVolumeClaim{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func reconcileDataSources(dataSourceInfos []dataSourceInfo, request *common.Request) ([]common.ReconcileFunc, error) {
	ownedDataSources, err := listAllOwnedDataSources(request)
	if err != nil {
		return nil, err
	}

	dsNames := make(map[string]struct{}, len(dataSourceInfos))
	var funcs []common.ReconcileFunc
	for i := range dataSourceInfos {
		dsInfo := dataSourceInfos[i] // Make a local copy
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return reconcileDataSource(dsInfo, request)
		})
		dsNames[dsInfo.dataSource.GetName()] = struct{}{}
	}

	// Remove owned DataSources that are not in the 'dataSourceInfos'
	for i := range ownedDataSources {
		if _, isUsed := dsNames[ownedDataSources[i].GetName()]; isUsed {
			continue
		}

		dataSource := ownedDataSources[i] // Make local copy
		dataSource.Namespace = internal.GoldenImagesNamespace
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			if !dataSource.GetDeletionTimestamp().IsZero() {
				return common.ResourceDeletedResult(&dataSource, common.OperationResultDeleted), nil
			}

			err := request.Client.Delete(request.Context, &dataSource)
			if errors.IsNotFound(err) {
				return common.ReconcileResult{
					Resource: &dataSource,
				}, nil
			}
			if err != nil {
				request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", dataSource.GetName(), err))
				return common.ReconcileResult{}, err
			}

			return common.ResourceDeletedResult(&dataSource, common.OperationResultDeleted), nil
		})
	}

	return funcs, nil
}

func reconcileDataSource(dsInfo dataSourceInfo, request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(dsInfo.dataSource).
		Options(common.ReconcileOptions{AlwaysCallUpdateFunc: true}).
		UpdateFunc(func(newRes, foundRes client.Object) {
			if dsInfo.autoUpdateEnabled {
				if foundRes.GetLabels() == nil {
					foundRes.SetLabels(make(map[string]string))
				}
				// We need to restore this label in case it was removed.
				// If it is removed, then CDI stops watching the DataSource.
				foundRes.GetLabels()[dataImportCronLabel] = dsInfo.dataImportCronName
				return
			}

			// Only set app labels if DIC does not exist
			common.AddAppLabels(request.Instance, operandName, operandComponent, foundRes)
			delete(foundRes.GetLabels(), dataImportCronLabel)

			foundRes.(*cdiv1beta1.DataSource).Spec = newRes.(*cdiv1beta1.DataSource).Spec
		}).
		Reconcile()
}

func getDataSourceReadyCondition(dataSource *cdiv1beta1.DataSource) *cdiv1beta1.DataSourceCondition {
	for i := range dataSource.Status.Conditions {
		condition := &dataSource.Status.Conditions[i]
		if condition.Type == cdiv1beta1.DataSourceReady {
			return condition
		}
	}
	return nil
}

func reconcileDataImportCron(dataImportCron *cdiv1beta1.DataImportCron, request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(dataImportCron).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRes.(*cdiv1beta1.DataImportCron).Spec = newRes.(*cdiv1beta1.DataImportCron).Spec
		}).
		ImmutableSpec(func(resource client.Object) interface{} {
			return resource.(*cdiv1beta1.DataImportCron).Spec
		}).
		Reconcile()
}

func reconcileDataImportCrons(dataImportCrons []cdiv1beta1.DataImportCron, request *common.Request) ([]common.ReconcileFunc, error) {
	ownedCrons, err := listAllOwnedDataImportCrons(request)
	if err != nil {
		return nil, err
	}

	crons := make(map[client.ObjectKey]struct{}, len(dataImportCrons))

	var funcs []common.ReconcileFunc
	for i := range dataImportCrons {
		cron := dataImportCrons[i] // Make a local copy
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return reconcileDataImportCron(&cron, request)
		})
		crons[client.ObjectKeyFromObject(&cron)] = struct{}{}
	}

	// Remove owned DataImportCrons that are not in the 'dataImportCrons' parameter
	for i := range ownedCrons {
		if _, isUsed := crons[client.ObjectKeyFromObject(&ownedCrons[i])]; isUsed {
			continue
		}

		cron := ownedCrons[i] // Make local copy
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			err := request.Client.Delete(request.Context, &cron)
			if err != nil && !errors.IsNotFound(err) {
				request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", cron.GetName(), err))
				return common.ReconcileResult{}, err
			}
			return common.ReconcileResult{
				Resource: &cron,
			}, nil
		})
	}

	return funcs, nil
}

func listAllOwnedDataSources(request *common.Request) ([]cdiv1beta1.DataSource, error) {
	return common.ListOwnedResources[cdiv1beta1.DataSourceList, cdiv1beta1.DataSource](request, client.InNamespace(internal.GoldenImagesNamespace))
}

func listAllOwnedDataImportCrons(request *common.Request) ([]cdiv1beta1.DataImportCron, error) {
	return common.ListOwnedResources[cdiv1beta1.DataImportCronList, cdiv1beta1.DataImportCron](request)
}

func (d *dataSources) reconcileNetworkPolicies() []common.ReconcileFunc {
	var funcs []common.ReconcileFunc
	for _, policy := range newNetworkPolicies(internal.GoldenImagesNamespace, d.runningOnOpenShift) {
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(policy).
				WithAppLabels(operandName, operandComponent).
				Reconcile()
		})
	}
	return funcs
}
