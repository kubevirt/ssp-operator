package data_sources

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	core "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal"
	"kubevirt.io/ssp-operator/internal/architecture"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	template_bundle "kubevirt.io/ssp-operator/internal/template-bundle"
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
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/status,verbs=get;list;watch
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
	sourceCollection   template_bundle.DataSourceCollection
	runningOnOpenShift bool
}

var _ operands.Operand = &dataSources{}

func New(sourceCollection template_bundle.DataSourceCollection, runningOnOpenShift bool) operands.Operand {
	return &dataSources{
		sourceCollection:   sourceCollection,
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

type dataSourcesAndCrons struct {
	dataSourceInfos []dataSourceInfo
	dataImportCrons []*cdiv1beta1.DataImportCron
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

	dicFuncs, err := reconcileDataImportCrons(dsAndCrons.dataImportCrons, request)
	if err != nil {
		return nil, err
	}
	funcs = append(funcs, dicFuncs...)

	return common.CollectResourceStatus(request, funcs...)
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
		ownedDataSources, err := listAllOwnedDataSources(request)
		if err != nil {
			return nil, err
		}

		for i := range ownedDataSources {
			objects = append(objects, &ownedDataSources[i])
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

func (d *dataSources) getDataSourcesAndCrons(request *common.Request) (dataSourcesAndCrons, error) {
	isMultiarch := ptr.Deref(request.Instance.Spec.EnableMultipleArchitectures, false)

	var cronByDataSource map[client.ObjectKey]*cdiv1beta1.DataImportCron
	if isMultiarch {
		var err error
		cronByDataSource, err = getCronsByDataSourceMultiArch(&request.Instance.Spec, d.sourceCollection, &request.Logger)
		if err != nil {
			return dataSourcesAndCrons{}, fmt.Errorf("failed to get DataImportCrons: %w", err)
		}
	} else {
		cronByDataSource = getCronsByDataSource(&request.Instance.Spec)
	}

	var dataSourceInfos []dataSourceInfo
	if isMultiarch {
		var err error
		dataSourceInfos, err = getDataSourceInfosMultiArch(d.sourceCollection, cronByDataSource, request)
		if err != nil {
			return dataSourcesAndCrons{}, fmt.Errorf("failed to get DataSources: %w", err)
		}

		clusterArchs, err := architecture.GetSSPArchs(&request.Instance.Spec)
		if err != nil {
			return dataSourcesAndCrons{}, fmt.Errorf("failed to get ClusterArchs: %w", err)
		}

		dataSourceInfos = addDataSourceReferenceForCrons(dataSourceInfos, request.Instance.Spec.CommonTemplates.DataImportCronTemplates, clusterArchs)
	} else {
		var err error
		dataSourceInfos, err = getDataSourceInfos(d.sourceCollection, cronByDataSource, request)
		if err != nil {
			return dataSourcesAndCrons{}, fmt.Errorf("failed to get DataSources: %w", err)
		}
	}

	for i := range dataSourceInfos {
		if !dataSourceInfos[i].autoUpdateEnabled {
			delete(cronByDataSource, client.ObjectKeyFromObject(dataSourceInfos[i].dataSource))
		}
	}

	return dataSourcesAndCrons{
		dataSourceInfos: dataSourceInfos,
		dataImportCrons: slices.Collect(maps.Values(cronByDataSource)),
	}, nil
}

func getCronsByDataSource(sspSpec *ssp.SSPSpec) map[client.ObjectKey]*cdiv1beta1.DataImportCron {
	cronTemplates := sspSpec.CommonTemplates.DataImportCronTemplates
	cronByDataSource := make(map[client.ObjectKey]*cdiv1beta1.DataImportCron, len(cronTemplates))
	for i := range cronTemplates {
		originalCron := cronTemplates[i].AsDataImportCron()
		cron := originalCron.DeepCopy()
		if cron.Namespace == "" {
			cron.Namespace = internal.GoldenImagesNamespace
		}
		// The architecture annotation should not be in the created DataImportCron.
		delete(cron.Annotations, DataImportCronArchsAnnotation)

		addToCronMap(cronByDataSource, cron)
	}

	return cronByDataSource
}

func getCronsByDataSourceMultiArch(sspSpec *ssp.SSPSpec, sourceCollection template_bundle.DataSourceCollection, logger *logr.Logger) (map[client.ObjectKey]*cdiv1beta1.DataImportCron, error) {
	if !ptr.Deref(sspSpec.EnableMultipleArchitectures, false) {
		return nil, fmt.Errorf("multi-architecture needs to be enabled")
	}

	clusterArchs, err := architecture.GetSSPArchs(sspSpec)
	if err != nil {
		return nil, err
	}

	cronByDataSource := map[client.ObjectKey]*cdiv1beta1.DataImportCron{}
	cronTemplates := sspSpec.CommonTemplates.DataImportCronTemplates
	for i := range cronTemplates {
		originalCron := cronTemplates[i].AsDataImportCron()

		// Need a copy, because it is modified later.
		cron := originalCron.DeepCopy()
		if cron.Namespace == "" {
			cron.Namespace = internal.GoldenImagesNamespace
		}

		archsAnnotationValue := cron.Annotations[DataImportCronArchsAnnotation]

		// The architecture annotation should not be in the created DataImportCron.
		delete(cron.Annotations, DataImportCronArchsAnnotation)

		if archsAnnotationValue == "" {
			// The ManagedDataSource needs to point to the default DataSource architecture.
			dsArchs, dsExists := sourceCollection[cron.Spec.ManagedDataSource]
			if !dsExists {
				addToCronMap(cronByDataSource, cron)
				continue
			}

			defaultArch := getDefaultDataSourceArch(clusterArchs, dsArchs)
			if defaultArch == "" {
				// If there is no compatible DataSource architecture, no DataSource is created.
				addToCronMap(cronByDataSource, cron)
				continue
			}

			cron.Spec.ManagedDataSource = cron.Spec.ManagedDataSource + "-" + string(defaultArch)
			addToCronMap(cronByDataSource, cron)
			continue
		}

		cronArchs := parseArchsAnnotation(archsAnnotationValue, logger)
		for _, arch := range cronArchs {
			if !slices.Contains(clusterArchs, arch) {
				continue
			}

			cronCopy := cron.DeepCopy()
			cronCopy.Name = cron.Name + "-" + string(arch)
			setDataImportCronArchFields(cronCopy, arch)
			addToCronMap(cronByDataSource, cronCopy)
		}
	}

	return cronByDataSource, nil
}

func addToCronMap(cronMap map[client.ObjectKey]*cdiv1beta1.DataImportCron, cron *cdiv1beta1.DataImportCron) {
	cronMap[client.ObjectKey{
		Name:      cron.Spec.ManagedDataSource,
		Namespace: cron.Namespace,
	}] = cron
}

// addDataSourceReferenceForCrons adds DataSource references for custom DataImportCron templates.
// The SSP object can contain DataImportCron templates that don't have a common template defined.
func addDataSourceReferenceForCrons(dataSourceInfos []dataSourceInfo, cronTemplates []ssp.DataImportCronTemplate, clusterArchs []architecture.Arch) []dataSourceInfo {
	for i := range cronTemplates {
		originalCron := cronTemplates[i].AsDataImportCron()
		cron := originalCron.DeepCopy()

		dsName := cron.Spec.ManagedDataSource
		if slices.ContainsFunc(dataSourceInfos, func(info dataSourceInfo) bool {
			return info.dataSource.Name == dsName
		}) {
			continue
		}

		archsAnnotationValue := cron.Annotations[DataImportCronArchsAnnotation]
		if archsAnnotationValue == "" {
			// The DataImportCron is not multi-arch.
			continue
		}

		cronArchs := parseArchsAnnotation(archsAnnotationValue, nil)
		defaultArch := getDefaultDataSourceArch(clusterArchs, cronArchs)
		if defaultArch == "" {
			// There is no compatible architecture of DataImportCron. It will not be created.
			continue
		}

		dataSourceInfos = append(dataSourceInfos, dataSourceInfo{
			dataSource: newDataSourceReference(dsName, dsName+"-"+string(defaultArch)),
		})
	}
	return dataSourceInfos
}

func setDataImportCronArchFields(cron *cdiv1beta1.DataImportCron, arch architecture.Arch) {
	archStr := string(arch)
	managedSource := cron.Spec.ManagedDataSource

	if cron.Labels == nil {
		cron.Labels = map[string]string{}
	}
	cron.Labels[common_templates.TemplateArchitectureLabel] = archStr
	cron.Labels[DataImportCronDataSourceNameLabel] = managedSource

	cron.Spec.ManagedDataSource = managedSource + "-" + archStr

	if cron.Spec.Template.Spec.Source != nil {
		source := cron.Spec.Template.Spec.Source
		if source.Registry != nil {
			registry := source.Registry
			if registry.Platform == nil {
				registry.Platform = &cdiv1beta1.PlatformOptions{}
			}
			registry.Platform.Architecture = archStr
		}
	}
}

func getDataSourceInfos(sourceCollection template_bundle.DataSourceCollection, cronByDataSource map[client.ObjectKey]*cdiv1beta1.DataImportCron, request *common.Request) ([]dataSourceInfo, error) {
	if ptr.Deref(request.Instance.Spec.EnableMultipleArchitectures, false) {
		return nil, fmt.Errorf(".spec.enableMultipleArchitectures needs to be false")
	}

	clusterArchs, err := architecture.GetSSPArchs(&request.Instance.Spec)
	if err != nil {
		return nil, err
	}
	var dataSourceInfos []dataSourceInfo
	for name := range sourceCollection.Names() {
		if !sourceCollection.Contains(name, clusterArchs[0]) {
			continue
		}

		dataSource := newDataSource(name)
		autoUpdateEnabled, err := dataSourceAutoUpdateEnabled(dataSource, cronByDataSource, request)
		if err != nil {
			return nil, err
		}

		var dicName string
		if dic, ok := cronByDataSource[client.ObjectKeyFromObject(dataSource)]; ok {
			dicName = dic.GetName()
		}

		dataSourceInfos = append(dataSourceInfos, dataSourceInfo{
			dataSource:         dataSource,
			autoUpdateEnabled:  autoUpdateEnabled,
			dataImportCronName: dicName,
		})
	}
	return dataSourceInfos, nil
}

func getDataSourceInfosMultiArch(sourceCollection template_bundle.DataSourceCollection, cronByDataSource map[client.ObjectKey]*cdiv1beta1.DataImportCron, request *common.Request) ([]dataSourceInfo, error) {
	if !ptr.Deref(request.Instance.Spec.EnableMultipleArchitectures, false) {
		return nil, fmt.Errorf("multi-architecture needs to be enabled")
	}

	clusterArchs, err := architecture.GetSSPArchs(&request.Instance.Spec)
	if err != nil {
		return nil, err
	}

	var dataSourceInfos []dataSourceInfo
	for name, dsArchs := range sourceCollection {
		defaultArch := getDefaultDataSourceArch(clusterArchs, dsArchs)
		if defaultArch == "" {
			// We can skip creating the DataSources, because none of its architectures
			// are supported on the cluster.
			continue
		}

		dataSourceInfos = append(dataSourceInfos, dataSourceInfo{
			dataSource: newDataSourceReference(name, name+"-"+string(defaultArch)),
		})

		for _, arch := range dsArchs {
			if !slices.Contains(clusterArchs, arch) {
				continue
			}

			dsName := name + "-" + string(arch)
			dataSource := newDataSource(dsName)
			dataSource.Labels = map[string]string{
				common_templates.TemplateArchitectureLabel: string(arch),
			}

			autoUpdateEnabled, err := dataSourceAutoUpdateEnabled(dataSource, cronByDataSource, request)
			if err != nil {
				return nil, err
			}

			if arch == defaultArch {
				// Special logic is needed for the default DataSource to keep backward compatibility.
				var err error
				dataSource, autoUpdateEnabled, err = handleDefaultDataSource(dataSource, autoUpdateEnabled, name, request)
				if err != nil {
					return nil, fmt.Errorf("faield to handle default DataSource: %w", err)
				}
			}

			var dicName string
			if dic, ok := cronByDataSource[client.ObjectKeyFromObject(dataSource)]; ok {
				dicName = dic.GetName()
			}

			dataSourceInfos = append(dataSourceInfos, dataSourceInfo{
				dataSource:         dataSource,
				autoUpdateEnabled:  autoUpdateEnabled,
				dataImportCronName: dicName,
			})
		}
	}
	return dataSourceInfos, nil
}

const dataImportCronLabel = "cdi.kubevirt.io/dataImportCron"

func dataSourceAutoUpdateEnabled(dataSource *cdiv1beta1.DataSource, cronByDataSource map[client.ObjectKey]*cdiv1beta1.DataImportCron, request *common.Request) (bool, error) {
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

func handleDefaultDataSource(dataSource *cdiv1beta1.DataSource, autoUpdate bool, originalPvcName string, request *common.Request) (*cdiv1beta1.DataSource, bool, error) {
	foundDataSource := &cdiv1beta1.DataSource{}
	err := request.Client.Get(request.Context, client.ObjectKeyFromObject(dataSource), foundDataSource)
	if err != nil && !errors.IsNotFound(err) {
		return nil, false, err
	}

	if errors.IsNotFound(err) {
		err := request.UncachedReader.Get(request.Context, client.ObjectKey{
			Name:      originalPvcName,
			Namespace: dataSource.Spec.Source.PVC.Namespace,
		}, &core.PersistentVolumeClaim{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, false, err
		}

		// For backward compatibility, if the original PVC exist,
		// the default DataSource should point to it.
		if !errors.IsNotFound(err) {
			dataSource.Spec.Source.PVC.Name = originalPvcName
			return dataSource, false, nil
		}
		return dataSource, autoUpdate, nil
	}

	if _, foundDsUsesAutoUpdate := foundDataSource.GetLabels()[dataImportCronLabel]; foundDsUsesAutoUpdate {
		// Found DS is labeled to use auto-update.
		return dataSource, autoUpdate, nil
	}

	dsReadyCondition := getDataSourceReadyCondition(foundDataSource)
	if dsReadyCondition == nil {
		// CDI has not yet seen this DataSource, so it is left unchanged.
		dataSource.Spec = foundDataSource.Spec
		return dataSource, false, nil
	}

	if dsReadyCondition.Status == core.ConditionTrue {
		// If the found DataSource is pointing to the old PVC, don't change it.
		sourcePVC := foundDataSource.Spec.Source.PVC
		if sourcePVC != nil && sourcePVC.Name == originalPvcName && sourcePVC.Namespace == dataSource.Namespace {
			dataSource.Spec.Source.PVC.Name = originalPvcName
			return dataSource, false, nil
		}
	}

	return dataSource, autoUpdate, nil
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

func reconcileDataImportCrons(dataImportCrons []*cdiv1beta1.DataImportCron, request *common.Request) ([]common.ReconcileFunc, error) {
	ownedCrons, err := listAllOwnedDataImportCrons(request)
	if err != nil {
		return nil, err
	}

	crons := make(map[client.ObjectKey]struct{}, len(dataImportCrons))

	var funcs []common.ReconcileFunc
	for i := range dataImportCrons {
		cron := dataImportCrons[i] // Make a local copy
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return reconcileDataImportCron(cron, request)
		})
		crons[client.ObjectKeyFromObject(cron)] = struct{}{}
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

func getDefaultDataSourceArch(clusterArchs, dataSourceArchs []architecture.Arch) architecture.Arch {
	// Default arch is the first one that is defined in the SSP and in the common templates
	for _, arch := range clusterArchs {
		if slices.Contains(dataSourceArchs, arch) {
			return arch
		}
	}
	return ""
}

func parseArchsAnnotation(value string, logger *logr.Logger) []architecture.Arch {
	var result []architecture.Arch
	for archStr := range strings.SplitSeq(value, ",") {
		arch, err := architecture.ToArch(strings.TrimSpace(archStr))
		if err != nil {
			// Ignoring invalid architectures
			if logger != nil {
				logger.V(4).Info("Unknown DataImportCron template architecture, ignoring it.", "value", archStr)
			}
			continue
		}
		result = append(result, arch)
	}
	return result
}
