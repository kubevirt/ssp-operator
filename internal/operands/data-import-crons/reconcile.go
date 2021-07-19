package dataimportcrons

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=dataimportcrons,verbs=get;list;watch;create;update;patch;delete

type dataImportCrons struct{}

var _ operands.Operand = &dataImportCrons{}

func GetOperand() operands.Operand {
	return &dataImportCrons{}
}

func (c *dataImportCrons) Name() string {
	return operandName
}

const (
	operandName      = "data-import-crons"
	operandComponent = common.AppComponentTemplating
)

func (c *dataImportCrons) AddWatchTypesToScheme(s *runtime.Scheme) error {
	return cdiv1beta1.AddToScheme(s)
}

func (c *dataImportCrons) WatchClusterTypes() []client.Object {
	return nil
}

func (c *dataImportCrons) WatchTypes() []client.Object {
	return []client.Object{
		&cdiv1beta1.DataImportCron{},
	}
}

func (c *dataImportCrons) Reconcile(request *common.Request) ([]common.ResourceStatus, error) {
	return common.CollectResourceStatus(request, reconcileDataImportCronsFuncs(request)...)
}

func (c *dataImportCrons) Cleanup(request *common.Request) error {
	var existingDataImportCrons cdiv1beta1.DataImportCronList
	requirement, err := labels.NewRequirement(common.AppKubernetesManagedByLabel, selection.Equals, []string{common.OperatorName})
	if err != nil {
		panic(err)
	}
	if err := request.Client.List(request.Context, &existingDataImportCrons, &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*requirement),
	}); err != nil {
		request.Logger.Error(err, fmt.Sprintf("Error listing resources for deletion: %s", err))
		return err
	}
	for _, obj := range existingDataImportCrons.Items {
		err := request.Client.Delete(request.Context, &obj)
		if err != nil && !errors.IsNotFound(err) {
			request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", obj.GetName(), err))
			return err
		}
	}
	return nil
}

func reconcileDataImportCronsFuncs(request *common.Request) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(request.Instance.Spec.DataImportCronTemplates))
	cronTemplates := request.Instance.Spec.DataImportCronTemplates
	for i := range cronTemplates {
		cron := &cronTemplates[i]
		funcs = append(funcs, func(request *common.Request) (common.ResourceStatus, error) {
			return common.CreateOrUpdate(request).
				NamespacedResource(cron).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newDataImportCron := newRes.(*cdiv1beta1.DataImportCron)
					foundDataImportCron := foundRes.(*cdiv1beta1.DataImportCron)
					foundDataImportCron.Labels = newDataImportCron.Labels
					foundDataImportCron.Annotations = newDataImportCron.Annotations
					foundDataImportCron.Spec = newDataImportCron.Spec
				}).
				Reconcile()
		})
	}
	return funcs
}
