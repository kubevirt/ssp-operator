package metrics

import (
	"strings"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	k8sv1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

var (
	rbdMetrics = []operatormetrics.Metric{
		vmRbdVolume,
	}

	vmRbdVolume = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_ssp_vm_rbd_block_volume_without_rxbounce",
			Help: "VM with RBD mounted Block volume (without rxbounce option set)",
			ExtraFields: map[string]string{
				"StabilityLevel": "ALPHA",
			},
		},
		[]string{"name", "namespace"},
	)
)

func SetVmWithVolume(vm *kubevirtv1.VirtualMachine, pvc *k8sv1.PersistentVolumeClaim, pv *k8sv1.PersistentVolume) {
	if pv == nil || pvc == nil {
		vmRbdVolume.WithLabelValues(vm.Name, vm.Namespace).Set(0)
		return
	}

	// If the volume is not using the RBD CSI driver, or if it is not using the block mode, it is not impacted by https://bugzilla.redhat.com/2109455
	if !usesRbdCsiDriver(pv) || *pvc.Spec.VolumeMode != k8sv1.PersistentVolumeBlock {
		vmRbdVolume.WithLabelValues(vm.Name, vm.Namespace).Set(0)
		return
	}

	mounter, ok := pv.Spec.CSI.VolumeAttributes["mounter"]
	// If mounter is not set, it is using the default mounter, which is "rbd"
	if ok && mounter != "rbd" {
		vmRbdVolume.WithLabelValues(vm.Name, vm.Namespace).Set(0)
		return
	}

	rxbounce, ok := pv.Spec.CSI.VolumeAttributes["mapOptions"]
	// If mapOptions is not set, or if it is set but does not contain "krbd:rxbounce", it might be impacted by https://bugzilla.redhat.com/2109455
	if !ok || !strings.Contains(rxbounce, "krbd:rxbounce") {
		vmRbdVolume.WithLabelValues(vm.Name, vm.Namespace).Set(1)
		return
	}

	vmRbdVolume.WithLabelValues(vm.Name, vm.Namespace).Set(0)
}

func usesRbdCsiDriver(pv *k8sv1.PersistentVolume) bool {
	return pv.Spec.CSI != nil && strings.Contains(pv.Spec.CSI.Driver, "rbd.csi.ceph.com")
}
