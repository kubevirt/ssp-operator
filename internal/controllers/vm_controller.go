package controllers

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
)

// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch

const vmControllerName = "vm-controller"

type vmController struct {
	log logr.Logger

	client client.Client
}

var _ Controller = &vmController{}

var _ reconcile.Reconciler = &vmController{}

func NewVmController() Controller {
	return &vmController{
		log: ctrl.Log.WithName("controllers").WithName("VirtualMachines"),
	}
}

func (v *vmController) Name() string {
	return vmControllerName
}

func (v *vmController) AddToManager(mgr ctrl.Manager, crdList crd_watch.CrdList) error {
	v.client = mgr.GetClient()

	if !crdList.CrdExists(getVmCrd()) {
		// If VM CRD doesn't exist, this controller does nothing
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(vmControllerName).
		For(&kubevirtv1.VirtualMachine{}).
		Complete(v)
}

func (v *vmController) GetWatchObjects() []WatchObject {
	return []WatchObject{{
		Object:  &kubevirtv1.VirtualMachine{},
		CrdName: getVmCrd(),
	}, {
		Object: &corev1.PersistentVolumeClaim{},
	}, {
		Object: &corev1.PersistentVolume{},
	}}
}

func (v *vmController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	vm := kubevirtv1.VirtualMachine{}
	if err := v.client.Get(ctx, req.NamespacedName, &vm); err != nil {
		if errors.IsNotFound(err) {
			// VM was deleted
			vm.Name = req.Name
			vm.Namespace = req.Namespace
			metrics.SetVmWithVolume(&vm, nil, nil)

			return ctrl.Result{}, nil
		}

		v.log.Error(err, "Could not find VM", "vm", req.NamespacedName)
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

	if err := v.setVmVolumesMetrics(ctx, &vm); err != nil {
		v.log.Error(err, "Could not set vm volumes metrics", "vm", req.NamespacedName)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, err
	}

	return ctrl.Result{}, nil
}

func getVmCrd() string {
	vmKind := strings.ToLower(kubevirtv1.VirtualMachineGroupVersionKind.Kind) + "s"
	return vmKind + "." + kubevirtv1.VirtualMachineGroupVersionKind.Group
}

func (v *vmController) setVmVolumesMetrics(ctx context.Context, vm *kubevirtv1.VirtualMachine) error {
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

		pvc, err := v.getPVC(ctx, vm, volumeName)
		if err != nil {
			return err
		}
		pv, err := v.getPV(ctx, vm, pvc)
		if err != nil {
			return err
		}

		metrics.SetVmWithVolume(vm, pvc, pv)
	}

	return result
}

func (v *vmController) getPVC(ctx context.Context, vm *kubevirtv1.VirtualMachine, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := v.client.Get(
		ctx,
		client.ObjectKey{
			Namespace: vm.Namespace,
			Name:      name,
		},
		pvc,
	)
	return pvc, err
}

func (v *vmController) getPV(ctx context.Context, vm *kubevirtv1.VirtualMachine, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	err := v.client.Get(
		ctx,
		client.ObjectKey{
			Namespace: vm.Namespace,
			Name:      pvc.Spec.VolumeName,
		},
		pv,
	)
	return pv, err
}
