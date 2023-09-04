package tests

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	promApi "github.com/prometheus/client_golang/api"
	promApiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promConfig "github.com/prometheus/common/config"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/utils/pointer"
	kubevirtv1 "kubevirt.io/api/core/v1"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	"kubevirt.io/ssp-operator/tests/env"
)

var _ = Describe("Prometheus Alerts", func() {
	BeforeEach(func() {
		waitUntilDeployed()
	})

	Context("SSPCommonTemplatesModificationReverted", func() {
		var (
			testTemplate testResource
		)
		BeforeEach(func() {
			strategy.SkipIfUpgradeLane()
			testTemplate = createTestTemplate()
		})
		It("[test_id:8363] Should fire SSPCommonTemplatesModificationReverted", func() {
			// we have to wait for prometheus to pick up the series before we increase it.
			waitForSeriesToBeDetected(metrics.Total_restored_common_templates_increase_query)
			expectTemplateUpdateToIncreaseTotalRestoredTemplatesCount(testTemplate)
			waitForAlertToActivate("SSPCommonTemplatesModificationReverted")
		})
	})

	Context("SSPFailingToReconcile Alert", func() {
		var (
			deploymentRes testResource
			finalizerName = "ssp.kubernetes.io/temp-protection"
		)

		AfterEach(func() {
			removeFinalizer(deploymentRes, finalizerName)
			strategy.RevertToOriginalSspCr()
			waitUntilDeployed()
		})

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()
			deploymentRes = testDeploymentResource()
		})

		It("[test_id:8364] should set SSPOperatorReconcilingProperly metrics to 0 on failing to reconcile", func() {
			// add a finalizer to the validator deployment, do that it can't be deleted
			addFinalizer(deploymentRes, finalizerName)
			// send a request to delete the validator deployment
			deleteDeployment(deploymentRes)
			validateSspIsFailingToReconcileMetric()

			waitForAlertToActivate("SSPFailingToReconcile")
		})
	})

	Context("SSPTemplateValidatorDown Alert", func() {
		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		It("[test_id:8376] Should fire SSPTemplateValidatorDown", func() {
			strategy.SkipSspUpdateTestsIfNeeded()
			var replicas int32 = 0
			updateSsp(func(foundSsp *sspv1beta1.SSP) {
				foundSsp.Spec.TemplateValidator = &sspv1beta1.TemplateValidator{
					Replicas: &replicas,
				}
			})
			waitUntilDeployed()
			waitForAlertToActivate("SSPTemplateValidatorDown")
		})
	})

	Context("SSPHighRateRejectedVms Alert", func() {
		var (
			template *templatev1.Template
		)
		BeforeEach(func() {
			template = TemplateWithRules()
		})

		AfterEach(func() {
			Expect(apiClient.Delete(ctx, template)).ToNot(HaveOccurred(), "Failed to delete template: %s", template.Name)
		})

		It("[test_id:8377] Should fire SSPHighRateRejectedVms", func() {
			waitForSeriesToBeDetected(metrics.Total_rejected_vms_increase_query)
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)
			for range [6]int{} {
				time.Sleep(time.Second * 5)
				failVmCreationToIncreaseRejectedVmsMetrics(template)
			}
			waitForAlertToActivate("SSPHighRateRejectedVms")
		})
	})

	Context("SSPDown Alert", func() {
		var (
			deployment        *apps.Deployment
			replicas          int32
			origReplicas      int32
			sspDeploymentKeys = types.NamespacedName{}
		)

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()
			sspDeploymentKeys = types.NamespacedName{
				Name:      strategy.GetSSPDeploymentName(),
				Namespace: strategy.GetSSPDeploymentNameSpace(),
			}
			replicas = 0
			deployment = &apps.Deployment{}
			Expect(apiClient.Get(ctx, sspDeploymentKeys, deployment)).ToNot(HaveOccurred())
			origReplicas = *deployment.Spec.Replicas
			deployment.Spec.Replicas = &replicas
			Expect(apiClient.Update(ctx, deployment)).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Eventually(func() error {
				Expect(apiClient.Get(ctx, sspDeploymentKeys, deployment)).ToNot(HaveOccurred())
				deployment.Spec.Replicas = &origReplicas
				return apiClient.Update(ctx, deployment)
			}, env.ShortTimeout(), time.Second).ShouldNot(HaveOccurred())
			Eventually(func() int32 {
				Expect(apiClient.Get(ctx, sspDeploymentKeys, deployment)).ToNot(HaveOccurred())
				return deployment.Status.ReadyReplicas
			}, env.ShortTimeout(), time.Second).Should(Equal(origReplicas))
		})

		It("[test_id:8365] Should fire SSPDown", func() {
			waitForAlertToActivate("SSPDown")
		})
	})

	Context("DeprecatedRHEL6Vm Alert", func() {
		var (
			vm  *kubevirtv1.VirtualMachine
			vmi *kubevirtv1.VirtualMachineInstance
		)

		BeforeEach(func() {
			vmi = NewRandomVMIWithBridgeInterface(strategy.GetNamespace())
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Labels = map[string]string{
				"vm.kubevirt.io/template": "rhel6-desktop-large",
			}
			vm.Spec.Running = pointer.Bool(true)
			eventuallyCreateVm(vm)
		})

		AfterEach(func() {
			Expect(apiClient.Delete(ctx, vm)).ToNot(HaveOccurred(), "Failed to delete vm: %s", vm.Name)
		})

		It("Should fire the DeprecatedRHEL6Vm alert if there is a rhel6 running vm", func() {
			waitForAlertToActivate(metrics.Rhel6AlertName)
		})

		It("Should deactivate the DeprecatedRHEL6Vm alert if the rhel6 running vm is stopped", func() {
			waitForAlertToActivate(metrics.Rhel6AlertName)
			Eventually(func() error {
				foundVm := &kubevirtv1.VirtualMachine{}
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(vm), foundVm)
				Expect(err).ToNot(HaveOccurred())
				foundVm.Spec.Running = pointer.Bool(false)
				return apiClient.Update(ctx, foundVm)
			}, env.Timeout(), time.Second).Should(Succeed())

			alertShouldNotBeActive(metrics.Rhel6AlertName)
		})
	})

	Context("VirtualMachineCRCErrors", func() {
		var vm *kubevirtv1.VirtualMachine
		var pvc *core.PersistentVolumeClaim
		var pv *core.PersistentVolume

		BeforeEach(func() {
			strategy.SkipIfUpgradeLane()
			pvc = nil
			pv = nil
		})

		AfterEach(func() {
			vmError := apiClient.Delete(ctx, vm)
			Expect(vmError).ToNot(HaveOccurred())

			if pvc != nil {
				pvcError := apiClient.Delete(ctx, pvc)
				Expect(pvcError).ToNot(HaveOccurred())
			}

			if pv != nil {
				pvError := apiClient.Delete(ctx, pv)
				Expect(pvError).ToNot(HaveOccurred())
			}
		})

		var createPVCAndPV = func(vmName string, rxbounceEnabled bool) {
			mapOptions := "random"
			if rxbounceEnabled {
				mapOptions = "krbd:rxbounce"
			}

			pv = &core.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: vmName,
				},
				Spec: core.PersistentVolumeSpec{
					AccessModes: []core.PersistentVolumeAccessMode{
						core.ReadWriteOnce,
					},
					Capacity: core.ResourceList{
						core.ResourceStorage: resource.MustParse("1Gi"),
					},
					PersistentVolumeSource: core.PersistentVolumeSource{
						CSI: &core.CSIPersistentVolumeSource{
							Driver: "openshift-storage.rbd.csi.ceph.com",
							VolumeAttributes: map[string]string{
								"clusterID":     "test-cluster",
								"mounter":       "rbd",
								"imageFeatures": "layering,deep-flatten,exclusive-lock",
								"mapOptions":    mapOptions,
							},
							VolumeHandle: vmName,
						},
					},
				},
			}
			Expect(apiClient.Create(ctx, pv)).ToNot(HaveOccurred())

			volumeMode := core.PersistentVolumeBlock
			pvc = &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vmName,
					Namespace: strategy.GetNamespace(),
				},
				Spec: core.PersistentVolumeClaimSpec{
					VolumeName: vmName,
					VolumeMode: &volumeMode,
					AccessModes: []core.PersistentVolumeAccessMode{
						core.ReadWriteOnce,
					},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}
			Expect(apiClient.Create(ctx, pvc)).ToNot(HaveOccurred())
		}

		var createResources = func(createDataVolume bool, rxbounceEnabled bool) string {
			vmName := fmt.Sprintf("testvmi-%v", rand.String(10))

			var volumes []kubevirtv1.Volume

			if createDataVolume {
				createPVCAndPV(vmName, rxbounceEnabled)
				volumes = append(volumes, kubevirtv1.Volume{
					Name: vmName,
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{
							Name: vmName,
						},
					},
				})
			}

			vmi := NewMinimalVMIWithNS(strategy.GetNamespace(), vmName)
			vmi.Spec = kubevirtv1.VirtualMachineInstanceSpec{
				Volumes: volumes,
			}
			vm = NewVirtualMachine(vmi)
			vm.Spec.Running = pointer.Bool(false)
			eventuallyCreateVm(vm)

			return vmName
		}

		It("[test_id:TODO] Should not create kubevirt_ssp_vm_rbd_volume when not using DataVolume or PVC", func() {
			vmName := createResources(false, true)
			seriesShouldNotBeDetected(fmt.Sprintf("kubevirt_ssp_vm_rbd_volume{name='%s'}", vmName))
			alertShouldNotBeActive("VirtualMachineCRCErrors")
		})

		It("[test_id:TODO] Should not fire VirtualMachineCRCErrors when rxbounce is enabled", func() {
			vmName := createResources(true, true)
			waitForSeriesToBeDetected(fmt.Sprintf("kubevirt_ssp_vm_rbd_volume{name='%s', rxbounce_enabled='true'}", vmName))
			alertShouldNotBeActive("VirtualMachineCRCErrors")
		})

		It("[test_id:TODO] Should fire VirtualMachineCRCErrors is disabled", func() {
			vmName := createResources(true, false)
			waitForSeriesToBeDetected(fmt.Sprintf("kubevirt_ssp_vm_rbd_volume{name='%s', rxbounce_enabled='false'}", vmName))
			waitForAlertToActivate("VirtualMachineCRCErrors")
		})
	})
})

func checkAlert(alertName string) (*promApiv1.Alert, error) {
	alerts, err := getPrometheusClient().Alerts(context.TODO())
	if err != nil {
		return nil, err
	}
	alert := getAlertByName(alerts, alertName)
	return alert, nil
}

func waitForAlertToActivate(alertName string) {
	Eventually(func() error {
		alert, err := checkAlert(alertName)
		if err != nil {
			return err
		}
		if alert != nil {
			return nil
		}
		return fmt.Errorf("alert %s not found", alertName)
	}, env.Timeout(), time.Second).ShouldNot(HaveOccurred())
}

func alertShouldNotBeActive(alertName string) {
	Eventually(func() error {
		alert, err := checkAlert(alertName)
		if err != nil {
			return err
		}
		if alert == nil {
			return nil
		}
		return fmt.Errorf("alert %s found", alertName)
	}, env.Timeout(), time.Second).ShouldNot(HaveOccurred())

	Consistently(func() error {
		alert, err := checkAlert(alertName)
		if err != nil {
			return err
		}
		if alert == nil {
			return nil
		}
		return fmt.Errorf("alert %s found", alertName)
	}, env.ShortTimeout(), time.Second).ShouldNot(HaveOccurred())
}

func waitForSeriesToBeDetected(seriesName string) {
	Eventually(func() bool {
		results, _, err := getPrometheusClient().Query(context.TODO(), seriesName, time.Now())
		Expect(err).ShouldNot(HaveOccurred())
		return results.String() != ""
	}, env.Timeout(), 10*time.Second).Should(BeTrue())
}

func seriesShouldNotBeDetected(seriesName string) {
	Eventually(func() bool {
		results, _, err := getPrometheusClient().Query(context.TODO(), seriesName, time.Now())
		if err != nil {
			return false
		}
		return results.String() == ""
	}, env.Timeout(), 10*time.Second).Should(BeTrue())

	Consistently(func() bool {
		results, _, err := getPrometheusClient().Query(context.TODO(), seriesName, time.Now())
		if err != nil {
			return false
		}
		return results.String() == ""
	}, env.ShortTimeout(), 10*time.Second).Should(BeTrue())
}

func getAlertByName(alerts promApiv1.AlertsResult, alertName string) *promApiv1.Alert {
	for _, alert := range alerts.Alerts {
		if string(alert.Labels["alertname"]) == alertName {
			return &alert
		}
	}
	return nil
}

var (
	promClient       promApiv1.API
	failedPromClient bool
)

func getPrometheusClient() promApiv1.API {
	if failedPromClient {
		// Using short circuit here, because this function is called
		// in an Eventually() loop
		Fail("Could not create prometheus client")
	}

	if promClient == nil {
		failedPromClient = true
		promClient = initializePromClient(getPrometheusUrl(), getAuthorizationTokenForPrometheus())
		failedPromClient = false
	}
	return promClient
}

func initializePromClient(prometheusUrl string, token string) promApiv1.API {
	defaultRoundTripper := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	c, err := promApi.NewClient(promApi.Config{
		Address:      prometheusUrl,
		RoundTripper: promConfig.NewAuthorizationCredentialsRoundTripper("Bearer", promConfig.Secret(token), defaultRoundTripper),
	})
	Expect(err).ShouldNot(HaveOccurred())
	return promApiv1.NewAPI(c)
}

func getPrometheusUrl() string {
	var route routev1.Route
	routeKey := types.NamespacedName{Name: "prometheus-k8s", Namespace: metrics.MonitorNamespace}

	err := apiClient.Get(ctx, routeKey, &route)
	Expect(err).ShouldNot(HaveOccurred())
	return fmt.Sprintf("https://%s", route.Spec.Host)
}

func getAuthorizationTokenForPrometheus() string {
	var token string
	Eventually(func() error {
		secretList := &core.SecretList{}
		namespace := client.InNamespace(metrics.MonitorNamespace)
		err := apiClient.List(ctx, secretList, namespace)
		if err != nil {
			return fmt.Errorf("error getting secret: %w", err)
		}
		var tokenBytes []byte
		var ok bool
		for _, secret := range secretList.Items {
			if strings.HasPrefix(secret.Name, "prometheus-k8s-token") {
				tokenBytes, ok = secret.Data["token"]
				if !ok {
					return errors.New("token not found in secret data")
				}
				break
			}
		}

		token = string(tokenBytes)
		return nil
	}, 10*time.Second, time.Second).Should(Succeed())
	return token
}
