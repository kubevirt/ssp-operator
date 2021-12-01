package controllers

import (
	"context"
	"sync"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/ssp-operator/controllers/finishable"
)

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

func CreateCrdController(mgr controllerruntime.Manager, requiredCrds []string) (finishable.Controller, error) {
	crds := make(map[string]bool, len(requiredCrds))
	for _, crd := range requiredCrds {
		crds[crd] = false
	}

	reconciler := &waitForCrds{
		client: mgr.GetClient(),
		crds:   crds,
	}

	initCtrl, err := finishable.NewController("init-controller", mgr, reconciler)
	if err != nil {
		return nil, err
	}

	err = initCtrl.Watch(&source.Kind{Type: &extv1.CustomResourceDefinition{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return initCtrl, nil
}

type waitForCrds struct {
	client client.Client

	lock sync.RWMutex
	crds map[string]bool
}

var _ finishable.Reconciler = &waitForCrds{}

func (w *waitForCrds) Reconcile(ctx context.Context, request reconcile.Request) (finishable.Result, error) {
	crdExists := true
	crd := &extv1.CustomResourceDefinition{}
	err := w.client.Get(ctx, request.NamespacedName, crd)
	if err != nil {
		if !errors.IsNotFound(err) {
			return finishable.Result{}, err
		}
		crdExists = false
	}

	// If CRD is being deleted, we treat it as not existing.
	if !crd.GetDeletionTimestamp().IsZero() {
		crdExists = false
	}

	key := request.NamespacedName.Name
	if w.isCrdRequired(key) {
		w.setCrdExists(key, crdExists)
	}

	return finishable.Result{Finished: w.allCrdsExist()}, nil
}

func (w *waitForCrds) isCrdRequired(key string) bool {
	w.lock.RLock()
	defer w.lock.RUnlock()

	_, exists := w.crds[key]
	return exists
}

func (w *waitForCrds) setCrdExists(key string, val bool) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.crds[key] = val
}

func (w *waitForCrds) allCrdsExist() bool {
	w.lock.RLock()
	defer w.lock.RUnlock()

	allExist := true
	for _, exists := range w.crds {
		allExist = allExist && exists
	}
	return allExist
}
