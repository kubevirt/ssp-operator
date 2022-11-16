package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ocpv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	core "k8s.io/api/core/v1"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
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
		table.DescribeTable("Adhere to defined TLSConfig", func(tlsConfigTestPermutation tlsConfigTestPermutation) {
			pod := operatorPod()
			applyTLSConfig(tlsConfigTestPermutation.openshiftTLSPolicy)
			testMetricsEndpoint(pod, tlsConfigTestPermutation)
			testWebhookEndpoint(pod, tlsConfigTestPermutation)
		},
			table.Entry("[test_id:???] old", oldPermutation),
			table.Entry("[test_id:???] intermediate", intermediatePermutation),
			table.Entry("[test_id:???] modern", modernPermutation),
			table.Entry("[test_id:???] custom", customPermutation),
		)
	})
})

func operatorPod() core.Pod {
	pods := &core.PodList{}
	err := apiClient.List(context.TODO(), pods, client.MatchingLabels{"control-plane": "ssp-operator"})
	Expect(err).ToNot(HaveOccurred())
	Expect(pods.Items).ToNot(BeEmpty())
	Expect(len(pods.Items)).To(Equal(1))
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

func getPemCertificate() []byte {
	var apiService apiregv1.APIService
	err := apiClient.Get(ctx, client.ObjectKey{Name: "v1.build.openshift.io"}, &apiService)
	Expect(err).ToNot(HaveOccurred())
	pemCertificate := apiService.Spec.CABundle
	return pemCertificate
}

func tryToAccessEndpoint(pod core.Pod, subpath string, port uint16, tlsConfig clientTLSOptions) (attemptedUrl string, err error) {
	conn, err := portForwarder.Connect(&pod, port)
	Expect(err).ToNot(HaveOccurred())
	defer conn.Close()

	certPool := x509.NewCertPool()
	pemCert := getPemCertificate()
	certPool.AppendCertsFromPEM(pemCert)

	client := &http.Client{
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

	// ssp-webhook-service.kubevirt.svc is used to access the endpoints we are testing.
	// It is used here for the metrics as well for simplicity, as it is the CN in the ca_cert
	// and the metrics just sit on a different port on the same pod.
	attemptedUrl = fmt.Sprintf("https://ssp-webhook-service.kubevirt.svc:%d%s", port, subpath)
	_, err = client.Get(attemptedUrl)
	return attemptedUrl, err
}

func (c tlsConfigTestPermutation) testTLSEndpointAccessible(pod core.Pod, subpath string, port uint16, tlsConfig clientTLSOptions) {
	_, err := tryToAccessEndpoint(pod, subpath, port, tlsConfig)
	Expect(err).ToNot(HaveOccurred(), "Can't access pod %s, at port %d, with tlsConfig %#v", pod.Name, port, tlsConfig)
}

func (c tlsConfigTestPermutation) testTLSEndpointNotAccessible(pod core.Pod, subpath string, port uint16, tlsConfig clientTLSOptions) {
	attemptedUrl, err := tryToAccessEndpoint(pod, subpath, port, tlsConfig)
	expectedErrString1 := fmt.Sprintf("Get \"%s\": remote error: tls: protocol version not supported", attemptedUrl)
	expectedErrString2 := fmt.Sprintf("Get \"%s\": remote error: tls: handshake failure", attemptedUrl)
	Expect(err).To(SatisfyAny(MatchError(expectedErrString1), MatchError(expectedErrString2)), "Should not have been able to access pod %s, at port %s, with tlsConfig %#v, %#v, %#v", pod.Name, port, tlsConfig, tlsConfig.MaxTLSVersion, tlsConfig.CipherIDs())
}

func (c tlsConfigTestPermutation) testEndpointAccessabilityWithTLS(pod core.Pod, subpath string, port uint16) {
	for _, config := range c.allowedConfigs {
		c.testTLSEndpointAccessible(pod, subpath, port, config)
	}
	for _, config := range c.disallowedConfigs {
		c.testTLSEndpointNotAccessible(pod, subpath, port, config)
	}
}

func testMetricsEndpoint(pod core.Pod, tlsConfig tlsConfigTestPermutation) {
	tlsConfig.testEndpointAccessabilityWithTLS(pod, "metrics", 8443)
}

func testWebhookEndpoint(pod core.Pod, tlsConfig tlsConfigTestPermutation) {
	tlsConfig.testEndpointAccessabilityWithTLS(pod, "", 9443)
}

func applyTLSConfig(tlsSecurityProfile *ocpv1.TLSSecurityProfile) {
	watch, err := StartWatch(sspListerWatcher)
	Expect(err).ToNot(HaveOccurred())
	defer watch.Stop()

	updateSsp(func(foundSsp *sspv1beta1.SSP) {
		foundSsp.Spec.TLSSecurityProfile = tlsSecurityProfile
	})
	err = WatchChangesUntil(watch, isStatusDeploying, env.Timeout())
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")
	err = WatchChangesUntil(watch, isStatusDeployed, env.Timeout())
	Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")
}
