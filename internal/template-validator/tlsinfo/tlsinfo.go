package tlsinfo

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"kubevirt.io/client-go/log"
)

const (
	CertFilename  = "tls.crt"
	KeyFilename   = "tls.key"
	retryInterval = 1 * time.Minute
)

type TLSInfo struct {
	CertsDirectory string
	cert           *tls.Certificate
	certLock       sync.Mutex
	stopCertReload chan struct{}
}

func (ti *TLSInfo) IsEnabled() bool {
	return ti.CertsDirectory != ""
}

func (ti *TLSInfo) Init() {
	if !ti.IsEnabled() {
		return
	}

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
		log.Log.Reason(err).Critical("Failed to create an inotify watcher")
		return nil, nil, err
	}

	err = watcher.Add(directory)
	if err != nil {
		watcher.Close()
		log.Log.Reason(err).Criticalf("Failed to establish a watch on %s", directory)
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
				log.Log.Reason(err).Errorf("An error occurred when watching %s", directory)
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
		log.Log.Reason(err).Infof("failed to load the certificate in %s", directory)
		return err
	}

	ti.certLock.Lock()
	defer ti.certLock.Unlock()
	ti.cert = cert

	log.Log.Infof("certificate from %s with common name '%s' retrieved.", directory, cert.Leaf.Subject.CommonName)
	return nil
}

func loadCertificates(directory string) (serverCrt *tls.Certificate, err error) {
	certPath := filepath.Join(directory, CertFilename)
	keyPath := filepath.Join(directory, KeyFilename)

	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyBytes, err := ioutil.ReadFile(keyPath)
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

func (ti *TLSInfo) CrateTlsConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert := ti.getCertificate()
			if cert == nil {
				return nil, errors.New("no server certificate, server is not yet ready to receive traffic")
			}
			return cert, nil
		},
	}
}
