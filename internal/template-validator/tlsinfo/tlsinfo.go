package tlsinfo

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/template-validator/filewatch"
	"kubevirt.io/ssp-operator/internal/template-validator/logger"
)

const (
	CertFilename       = "tls.crt"
	KeyFilename        = "tls.key"
	TLSOptionsFilename = "tls-config.json"

	CiphersEnvName       = "TLS_CIPHERS"
	TLSMinVersionEnvName = "TLS_MIN_VERSION"
)

type TLSInfo struct {
	CertsDirectory      string
	TLSOptionsDirectory string

	lock sync.Mutex

	watch     filewatch.Watch
	stopWatch chan struct{}

	cert      *tls.Certificate
	certError error

	cipherSuites    []uint16
	minTLSVersion   uint16
	tlsOptionsError error
}

func (ti *TLSInfo) Init() error {
	if ti.stopWatch != nil {
		return fmt.Errorf("certificate watcher was already initialized")
	}

	ti.watch = filewatch.New()

	if err := ti.initCerts(); err != nil {
		return err
	}

	if err := ti.initCryptoConfig(); err != nil {
		return err
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

const watchErrorFmt = "error adding directory to watch: %w"

func (ti *TLSInfo) initCryptoConfig() error {
	directory := ti.TLSOptionsDirectory
	ti.updateTLSOptions(directory)

	err := ti.watch.Add(directory, func() {
		ti.updateTLSOptions(directory)
	})
	if err != nil {
		return fmt.Errorf(watchErrorFmt, err)
	}
	return nil
}

func (ti *TLSInfo) initCerts() error {
	directory := ti.CertsDirectory
	ti.updateCertificates(directory)

	err := ti.watch.Add(directory, func() {
		ti.updateCertificates(directory)
	})
	if err != nil {
		return fmt.Errorf(watchErrorFmt, err)
	}
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

	ti.lock.Lock()
	defer ti.lock.Unlock()
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

func (ti *TLSInfo) updateTLSOptions(directory string) {
	tlsOptions, err := loadTLSOptions(directory)

	ti.lock.Lock()
	defer ti.lock.Unlock()

	if err != nil {
		ti.minTLSVersion = 0
		ti.cipherSuites = nil
		ti.tlsOptionsError = err
		logger.Log.Error(err, "failed to load TLS options",
			"directory", directory)
		return
	}

	if tlsOptions.IsEmpty() {
		// Using default configuration
		ti.minTLSVersion = 0
		ti.cipherSuites = nil
		ti.tlsOptionsError = nil
		logger.Log.Info("TLS options are empty, using default configuration")
		return
	}

	minVersion, err := tlsOptions.MinTLSVersionId()
	if err != nil {
		ti.minTLSVersion = 0
		ti.cipherSuites = nil
		ti.tlsOptionsError = fmt.Errorf("TLS Configuration broken, min version misconfigured: %w", err)
		logger.Log.Error(ti.tlsOptionsError, "TLS options are not valid",
			"directory", directory)
		return
	}

	ti.minTLSVersion = minVersion
	ti.cipherSuites = common.CipherIDs(tlsOptions.OpenSSLCipherNames, nil)
	ti.tlsOptionsError = nil
	logger.Log.Info("TLS options retrieved", "directory", directory)
}

func loadTLSOptions(directory string) (*common.SSPTLSOptions, error) {
	optionsFilePath := filepath.Join(directory, TLSOptionsFilename)
	optionsJson, err := os.ReadFile(optionsFilePath)
	if err != nil {
		return nil, err
	}

	tlsOptions := &common.SSPTLSOptions{}
	if err = json.Unmarshal(optionsJson, tlsOptions); err != nil {
		return nil, err
	}

	return tlsOptions, nil
}

func (ti *TLSInfo) CreateTlsConfig() (*tls.Config, error) {
	ti.lock.Lock()
	defer ti.lock.Unlock()

	if ti.certError != nil {
		return nil, fmt.Errorf("error getting certificate: %w", ti.certError)
	}

	if ti.tlsOptionsError != nil {
		return nil, fmt.Errorf("error getting TLS options: %w", ti.tlsOptionsError)
	}

	if ti.cert == nil {
		return nil, fmt.Errorf("the TLS certificate is unexpectedly nil")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*ti.cert},
		MinVersion:   ti.minTLSVersion,
		CipherSuites: ti.cipherSuites,
	}

	return tlsConfig, nil
}
