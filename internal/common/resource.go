package common

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	libhandler "github.com/operator-framework/operator-lib/handler"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ssp "kubevirt.io/ssp-operator/api/v1alpha1"
)

type Resource interface {
	metav1.Object
	runtime.Object
}

type ResourceUpdateFunc = func(new Resource, found Resource) bool

func NoUpdate(_ Resource, _ Resource) bool {
	return false
}

func CreateOrUpdateResource(request *Request, resource Resource, found Resource, updateResource ResourceUpdateFunc) error {
	err := controllerutil.SetControllerReference(request.Instance, resource, request.Scheme)
	if err != nil {
		return err
	}
	return createOrUpdate(request, resource, found, updateResource)
}

func CreateOrUpdateClusterResource(request *Request, resource Resource, found Resource, updateResource ResourceUpdateFunc) error {
	addOwnerAnnotations(resource, request.Instance)
	return createOrUpdate(request, resource, found, updateResource)
}

func createOrUpdate(request *Request, resource Resource, found Resource, updateResource ResourceUpdateFunc) error {
	err := request.Client.Get(request.Context,
		types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()},
		found)
	if errors.IsNotFound(err) {
		gvk, _ := apiutil.GVKForObject(resource, request.Scheme)
		request.Logger.Info(fmt.Sprintf("Creating %s resource: %s",
			gvk.Kind,
			resource.GetName()))
		return request.Client.Create(request.Context, resource)
	}
	if err != nil {
		return err
	}

	resource.SetResourceVersion(found.GetResourceVersion())

	// The order of the || operator arguments is chosen
	// to avoid short-circuit evaluation
	resourceChanged := updateLabels(resource, found)
	resourceChanged = updateAnnotations(resource, found) || resourceChanged
	resourceChanged = updateResource(resource, found) || resourceChanged

	if resourceChanged {
		request.Logger.Info(fmt.Sprintf("Updating %s resource: %s",
			found.GetObjectKind().GroupVersionKind().Kind,
			found.GetName()))
		return request.Client.Update(request.Context, found)
	}

	return nil
}

func addOwnerAnnotations(resource Resource, ssp *ssp.SSP) {
	if resource.GetAnnotations() == nil {
		resource.SetAnnotations(map[string]string{})
	}
	annotations := resource.GetAnnotations()
	annotations[libhandler.TypeAnnotation] = "SSP.ssp.kubevirt.io"
	annotations[libhandler.NamespacedNameAnnotation] = ssp.Namespace + "/" + ssp.Name
}

func updateAnnotations(new Resource, found Resource) bool {
	if new.GetAnnotations() == nil || len(new.GetAnnotations()) == 0 {
		return false
	}
	if found.GetAnnotations() == nil {
		found.SetAnnotations(new.GetAnnotations())
		return true
	}
	return updateStringMap(new.GetAnnotations(), found.GetAnnotations())
}

func updateLabels(new Resource, found Resource) bool {
	if new.GetLabels() == nil || len(new.GetLabels()) == 0 {
		return false
	}
	if found.GetLabels() == nil {
		found.SetLabels(new.GetLabels())
		return true
	}
	return updateStringMap(new.GetLabels(), found.GetLabels())
}

func updateStringMap(new map[string]string, found map[string]string) bool {
	changed := false
	for label, labelVal := range new {
		foundVal, ok := found[label]
		if !ok || foundVal != labelVal {
			found[label] = labelVal
			changed = true
		}
	}
	return changed
}
