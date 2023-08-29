package metrics

import (
	"strconv"
	"strings"

	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	k8sv1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

var (
	rbdMetrics = []operatormetrics.Metric{
		vmRbdVolume,
	}

	vmRbdVolume = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: metricPrefix + "vm_rbd_volume",
			Help: "VM with RBD mounted volume",
			ExtraFields: map[string]string{
				"StabilityLevel": "ALPHA",
			},
		},
		[]string{"name", "namespace", "pv_name", "volume_mode", "rxbounce_enabled"},
	)
)

func SetVmWithVolume(vm *kubevirtv1.VirtualMachine, pvc *k8sv1.PersistentVolumeClaim, pv *k8sv1.PersistentVolume) {
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

	vmRbdVolume.WithLabelValues(
		vm.Name,
		vm.Namespace,
		pv.Name,
		string(*pvc.Spec.VolumeMode),
		strconv.FormatBool(strings.Contains(rxbounce, "krbd:rxbounce")),
	).Set(1)
}
