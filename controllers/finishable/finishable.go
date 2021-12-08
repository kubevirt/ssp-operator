package finishable

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Reconciler interface extends reconcile.Reconciler interface
// with the option to stop the reconciliation.
type Reconciler interface {
	Reconcile(context.Context, reconcile.Request) (Result, error)
}

type Controller interface {
	manager.Runnable

	Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error
}

type Result struct {
	reconcile.Result
	Finished bool
}

type wrapper struct {
	reconciler Reconciler
	stopFunc   func()
}

var _ reconcile.Reconciler = &wrapper{}

func (w *wrapper) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	res, err := w.reconciler.Reconcile(ctx, request)
	if res.Finished {
		w.stopFunc()
	}
	return res.Result, err
}

type controller struct {
	reconcilerWrapper *wrapper
	innerController   ctrl.Controller
}

var _ Controller = &controller{}

func (c *controller) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	return c.innerController.Watch(src, eventhandler, predicates...)
}

func (c *controller) Start(ctx context.Context) error {
	innerCtx, innerCtxCancel := context.WithCancel(ctx)
	// Using defer here in case the fllowing innerController.Start()
	// will end with an error. In that case the context would not be closed.
	// Closing a context multiple times is supported.
	defer innerCtxCancel()

	c.reconcilerWrapper.stopFunc = innerCtxCancel

	return c.innerController.Start(innerCtx)
}

// NewController returns a controller that can be stopped from the Reconciler implementation.
func NewController(name string, mgr manager.Manager, reconciler Reconciler) (Controller, error) {
	wrap := &wrapper{reconciler: reconciler}

	innerController, err := ctrl.NewUnmanaged(name, mgr, ctrl.Options{
		Reconciler: wrap,
	})
	if err != nil {
		return nil, err
	}

	return &controller{
		reconcilerWrapper: wrap,
		innerController:   innerController,
	}, nil
}
