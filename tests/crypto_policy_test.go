package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"

	ocpv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	core "k8s.io/api/core/v1"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/tests/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Crypto Policy", func() {
	const tls10AllowedCipher = "ECDHE-ECDSA-AES128-SHA"
	var (
		old          = ocpv1.TLSSecurityProfile{Type: "Old", Old: &ocpv1.OldTLSProfile{}}
		intermediate = ocpv1.TLSSecurityProfile{Type: "Intermediate", Intermediate: &ocpv1.IntermediateTLSProfile{}}
		modern       = ocpv1.TLSSecurityProfile{Type: "Modern", Modern: &ocpv1.ModernTLSProfile{}}
		custom       = ocpv1.TLSSecurityProfile{Type: "Custom",
			Custom: &ocpv1.CustomTLSProfile{
				TLSProfileSpec: ocpv1.TLSProfileSpec{
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"},
					MinTLSVersion: ocpv1.VersionTLS13,
				},
			},
		}

		// Note that "crypto/tls" does not support setting max tls version to anything below 1.2
		oldPermutation tlsConfigTestPermutation = tlsConfigTestPermutation{
			openshiftTLSPolicy: &old,
			allowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS12,
					OpenSSLCipherNames: []string{},
				},
			},
			disallowedConfigs: []clientTLSOptions{},
		}

		intermediatePermutation tlsConfigTestPermutation = tlsConfigTestPermutation{
			openshiftTLSPolicy: &intermediate,
			allowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS12,
					OpenSSLCipherNames: []string{},
				},
			},
			disallowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS12,
					OpenSSLCipherNames: []string{tls10AllowedCipher},
				},
			},
		}

		modernPermutation tlsConfigTestPermutation = tlsConfigTestPermutation{
			openshiftTLSPolicy: &modern,
			allowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS13,
					OpenSSLCipherNames: []string{},
				},
			},
			disallowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS12,
					OpenSSLCipherNames: []string{},
				},
			},
		}

		customPermutation tlsConfigTestPermutation = tlsConfigTestPermutation{
			openshiftTLSPolicy: &custom,
			allowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS13,
					OpenSSLCipherNames: []string{},
				},
			},
			disallowedConfigs: []clientTLSOptions{
				{
					MaxTLSVersion:      tls.VersionTLS12,
					OpenSSLCipherNames: []string{},
				},
			},
		}
	)

	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
		waitUntilDeployed()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	Context("setting Crypto Policy", func() {
		DescribeTable("Adhere to defined TLSConfig", func(tlsConfigTestPermutation tlsConfigTestPermutation) {
			pod := operatorPod()
			validatorPod := templateValidatorPod()

			applyTLSConfig(tlsConfigTestPermutation.openshiftTLSPolicy)
			Expect(testMetricsEndpoint(pod, tlsConfigTestPermutation)).To(Succeed())
			Expect(testWebhookEndpoint(pod, tlsConfigTestPermutation)).To(Succeed())

			// Using larger timeout, because it usually takes more than a minute
			// for the ConfigMap to be propagated to the pod.
			Eventually(func() error {
				return testValidatorEndpoint(validatorPod, tlsConfigTestPermutation)
			}, env.Timeout(), time.Second).Should(Succeed())
		},
			Entry("[test_id:9360] old", oldPermutation),
			Entry("[test_id:9276] intermediate", intermediatePermutation),
			Entry("[test_id:9477] modern", modernPermutation),
			Entry("[test_id:9280] custom", customPermutation),
		)
	})
})

func operatorPod() core.Pod {
	pods := &core.PodList{}
	err := apiClient.List(context.TODO(), pods, client.MatchingLabels{"control-plane": "ssp-operator"})
	Expect(err).ToNot(HaveOccurred())
	Expect(pods.Items).ToNot(BeEmpty())
	Expect(pods.Items).To(HaveLen(1))
	return pods.Items[0]
}

func templateValidatorPod() core.Pod {
	pods := &core.PodList{}
	err := apiClient.List(context.TODO(), pods, client.MatchingLabels{
		common.AppKubernetesNameLabel:      "template-validator",
		common.AppKubernetesComponentLabel: string(common.AppComponentTemplating),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(pods.Items).ToNot(BeEmpty())
	return pods.Items[0]
}

type tlsConfigTestPermutation struct {
	openshiftTLSPolicy *ocpv1.TLSSecurityProfile
	allowedConfigs     []clientTLSOptions
	disallowedConfigs  []clientTLSOptions
}

type clientTLSOptions struct {
	OpenSSLCipherNames []string
	MaxTLSVersion      uint16
}

func (s *clientTLSOptions) CipherIDs() []uint16 {
	var cipherSuites []uint16
	for _, cipherName := range crypto.OpenSSLToIANACipherSuites(s.OpenSSLCipherNames) {
		id, ok := common.GetKnownCipherId(cipherName)
		if !ok {
			Fail("Provided unrecognizable ciphers in clientTLSOptions")
		}
		cipherSuites = append(cipherSuites, id)
	}
	return cipherSuites
}

func getCaCertificate() []byte {
	var service core.Service
	err := apiClient.Get(ctx, client.ObjectKey{Name: strategy.GetSSPWebhookServiceName(), Namespace: strategy.GetSSPDeploymentNameSpace()}, &service)
	Expect(err).ToNot(HaveOccurred())

	var ca []byte
	if service.Annotations["service.beta.openshift.io/serving-cert-secret-name"] != "" {
		var apiService apiregv1.APIService
		err = apiClient.Get(ctx, client.ObjectKey{Name: "v1.build.openshift.io"}, &apiService)
		Expect(err).ToNot(HaveOccurred())
		ca = apiService.Spec.CABundle
	} else {
		var secret core.Secret
		err := apiClient.Get(ctx, client.ObjectKey{Name: strategy.GetSSPWebhookServiceName() + "-cert", Namespace: strategy.GetSSPDeploymentNameSpace()}, &secret)
		Expect(err).ToNot(HaveOccurred())
		ca = secret.Data["olmCAKey"]
	}

	Expect(ca).ToNot(BeEmpty())
	return ca
}

func tryToAccessEndpoint(pod core.Pod, serviceName string, subpath string, port uint16, tlsConfig clientTLSOptions) (attemptedUrl string, err error) {
	conn, err := portForwarder.Connect(&pod, port)
	Expect(err).ToNot(HaveOccurred())
	defer conn.Close()

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(getCaCertificate())

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return conn, nil
			},
			TLSClientConfig: &tls.Config{CipherSuites: tlsConfig.CipherIDs(), MaxVersion: tlsConfig.MaxTLSVersion, RootCAs: certPool},
		},
	}

	if subpath != "" {
		subpath = "/" + subpath
	}

	// ${serviceName}.${serviceNamespace}.svc is used to access the endpoints we are testing.
	attemptedUrl = fmt.Sprintf("https://%s.%s.svc:%d%s", serviceName, pod.Namespace, port, subpath)
	_, err = httpClient.Get(attemptedUrl)
	return attemptedUrl, err
}

func (c tlsConfigTestPermutation) testTLSEndpointAccessible(pod core.Pod, serviceName string, subpath string, port uint16, tlsConfig clientTLSOptions) error {
	_, err := tryToAccessEndpoint(pod, serviceName, subpath, port, tlsConfig)
	if err != nil {
		return fmt.Errorf("can't access pod %s, at port %d, with tlsConfig %#v: %w", pod.Name, port, tlsConfig, err)
	}
	return nil
}

func (c tlsConfigTestPermutation) testTLSEndpointNotAccessible(pod core.Pod, serviceName string, subpath string, port uint16, tlsConfig clientTLSOptions) error {
	attemptedUrl, err := tryToAccessEndpoint(pod, serviceName, subpath, port, tlsConfig)
	expectedErrString1 := fmt.Sprintf("Get \"%s\": remote error: tls: protocol version not supported", attemptedUrl)
	expectedErrString2 := fmt.Sprintf("Get \"%s\": remote error: tls: handshake failure", attemptedUrl)

	if err == nil {
		return fmt.Errorf("should not have been able to access pod %s, at port %d, with tlsConfig %#v, %#v, %#v", pod.Name, port, tlsConfig, tlsConfig.MaxTLSVersion, tlsConfig.CipherIDs())
	}
	if err.Error() == expectedErrString1 || err.Error() == expectedErrString2 {
		return nil
	}
	return fmt.Errorf("unexpected error when accessing endpoint: %w", err)
}

func (c tlsConfigTestPermutation) testEndpointAccessabilityWithTLS(pod core.Pod, serviceName string, subpath string, port uint16) error {
	for _, config := range c.allowedConfigs {
		err := c.testTLSEndpointAccessible(pod, serviceName, subpath, port, config)
		if err != nil {
			return fmt.Errorf("failed accessing endpoint with allowed config: %w", err)
		}
	}
	for _, config := range c.disallowedConfigs {
		err := c.testTLSEndpointNotAccessible(pod, serviceName, subpath, port, config)
		if err != nil {
			return fmt.Errorf("error when accessing endpoint with disallowed config: %w", err)
		}
	}
	return nil
}

func testMetricsEndpoint(pod core.Pod, tlsConfig tlsConfigTestPermutation) error {
	// webhook service name is used here for the metrics for simplicity, as it is the CN in the ca_cert
	// and the metrics just sit on a different port on the same pod.
	return tlsConfig.testEndpointAccessabilityWithTLS(pod, strategy.GetSSPWebhookServiceName(), "metrics", 8443)
}

func testWebhookEndpoint(pod core.Pod, tlsConfig tlsConfigTestPermutation) error {
	return tlsConfig.testEndpointAccessabilityWithTLS(pod, strategy.GetSSPWebhookServiceName(), "", 9443)
}

func testValidatorEndpoint(pod core.Pod, tlsConfig tlsConfigTestPermutation) error {
	return tlsConfig.testEndpointAccessabilityWithTLS(pod, template_validator.ServiceName, "", 8443)
}

func applyTLSConfig(tlsSecurityProfile *ocpv1.TLSSecurityProfile) {
	watch, err := StartWatch(sspListerWatcher)
	Expect(err).ToNot(HaveOccurred())
	defer watch.Stop()

	updateSsp(func(foundSsp *ssp.SSP) {
		foundSsp.Spec.TLSSecurityProfile = tlsSecurityProfile
	})
	err = WatchChangesUntil(watch, isStatusDeploying, env.Timeout())
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")
	err = WatchChangesUntil(watch, isStatusDeployed, env.Timeout())
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")
}
