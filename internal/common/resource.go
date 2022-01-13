package common

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type StatusMessage = *string

type ResourceStatus struct {
	Progressing  StatusMessage
	NotAvailable StatusMessage
	Degraded     StatusMessage
}

type ReconcileResult struct {
	Status          ResourceStatus
	Resource        client.Object
	OperationResult controllerutil.OperationResult
}

type CleanupResult struct {
	Resource client.Object
	Deleted  bool
}

type ReconcileFunc = func(*Request) (ReconcileResult, error)

func CollectResourceStatus(request *Request, funcs ...ReconcileFunc) ([]ReconcileResult, error) {
	res := make([]ReconcileResult, 0, len(funcs))
	for _, f := range funcs {
		status, err := f(request)
		if err != nil {
			return nil, err
		}
		res = append(res, status)
	}
	return res, nil
}

type ResourceUpdateFunc = func(expected, found client.Object)
type ResourceStatusFunc = func(resource client.Object) ResourceStatus
type ResourceSpecGetter = func(resource client.Object) interface{}

type ReconcileBuilder interface {
	NamespacedResource(client.Object) ReconcileBuilder
	ClusterResource(client.Object) ReconcileBuilder
	WithAppLabels(name string, component AppComponent) ReconcileBuilder
	UpdateFunc(ResourceUpdateFunc) ReconcileBuilder
	StatusFunc(ResourceStatusFunc) ReconcileBuilder
	ImmutableSpec(getter ResourceSpecGetter) ReconcileBuilder

	Reconcile() (ReconcileResult, error)
}

type reconcileBuilder struct {
	request           *Request
	resource          client.Object
	isClusterResource bool

	addLabels        bool
	operandName      string
	operandComponent AppComponent

	updateFunc ResourceUpdateFunc
	statusFunc ResourceStatusFunc

	immutableSpec bool
	specGetter    ResourceSpecGetter
}

var _ ReconcileBuilder = &reconcileBuilder{}

var (
	SSPOperatorReconcilingProperly = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ssp_operator_reconciling_properly",
		Help: "Set to 1 if the reconcile process of all operands completes with no errors, and to 0 otherwise",
	})
)

func (r *reconcileBuilder) NamespacedResource(resource client.Object) ReconcileBuilder {
	r.resource = resource
	r.isClusterResource = false
	return r
}

func (r *reconcileBuilder) ClusterResource(resource client.Object) ReconcileBuilder {
	r.resource = resource
	r.isClusterResource = true
	return r
}

func (r *reconcileBuilder) UpdateFunc(updateFunc ResourceUpdateFunc) ReconcileBuilder {
	r.updateFunc = updateFunc
	return r
}

func (r *reconcileBuilder) StatusFunc(statusFunc ResourceStatusFunc) ReconcileBuilder {
	r.statusFunc = statusFunc
	return r
}

func (r *reconcileBuilder) WithAppLabels(name string, component AppComponent) ReconcileBuilder {
	r.addLabels = true
	r.operandName = name
	r.operandComponent = component
	return r
}

func (r *reconcileBuilder) ImmutableSpec(specGetter ResourceSpecGetter) ReconcileBuilder {
	r.immutableSpec = true
	r.specGetter = specGetter
	return r
}

func (r *reconcileBuilder) Reconcile() (ReconcileResult, error) {
	if r.addLabels {
		AddAppLabels(r.request.Instance, r.operandName, r.operandComponent, r.resource)
	}
	return createOrUpdate(
		r.request,
		r.resource,
		r.isClusterResource,
		r.immutableSpec,
		r.updateFunc,
		r.statusFunc,
		r.specGetter,
	)
}

func CreateOrUpdate(request *Request) ReconcileBuilder {
	if request == nil {
		panic("Request should not be nil")
	}

	return &reconcileBuilder{
		request: request,
		updateFunc: func(_, _ client.Object) {
			// Empty function
		},
		statusFunc: func(_ client.Object) ResourceStatus {
			return ResourceStatus{}
		},

		immutableSpec: false,
		specGetter: func(_ client.Object) interface{} {
			return nil
		},
	}
}

func Cleanup(request *Request, resource client.Object) (CleanupResult, error) {
	found := newEmptyResource(resource)
	err := request.Client.Get(request.Context, client.ObjectKeyFromObject(resource), found)
	if errors.IsNotFound(err) {
		return CleanupResult{
			Resource: resource,
			Deleted:  true,
		}, nil
	}
	if err != nil {
		return CleanupResult{}, err
	}

	isOwned, err := isResourceOwned(request, resource, found)
	if err != nil {
		return CleanupResult{}, err
	}

	if !isOwned {
		// The found resource is not own by this SSP resource.
		// The owned resource was deleted and a new one created with the same name
		return CleanupResult{
			Resource: resource,
			Deleted:  true,
		}, nil
	}

	if found.GetDeletionTimestamp().IsZero() {
		err = request.Client.Delete(request.Context, found)
		if errors.IsNotFound(err) {
			return CleanupResult{
				Resource: resource,
				Deleted:  true,
			}, nil
		}
		if err != nil {
			request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", resource.GetName(), err))
			return CleanupResult{}, err
		}
	}

	return CleanupResult{
		Resource: resource,
		Deleted:  false,
	}, nil
}

func DeleteAll(request *Request, resources ...client.Object) ([]CleanupResult, error) {
	results := []CleanupResult{}
	for _, obj := range resources {
		result, err := Cleanup(request, obj)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func createOrUpdate(request *Request, resource client.Object, isClusterRes bool, isImmutable bool, updateResource ResourceUpdateFunc, statusFunc ResourceStatusFunc, specGetter ResourceSpecGetter) (ReconcileResult, error) {
	err := setOwner(request, resource, isClusterRes)
	if err != nil {
		return ReconcileResult{}, err
	}

	found := newEmptyResource(resource)
	found.SetName(resource.GetName())
	found.SetNamespace(resource.GetNamespace())
	mutateFn := func() error {
		if !found.GetDeletionTimestamp().IsZero() {
			// Skip update, because the resource is being deleted
			return nil
		}

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
	}

	var res controllerutil.OperationResult
	if isImmutable {
		res, err = createOrUpdateWithImmutableSpec(request.Context, request.Client, found, mutateFn, specGetter)
	} else {
		res, err = controllerutil.CreateOrUpdate(request.Context, request.Client, found, mutateFn)
	}

	if err != nil {
		request.Logger.V(1).Info(fmt.Sprintf("Resource create/update failed: %v", err))
		return ReconcileResult{}, err
	}
	if !found.GetDeletionTimestamp().IsZero() {
		request.VersionCache.RemoveObj(found)
		return resourceDeletedResult(resource, res), nil
	}

	request.VersionCache.Add(found)
	logOperation(res, found, request.Logger)

	status := statusFunc(found)
	return ReconcileResult{status, resource, res}, nil
}

// This function is mostly copied from controllerutil.CreateOrUpdate
func createOrUpdateWithImmutableSpec(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, specGetter ResourceSpecGetter) (controllerutil.OperationResult, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(ctx, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if err := mutate(f, key, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	existing := obj.DeepCopyObject()
	if err := mutate(f, key, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return controllerutil.OperationResultNone, nil
	}

	if !equality.Semantic.DeepEqual(specGetter(existing.(client.Object)), specGetter(obj)) {
		// If the specs are not equal, delete the existing object.
		// It will be recreated in the next reconcile iteration
		if err := c.Delete(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		// It is ok to return OperationResultUpdated when the resource was deleted,
		// because it will be recreated in the next iteration.
		return controllerutil.OperationResultUpdated, nil
	}

	if err := c.Update(ctx, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

// This function is a copy of controllerutil.mutate
func mutate(f controllerutil.MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}

func setOwner(request *Request, resource client.Object, isClusterRes bool) error {
	if isClusterRes {
		resource.SetOwnerReferences(nil)
		return libhandler.SetOwnerAnnotations(request.Instance, resource)
	} else {
		delete(resource.GetAnnotations(), libhandler.NamespacedNameAnnotation)
		delete(resource.GetAnnotations(), libhandler.TypeAnnotation)
		return controllerutil.SetControllerReference(request.Instance, resource, request.Client.Scheme())
	}
}

func newEmptyResource(resource client.Object) client.Object {
	return reflect.New(reflect.TypeOf(resource).Elem()).Interface().(client.Object)
}

func updateAnnotations(expected, found client.Object) {
	if found.GetAnnotations() == nil {
		found.SetAnnotations(expected.GetAnnotations())
		return
	}
	updateStringMap(expected.GetAnnotations(), found.GetAnnotations())
}

func updateLabels(expected, found client.Object) {
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

func logOperation(result controllerutil.OperationResult, resource client.Object, logger logr.Logger) {
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

func isResourceOwned(request *Request, expectedObj, foundObj client.Object) (bool, error) {
	if expectedObj.GetNamespace() == request.Instance.GetNamespace() {
		expectedObj.SetOwnerReferences(nil)
		err := controllerutil.SetControllerReference(request.Instance, expectedObj, request.Client.Scheme())
		if err != nil {
			return false, err
		}
	}

	err := libhandler.SetOwnerAnnotations(request.Instance, expectedObj)
	if err != nil {
		return false, err
	}

	for _, reference := range foundObj.GetOwnerReferences() {
		if reflect.DeepEqual(reference, expectedObj.GetOwnerReferences()[0]) {
			return true, nil
		}
	}
	if foundObj.GetAnnotations() != nil {
		if foundObj.GetAnnotations()[libhandler.TypeAnnotation] == expectedObj.GetAnnotations()[libhandler.TypeAnnotation] &&
			foundObj.GetAnnotations()[libhandler.NamespacedNameAnnotation] == expectedObj.GetAnnotations()[libhandler.NamespacedNameAnnotation] {
			return true, nil
		}
	}

	return false, nil
}

func resourceDeletedResult(resource client.Object, res controllerutil.OperationResult) ReconcileResult {
	message := "Resource is being deleted."
	return ReconcileResult{
		Status: ResourceStatus{
			Progressing:  &message,
			NotAvailable: &message,
			Degraded:     &message,
		},
		Resource:        resource,
		OperationResult: res,
	}
}
