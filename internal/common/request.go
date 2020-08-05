package common

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
