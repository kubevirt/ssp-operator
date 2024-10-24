package tlsinfo

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const certSubject = "test.kubevirt.io"

var _ = Describe("TlsInfo", func() {

	var certDir string

	BeforeEach(func() {
		certDir = GinkgoT().TempDir()
	})

	It("should fail if no certificate exists", func() {
		tlsInfo := TLSInfo{CertsDirectory: certDir}
		Expect(tlsInfo.Init()).To(Succeed())
		defer tlsInfo.Clean()

		Consistently(func() error {
			_, err := tlsInfo.CreateTlsConfig()
			return err
		}, time.Second).Should(MatchError(ContainSubstring("no such file or directory")))
	})

	It("should load certificate", func() {
		writeCertificate(certDir)
		tlsInfo := TLSInfo{CertsDirectory: certDir}
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
		tlsInfo := TLSInfo{CertsDirectory: certDir}
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
		writeCertificate(certDir)
		tlsInfo := TLSInfo{CertsDirectory: certDir}
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

func TestTlsInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Certificate Reload Suite")
}
