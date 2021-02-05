package common

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
)

type Request struct {
	reconcile.Request
	Client       client.Client
	Context      context.Context
	Instance     *ssp.SSP
	Logger       logr.Logger
	VersionCache VersionCache
}
