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

	"github.com/go-logr/logr"
	sspopv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ssp "kubevirt.io/ssp-operator/api/v1alpha1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	node_labeller "kubevirt.io/ssp-operator/internal/operands/node-labeller"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
)

const finalizerName = "finalize.ssp.kubevirt.io"

var sspOperands = []operands.Operand{
	metrics.GetOperand(),
	template_validator.GetOperand(),
	common_templates.GetOperand(),
	node_labeller.GetOperand(),
	// TODO - add other operands here
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

func (r *SSPReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("ssp", req.NamespacedName)

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
		Request:              req,
		Client:               r,
		Scheme:               r.Scheme,
		Context:              ctx,
		Instance:             instance,
		Logger:               reqLogger,
		ResourceVersionCache: r.SubresourceCache,
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

	statuses, err := reconcileOperands(sspRequest)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = updateStatus(sspRequest, statuses)
	return ctrl.Result{}, err
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
	return request.Client.Status().Update(request.Context, request.Instance)
}

func cleanup(request *common.Request) error {
	if controllerutil.ContainsFinalizer(request.Instance, finalizerName) {
		request.Instance.Status.Phase = lifecycleapi.PhaseDeleting
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
	err := request.Client.Status().Update(request.Context, request.Instance)
	if errors.IsConflict(err) || errors.IsNotFound(err) {
		// These errors are ignored. They can happen if the CR was removed
		// before the status update call is executed.
		return nil
	}
	return err
}

func reconcileOperands(sspRequest *common.Request) ([]common.ResourceStatus, error) {
	allStatuses := make([]common.ResourceStatus, 0, len(sspOperands))
	for _, operand := range sspOperands {
		statuses, err := operand.Reconcile(sspRequest)
		if err != nil {
			return nil, err
		}
		allStatuses = append(allStatuses, statuses...)
	}
	return allStatuses, nil
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

	if len(notAvailable) == 0 && len(progressing) == 0 && len(degraded) == 0 {
		sspStatus.Phase = lifecycleapi.PhaseDeployed
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

func (r *SSPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.SubresourceCache = common.VersionCache{}

	builder := ctrl.NewControllerManagedBy(mgr).For(&ssp.SSP{})
	watchClusterResources(builder)
	watchNamespacedResources(builder)
	return builder.Complete(r)
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
	sspopv1.AddToScheme(scheme)
	for _, operand := range sspOperands {
		err := operand.AddWatchTypesToScheme(scheme)
		if err != nil {
			return err
		}
	}
	return nil
}
