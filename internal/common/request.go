package common

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1alpha1"
)

type Request struct {
	reconcile.Request
	Client               client.Client
	Scheme               *runtime.Scheme
	Context              context.Context
	Instance             *ssp.SSP
	Logger               logr.Logger
	ResourceVersionCache VersionCache
}
