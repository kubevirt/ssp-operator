package controllers

import (
	ctrl "sigs.k8s.io/controller-runtime"

	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
)

type Controller interface {
	Name() string
	AddToManager(mgr ctrl.Manager, crdList crd_watch.CrdList) error
	RequiredCrds() []string
}
