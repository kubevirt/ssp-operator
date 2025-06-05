package common

import (
	"context"

	"github.com/go-logr/logr"
	osconfv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
)

type Request struct {
	reconcile.Request
	Client          client.Client
	UncachedReader  client.Reader
	Context         context.Context
	Instance        *ssp.SSP
	InstanceChanged bool
	Logger          logr.Logger
	VersionCache    VersionCache
	TopologyMode    osconfv1.TopologyMode

	CrdList crd_watch.CrdList
}

func (r *Request) IsSingleReplicaTopologyMode() bool {
	return r.TopologyMode == osconfv1.SingleReplicaTopologyMode
}
