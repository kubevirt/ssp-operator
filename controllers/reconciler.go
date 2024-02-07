package controllers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ControllerReconciler interface {
	reconcile.Reconciler

	Start(ctx context.Context, mgr ctrl.Manager) error
	Name() string
}
