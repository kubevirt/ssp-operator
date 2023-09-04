package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"strings"
)

const (
	vmControllerName = "vm-controller"
	rhel6MetricName  = "kubevirt_vm_rhel6"
)

var (
	VmRhel6 = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: rhel6MetricName,
			Help: "Indication for a VirtualMachine that is based on rhel6 template",
		},
		[]string{
			"namespace", "name",
		},
	)

	VmRbdVolume = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubevirt_ssp_vm_rbd_volume",
			Help: "VM with RBD mounted volume",
		},
		[]string{
			"name", "namespace", "pv_name", "volume_mode", "rxbounce_enabled",
		},
	)
)

// Annotation to generate RBAC roles to read virtualmachines
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch

func CreateVmController(mgr ctrl.Manager) (*vmReconciler, error) {
	return newVmReconciler(mgr)
}

func (r *vmReconciler) Name() string {
	return vmControllerName
}

func (r *vmReconciler) Start(ctx context.Context, mgr ctrl.Manager) error {
	return r.setupController(mgr)
}

func (r *vmReconciler) setupController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("vm-controller").
		For(&kubevirtv1.VirtualMachine{}).
		Complete(r)
}

// vmReconciler watches the vms in the cluster
type vmReconciler struct {
	client client.Client
	log    logr.Logger
}

func newVmReconciler(mgr ctrl.Manager) (*vmReconciler, error) {
	logger := ctrl.Log.WithName("controllers").WithName("VirtualMachines")
	reconciler := &vmReconciler{
		client: mgr.GetClient(),
		log:    logger,
	}

	return reconciler, nil
}

func (r *vmReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	r.log.V(1).Info("Starting vm reconciliation...", "request", req.String())
	vm := kubevirtv1.VirtualMachine{}
	err = r.client.Get(ctx, req.NamespacedName, &vm)
	if err != nil {
		if errors.IsNotFound(err) {
			VmRhel6.WithLabelValues(req.Namespace, req.Name).Set(0)
			r.log.Info("VM not found", "vm", req.NamespacedName)
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		r.log.V(1).Info("Error retrieving the VM", "vm", req.NamespacedName)
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if hasRhel6TemplateLabel(&vm) {
		if vm.Status.PrintableStatus == kubevirtv1.VirtualMachineStatusRunning {
			VmRhel6.WithLabelValues(vm.GetNamespace(), vm.GetName()).Set(1)
		} else {
			VmRhel6.WithLabelValues(vm.GetNamespace(), vm.GetName()).Set(0)
		}
	}

	if err := r.setVmVolumesMetrics(ctx, &vm); err != nil {
		r.log.Error(err, "Error setting vm volumes metrics", "vm", req.NamespacedName)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

func hasRhel6TemplateLabel(vm client.Object) bool {
	if value, exists := vm.GetLabels()["vm.kubevirt.io/template"]; exists && strings.HasPrefix(value, "rhel6") {
		return true
	}

	return false
}

func (r *vmReconciler) setVmVolumesMetrics(ctx context.Context, vm *kubevirtv1.VirtualMachine) error {
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

		setVmWithVolume(vm, pvc, pv)
	}

	return result
}

func (r *vmReconciler) getPVC(ctx context.Context, vm *kubevirtv1.VirtualMachine, name string) (*corev1.PersistentVolumeClaim, error) {
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

func (r *vmReconciler) getPV(ctx context.Context, vm *kubevirtv1.VirtualMachine, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
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

func setVmWithVolume(vm *kubevirtv1.VirtualMachine, pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume) {
	if pv.Spec.PersistentVolumeSource.CSI == nil || !strings.Contains(pv.Spec.PersistentVolumeSource.CSI.Driver, "rbd.csi.ceph.com") {
		return
	}

	mounter, ok := pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["mounter"]
	if ok && mounter != "rbd" {
		return
	}

	rxbounce, ok := pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["mapOptions"]
	if !ok {
		rxbounce = ""
	}

	VmRbdVolume.WithLabelValues(
		vm.Name,
		vm.Namespace,
		pv.Name,
		string(*pvc.Spec.VolumeMode),
		strconv.FormatBool(strings.Contains(rxbounce, "krbd:rxbounce")),
	).Set(1)
}

var _ reconcile.Reconciler = &vmReconciler{}
