/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	node_labeller "kubevirt.io/ssp-operator/internal/operands/node-labeller"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
)

const finalizerName = "finalize.ssp.kubevirt.io"
const defaultOperatorVersion = "devel"

var sspOperands = []operands.Operand{
	metrics.GetOperand(),
	template_validator.GetOperand(),
	common_templates.GetOperand(),
	node_labeller.GetOperand(),
}

// List of legacy CRDs and their corresponding kinds
var kvsspCRDs = map[string]string{
	"kubevirtmetricsaggregations.ssp.kubevirt.io":    "KubevirtMetricsAggregation",
	"kubevirttemplatevalidators.ssp.kubevirt.io":     "KubevirtTemplateValidator",
	"kubevirtcommontemplatesbundles.ssp.kubevirt.io": "KubevirtCommonTemplatesBundle",
	"kubevirtnodelabellerbundles.ssp.kubevirt.io":    "KubevirtNodeLabellerBundle",
}

// SSPReconciler reconciles a SSP object
type SSPReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	LastSspSpec      ssp.SSPSpec
	SubresourceCache common.VersionCache
}

// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=ssps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=ssps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=ssps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=kubevirtcommontemplatesbundles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=kubevirtmetricsaggregations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=kubevirtnodelabellerbundles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=kubevirttemplatevalidators,verbs=get;list;watch;create;update;patch;delete

func (r *SSPReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("ssp", req.NamespacedName)
	reqLogger.V(1).Info("Starting reconciliation...")

	ctx := context.Background()

	// Fetch the SSP instance
	instance := &ssp.SSP{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	r.clearCacheIfNeeded(instance)

	sspRequest := &common.Request{
		Request:      req,
		Client:       r,
		Scheme:       r.Scheme,
		Context:      ctx,
		Instance:     instance,
		Logger:       reqLogger,
		VersionCache: r.SubresourceCache,
	}

	if !isInitialized(sspRequest.Instance) {
		err := initialize(sspRequest)
		// No need to requeue here, because
		// the update will trigger reconciliation again
		return ctrl.Result{}, err
	}

	if isBeingDeleted(sspRequest.Instance) {
		err := cleanup(sspRequest)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.clearCache()
		return ctrl.Result{}, nil
	}

	if isPaused(instance) {
		if instance.Status.Paused {
			return ctrl.Result{}, nil
		}
		reqLogger.Info(fmt.Sprintf("Pausing SSP operator on resource: %v/%v", instance.Namespace, instance.Name))
		instance.Status.Paused = true
		instance.Status.ObservedGeneration = instance.Generation
		err := r.Status().Update(ctx, instance)
		return ctrl.Result{}, err
	}

	sspRequest.Logger.V(1).Info("Updating CR status prior to operand reconciliation...")
	err = preUpdateStatus(sspRequest)
	if err != nil {
		return ctrl.Result{}, err
	}
	sspRequest.Logger.V(1).Info("CR status updated")

	sspRequest.Logger.V(1).Info("Reconciling operands...")
	statuses, err := reconcileOperands(sspRequest)
	if err != nil {
		return handleError(sspRequest, err)
	}
	sspRequest.Logger.V(1).Info("Operands reconciled")

	sspRequest.Logger.V(1).Info("Updating CR status post reconciliation...")
	err = updateStatus(sspRequest, statuses)
	if err != nil {
		return ctrl.Result{}, err
	}
	sspRequest.Logger.V(1).Info("CR status updated")

	return ctrl.Result{}, nil
}

func (r *SSPReconciler) clearCacheIfNeeded(sspObj *ssp.SSP) {
	if !reflect.DeepEqual(r.LastSspSpec, sspObj.Spec) {
		r.SubresourceCache = common.VersionCache{}
		r.LastSspSpec = sspObj.Spec
	}
}

func (r *SSPReconciler) clearCache() {
	r.LastSspSpec = ssp.SSPSpec{}
	r.SubresourceCache = common.VersionCache{}
}

func getOperatorVersion() string {
	return common.EnvOrDefault(common.OperatorVersionKey, defaultOperatorVersion)
}

func isPaused(object metav1.Object) bool {
	if object.GetAnnotations() == nil {
		return false
	}
	pausedStr, ok := object.GetAnnotations()[ssp.OperatorPausedAnnotation]
	if !ok {
		return false
	}
	paused, err := strconv.ParseBool(pausedStr)
	if err != nil {
		return false
	}
	return paused
}

func isBeingDeleted(object metav1.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

func isInitialized(ssp *ssp.SSP) bool {
	return isBeingDeleted(ssp) || ssp.Status.Phase != lifecycleapi.PhaseEmpty
}

func initialize(request *common.Request) error {
	controllerutil.AddFinalizer(request.Instance, finalizerName)
	err := request.Client.Update(request.Context, request.Instance)
	if err != nil {
		return err
	}

	request.Instance.Status.Phase = lifecycleapi.PhaseDeploying
	request.Instance.Status.ObservedGeneration = request.Instance.Generation
	return request.Client.Status().Update(request.Context, request.Instance)
}

func cleanup(request *common.Request) error {
	if controllerutil.ContainsFinalizer(request.Instance, finalizerName) {
		request.Instance.Status.Phase = lifecycleapi.PhaseDeleting
		request.Instance.Status.ObservedGeneration = request.Instance.Generation
		err := request.Client.Status().Update(request.Context, request.Instance)
		if err != nil {
			return err
		}
		for _, operand := range sspOperands {
			err = operand.Cleanup(request)
			if err != nil {
				return err
			}
		}
		controllerutil.RemoveFinalizer(request.Instance, finalizerName)
		err = request.Client.Update(request.Context, request.Instance)
		if err != nil {
			return err
		}
	}

	request.Instance.Status.Phase = lifecycleapi.PhaseDeleted
	request.Instance.Status.ObservedGeneration = request.Instance.Generation
	err := request.Client.Status().Update(request.Context, request.Instance)
	if errors.IsConflict(err) || errors.IsNotFound(err) {
		// These errors are ignored. They can happen if the CR was removed
		// before the status update call is executed.
		return nil
	}
	return err
}

func pauseCRs(sspRequest *common.Request, kinds []string) error {
	patch := []byte(`{
  "metadata":{
    "annotations":{"kubevirt.io/operator.paused": "true"}
  }
}`)
	for _, kind := range kinds {
		crs := &unstructured.UnstructuredList{}
		crs.SetKind(kind)
		crs.SetAPIVersion("ssp.kubevirt.io/v1")
		err := sspRequest.Client.List(sspRequest.Context, crs)
		if err != nil {
			sspRequest.Logger.Error(err, fmt.Sprintf("Error listing %s CRs: %s", kind, err))
			return err
		}
		for _, item := range crs.Items {
			err = sspRequest.Client.Patch(sspRequest.Context, &item, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				// Patching failed, maybe the CR just got removed? Log an error but keep going.
				sspRequest.Logger.Error(err, fmt.Sprintf("Error pausing %s from namespace %s: %s",
					item.GetName(), item.GetNamespace(), err))
			}
		}
	}

	return nil
}

func listExistingCRDKinds(sspRequest *common.Request) []string {
	// Get the list of all CRDs and build a list of the SSP ones
	crds := &unstructured.UnstructuredList{}
	crds.SetKind("CustomResourceDefinition")
	crds.SetAPIVersion("apiextensions.k8s.io/v1")
	err := sspRequest.Client.List(sspRequest.Context, crds)
	foundKinds := make([]string, 0, len(kvsspCRDs))
	if err == nil {
		for _, item := range crds.Items {
			name := item.GetName()
			for crd, kind := range kvsspCRDs {
				if crd == name {
					foundKinds = append(foundKinds, kind)
					break
				}
			}
		}
	}

	return foundKinds
}

func reconcileOperands(sspRequest *common.Request) ([]common.ResourceStatus, error) {
	kinds := listExistingCRDKinds(sspRequest)

	// Mark existing CRs as paused
	err := pauseCRs(sspRequest, kinds)
	if err != nil {
		return nil, err
	}

	// Reconcile all operands
	allStatuses := make([]common.ResourceStatus, 0, len(sspOperands))
	for _, operand := range sspOperands {
		sspRequest.Logger.V(1).Info(fmt.Sprintf("Reconciling operand: %s", operand.Name()))
		statuses, err := operand.Reconcile(sspRequest)
		if err != nil {
			sspRequest.Logger.V(1).Info(fmt.Sprintf("Operand reconciliation failed: %s", err.Error()))
			return nil, err
		}
		allStatuses = append(allStatuses, statuses...)
	}

	return allStatuses, nil
}

func preUpdateStatus(request *common.Request) error {
	operatorVersion := getOperatorVersion()

	sspStatus := &request.Instance.Status
	sspStatus.Phase = lifecycleapi.PhaseDeploying
	sspStatus.ObservedGeneration = request.Instance.Generation
	sspStatus.OperatorVersion = operatorVersion
	sspStatus.TargetVersion = operatorVersion

	if sspStatus.Paused {
		request.Logger.Info(fmt.Sprintf("Unpausing SSP operator on resource: %v/%v",
			request.Instance.Namespace, request.Instance.Name))
	}
	sspStatus.Paused = false

	if !conditionsv1.IsStatusConditionPresentAndEqual(sspStatus.Conditions, conditionsv1.ConditionAvailable, v1.ConditionFalse) {
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "available",
			Message: "Reconciling SSP resources",
		})
	}

	if !conditionsv1.IsStatusConditionPresentAndEqual(sspStatus.Conditions, conditionsv1.ConditionProgressing, v1.ConditionTrue) {
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "progressing",
			Message: "Reconciling SSP resources",
		})
	}

	if !conditionsv1.IsStatusConditionPresentAndEqual(sspStatus.Conditions, conditionsv1.ConditionDegraded, v1.ConditionTrue) {
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "degraded",
			Message: "Reconciling SSP resources",
		})
	}

	return request.Client.Status().Update(request.Context, request.Instance)
}

func updateStatus(request *common.Request, statuses []common.ResourceStatus) error {
	notAvailable := make([]common.ResourceStatus, 0, len(statuses))
	progressing := make([]common.ResourceStatus, 0, len(statuses))
	degraded := make([]common.ResourceStatus, 0, len(statuses))
	for _, status := range statuses {
		if status.NotAvailable != nil {
			notAvailable = append(notAvailable, status)
		}
		if status.Progressing != nil {
			progressing = append(progressing, status)
		}
		if status.Degraded != nil {
			degraded = append(degraded, status)
		}
	}

	sspStatus := &request.Instance.Status
	switch len(notAvailable) {
	case 0:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionTrue,
			Reason:  "available",
			Message: "All SSP resources are available",
		})
	case 1:
		status := notAvailable[0]
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "available",
			Message: prefixResourceTypeAndName(*status.NotAvailable, status.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "available",
			Message: fmt.Sprintf("%d SSP resources are not available", len(notAvailable)),
		})
	}

	switch len(progressing) {
	case 0:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionFalse,
			Reason:  "progressing",
			Message: "No SSP resources are progressing",
		})
	case 1:
		status := progressing[0]
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "progressing",
			Message: prefixResourceTypeAndName(*status.Progressing, status.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "progressing",
			Message: fmt.Sprintf("%d SSP resources are progressing", len(progressing)),
		})
	}

	switch len(degraded) {
	case 0:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionFalse,
			Reason:  "degraded",
			Message: "No SSP resources are degraded",
		})
	case 1:
		status := degraded[0]
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "degraded",
			Message: prefixResourceTypeAndName(*status.Degraded, status.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "degraded",
			Message: fmt.Sprintf("%d SSP resources are degraded", len(degraded)),
		})
	}

	sspStatus.ObservedGeneration = request.Instance.Generation
	if len(notAvailable) == 0 && len(progressing) == 0 && len(degraded) == 0 {
		sspStatus.Phase = lifecycleapi.PhaseDeployed
		sspStatus.ObservedVersion = getOperatorVersion()
	} else {
		sspStatus.Phase = lifecycleapi.PhaseDeploying
	}

	return request.Client.Status().Update(request.Context, request.Instance)
}

func prefixResourceTypeAndName(message string, resource controllerutil.Object) string {
	return fmt.Sprintf("%s %s/%s: %s",
		resource.GetObjectKind().GroupVersionKind().Kind,
		resource.GetNamespace(),
		resource.GetName(),
		message)
}

func handleError(request *common.Request, errParam error) (ctrl.Result, error) {
	if errParam == nil {
		return ctrl.Result{}, nil
	}

	if errors.IsConflict(errParam) {
		// Conflict happens if multiple components modify the same resource.
		// Ignore the error and restart reconciliation.
		return ctrl.Result{Requeue: true}, nil
	}

	// Default error handling, if error is not known
	errorMsg := fmt.Sprintf("Error: %v", errParam)
	sspStatus := &request.Instance.Status
	sspStatus.Phase = lifecycleapi.PhaseDeploying
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionAvailable,
		Status:  v1.ConditionFalse,
		Reason:  "available",
		Message: errorMsg,
	})
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionProgressing,
		Status:  v1.ConditionTrue,
		Reason:  "progressing",
		Message: errorMsg,
	})
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionDegraded,
		Status:  v1.ConditionTrue,
		Reason:  "degraded",
		Message: errorMsg,
	})
	err := request.Client.Status().Update(request.Context, request.Instance)
	if err != nil {
		request.Logger.Error(err, "Error updating SSP status.")
	}

	return ctrl.Result{}, errParam
}

func (r *SSPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.SubresourceCache = common.VersionCache{}

	builder := ctrl.NewControllerManagedBy(mgr)
	watchSspResource(builder)
	watchClusterResources(builder)
	watchNamespacedResources(builder)
	return builder.Complete(r)
}

func watchSspResource(bldr *ctrl.Builder) {
	// Predicate is used to only reconcile on these changes to the SSP resource:
	// - any change in spec - checked with generation
	// - deletion timestamp - to trigger cleanup when SSP CR is being deleted
	// - labels or annotations - to detect if reconciliation should be paused or unpaused
	// - finalizers - to trigger reconciliation after initialization
	//
	// Importantly, the reconciliation is not triggered on status change.
	// Otherwise it would cause a reconciliation loop.
	pred := predicate.Funcs{UpdateFunc: func(event event.UpdateEvent) bool {
		oldMeta := event.MetaOld
		newMeta := event.MetaNew
		return newMeta.GetGeneration() != oldMeta.GetGeneration() ||
			!newMeta.GetDeletionTimestamp().Equal(oldMeta.GetDeletionTimestamp()) ||
			!reflect.DeepEqual(newMeta.GetLabels(), oldMeta.GetLabels()) ||
			!reflect.DeepEqual(newMeta.GetAnnotations(), oldMeta.GetAnnotations()) ||
			!reflect.DeepEqual(newMeta.GetFinalizers(), oldMeta.GetFinalizers())

	}}

	bldr.For(&ssp.SSP{}, builder.WithPredicates(pred))
}

func watchNamespacedResources(builder *ctrl.Builder) {
	watchResources(builder,
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &ssp.SSP{},
		},
		operands.Operand.WatchTypes,
	)
}

func watchClusterResources(builder *ctrl.Builder) {
	watchResources(builder,
		&libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Group: "ssp.kubevirt.io",
				Kind:  "SSP",
			},
		},
		operands.Operand.WatchClusterTypes,
	)
}

func watchResources(builder *ctrl.Builder, handler handler.EventHandler, watchTypesFunc func(operands.Operand) []runtime.Object) {
	watchedTypes := make(map[reflect.Type]struct{})
	for _, operand := range sspOperands {
		for _, t := range watchTypesFunc(operand) {
			if _, ok := watchedTypes[reflect.TypeOf(t)]; ok {
				continue
			}

			builder.Watches(&source.Kind{Type: t}, handler)
			watchedTypes[reflect.TypeOf(t)] = struct{}{}
		}
	}
}

func InitScheme(scheme *runtime.Scheme) error {
	for _, operand := range sspOperands {
		err := operand.AddWatchTypesToScheme(scheme)
		if err != nil {
			return err
		}
	}
	return nil
}
