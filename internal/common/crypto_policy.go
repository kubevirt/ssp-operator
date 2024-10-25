package common

import (
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
	ocpv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
)

type SSPTLSOptions struct {
	MinTLSVersion      string   `json:"minTLSVersion,omitempty"`
	OpenSSLCipherNames []string `json:"openSSLCipherNames,omitempty"`
}

func (s *SSPTLSOptions) IsEmpty() bool {
	return len(s.OpenSSLCipherNames) == 0 && s.MinTLSVersion == ""
}

func (s *SSPTLSOptions) MinTLSVersionId() (uint16, error) {
	switch s.MinTLSVersion {
	case "":
		return tls.VersionTLS10, nil
	case "1.0":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("invalid TLSMinVersion %v: expects 1.0, 1.1, 1.2, 1.3 or empty", s.MinTLSVersion)
	}
}

func NewSSPTLSOptions(tlsSecurityProfile *ocpv1.TLSSecurityProfile, logger *logr.Logger) (*SSPTLSOptions, error) {
	ciphers, tlsProfile := selectCipherSuitesAndMinTLSVersion(tlsSecurityProfile)

	if logger != nil {
		logger.Info("Got Ciphers and tlsProfile:", "ciphers: ", ciphers, "tlsProfile: ", tlsProfile)
	}

	minVersion, err := tlsVersionToHumanReadable(tlsProfile)
	if err != nil {
		return nil, err
	}
	return &SSPTLSOptions{
		MinTLSVersion:      minVersion,
		OpenSSLCipherNames: ciphers,
	}, nil
}

func GetKnownCipherId(IANACipherName string) (uint16, bool) {
	for _, knownCipher := range tls.CipherSuites() {
		if knownCipher.Name == IANACipherName {
			return knownCipher.ID, true
		}
	}
	return 0, false
}

func CipherIDs(names []string, logger *logr.Logger) (cipherSuites []uint16) {
	for _, cipherName := range crypto.OpenSSLToIANACipherSuites(names) {
		if id, ok := GetKnownCipherId(cipherName); ok {
			cipherSuites = append(cipherSuites, id)
		} else {
			if logger != nil {
				logger.WithName("TLSSecurityProfile").Info("Unsupported cipher name: ", "Cipher Name", cipherName)
			}
		}
	}
	return
}

func selectCipherSuitesAndMinTLSVersion(profile *ocpv1.TLSSecurityProfile) ([]string, ocpv1.TLSProtocolVersion) {
	if profile == nil {
		return nil, ""
	}
	if profile.Custom != nil {
		return profile.Custom.TLSProfileSpec.Ciphers, profile.Custom.TLSProfileSpec.MinTLSVersion
	}
	tlsProfileSpec := ocpv1.TLSProfiles[profile.Type]
	return tlsProfileSpec.Ciphers, tlsProfileSpec.MinTLSVersion
}

func tlsVersionToHumanReadable(version ocpv1.TLSProtocolVersion) (string, error) {
	switch version {
	case "":
		return "", nil
	case ocpv1.VersionTLS10:
		return "1.0", nil
	case ocpv1.VersionTLS11:
		return "1.1", nil
	case ocpv1.VersionTLS12:
		return "1.2", nil
	case ocpv1.VersionTLS13:
		return "1.3", nil
	default:
		return "", fmt.Errorf("invalid ocpv1.VersionTLS %v", version)
	}
}
