package controllers

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ControllerReconciler interface {
	Start(ctx context.Context, mgr ctrl.Manager) error
	Name() string
}
