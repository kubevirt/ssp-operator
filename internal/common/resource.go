package common

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type OperationResult string

const (
	OperationResultNone    OperationResult = "unchanged"
	OperationResultCreated OperationResult = "created"
	OperationResultUpdated OperationResult = "updated"
	OperationResultDeleted OperationResult = "deleted"
)

type StatusMessage = *string

type ResourceStatus struct {
	Progressing  StatusMessage
	NotAvailable StatusMessage
	Degraded     StatusMessage
}

type ReconcileResult struct {
	Status          ResourceStatus
	InitialResource client.Object
	Resource        client.Object
	OperationResult OperationResult
}

func (r *ReconcileResult) IsSuccess() bool {
	return r.Status.Progressing == nil &&
		r.Status.NotAvailable == nil &&
		r.Status.Degraded == nil
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

type ReconcileOptions struct {
	// AlwaysCallUpdateFunc specifies if the UpdateFunc should be called
	// on changes that don't increase the .metadata.generation field.
	// For example, labels and annotations.
	AlwaysCallUpdateFunc bool
}

type ReconcileBuilder interface {
	NamespacedResource(client.Object) ReconcileBuilder
	ClusterResource(client.Object) ReconcileBuilder
	WithAppLabels(name string, component AppComponent) ReconcileBuilder
	UpdateFunc(ResourceUpdateFunc) ReconcileBuilder
	StatusFunc(ResourceStatusFunc) ReconcileBuilder
	ImmutableSpec(getter ResourceSpecGetter) ReconcileBuilder

	Options(options ReconcileOptions) ReconcileBuilder

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

	options ReconcileOptions
}

var _ ReconcileBuilder = &reconcileBuilder{}

var (
	SSPOperatorReconcileSucceeded = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kubevirt_ssp_operator_reconcile_succeeded",
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

func (r *reconcileBuilder) Options(options ReconcileOptions) ReconcileBuilder {
	r.options = options
	return r
}

func (r *reconcileBuilder) Reconcile() (ReconcileResult, error) {
	if r.addLabels {
		AddAppLabels(r.request.Instance, r.operandName, r.operandComponent, r.resource)
	}

	err := setOwner(r.request, r.resource, r.isClusterResource)
	if err != nil {
		return ReconcileResult{}, err
	}

	found := newEmptyResource(r.resource)
	found.SetName(r.resource.GetName())
	found.SetNamespace(r.resource.GetNamespace())
	mutateFn := func() error {
		if !found.GetDeletionTimestamp().IsZero() {
			// Skip update, because the resource is being deleted
			return nil
		}

		// We expect users will not add any other owner references,
		// if that is not correct, this code needs to be changed.
		found.SetOwnerReferences(r.resource.GetOwnerReferences())

		UpdateLabels(r.resource, found)
		updateAnnotations(r.resource, found)
		if r.options.AlwaysCallUpdateFunc || !r.request.VersionCache.Contains(found) {
			// The generation was updated by other cluster components,
			// operator needs to update the resource
			r.updateFunc(r.resource, found)
		}
		return nil
	}

	res, existing, err := r.createOrUpdateWithImmutableSpec(found, mutateFn)
	if err != nil {
		r.request.Logger.Info(fmt.Sprintf("Resource create/update failed: %v", err))
		return ReconcileResult{}, err
	}
	if res == OperationResultDeleted || !found.GetDeletionTimestamp().IsZero() {
		r.request.VersionCache.RemoveObj(found)
		return ResourceDeletedResult(r.resource, res), nil
	}

	r.request.VersionCache.Add(found)
	logOperation(res, found, r.request.Logger)

	status := r.statusFunc(found)
	return ReconcileResult{status, existing, r.resource, res}, nil
}

func CreateOrUpdate(request *Request) ReconcileBuilder {
	if request == nil {
		panic("Request should not be nil")
	}

	return &reconcileBuilder{
		request:       request,
		updateFunc:    defaultUpdateFunc,
		statusFunc:    defaultStatusFunc,
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
	var results []CleanupResult
	for _, obj := range resources {
		result, err := Cleanup(request, obj)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// This function was initially copied from controllerutil.CreateOrUpdate
func (r *reconcileBuilder) createOrUpdateWithImmutableSpec(obj client.Object, f controllerutil.MutateFn) (OperationResult, client.Object, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := r.request.Client.Get(r.request.Context, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return OperationResultNone, nil, err
		}
		if err := mutate(f, key, obj); err != nil {
			return OperationResultNone, nil, err
		}
		if err := r.request.Client.Create(r.request.Context, obj); err != nil {
			return OperationResultNone, nil, err
		}
		return OperationResultCreated, nil, nil
	}

	existing := obj.DeepCopyObject().(client.Object)
	if err := mutate(f, key, obj); err != nil {
		return OperationResultNone, existing, err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return OperationResultNone, existing, nil
	}

	// If the resource is immutable and specs are not equal, delete it.
	// It will be recreated in the next iteration.
	if r.immutableSpec && !equality.Semantic.DeepEqual(r.specGetter(existing), r.specGetter(obj)) {
		if err := r.request.Client.Delete(r.request.Context, obj); err != nil {
			return OperationResultNone, existing, err
		}
		return OperationResultDeleted, existing, nil
	}

	if err := r.request.Client.Update(r.request.Context, obj); err != nil {
		return OperationResultNone, existing, err
	}
	return OperationResultUpdated, existing, nil
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

func UpdateLabels(expected, found client.Object) {
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

func logOperation(result OperationResult, resource client.Object, logger logr.Logger) {
	switch result {
	case OperationResultCreated:
		logger.Info(fmt.Sprintf("Created %s resource: %s",
			resource.GetObjectKind().GroupVersionKind().Kind,
			resource.GetName()))
	case OperationResultUpdated:
		logger.Info(fmt.Sprintf("Updated %s resource: %s",
			resource.GetObjectKind().GroupVersionKind().Kind,
			resource.GetName()))
	case OperationResultDeleted:
		logger.Info(fmt.Sprintf("Deleted %s resource: %s",
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

func ResourceDeletedResult(resource client.Object, res OperationResult) ReconcileResult {
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

func defaultUpdateFunc(newObj, foundObj client.Object) {
	switch newTyped := newObj.(type) {
	case *core.ConfigMap:
		foundConfigMap := foundObj.(*core.ConfigMap)
		foundConfigMap.Immutable = newTyped.Immutable
		foundConfigMap.Data = newTyped.Data
		foundConfigMap.BinaryData = newTyped.BinaryData

	case *core.Namespace:
		// Intentionally empty

	case *core.Service:
		foundService := foundObj.(*core.Service)
		// ClusterIP should not be updated
		newTyped.Spec.ClusterIP = foundService.Spec.ClusterIP
		foundService.Spec = newTyped.Spec

	case *core.ServiceAccount:
		// Intentionally empty

	case *rbac.ClusterRole:
		foundObj.(*rbac.ClusterRole).Rules = newTyped.Rules

	case *rbac.ClusterRoleBinding:
		foundBinding := foundObj.(*rbac.ClusterRoleBinding)
		foundBinding.RoleRef = newTyped.RoleRef
		foundBinding.Subjects = newTyped.Subjects

	case *rbac.Role:
		foundObj.(*rbac.Role).Rules = newTyped.Rules

	case *rbac.RoleBinding:
		foundBinding := foundObj.(*rbac.RoleBinding)
		foundBinding.RoleRef = newTyped.RoleRef
		foundBinding.Subjects = newTyped.Subjects

	case *apps.DaemonSet:
		foundObj.(*apps.DaemonSet).Spec = newTyped.Spec

	case *apps.Deployment:
		foundObj.(*apps.Deployment).Spec = newTyped.Spec

	case *instancetypev1alpha2.VirtualMachineClusterInstancetype:
		foundObj.(*instancetypev1alpha2.VirtualMachineClusterInstancetype).Spec = newTyped.Spec

	case *instancetypev1alpha2.VirtualMachineClusterPreference:
		foundObj.(*instancetypev1alpha2.VirtualMachineClusterPreference).Spec = newTyped.Spec

	case *instancetypev1beta1.VirtualMachineClusterInstancetype:
		foundObj.(*instancetypev1beta1.VirtualMachineClusterInstancetype).Spec = newTyped.Spec

	case *instancetypev1beta1.VirtualMachineClusterPreference:
		foundObj.(*instancetypev1beta1.VirtualMachineClusterPreference).Spec = newTyped.Spec

	case *routev1.Route:
		foundObj.(*routev1.Route).Spec = newTyped.Spec

	case *promv1.PrometheusRule:
		foundObj.(*promv1.PrometheusRule).Spec = newTyped.Spec

	case *promv1.ServiceMonitor:
		foundObj.(*promv1.ServiceMonitor).Spec = newTyped.Spec

	case *tekton.Task:
		foundObj.(*tekton.Task).Spec = newTyped.Spec

	default:
		panic(fmt.Sprintf("Default update is not supported for type: %T", newObj))
	}
}

func defaultStatusFunc(obj client.Object) ResourceStatus {
	status := ResourceStatus{}

	switch objTyped := obj.(type) {
	case *apps.DaemonSet:
		if objTyped.Status.NumberReady != objTyped.Status.DesiredNumberScheduled {
			msg := fmt.Sprintf("Not all pods for daemonset %s/%s are ready. (ready pods: %d, desired pods: %d)",
				objTyped.Namespace,
				objTyped.Name,
				objTyped.Status.NumberReady,
				objTyped.Status.DesiredNumberScheduled)
			status.NotAvailable = &msg
			status.Progressing = &msg
			status.Degraded = &msg
		}

	case *apps.Deployment:
		numberOfReplicas := *objTyped.Spec.Replicas
		if numberOfReplicas > 0 && objTyped.Status.AvailableReplicas == 0 {
			msg := fmt.Sprintf("No pods for deployment %s/%s are running. Expected: %d",
				objTyped.Namespace,
				objTyped.Name,
				objTyped.Status.Replicas)
			status.NotAvailable = &msg
		}
		if objTyped.Status.AvailableReplicas != numberOfReplicas {
			msg := fmt.Sprintf(
				"Not all pods for deployment %s/%s are running. Expected: %d, running: %d",
				objTyped.Namespace,
				objTyped.Name,
				numberOfReplicas,
				objTyped.Status.AvailableReplicas,
			)
			status.Progressing = &msg
			status.Degraded = &msg
		}

	default:
		// Do nothing
	}
	return status
}
