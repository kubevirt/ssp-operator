package controllers

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
)

type Controller interface {
	Name() string
	AddToManager(mgr ctrl.Manager, crdList crd_watch.CrdList) error
	GetWatchObjects() []WatchObject
}

type WatchObject struct {
	// Object is the object that this option applies to.
	Object client.Object

	// CrdName is the name of the CRD that defines this object.
	CrdName string

	// WatchOnlyOperatorNamespace sets if the cache should only watch the object type
	// in the same namespace where the operator is defined.
	WatchOnlyOperatorNamespace bool

	// WatchOnlyObjectsWithLabel sets if the cache should only watch objets
	/// with label "ssp.kubevirt.io/watched".
	WatchOnlyObjectsWithLabel bool
}
