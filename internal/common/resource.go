package common

import (
	"fmt"
	"github.com/go-logr/logr"
	"reflect"

	libhandler "github.com/operator-framework/operator-lib/handler"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ResourceUpdateFunc = func(expected, found controllerutil.Object)

func CreateOrUpdateResource(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc) error {
	err := controllerutil.SetControllerReference(request.Instance, resource, request.Scheme)
	if err != nil {
		return err
	}
	return createOrUpdate(request, resource, updateResource)
}

func CreateOrUpdateClusterResource(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc) error {
	err := libhandler.SetOwnerAnnotations(request.Instance, resource)
	if err != nil {
		return err
	}
	return createOrUpdate(request, resource, updateResource)
}

func createOrUpdate(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc) error {
	found := newEmptyResource(resource)
	found.SetName(resource.GetName())
	found.SetNamespace(resource.GetNamespace())
	res, err := controllerutil.CreateOrUpdate(request.Context, request.Client, found, func() error {
		// We expect users will not add any other owner references,
		// if that is not correct, this code needs to be changed.
		found.SetOwnerReferences(resource.GetOwnerReferences())

		updateLabels(resource, found)
		updateAnnotations(resource, found)
		updateResource(resource, found)
		return nil
	})
	if err != nil {
		return err
	}

	logOperation(res, found, request.Logger)
	return nil
}

func newEmptyResource(resource controllerutil.Object) controllerutil.Object {
	return reflect.New(reflect.TypeOf(resource).Elem()).Interface().(controllerutil.Object)
}

func updateAnnotations(expected, found controllerutil.Object) {
	if found.GetAnnotations() == nil {
		found.SetAnnotations(expected.GetAnnotations())
		return
	}
	updateStringMap(expected.GetAnnotations(), found.GetAnnotations())
}

func updateLabels(expected, found controllerutil.Object) {
	if found.GetLabels() == nil {
		found.SetLabels(expected.GetLabels())
		return
	}
	updateStringMap(expected.GetLabels(), found.GetLabels())
}

func updateStringMap(expected, found map[string]string) {
	if expected == nil {
		return
	}
	for key, val := range expected {
		found[key] = val
	}
}

func logOperation(result controllerutil.OperationResult, resource controllerutil.Object, logger logr.Logger) {
	if result == controllerutil.OperationResultCreated {
		logger.Info(fmt.Sprintf("Created %s resource: %s",
			resource.GetObjectKind().GroupVersionKind().Kind,
			resource.GetName()))
	} else if result == controllerutil.OperationResultUpdated {
		logger.Info(fmt.Sprintf("Updated %s resource: %s",
			resource.GetObjectKind().GroupVersionKind().Kind,
			resource.GetName()))
	}
}
