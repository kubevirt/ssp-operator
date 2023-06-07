package controllers

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/api/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	vmControllerName = "vm-controller"
	rhel6MetricName  = "kubevirt_vm_rhel6"
	kubevirtVMCRD    = "virtualmachines.kubevirt.io"
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
)

// Annotation to generate RBAC roles to read virtualmachines
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch

func CreateVmController(mgr ctrl.Manager) (*vmReconciler, error) {
	return newVmReconciler(mgr)
}

func getVMControllerRequiredCRD() string {
	return kubevirtVMCRD
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
		For(&kubevirtv1.VirtualMachine{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return hasRhel6TemplateLabel(object)
			}))).
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

	if vm.Status.PrintableStatus == kubevirtv1.VirtualMachineStatusRunning {
		VmRhel6.WithLabelValues(vm.GetNamespace(), vm.GetName()).Set(1)
	} else {
		VmRhel6.WithLabelValues(vm.GetNamespace(), vm.GetName()).Set(0)
	}

	return ctrl.Result{}, err
}

func hasRhel6TemplateLabel(vm client.Object) bool {
	if value, exists := vm.GetLabels()["vm.kubevirt.io/template"]; exists && strings.HasPrefix(value, "rhel6") {
		return true
	}

	return false
}

var _ reconcile.Reconciler = &vmReconciler{}
