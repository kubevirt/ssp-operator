package common

import (
	"context"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/ssp-operator/pkg/apis/ssp/v1"
)

type Request struct {
	reconcile.Request
	Client   client.Client
	Scheme   *runtime.Scheme
	Context  context.Context
	Instance *v1.SSP
	Logger   logr.Logger
}

func (r *Request) SetControllerReferenceFor(controlled metav1.Object) error {
	return controllerutil.SetControllerReference(r.Instance, controlled, r.Scheme)
}
