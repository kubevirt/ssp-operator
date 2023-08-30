package metrics

import (
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ioprometheusclient "github.com/prometheus/client_model/go"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const rbdDriver = "openshift-storage.rbd.csi.ceph.com"

var _ = Describe("rbd_metrics", func() {
	var vm *kubevirtv1.VirtualMachine
	var pvc *k8sv1.PersistentVolumeClaim
	var pv *k8sv1.PersistentVolume

	BeforeEach(func() {
		vm = &kubevirtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vm",
				Namespace: "test-namespace",
			},
		}

		vmRbdVolume.Reset()
	})

	var setupVolumes = func(driver, mounter, mapOptions string) {
		volumeMode := k8sv1.PersistentVolumeBlock
		pvc = &k8sv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test-namespace",
			},
			Spec: k8sv1.PersistentVolumeClaimSpec{
				VolumeName: "test-pv",
				VolumeMode: &volumeMode,
			},
		}

		pv = &k8sv1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pv",
			},
			Spec: k8sv1.PersistentVolumeSpec{
				PersistentVolumeSource: k8sv1.PersistentVolumeSource{
					CSI: &k8sv1.CSIPersistentVolumeSource{
						Driver: driver,
						VolumeAttributes: map[string]string{
							"clusterID":     "test-cluster",
							"imageFeatures": "layering,deep-flatten,exclusive-lock",
							"mapOptions":    mapOptions,
						},
					},
				},
			},
		}

		if mounter != "" {
			pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["mounter"] = mounter
		}
	}

	DescribeTable("SetVmWithVolume",
		func(driver, mounter, mapOptions string, metricExists bool) {
			setupVolumes(driver, mounter, mapOptions)
			SetVmWithVolume(vm, pvc, pv)

			dto := &ioprometheusclient.Metric{}
			err := vmRbdVolume.WithLabelValues(
				vm.Name,
				vm.Namespace,
				pv.Name,
				string(*pvc.Spec.VolumeMode),
				strconv.FormatBool(strings.Contains(mapOptions, "krbd:rxbounce")),
			).Write(dto)

			Expect(err).ToNot(HaveOccurred())

			if metricExists {
				Expect(dto.GetGauge().GetValue()).To(Equal(float64(1)))
			} else {
				Expect(dto.GetGauge().GetValue()).To(Equal(float64(0)))
			}
		},
		Entry("rbd driver and default mounter", rbdDriver, "", "krbd:rxbounce", true),
		Entry("rbd driver and rbd mounter", rbdDriver, "rbd", "krbd:rxbounce", true),
		Entry("non-rbd driver", "random", "rbd", "krbd:rxbounce", false),
		Entry("non-rbd mounter", rbdDriver, "random", "krbd:rxbounce", false),
		Entry("krbd:rxbounce not enabled", rbdDriver, "rbd", "random", true),
	)
})
