package ssp

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	sspv1 "kubevirt.io/ssp-operator/pkg/apis/ssp/v1"
)

var log = logf.Log.WithName("controller_ssp")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new SSP Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSSP{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ssp-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SSP
	err = c.Watch(&source.Kind{Type: &sspv1.SSP{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return addSecondaryWatches(c,
		metrics.WatchTypes,
		// TODO - add other watch types here
	)
}

func addSecondaryWatches(c controller.Controller, watchTypesGetters ...func() []runtime.Object) error {
	for _, watchTypes := range watchTypesGetters {
		for _, t := range watchTypes() {
			err := c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &sspv1.SSP{},
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// blank assignment to verify that ReconcileSSP implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSSP{}

// ReconcileSSP reconciles a SSP object
type ReconcileSSP struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SSP object and makes changes based on the state read
// and what is in the SSP.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSSP) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SSP")

	sspRequest := &common.Request{
		Request: request,
		Client:  r.client,
		Scheme:  r.scheme,
		Context: context.TODO(),
		Logger:  reqLogger,
	}

	// Fetch the SSP instance
	instance := &sspv1.SSP{}
	err := r.client.Get(sspRequest.Context, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	sspRequest.Instance = instance

	for _, f := range []func(*common.Request) error{
		metrics.Reconcile,
		// TODO - add other reconcile methods here
	} {
		if err := f(sspRequest); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
