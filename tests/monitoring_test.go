package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"gomodules.xyz/jsonpatch/v2"

	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	promApi "github.com/prometheus/client_golang/api"
	promApiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promConfig "github.com/prometheus/common/config"
	apps "k8s.io/api/apps/v1"
	authnv1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules/recordingrules"
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
			waitForSeriesToBeDetected(recordingrules.CommonTemplatesRestoredIncreaseQuery)
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

		It("[test_id:8364] should set SSPOperatorReconcileSucceeded metrics to 0 on failing to reconcile", func() {
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
			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
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
			waitForSeriesToBeDetected(recordingrules.TemplateValidatorRejectedIncreaseQuery)
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
			}, env.Timeout(), time.Second).Should(Equal(origReplicas))
		})

		It("[test_id:8365] Should fire SSPDown", func() {
			waitForAlertToActivate("SSPDown")
		})
	})

	Context("VMStorageClassWarning", func() {
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
			Expect(client.IgnoreNotFound(vmError)).ToNot(HaveOccurred())

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
					Resources: core.VolumeResourceRequirements{
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

			vmi := NewMinimalVMIWithNS(strategy.GetNamespace(), vmName)
			vm = NewVirtualMachine(vmi)
			vm.Spec.RunStrategy = ptr.To(kubevirtv1.RunStrategyAlways)
			vm.Spec.Template.ObjectMeta.Annotations = map[string]string{
				"vm.kubevirt.io/os": "windows-10",
			}

			eventuallyCreateVm(vm)
			waitForSeriesToBeDetected(fmt.Sprintf("kubevirt_vmi_info{name='%s', os='windows-10'} == 1", vmi.Name))

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

			operation := jsonpatch.NewOperation("add", "/spec/template/spec/volumes", volumes)
			patch := encodePatch([]jsonpatch.Operation{operation})
			err := apiClient.Patch(ctx, vm, patch)
			Expect(err).NotTo(HaveOccurred())

			return vmName
		}

		It("[test_id:TODO] Should not fire VMStorageClassWarning when rxbounce is enabled", func() {
			vmName := createResources(true, true)
			waitForSeriesToBeDetected(fmt.Sprintf("kubevirt_ssp_vm_rbd_block_volume_without_rxbounce{name='%s'} == 0", vmName))
			alertShouldNotBeActive("VMStorageClassWarning")
		})

		It("[test_id:10549] Should fire VMStorageClassWarning when rxbounce is disabled", func() {
			vmName := createResources(true, false)
			waitForSeriesToBeDetected(fmt.Sprintf("kubevirt_ssp_vm_rbd_block_volume_without_rxbounce{name='%s'} == 1", vmName))
			waitForAlertToActivate("VMStorageClassWarning")

			err := apiClient.Delete(ctx, vm)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) error {
				return apiClient.Get(ctx, types.NamespacedName{Name: vm.Name, Namespace: vm.Namespace}, vm)
			}, 1*time.Minute, 5*time.Second).Should(MatchError(k8serrors.IsNotFound, "IsNotFound"))

			waitForSeriesToBeDetected(fmt.Sprintf("kubevirt_ssp_vm_rbd_block_volume_without_rxbounce{name='%s'} == 0", vmName))
			alertShouldNotBeActive("VMStorageClassWarning")
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
	EventuallyWithOffset(1, func() error {
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
	EventuallyWithOffset(1, func() error {
		alert, err := checkAlert(alertName)
		if err != nil {
			return err
		}
		if alert == nil || alert.State == "inactive" {
			return nil
		}
		return fmt.Errorf("alert %s found", alertName)
	}, env.Timeout(), 10*time.Second).MustPassRepeatedly(10).ShouldNot(HaveOccurred())
}

func waitForSeriesToBeDetected(seriesName string) {
	EventuallyWithOffset(1, func() bool {
		results, _, err := getPrometheusClient().Query(context.TODO(), seriesName, time.Now())
		Expect(err).ShouldNot(HaveOccurred())
		return results.String() != ""
	}, env.Timeout(), 10*time.Second).Should(BeTrue())
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
		RoundTripper: promConfig.NewAuthorizationCredentialsRoundTripper("Bearer", promConfig.NewInlineSecret(token), defaultRoundTripper),
	})
	Expect(err).ShouldNot(HaveOccurred())
	return promApiv1.NewAPI(c)
}

func getPrometheusUrl() string {
	var route routev1.Route
	routeKey := types.NamespacedName{Name: "prometheus-k8s", Namespace: metrics.MonitorNamespace}

	err := apiClient.Get(ctx, routeKey, &route)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(route.Status.Ingress).ToNot(BeEmpty())
	return fmt.Sprintf("https://%s", route.Status.Ingress[0].Host)
}

func getAuthorizationTokenForPrometheus() string {
	const serviceAccountName = "prometheus-k8s"
	tokenReview, err := coreClient.CoreV1().ServiceAccounts(metrics.MonitorNamespace).
		CreateToken(ctx, serviceAccountName, &authnv1.TokenRequest{}, metav1.CreateOptions{})

	Expect(err).ToNot(HaveOccurred())

	return tokenReview.Status.Token
}
