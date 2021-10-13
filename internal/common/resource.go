package common

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/prometheus/client_golang/prometheus"
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

type ReconcileBuilder interface {
	NamespacedResource(client.Object) ReconcileBuilder
	ClusterResource(client.Object) ReconcileBuilder
	WithAppLabels(name string, component AppComponent) ReconcileBuilder
	UpdateFunc(ResourceUpdateFunc) ReconcileBuilder
	StatusFunc(ResourceStatusFunc) ReconcileBuilder

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

func (r *reconcileBuilder) Reconcile() (ReconcileResult, error) {
	if r.addLabels {
		AddAppLabels(r.request.Instance, r.operandName, r.operandComponent, r.resource)
	}
	return createOrUpdate(
		r.request,
		r.resource,
		r.isClusterResource,
		r.updateFunc,
		r.statusFunc,
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
	}
}

func createOrUpdate(request *Request, resource client.Object, isClusterRes bool, updateResource ResourceUpdateFunc, statusFunc ResourceStatusFunc) (ReconcileResult, error) {
	err := setOwner(request, resource, isClusterRes)
	if err != nil {
		return ReconcileResult{}, err
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
		return ReconcileResult{}, err
	}

	request.VersionCache.Add(found)
	logOperation(res, found, request.Logger)

	status := statusFunc(found)
	return ReconcileResult{status, resource, res}, nil
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
