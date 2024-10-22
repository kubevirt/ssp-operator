package tlsinfo

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/template-validator/filewatch"
	"kubevirt.io/ssp-operator/internal/template-validator/logger"
)

const (
	CertFilename         = "tls.crt"
	KeyFilename          = "tls.key"
	CiphersEnvName       = "TLS_CIPHERS"
	TLSMinVersionEnvName = "TLS_MIN_VERSION"
)

type TLSInfo struct {
	CertsDirectory string

	watch     filewatch.Watch
	stopWatch chan struct{}

	cert          *tls.Certificate
	certError     error
	certLock      sync.Mutex
	sspTLSOptions common.SSPTLSOptions
}

func (ti *TLSInfo) Init() error {
	if err := ti.initCerts(); err != nil {
		return err
	}
	ti.initCryptoConfig()
	return nil
}

func (ti *TLSInfo) initCryptoConfig() {
	nonSplitCiphers, _ := os.LookupEnv(CiphersEnvName)
	ti.sspTLSOptions.OpenSSLCipherNames = strings.Split(nonSplitCiphers, ",")
	ti.sspTLSOptions.MinTLSVersion, _ = os.LookupEnv(TLSMinVersionEnvName)
}

func (ti *TLSInfo) initCerts() error {
	if ti.stopWatch != nil {
		return fmt.Errorf("certificate watcher was already initialized")
	}

	directory := ti.CertsDirectory
	ti.updateCertificates(directory)

	ti.watch = filewatch.New()
	err := ti.watch.Add(directory, func() {
		ti.updateCertificates(directory)
	})
	if err != nil {
		return fmt.Errorf("error adding directory to watch: %w", err)
	}

	ti.stopWatch = make(chan struct{})
	go func() {
		for {
			select {
			case <-ti.stopWatch:
				return
			default:
			}

			watchErr := ti.watch.Run(ti.stopWatch)
			if watchErr == nil {
				return
			}
			// Log error and restart watch
			logger.Log.Info("failed watching files", "error", watchErr)

			select {
			case <-ti.stopWatch:
				return
			case <-time.After(time.Second):
			}
		}
	}()

	return nil
}

func (ti *TLSInfo) Clean() {
	if ti.stopWatch != nil {
		close(ti.stopWatch)
		ti.stopWatch = nil
	}
}

func (ti *TLSInfo) updateCertificates(directory string) {
	cert, err := loadCertificates(directory)

	ti.certLock.Lock()
	defer ti.certLock.Unlock()
	if err != nil {
		ti.cert = nil
		ti.certError = err
		logger.Log.Info("failed to load certificates",
			"directory", directory,
			"error", err)
		return
	}

	ti.cert = cert
	ti.certError = nil
	logger.Log.Info("certificate retrieved", "directory", directory, "name", cert.Leaf.Subject.CommonName)
}

func loadCertificates(directory string) (serverCrt *tls.Certificate, err error) {
	certPath := filepath.Join(directory, CertFilename)
	keyPath := filepath.Join(directory, KeyFilename)

	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %v\n", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to load leaf certificate: %v\n", err)
	}
	cert.Leaf = leaf
	return &cert, nil
}

func (ti *TLSInfo) getCertificate() (*tls.Certificate, error) {
	ti.certLock.Lock()
	defer ti.certLock.Unlock()

	if ti.certError != nil {
		return nil, ti.certError
	}

	return ti.cert, nil
}

func (ti *TLSInfo) CreateTlsConfig() *tls.Config {
	tlsConfig := &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := ti.getCertificate()
			if err != nil {
				return nil, fmt.Errorf("error getting certificate: %w", err)
			}
			return cert, nil
		},
	}

	if !ti.sspTLSOptions.IsEmpty() {
		tlsConfig.CipherSuites = common.CipherIDs(ti.sspTLSOptions.OpenSSLCipherNames, nil)
		minVersion, err := ti.sspTLSOptions.MinTLSVersionId()
		if err != nil {
			panic(fmt.Sprintf("TLS Configuration broken, min version misconfigured %v", err))
		}
		tlsConfig.MinVersion = minVersion
	}

	return tlsConfig
}
