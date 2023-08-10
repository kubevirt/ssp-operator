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
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	osconfv1 "github.com/openshift/api/config/v1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	handler_hook "kubevirt.io/ssp-operator/internal/controller/handler-hook"
	"kubevirt.io/ssp-operator/internal/controller/predicates"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/operands"
)

const (
	finalizerName    = "ssp.kubevirt.io/finalizer"
	oldFinalizerName = "finalize.ssp.kubevirt.io"

	templateBundleDir = "data/common-templates-bundle/"
)

// List of legacy CRDs and their corresponding kinds
var kvsspCRDs = map[string]string{
	"kubevirtmetricsaggregations.ssp.kubevirt.io":    "KubevirtMetricsAggregation",
	"kubevirttemplatevalidators.ssp.kubevirt.io":     "KubevirtTemplateValidator",
	"kubevirtcommontemplatesbundles.ssp.kubevirt.io": "KubevirtCommonTemplatesBundle",
}

// sspReconciler reconciles a SSP object
type sspReconciler struct {
	client           client.Client
	uncachedReader   client.Reader
	log              logr.Logger
	operands         []operands.Operand
	lastSspSpec      ssp.SSPSpec
	subresourceCache common.VersionCache
	topologyMode     osconfv1.TopologyMode
	crdList          crd_watch.CrdList
	areCrdsMissing   bool
}

func NewSspReconciler(client client.Client, uncachedReader client.Reader, infrastructureTopology osconfv1.TopologyMode, operands []operands.Operand, crdList crd_watch.CrdList) *sspReconciler {
	return &sspReconciler{
		client:           client,
		uncachedReader:   uncachedReader,
		log:              ctrl.Log.WithName("controllers").WithName("SSP"),
		operands:         operands,
		subresourceCache: common.VersionCache{},
		topologyMode:     infrastructureTopology,
		crdList:          crdList,
	}
}

var _ reconcile.Reconciler = &sspReconciler{}

// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=ssps,verbs=list;watch;update
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=ssps/status,verbs=update
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=ssps/finalizers,verbs=update
// +kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures;clusterversions,verbs=get
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list

func (r *sspReconciler) setupController(mgr ctrl.Manager) error {
	eventHandlerHook := func(request ctrl.Request, obj client.Object) {
		r.log.Info("Reconciliation event received",
			"ssp", request.NamespacedName,
			"cause_type", reflect.TypeOf(obj).Elem().Name(),
			"cause_object", obj.GetNamespace()+"/"+obj.GetName(),
		)
	}

	builder := ctrl.NewControllerManagedBy(mgr)
	watchSspResource(builder)

	r.areCrdsMissing = len(r.crdList.MissingCrds()) > 0

	// Register watches for created objects only if all required CRDs exist
	watchClusterResources(builder, r.crdList, r.operands, eventHandlerHook)
	watchNamespacedResources(builder, r.crdList, r.operands, eventHandlerHook)

	return builder.Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *sspReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	defer func() {
		if err != nil {
			common.SSPOperatorReconcileSucceeded.Set(0)
		}
	}()
	reqLogger := r.log.WithValues("ssp", req.NamespacedName)
	reqLogger.Info("Starting reconciliation")

	// Fetch the SSP instance
	instance := &ssp.SSP{}
	err = r.client.Get(ctx, req.NamespacedName, instance)
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
	restartNeeded := r.isRestartNeeded(instance)
	r.clearCacheIfNeeded(instance)

	sspRequest := &common.Request{
		Request:        req,
		Client:         r.client,
		UncachedReader: r.uncachedReader,
		Context:        ctx,
		Instance:       instance,
		Logger:         reqLogger,
		VersionCache:   r.subresourceCache,
		TopologyMode:   r.topologyMode,
		CrdList:        r.crdList,
	}

	if restartNeeded {
		r.restart(sspRequest)
	}

	if !isInitialized(sspRequest.Instance) {
		err := initialize(sspRequest)
		// No need to requeue here, because
		// the update will trigger reconciliation again
		return ctrl.Result{}, err
	}

	if updated, err := updateSsp(sspRequest); updated || (err != nil) {
		// SSP was updated, and the update will trigger reconciliation again.
		return ctrl.Result{}, err
	}

	if isBeingDeleted(sspRequest.Instance) {
		err := r.cleanup(sspRequest)
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
		err := r.client.Status().Update(ctx, instance)
		return ctrl.Result{}, err
	}

	if r.areCrdsMissing {
		err := updateStatusMissingCrds(sspRequest, r.crdList.MissingCrds())
		return ctrl.Result{}, err
	}

	sspRequest.Logger.V(1).Info("Updating CR status prior to operand reconciliation...")
	err = preUpdateStatus(sspRequest)
	if err != nil {
		return handleError(sspRequest, err, sspRequest.Logger)
	}

	sspRequest.Logger.V(1).Info("CR status updated")

	sspRequest.Logger.Info("Reconciling operands...")
	reconcileResults, err := r.reconcileOperands(sspRequest)
	if err != nil {
		return handleError(sspRequest, err, sspRequest.Logger)
	}
	sspRequest.Logger.V(1).Info("Operands reconciled")

	sspRequest.Logger.V(1).Info("Updating CR status post reconciliation...")
	err = updateStatus(sspRequest, reconcileResults)
	if err != nil {
		return ctrl.Result{}, err
	}
	sspRequest.Logger.Info("CR status updated")

	if sspRequest.Instance.Status.Phase == lifecycleapi.PhaseDeployed {
		common.SSPOperatorReconcileSucceeded.Set(1)
	} else {
		common.SSPOperatorReconcileSucceeded.Set(0)
	}

	return ctrl.Result{}, nil
}

func (r *sspReconciler) isRestartNeeded(sspObj *ssp.SSP) bool {
	if reflect.DeepEqual(r.lastSspSpec, ssp.SSPSpec{}) {
		return false
	}
	if !reflect.DeepEqual(r.lastSspSpec.TLSSecurityProfile, sspObj.Spec.TLSSecurityProfile) {
		return true
	}
	return false
}

func (r *sspReconciler) restart(request *common.Request) {
	r.log.Info("TLSSecurityProfile changed, restarting")
	err := setSspResourceDeploying(request)
	if err != nil {
		r.log.Info("Error at setSspResourceDeploying", "Error: ", err)
	}
	os.Exit(0)
}

func (r *sspReconciler) clearCacheIfNeeded(sspObj *ssp.SSP) {
	if !reflect.DeepEqual(r.lastSspSpec, sspObj.Spec) {
		r.subresourceCache = common.VersionCache{}
		r.lastSspSpec = sspObj.Spec
	}
}

func (r *sspReconciler) clearCache() {
	r.lastSspSpec = ssp.SSPSpec{}
	r.subresourceCache = common.VersionCache{}
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
	return updateSspResource(request)
}

func updateSsp(request *common.Request) (bool, error) {
	updated := false

	// Update old finalizer to new one
	if controllerutil.ContainsFinalizer(request.Instance, oldFinalizerName) {
		controllerutil.RemoveFinalizer(request.Instance, oldFinalizerName)
		controllerutil.AddFinalizer(request.Instance, finalizerName)
		updated = true
	}

	if !updated {
		return false, nil
	}

	err := updateSspResource(request)
	return err == nil, err
}

func setSspResourceDeploying(request *common.Request) error {
	request.Instance.Status.Phase = lifecycleapi.PhaseDeploying
	request.Instance.Status.ObservedGeneration = request.Instance.Generation
	return request.Client.Status().Update(request.Context, request.Instance)
}

func updateSspResource(request *common.Request) error {
	err := request.Client.Update(request.Context, request.Instance)
	if err != nil {
		return err
	}

	return setSspResourceDeploying(request)
}

func (r *sspReconciler) cleanup(request *common.Request) error {
	if controllerutil.ContainsFinalizer(request.Instance, finalizerName) ||
		controllerutil.ContainsFinalizer(request.Instance, oldFinalizerName) {
		sspStatus := &request.Instance.Status
		sspStatus.Phase = lifecycleapi.PhaseDeleting
		sspStatus.ObservedGeneration = request.Instance.Generation
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: "Deleting SSP resources",
		})
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: "Deleting SSP resources",
		})
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: "Deleting SSP resources",
		})

		err := request.Client.Status().Update(request.Context, request.Instance)
		if err != nil {
			return err
		}

		pendingCount := 0
		for _, operand := range r.operands {
			cleanupResults, err := operand.Cleanup(request)
			if err != nil {
				return err
			}

			for _, result := range cleanupResults {
				if !result.Deleted {
					pendingCount += 1
				}
			}
		}

		if pendingCount > 0 {
			// Will retry cleanup on next reconciliation iteration
			return nil
		}

		controllerutil.RemoveFinalizer(request.Instance, finalizerName)
		controllerutil.RemoveFinalizer(request.Instance, oldFinalizerName)
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
	if err != nil {
		return nil
	}

	for _, item := range crds.Items {
		name := item.GetName()
		for crd, kind := range kvsspCRDs {
			if crd == name {
				foundKinds = append(foundKinds, kind)
				break
			}
		}
	}

	return foundKinds
}

func (r *sspReconciler) reconcileOperands(sspRequest *common.Request) ([]common.ReconcileResult, error) {
	kinds := listExistingCRDKinds(sspRequest)

	// Mark existing CRs as paused
	err := pauseCRs(sspRequest, kinds)
	if err != nil {
		return nil, err
	}

	// Reconcile all operands
	allReconcileResults := make([]common.ReconcileResult, 0, len(r.operands))
	for _, operand := range r.operands {
		sspRequest.Logger.V(1).Info(fmt.Sprintf("Reconciling operand: %s", operand.Name()))
		reconcileResults, err := operand.Reconcile(sspRequest)
		if err != nil {
			sspRequest.Logger.Info(fmt.Sprintf("Operand reconciliation failed: %s", err.Error()))
			return nil, err
		}
		allReconcileResults = append(allReconcileResults, reconcileResults...)
	}

	return allReconcileResults, nil
}

func preUpdateStatus(request *common.Request) error {
	operatorVersion := common.GetOperatorVersion()

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
			Reason:  "Available",
			Message: "Reconciling SSP resources",
		})
	}

	if !conditionsv1.IsStatusConditionPresentAndEqual(sspStatus.Conditions, conditionsv1.ConditionProgressing, v1.ConditionTrue) {
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: "Reconciling SSP resources",
		})
	}

	if !conditionsv1.IsStatusConditionPresentAndEqual(sspStatus.Conditions, conditionsv1.ConditionDegraded, v1.ConditionTrue) {
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: "Reconciling SSP resources",
		})
	}

	return request.Client.Status().Update(request.Context, request.Instance)
}

func updateStatus(request *common.Request, reconcileResults []common.ReconcileResult) error {
	notAvailable := make([]common.ReconcileResult, 0, len(reconcileResults))
	progressing := make([]common.ReconcileResult, 0, len(reconcileResults))
	degraded := make([]common.ReconcileResult, 0, len(reconcileResults))
	for _, reconcileResult := range reconcileResults {
		if reconcileResult.Status.NotAvailable != nil {
			notAvailable = append(notAvailable, reconcileResult)
		}
		if reconcileResult.Status.Progressing != nil {
			progressing = append(progressing, reconcileResult)
		}
		if reconcileResult.Status.Degraded != nil {
			degraded = append(degraded, reconcileResult)
		}
	}

	sspStatus := &request.Instance.Status
	switch len(notAvailable) {
	case 0:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionTrue,
			Reason:  "Available",
			Message: "All SSP resources are available",
		})
	case 1:
		reconcileResult := notAvailable[0]
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: prefixResourceTypeAndName(*reconcileResult.Status.NotAvailable, reconcileResult.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: fmt.Sprintf("%d SSP resources are not available", len(notAvailable)),
		})
	}

	switch len(progressing) {
	case 0:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionFalse,
			Reason:  "Progressing",
			Message: "No SSP resources are progressing",
		})
	case 1:
		reconcileResult := progressing[0]
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: prefixResourceTypeAndName(*reconcileResult.Status.Progressing, reconcileResult.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: fmt.Sprintf("%d SSP resources are progressing", len(progressing)),
		})
	}

	switch len(degraded) {
	case 0:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionFalse,
			Reason:  "Degraded",
			Message: "No SSP resources are degraded",
		})
	case 1:
		reconcileResult := degraded[0]
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: prefixResourceTypeAndName(*reconcileResult.Status.Degraded, reconcileResult.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: fmt.Sprintf("%d SSP resources are degraded", len(degraded)),
		})
	}

	sspStatus.ObservedGeneration = request.Instance.Generation
	if len(notAvailable) == 0 && len(progressing) == 0 && len(degraded) == 0 {
		sspStatus.Phase = lifecycleapi.PhaseDeployed
		sspStatus.ObservedVersion = common.GetOperatorVersion()
	} else {
		sspStatus.Phase = lifecycleapi.PhaseDeploying
	}

	return request.Client.Status().Update(request.Context, request.Instance)
}

func updateStatusMissingCrds(request *common.Request, missingCrds []string) error {
	sspStatus := &request.Instance.Status

	message := fmt.Sprintf("Required CRDs are missing: %s", strings.Join(missingCrds, ", "))
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionAvailable,
		Status:  v1.ConditionFalse,
		Reason:  "Available",
		Message: message,
	})

	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionProgressing,
		Status:  v1.ConditionTrue,
		Reason:  "Progressing",
		Message: message,
	})

	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionDegraded,
		Status:  v1.ConditionTrue,
		Reason:  "Degraded",
		Message: message,
	})

	return request.Client.Status().Update(request.Context, request.Instance)
}

func prefixResourceTypeAndName(message string, resource client.Object) string {
	return fmt.Sprintf("%s %s/%s: %s",
		resource.GetObjectKind().GroupVersionKind().Kind,
		resource.GetNamespace(),
		resource.GetName(),
		message)
}

func handleError(request *common.Request, errParam error, logger logr.Logger) (ctrl.Result, error) {
	if errParam == nil {
		return ctrl.Result{}, nil
	}

	if errors.IsConflict(errParam) {
		// Conflict happens if multiple components modify the same resource.
		// Ignore the error and restart reconciliation.
		logger.Info("Restarting reconciliation",
			"cause", errParam.Error(),
		)
		return ctrl.Result{Requeue: true}, nil
	}

	// Default error handling, if error is not known
	errorMsg := fmt.Sprintf("Error: %v", errParam)
	sspStatus := &request.Instance.Status
	sspStatus.Phase = lifecycleapi.PhaseDeploying
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionAvailable,
		Status:  v1.ConditionFalse,
		Reason:  "Available",
		Message: errorMsg,
	})
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionProgressing,
		Status:  v1.ConditionTrue,
		Reason:  "Progressing",
		Message: errorMsg,
	})
	conditionsv1.SetStatusCondition(&sspStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionDegraded,
		Status:  v1.ConditionTrue,
		Reason:  "Degraded",
		Message: errorMsg,
	})
	err := request.Client.Status().Update(request.Context, request.Instance)
	if err != nil {
		request.Logger.Error(err, "Error updating SSP status.")
	}

	return ctrl.Result{}, errParam
}

func watchSspResource(bldr *ctrl.Builder) {
	// Predicate is used to only reconcile on these changes to the SSP resource:
	// - changes in spec, labels and annotations - using relevantChangesPredicate()
	// - deletion timestamp - to trigger cleanup when SSP CR is being deleted
	// - finalizers - to trigger reconciliation after initialization
	//
	// Importantly, the reconciliation is not triggered on status change.
	// Otherwise, it would cause a reconciliation loop.
	pred := predicate.Or(
		relevantChangesPredicate(),
		predicate.Funcs{UpdateFunc: func(event event.UpdateEvent) bool {
			return !event.ObjectNew.GetDeletionTimestamp().Equal(event.ObjectOld.GetDeletionTimestamp()) ||
				!reflect.DeepEqual(event.ObjectNew.GetFinalizers(), event.ObjectOld.GetFinalizers())
		}},
	)

	bldr.For(&ssp.SSP{}, builder.WithPredicates(pred))
}

func watchNamespacedResources(builder *ctrl.Builder, crdList crd_watch.CrdList, sspOperands []operands.Operand, eventHandlerHook handler_hook.HookFunc) {
	watchResources(builder,
		crdList,
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &ssp.SSP{},
		},
		sspOperands,
		operands.Operand.WatchTypes,
		eventHandlerHook,
	)
}

func watchClusterResources(builder *ctrl.Builder, crdList crd_watch.CrdList, sspOperands []operands.Operand, eventHandlerHook handler_hook.HookFunc) {
	watchResources(builder,
		crdList,
		&libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Group: ssp.GroupVersion.Group,
				Kind:  "SSP",
			},
		},
		sspOperands,
		operands.Operand.WatchClusterTypes,
		eventHandlerHook,
	)
}

func watchResources(ctrlBuilder *ctrl.Builder, crdList crd_watch.CrdList, handler handler.EventHandler, sspOperands []operands.Operand, watchTypesFunc func(operands.Operand) []operands.WatchType, hookFunc handler_hook.HookFunc) {
	// Deduplicate watches
	watchedTypes := make(map[reflect.Type]operands.WatchType)
	for _, operand := range sspOperands {
		for _, watchType := range watchTypesFunc(operand) {
			key := reflect.TypeOf(watchType.Object)
			// If at least one watchType wants to watch full object,
			// then the stored watchType should also watch full object.
			if wt, ok := watchedTypes[key]; ok {
				watchType.WatchFullObject = watchType.WatchFullObject || wt.WatchFullObject
			}
			watchedTypes[key] = watchType
		}
	}

	for _, watchType := range watchedTypes {
		if watchType.Crd != "" && !crdList.CrdExists(watchType.Crd) {
			// Do not watch resources without CRD
			continue
		}

		var predicates []predicate.Predicate
		if !watchType.WatchFullObject {
			predicates = []predicate.Predicate{relevantChangesPredicate()}
		}

		ctrlBuilder.Watches(
			&source.Kind{Type: watchType.Object},
			handler_hook.New(handler, hookFunc),
			builder.WithPredicates(predicates...),
		)
	}
}

// relevantChangesPredicate is used to only reconcile on certain changes to watched resources
// - any change in spec
// - labels or annotations - to detect if necessary labels or annotations were modified or removed
func relevantChangesPredicate() predicate.Predicate {
	return predicate.Or(
		predicate.LabelChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
		predicates.SpecChangedPredicate{},
	)
}
