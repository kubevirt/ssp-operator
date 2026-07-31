package tlsinfo

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ocpconfigv1 "github.com/openshift/api/config/v1"

	"kubevirt.io/ssp-operator/internal/common"
)

const certSubject = "test.kubevirt.io"

var _ = Describe("TlsInfo", func() {

	var (
		certDir       string
		tlsOptionsDir string

		tlsOptions *common.SSPTLSOptions

		expectedVersion      uint16
		expectedCipherSuites []uint16
		expectedCurveIDs     []tls.CurveID

		tlsInfo *TLSInfo
	)

	BeforeEach(func() {
		certDir = GinkgoT().TempDir()
		tlsOptionsDir = GinkgoT().TempDir()

		writeCertificate(certDir)

		var err error
		tlsOptions, err = common.NewSSPTLSOptions(
			&ocpconfigv1.TLSSecurityProfile{
				Type:         ocpconfigv1.TLSProfileIntermediateType,
				Intermediate: &ocpconfigv1.IntermediateTLSProfile{},
			},
			nil,
		)
		Expect(err).ToNot(HaveOccurred())

		writeTLSOptions(tlsOptionsDir, tlsOptions)

		expectedVersion = tls.VersionTLS12
		expectedCipherSuites = []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		}
		expectedCurveIDs = []tls.CurveID{
			tls.X25519MLKEM768,
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		}

		tlsInfo = &TLSInfo{
			CertsDirectory:      certDir,
			TLSOptionsDirectory: tlsOptionsDir,
		}
	})

	Context("loading certificates", func() {
		It("should fail if no certificate exists", func() {
			Expect(os.Remove(filepath.Join(certDir, CertFilename))).To(Succeed())
			Expect(os.Remove(filepath.Join(certDir, KeyFilename))).To(Succeed())

			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Consistently(func() error {
				_, err := tlsInfo.CreateTlsConfig()
				return err
			}, time.Second).Should(MatchError(ContainSubstring("no such file or directory")))
		})

		It("should load certificate", func() {
			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.Certificates).To(HaveLen(1))
				g.Expect(tlsConfig.Certificates[0].Leaf.Subject.CommonName).To(Equal(certSubject))
			}, time.Second).Should(Succeed())
		})

		It("should reload new certificate", func() {
			Expect(os.Remove(filepath.Join(certDir, CertFilename))).To(Succeed())
			Expect(os.Remove(filepath.Join(certDir, KeyFilename))).To(Succeed())

			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Consistently(func() error {
				_, err := tlsInfo.CreateTlsConfig()
				return err
			}, time.Second).Should(MatchError(ContainSubstring("no such file or directory")))

			writeCertificate(certDir)

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.Certificates).To(HaveLen(1))
				g.Expect(tlsConfig.Certificates[0].Leaf.Subject.CommonName).To(Equal(certSubject))
			}, time.Second).Should(Succeed())
		})

		It("should fail if certificate file is invalid", func() {
			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.Certificates).To(HaveLen(1))
				g.Expect(tlsConfig.Certificates[0].Leaf.Subject.CommonName).To(Equal(certSubject))
			}, time.Second).Should(Succeed())

			Expect(os.WriteFile(filepath.Join(certDir, CertFilename), []byte{}, 0777)).To(Succeed())

			Eventually(func() error {
				_, err := tlsInfo.CreateTlsConfig()
				return err
			}, time.Second).Should(MatchError(ContainSubstring(
				"error getting certificate: failed to load certificate:",
			)))
		})
	})

	Context("loading TLS options", func() {
		It("should fail if TLS options file does not exist", func() {
			Expect(os.Remove(filepath.Join(tlsOptionsDir, TLSOptionsFilename))).To(Succeed())

			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Consistently(func() error {
				_, err := tlsInfo.CreateTlsConfig()
				return err
			}, time.Second).Should(MatchError(ContainSubstring("no such file or directory")))
		})

		It("should load TLS options", func() {
			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.CipherSuites).To(ContainElements(expectedCipherSuites))
				g.Expect(tlsConfig.MinVersion).To(Equal(expectedVersion))
				g.Expect(tlsConfig.CurvePreferences).To(Equal(expectedCurveIDs))
			}, time.Second).Should(Succeed())
		})

		It("should reload TLS options", func() {
			Expect(os.Remove(filepath.Join(tlsOptionsDir, TLSOptionsFilename))).To(Succeed())

			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Consistently(func() error {
				_, err := tlsInfo.CreateTlsConfig()
				return err
			}, time.Second).Should(MatchError(ContainSubstring("no such file or directory")))

			writeTLSOptions(tlsOptionsDir, tlsOptions)

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.CipherSuites).To(ContainElements(expectedCipherSuites))
				g.Expect(tlsConfig.MinVersion).To(Equal(expectedVersion))
				g.Expect(tlsConfig.CurvePreferences).To(Equal(expectedCurveIDs))
			}, time.Second).Should(Succeed())
		})

		It("should fail if TLS options are invalid", func() {
			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.CipherSuites).To(ContainElements(expectedCipherSuites))
				g.Expect(tlsConfig.MinVersion).To(Equal(expectedVersion))
				g.Expect(tlsConfig.CurvePreferences).To(Equal(expectedCurveIDs))
			}, time.Second).Should(Succeed())

			writeTLSOptions(tlsOptionsDir, &common.SSPTLSOptions{
				MinTLSVersion:      "INVALID_TLS_VERSION",
				OpenSSLCipherNames: []string{"invalid-cypher-name"},
			})

			Eventually(func() error {
				_, err := tlsInfo.CreateTlsConfig()
				return err
			}, time.Second).Should(MatchError(ContainSubstring(
				"error getting TLS options: TLS Configuration broken, min version misconfigured:",
			)))
		})

		It("should load TLS curve groups from a custom profile", func() {
			customCurveIDs := []tls.CurveID{tls.X25519, tls.CurveP256}
			writeTLSOptions(tlsOptionsDir, &common.SSPTLSOptions{
				MinTLSVersion:      "1.2",
				OpenSSLCipherNames: tlsOptions.OpenSSLCipherNames,
				TLSGroups:          []ocpconfigv1.TLSGroup{ocpconfigv1.TLSGroupX25519, ocpconfigv1.TLSGroupSecP256r1},
			})

			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.CurvePreferences).To(Equal(customCurveIDs))
			}, time.Second).Should(Succeed())
		})

		It("should ignore unsupported TLS group names", func() {
			writeTLSOptions(tlsOptionsDir, &common.SSPTLSOptions{
				MinTLSVersion:      "1.2",
				OpenSSLCipherNames: tlsOptions.OpenSSLCipherNames,
				TLSGroups:          []ocpconfigv1.TLSGroup{ocpconfigv1.TLSGroupX25519, "unsupported-group"},
			})

			Expect(tlsInfo.Init()).To(Succeed())
			defer tlsInfo.Clean()

			Eventually(func(g Gomega) {
				tlsConfig, err := tlsInfo.CreateTlsConfig()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.CurvePreferences).To(Equal([]tls.CurveID{tls.X25519}))
			}, time.Second).Should(Succeed())
		})
	})
})

func writeCertificate(dir string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		Fail(err.Error())
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          new(big.Int).SetInt64(0),
		Subject:               pkix.Name{CommonName: certSubject},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(24 * time.Hour).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		Fail(err.Error())
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	Expect(os.WriteFile(filepath.Join(dir, CertFilename), certPEM, 0777)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dir, KeyFilename), keyPEM, 0777)).To(Succeed())
}

func writeTLSOptions(dir string, tlsOptions *common.SSPTLSOptions) {
	tlsOptionsJson, err := json.Marshal(tlsOptions)
	Expect(err).ToNot(HaveOccurred())

	Expect(os.WriteFile(filepath.Join(dir, TLSOptionsFilename), tlsOptionsJson, 0600)).To(Succeed())
}

func TestTlsInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Certificate Reload Suite")
}
