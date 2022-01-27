package data_sources

import (
	"fmt"

	"github.com/operator-framework/operator-lib/handler"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=dataimportcrons,verbs=get;list;watch;create;update;patch;delete

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

func init() {
	utilruntime.Must(cdiv1beta1.AddToScheme(common.Scheme))
}

func WatchClusterTypes() []client.Object {
	return []client.Object{
		&rbac.ClusterRole{},
		&rbac.Role{},
		&rbac.RoleBinding{},
		&core.Namespace{},
		&cdiv1beta1.DataSource{},
		&cdiv1beta1.DataImportCron{},
	}
}

type dataSources struct {
	sources []cdiv1beta1.DataSource
}

var _ operands.Operand = &dataSources{}

func New(sources []cdiv1beta1.DataSource) operands.Operand {
	return &dataSources{
		sources: sources,
	}
}

func (d *dataSources) Name() string {
	return operandName
}

func (d *dataSources) WatchTypes() []client.Object {
	return nil
}

func (d *dataSources) WatchClusterTypes() []client.Object {
	return WatchClusterTypes()
}

func (d *dataSources) RequiredCrds() []string {
	return []string{
		"datavolumes.cdi.kubevirt.io",
		"datasources.cdi.kubevirt.io",
		"dataimportcrons.cdi.kubevirt.io",
	}
}

func (d *dataSources) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	funcs := []common.ReconcileFunc{
		reconcileGoldenImagesNS,
		reconcileViewRole,
		reconcileViewRoleBinding,
		reconcileEditRole,
	}

	dsAndCrons, err := d.getManagedDataSourcesAndCrons(request)
	if err != nil {
		return nil, err
	}

	dsFuncs, err := reconcileDataSources(dsAndCrons.managedDataSources,
		dsAndCrons.transitioningDataSources,
		request)
	if err != nil {
		return nil, err
	}
	funcs = append(funcs, dsFuncs...)

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
	ownedCrons, err := listAllOwnedDataImportCrons(request)
	if err != nil {
		return nil, err
	}

	results := []common.CleanupResult{}
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

	objects := []client.Object{}
	for i := range d.sources {
		ds := d.sources[i]
		ds.Namespace = ssp.GoldenImagesNSname
		objects = append(objects, &ds)
	}

	objects = append(objects,
		newGoldenImagesNS(ssp.GoldenImagesNSname),
		newViewRole(ssp.GoldenImagesNSname),
		newViewRoleBinding(ssp.GoldenImagesNSname),
		newEditRole())

	return common.DeleteAll(request, objects...)
}

func reconcileGoldenImagesNS(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newGoldenImagesNS(ssp.GoldenImagesNSname)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileViewRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newViewRole(ssp.GoldenImagesNSname)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRole := foundRes.(*rbac.Role)
			newRole := newRes.(*rbac.Role)
			foundRole.Rules = newRole.Rules
		}).
		Reconcile()
}

func reconcileViewRoleBinding(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newViewRoleBinding(ssp.GoldenImagesNSname)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newBinding := newRes.(*rbac.RoleBinding)
			foundBinding := foundRes.(*rbac.RoleBinding)
			foundBinding.Subjects = newBinding.Subjects
			foundBinding.RoleRef = newBinding.RoleRef
		}).
		Reconcile()
}

func reconcileEditRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newEditRole()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newRole := newRes.(*rbac.ClusterRole)
			foundRole := foundRes.(*rbac.ClusterRole)
			foundRole.Rules = newRole.Rules
		}).
		Reconcile()
}

type dataSourcesAndCrons struct {
	managedDataSources       []cdiv1beta1.DataSource
	transitioningDataSources []cdiv1beta1.DataSource
	dataImportCrons          []cdiv1beta1.DataImportCron
}

func (d *dataSources) getManagedDataSourcesAndCrons(request *common.Request) (dataSourcesAndCrons, error) {
	cronTemplates := request.Instance.Spec.CommonTemplates.DataImportCronTemplates
	cronByDataSourceName := make(map[string]*ssp.DataImportCronTemplate, len(cronTemplates))
	for i := range cronTemplates {
		cron := &cronTemplates[i]
		cronByDataSourceName[cron.Spec.ManagedDataSource] = cron
	}

	// DataSources managed by the SSP operator
	var managedDataSources []cdiv1beta1.DataSource
	var transitioningDataSources []cdiv1beta1.DataSource
	for _, dataSource := range d.sources {
		dataSource.Namespace = ssp.GoldenImagesNSname
		managedState, err := dataSourceManaged(&dataSource, cronByDataSourceName, request)
		if err != nil {
			return dataSourcesAndCrons{}, err
		}
		switch managedState {
		case managedDataSource:
			managedDataSources = append(managedDataSources, dataSource)
		case transitioningDataSource:
			transitioningDataSources = append(transitioningDataSources, dataSource)
		}
	}

	for i := range managedDataSources {
		delete(cronByDataSourceName, managedDataSources[i].GetName())
	}

	managedDataImportCrons := make([]cdiv1beta1.DataImportCron, 0, len(cronByDataSourceName))
	for _, cronTemplate := range cronByDataSourceName {
		managedDataImportCrons = append(managedDataImportCrons, cronTemplate.AsDataImportCron())
	}

	return dataSourcesAndCrons{
		managedDataSources:       managedDataSources,
		transitioningDataSources: transitioningDataSources,
		dataImportCrons:          managedDataImportCrons,
	}, nil
}

type dataSourceState string

const (
	// DataSource is managed by SSP operator
	managedDataSource dataSourceState = "managedDataSource"

	// DataSource is transitioning to not managed by SSP
	transitioningDataSource dataSourceState = "transitioningDataSource"

	// DataSource is not managed by SSP
	unmanagedDataSource dataSourceState = "unmanagedDataSource"
)

const dataImportCronLabel = "cdi.kubevirt.io/dataImportCron"

func dataSourceManaged(dataSource *cdiv1beta1.DataSource, cronByDataSourceName map[string]*ssp.DataImportCronTemplate, request *common.Request) (dataSourceState, error) {
	_, cronExists := cronByDataSourceName[dataSource.GetName()]
	if !cronExists {
		// If DataImportCron does not exist for this DataSource, SSP needs to reconcile it.
		return managedDataSource, nil
	}

	// Check existing data source. The Get call uses cache.
	foundDataSource := &cdiv1beta1.DataSource{}
	err := request.Client.Get(request.Context, client.ObjectKeyFromObject(dataSource), foundDataSource)
	if errors.IsNotFound(err) {
		// Checking if PVC exists. This is an unchanged API call, but it is only called when DataSource
		// does not exist, and there is a small number of DataSources reconciled by this operator.
		err := request.UncachedReader.Get(request.Context, client.ObjectKey{
			Name:      dataSource.Spec.Source.PVC.Name,
			Namespace: dataSource.Spec.Source.PVC.Namespace,
		}, &core.PersistentVolumeClaim{})
		if errors.IsNotFound(err) {
			// Referenced PVC does not exist. DataSource will be managed by DataImportCron.
			return unmanagedDataSource, nil
		}
		if err != nil {
			return "", err
		}

		// PVC referenced by this DataSource exists. DataSource is managed by SSP operator.
		return managedDataSource, nil
	}
	if err != nil {
		return "", err
	}

	if _, labelExists := foundDataSource.GetLabels()[dataImportCronLabel]; labelExists {
		var isOwnedBySsp = common.CheckOwnerAnnotation(foundDataSource, request.Instance)
		if isOwnedBySsp {
			// This case happens when the label is added to a DataSource with existing PVC
			return transitioningDataSource, nil
		}
		// This DataSource is managed by a DataImportCron
		return unmanagedDataSource, nil
	}

	dsReadyCondition := getDataSourceReadyCondition(foundDataSource)
	if dsReadyCondition != nil && dsReadyCondition.Status != core.ConditionTrue {
		// DataSource is currently not managed by a DataImportCron,
		// but it does not refer to an existing PVC. DataImportCron will manage it.
		return transitioningDataSource, nil
	}

	// DataSource is currently not managed by a DataImportCron,
	// and it refers to an existing PVC. SSP will manage it.
	return managedDataSource, nil
}

func reconcileDataSources(managedDataSources []cdiv1beta1.DataSource, transitioningDataSources []cdiv1beta1.DataSource, request *common.Request) ([]common.ReconcileFunc, error) {
	ownedDataSources, err := listAllOwnedDataSources(request)
	if err != nil {
		return nil, err
	}

	dsNames := make(map[string]struct{}, len(managedDataSources)+len(transitioningDataSources))

	var funcs []common.ReconcileFunc
	for i := range managedDataSources {
		dataSource := managedDataSources[i] // Make a local copy
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return reconcileDataSource(&dataSource, request)
		})
		dsNames[dataSource.GetName()] = struct{}{}
	}

	for i := range transitioningDataSources {
		dataSource := transitioningDataSources[i] // Make a local copy
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			foundDataSource := &cdiv1beta1.DataSource{}
			err := request.Client.Get(request.Context, client.ObjectKeyFromObject(&dataSource), foundDataSource)
			if err != nil && !errors.IsNotFound(err) {
				return common.ReconcileResult{}, err
			}

			result := common.ReconcileResult{
				Resource:        &dataSource,
				OperationResult: common.OperationResultNone,
			}

			if common.CheckOwnerAnnotation(foundDataSource, request.Instance) {
				delete(foundDataSource.GetAnnotations(), handler.TypeAnnotation)
				delete(foundDataSource.GetAnnotations(), handler.NamespacedNameAnnotation)
				err := request.Client.Update(request.Context, foundDataSource)
				if err != nil && !errors.IsNotFound(err) {
					return common.ReconcileResult{}, err
				}
				result.OperationResult = common.OperationResultUpdated
			}

			return result, nil
		})
		dsNames[dataSource.GetName()] = struct{}{}
	}

	// Remove owned DataSources that are not in the 'managedDataSources' or 'transitioningDataSources'
	for i := range ownedDataSources {
		if _, isUsed := dsNames[ownedDataSources[i].GetName()]; isUsed {
			continue
		}

		dataSource := ownedDataSources[i] // Make local copy
		dataSource.Namespace = ssp.GoldenImagesNSname
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

func reconcileDataSource(dataSource *cdiv1beta1.DataSource, request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(dataSource).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			// Remove the dataImportCronLabel to signal that
			// this DataSource is not managed by a DataImportCron.
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

	cronNames := make(map[string]struct{}, len(dataImportCrons))

	var funcs []common.ReconcileFunc
	for i := range dataImportCrons {
		cron := dataImportCrons[i] // Make a local copy
		cron.Namespace = ssp.GoldenImagesNSname
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return reconcileDataImportCron(&cron, request)
		})
		cronNames[cron.GetName()] = struct{}{}
	}

	// Remove owned DataImportCrons that are not in the 'dataImportCrons' parameter
	for i := range ownedCrons {
		if _, isUsed := cronNames[ownedCrons[i].GetName()]; isUsed {
			continue
		}

		cron := ownedCrons[i] // Make local copy
		cron.Namespace = ssp.GoldenImagesNSname
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
	foundDataSources := &cdiv1beta1.DataSourceList{}
	err := request.Client.List(request.Context, foundDataSources, client.InNamespace(ssp.GoldenImagesNSname))
	if err != nil {
		return nil, err
	}

	owned := make([]cdiv1beta1.DataSource, 0, len(foundDataSources.Items))
	for _, item := range foundDataSources.Items {
		if !common.CheckOwnerAnnotation(&item, request.Instance) {
			continue
		}
		owned = append(owned, item)
	}
	return owned, nil
}

func listAllOwnedDataImportCrons(request *common.Request) ([]cdiv1beta1.DataImportCron, error) {
	foundCrons := &cdiv1beta1.DataImportCronList{}
	err := request.Client.List(request.Context, foundCrons, client.InNamespace(ssp.GoldenImagesNSname))
	if err != nil {
		return nil, err
	}

	owned := make([]cdiv1beta1.DataImportCron, 0, len(foundCrons.Items))
	for _, item := range foundCrons.Items {
		if !common.CheckOwnerAnnotation(&item, request.Instance) {
			continue
		}
		owned = append(owned, item)
	}
	return owned, nil
}
