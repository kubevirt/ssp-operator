package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
)

// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch

const vmControllerName = "vm-controller"

type VmReconciler struct {
	client client.Client
	log    logr.Logger
}

func CreateVmController(mgr ctrl.Manager) (*VmReconciler, error) {
	return newVmReconciler(mgr)
}

func (r *VmReconciler) Name() string {
	return vmControllerName
}

func (r *VmReconciler) Start(_ context.Context, mgr ctrl.Manager) error {
	return r.setupController(mgr)
}

func newVmReconciler(mgr ctrl.Manager) (*VmReconciler, error) {
	logger := ctrl.Log.WithName("controllers").WithName("VirtualMachines")
	reconciler := &VmReconciler{
		client: mgr.GetClient(),
		log:    logger,
	}

	return reconciler, nil
}

func (r *VmReconciler) setupController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(vmControllerName).
		For(&kubevirtv1.VirtualMachine{}).
		Complete(r)
}

func (r *VmReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	vm := kubevirtv1.VirtualMachine{}
	if err := r.client.Get(ctx, req.NamespacedName, &vm); err != nil {
		if errors.IsNotFound(err) {
			// VM was deleted
			vm.Name = req.Name
			vm.Namespace = req.Namespace
			metrics.SetVmWithVolume(&vm, nil, nil)

			return ctrl.Result{}, nil
		}

		r.log.Error(err, "Could not find VM", "vm", req.NamespacedName)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, err
	}

	if vm.Status.PrintableStatus == kubevirtv1.VirtualMachineStatusProvisioning {
		// The status Provisioning is set when not all resources are created yet.
		// This way we will avoid reconciliation looping while waiting for the resources to be created.
		return ctrl.Result{}, nil
	}

	if err := r.setVmVolumesMetrics(ctx, &vm); err != nil {
		r.log.Error(err, "Could not set vm volumes metrics", "vm", req.NamespacedName)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, err
	}

	return ctrl.Result{}, nil
}

func (r *VmReconciler) setVmVolumesMetrics(ctx context.Context, vm *kubevirtv1.VirtualMachine) error {
	var result error

	for _, volume := range vm.Spec.Template.Spec.Volumes {
		volumeName := ""
		if volume.DataVolume != nil {
			volumeName = volume.DataVolume.Name
		} else if volume.PersistentVolumeClaim != nil {
			volumeName = volume.PersistentVolumeClaim.ClaimName
		} else {
			continue
		}

		pvc, err := r.getPVC(ctx, vm, volumeName)
		if err != nil {
			return err
		}
		pv, err := r.getPV(ctx, vm, pvc)
		if err != nil {
			return err
		}

		metrics.SetVmWithVolume(vm, pvc, pv)
	}

	return result
}

func (r *VmReconciler) getPVC(ctx context.Context, vm *kubevirtv1.VirtualMachine, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(
		ctx,
		client.ObjectKey{
			Namespace: vm.Namespace,
			Name:      name,
		},
		pvc,
	)
	return pvc, err
}

func (r *VmReconciler) getPV(ctx context.Context, vm *kubevirtv1.VirtualMachine, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	err := r.client.Get(
		ctx,
		client.ObjectKey{
			Namespace: vm.Namespace,
			Name:      pvc.Spec.VolumeName,
		},
		pv,
	)
	return pv, err
}

var _ reconcile.Reconciler = &VmReconciler{}
