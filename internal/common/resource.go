package common

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"

	libhandler "github.com/operator-framework/operator-lib/handler"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type StatusMessage = *string

type ResourceStatus struct {
	Resource     controllerutil.Object
	Progressing  StatusMessage
	NotAvailable StatusMessage
	Degraded     StatusMessage
}

type ReconcileFunc = func(*Request) (ResourceStatus, error)

func CollectResourceStatus(request *Request, funcs ...ReconcileFunc) ([]ResourceStatus, error) {
	res := make([]ResourceStatus, 0, len(funcs))
	for _, f := range funcs {
		status, err := f(request)
		if err != nil {
			return nil, err
		}
		res = append(res, status)
	}
	return res, nil
}

type ResourceUpdateFunc = func(expected, found controllerutil.Object)
type ResourceStatusFunc = func(resource controllerutil.Object) ResourceStatus

func CreateOrUpdateResource(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc) (ResourceStatus, error) {
	return createOrUpdate(request, resource, false, updateResource, statusOk)
}

func CreateOrUpdateResourceWithStatus(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc, statusFunc ResourceStatusFunc) (ResourceStatus, error) {
	return createOrUpdate(request, resource, false, updateResource, statusFunc)
}

func CreateOrUpdateClusterResource(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc) (ResourceStatus, error) {
	return createOrUpdate(request, resource, true, updateResource, statusOk)
}

func CreateOrUpdateClusterResourceWithStatus(request *Request, resource controllerutil.Object, updateResource ResourceUpdateFunc, statusFunc ResourceStatusFunc) (ResourceStatus, error) {
	return createOrUpdate(request, resource, true, updateResource, statusFunc)
}

func statusOk(_ controllerutil.Object) ResourceStatus {
	return ResourceStatus{}
}

func createOrUpdate(request *Request, resource controllerutil.Object, isClusterRes bool, updateResource ResourceUpdateFunc, statusFunc ResourceStatusFunc) (ResourceStatus, error) {
	err := setOwner(request, resource, isClusterRes)
	if err != nil {
		return ResourceStatus{}, err
	}

	found := newEmptyResource(resource)
	found.SetName(resource.GetName())
	found.SetNamespace(resource.GetNamespace())
	res, err := controllerutil.CreateOrUpdate(request.Context, request.Client, found, func() error {
		// We expect users will not add any other owner references,
		// if that is not correct, this code needs to be changed.
		found.SetOwnerReferences(resource.GetOwnerReferences())

		updateLabels(resource, found)
		updateAnnotations(resource, found)
		if !request.VersionCache.Contains(found) {
			// The generation was updated by other cluster components,
			// operator needs to update the resource
			updateResource(resource, found)
		}
		return nil
	})
	if err != nil {
		request.Logger.V(1).Info(fmt.Sprintf("Resource create/update failed: %v", err))
		return ResourceStatus{}, err
	}

	request.VersionCache.Add(found)
	logOperation(res, found, request.Logger)

	status := statusFunc(found)
	status.Resource = resource
	return status, nil
}

func setOwner(request *Request, resource controllerutil.Object, isClusterRes bool) error {
	if isClusterRes {
		resource.SetOwnerReferences(nil)
		return libhandler.SetOwnerAnnotations(request.Instance, resource)
	} else {
		delete(resource.GetAnnotations(), libhandler.NamespacedNameAnnotation)
		delete(resource.GetAnnotations(), libhandler.TypeAnnotation)
		return controllerutil.SetControllerReference(request.Instance, resource, request.Scheme)
	}
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
