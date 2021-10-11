package data_sources

import (
	"fmt"

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
		&cdiv1beta1.DataImportCron{},
	}
}

type dataSources struct{}

var _ operands.Operand = &dataSources{}

func New() operands.Operand {
	return &dataSources{}
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

func (d *dataSources) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	funcs := []common.ReconcileFunc{
		reconcileGoldenImagesNS,
		reconcileViewRole,
		reconcileViewRoleBinding,
		reconcileEditRole,
	}

	dataImportCronFuncs, err := reconcileDataImportCrons(request)
	if err != nil {
		return nil, err
	}
	funcs = append(funcs, dataImportCronFuncs...)

	return common.CollectResourceStatus(request, funcs...)
}

func (d *dataSources) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	objects := []client.Object{
		newGoldenImagesNS(ssp.GoldenImagesNSname),
		newViewRole(ssp.GoldenImagesNSname),
		newViewRoleBinding(ssp.GoldenImagesNSname),
		newEditRole(),
	}

	ownedCrons, err := listAllOwnedDataImportCrons(request)
	if err != nil {
		return nil, err
	}
	for i := range ownedCrons {
		objects = append(objects, &ownedCrons[i])
	}

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

func reconcileDataImportCrons(request *common.Request) ([]common.ReconcileFunc, error) {
	crons := map[string]cdiv1beta1.DataImportCron{}
	for _, template := range request.Instance.Spec.CommonTemplates.DataImportCronTemplates {
		crons[template.Name] = template.AsDataImportCron()
	}

	var funcs []common.ReconcileFunc

	for _, cronLoopVar := range crons {
		cron := cronLoopVar // Make a local copy
		cron.Namespace = ssp.GoldenImagesNSname
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(&cron).
				SetImmutable(true).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					foundRes.(*cdiv1beta1.DataImportCron).Spec = newRes.(*cdiv1beta1.DataImportCron).Spec
				}).
				Reconcile()
		})
	}

	ownedCrons, err := listAllOwnedDataImportCrons(request)
	if err != nil {
		return nil, err
	}

	for i := range ownedCrons {
		cron := ownedCrons[i]
		if _, isUsed := crons[cron.GetName()]; isUsed {
			continue
		}

		// Unused DataImportCrons will be removed
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
