package controllers

import ctrl "sigs.k8s.io/controller-runtime"

type Controller interface {
	Name() string
	AddToManager(mgr ctrl.Manager) error
}
