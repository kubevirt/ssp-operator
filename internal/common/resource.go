package common

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	libhandler "github.com/operator-framework/operator-lib/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		err := libhandler.SetOwnerAnnotations(request.Instance, resource)
		if err != nil {
			return err
		}

		// Removing ownerReferences for objects prior to creation
		resource.SetOwnerReferences(nil)

		// Removing ownerReferences for existing objects
		existingRes := newEmptyResource(resource)
		key, err := client.ObjectKeyFromObject(resource)
		if err != nil {
			return err
		}

		err = request.Client.Get(request.Context, key, existingRes)
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if err == nil {
			if len(existingRes.GetOwnerReferences()) > 0 {
				request.Logger.Info(fmt.Sprintf("Patching %s to remove ownerReferences", key))
				patch := client.RawPatch(types.JSONPatchType,
					[]byte(`[{"op": "replace", "path": "/metadata/ownerReferences", "value": []}, {"op": "remove", "path": "/metadata/ownerReferences"}]`))
				err = request.Client.Patch(request.Context, existingRes, patch)

				if err != nil {
					return err
				}
			}
		}

		return nil
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
