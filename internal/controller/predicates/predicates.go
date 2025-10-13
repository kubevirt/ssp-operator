package predicates

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type SpecChangedPredicate struct {
	predicate.Funcs
}

func (p SpecChangedPredicate) Update(e event.UpdateEvent) bool {
	newSpec, exists := getSpec(e.ObjectNew)
	if !exists {
		return true
	}

	oldSpec, exists := getSpec(e.ObjectOld)
	if !exists {
		return true
	}

	return !reflect.DeepEqual(newSpec, oldSpec)
}

func getSpec(obj client.Object) (interface{}, bool) {
	val := reflect.ValueOf(obj).Elem()
	specVal := val.FieldByName("Spec")
	if !specVal.IsValid() {
		return nil, false
	}
	return specVal.Interface(), true
}

type DeletionTimestampChangedPredicate struct {
	predicate.Funcs
}

func (p DeletionTimestampChangedPredicate) Update(e event.UpdateEvent) bool {
	return !e.ObjectNew.GetDeletionTimestamp().Equal(e.ObjectOld.GetDeletionTimestamp())
}

type FinalizerChangedPredicate struct {
	predicate.Funcs
}

func (p FinalizerChangedPredicate) Update(e event.UpdateEvent) bool {
	return !reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers())
}
