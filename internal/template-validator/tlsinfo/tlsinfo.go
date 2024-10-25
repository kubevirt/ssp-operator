package tlsinfo

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/template-validator/logger"
)

const (
	CertFilename         = "tls.crt"
	KeyFilename          = "tls.key"
	retryInterval        = 1 * time.Minute
	CiphersEnvName       = "TLS_CIPHERS"
	TLSMinVersionEnvName = "TLS_MIN_VERSION"
)

type TLSInfo struct {
	CertsDirectory string
	cert           *tls.Certificate
	certLock       sync.Mutex
	stopCertReload chan struct{}
	sspTLSOptions  common.SSPTLSOptions
}

func (ti *TLSInfo) Init() {
	ti.initCerts()
	ti.initCryptoConfig()
}

func (ti *TLSInfo) initCryptoConfig() {
	nonSplitCiphers, _ := os.LookupEnv(CiphersEnvName)
	ti.sspTLSOptions.OpenSSLCipherNames = strings.Split(nonSplitCiphers, ",")
	ti.sspTLSOptions.MinTLSVersion, _ = os.LookupEnv(TLSMinVersionEnvName)
}

func (ti *TLSInfo) initCerts() {
	directory := ti.CertsDirectory
	filesChanged, watcherCloser, err := watchDirectory(directory)
	if err != nil {
		panic(err)
	}

	ti.stopCertReload = make(chan struct{})
	notify(filesChanged)

	go func() {
		defer watcherCloser.Close()
		for {
			select {
			case <-filesChanged:
				err := ti.updateCertificates(directory)
				if err != nil {
					go func() {
						time.Sleep(retryInterval)
						notify(filesChanged)
					}()
				}
			case <-ti.stopCertReload:
				return
			}
		}
	}()
}

func (ti *TLSInfo) Clean() {
	if ti.stopCertReload != nil {
		close(ti.stopCertReload)
	}
}

func watchDirectory(directory string) (chan struct{}, io.Closer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Log.Error(err, "Failed to create an inotify watcher")
		return nil, nil, err
	}

	err = watcher.Add(directory)
	if err != nil {
		watcher.Close()
		logger.Log.Error(err, "Failed to establish a watch", "directory", directory)
		return nil, nil, err
	}

	filesChanged := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op != fsnotify.Chmod {
					notify(filesChanged)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Log.Error(err, "An error occurred while watching", "directory", directory)
			}
		}
	}()

	return filesChanged, watcher, nil
}

func notify(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

func (ti *TLSInfo) updateCertificates(directory string) error {
	cert, err := loadCertificates(directory)
	if err != nil {
		logger.Log.Info("failed to load certificates",
			"directory", directory,
			"error", err)
		return err
	}

	ti.certLock.Lock()
	defer ti.certLock.Unlock()
	ti.cert = cert

	logger.Log.Info("certificate retrieved", "directory", directory, "name", cert.Leaf.Subject.CommonName)
	return nil
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

func (ti *TLSInfo) getCertificate() *tls.Certificate {
	ti.certLock.Lock()
	defer ti.certLock.Unlock()
	return ti.cert
}

func (ti *TLSInfo) CreateTlsConfig() *tls.Config {
	tlsConfig := &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert := ti.getCertificate()
			if cert == nil {
				return nil, errors.New("no server certificate, server is not yet ready to receive traffic")
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
