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
	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
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
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
// TODO - create roles for template validator

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

	updated, err := initialize(sspRequest)
	if updated || err != nil {
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

	for _, operand := range sspOperands {
		if err := operand.Reconcile(sspRequest); err != nil {
			return ctrl.Result{}, err
		}
	}

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

func isBeingDeleted(object metav1.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

func initialize(request *common.Request) (bool, error) {
	changed := false

	if !isBeingDeleted(request.Instance) {
		if !controllerutil.ContainsFinalizer(request.Instance, finalizerName) {
			controllerutil.AddFinalizer(request.Instance, finalizerName)
			changed = true
		}
	}

	var err error
	if changed {
		err = request.Client.Update(request.Context, request.Instance)
	}
	return changed, err
}

func cleanup(request *common.Request) error {
	if !controllerutil.ContainsFinalizer(request.Instance, finalizerName) {
		return nil
	}

	for _, operand := range sspOperands {
		err := operand.Cleanup(request)
		if err != nil {
			return err
		}
	}

	controllerutil.RemoveFinalizer(request.Instance, finalizerName)
	return request.Client.Update(request.Context, request.Instance)
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
	for _, operand := range sspOperands {
		err := operand.AddWatchTypesToScheme(scheme)
		if err != nil {
			return err
		}
	}
	return nil
}
