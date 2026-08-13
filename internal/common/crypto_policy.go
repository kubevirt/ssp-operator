package common

import (
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
	ocpv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
)

type SSPTLSOptions struct {
	MinTLSVersion      string           `json:"minTLSVersion,omitempty"`
	OpenSSLCipherNames []string         `json:"openSSLCipherNames,omitempty"`
	TLSGroups          []ocpv1.TLSGroup `json:"tlsGroups,omitempty"`
}

func (s *SSPTLSOptions) IsEmpty() bool {
	return len(s.OpenSSLCipherNames) == 0 && s.MinTLSVersion == "" && len(s.TLSGroups) == 0
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
	tlsProfileSpec := selectTLSProfileSpec(tlsSecurityProfile)
	if tlsProfileSpec == nil {
		return &SSPTLSOptions{}, nil
	}

	if logger != nil {
		logger.Info("Got tlsProfileSpec:",
			"ciphers: ", tlsProfileSpec.Ciphers,
			"groups: ", tlsProfileSpec.Groups,
			"minVersion: ", tlsProfileSpec.MinTLSVersion)
	}

	minVersion, err := tlsVersionToHumanReadable(tlsProfileSpec.MinTLSVersion)
	if err != nil {
		return nil, err
	}
	return &SSPTLSOptions{
		MinTLSVersion:      minVersion,
		OpenSSLCipherNames: tlsProfileSpec.Ciphers,
		TLSGroups:          tlsProfileSpec.Groups,
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

func selectTLSProfileSpec(profile *ocpv1.TLSSecurityProfile) *ocpv1.TLSProfileSpec {
	if profile == nil {
		return nil
	}
	if profile.Custom != nil {
		return profile.Custom.TLSProfileSpec.DeepCopy()
	}

	return ocpv1.TLSProfiles[profile.Type].DeepCopy()
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

// TODO: This can be replaced when https://github.com/openshift/library-go/pull/2347 is merged.
var tlsGroupToCurveID = map[ocpv1.TLSGroup]tls.CurveID{
	ocpv1.TLSGroupX25519:             tls.X25519,
	ocpv1.TLSGroupSecP256r1:          tls.CurveP256,
	ocpv1.TLSGroupSecP384r1:          tls.CurveP384,
	ocpv1.TLSGroupSecP521r1:          tls.CurveP521,
	ocpv1.TLSGroupX25519MLKEM768:     tls.X25519MLKEM768,
	ocpv1.TLSGroupSecP256r1MLKEM768:  tls.SecP256r1MLKEM768,
	ocpv1.TLSGroupSecP384r1MLKEM1024: tls.SecP384r1MLKEM1024,
}

func CurveIDs(groups []ocpv1.TLSGroup, logger *logr.Logger) ([]tls.CurveID, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	var curveIDs []tls.CurveID
	var unsupported []ocpv1.TLSGroup
	for _, group := range groups {
		if curveID, ok := tlsGroupToCurveID[group]; ok {
			curveIDs = append(curveIDs, curveID)
		} else {
			if logger != nil {
				logger.WithName("TLSSecurityProfile").Info("Unsupported TLS group name: ", "TLS group name", group)
			}
			unsupported = append(unsupported, group)
		}
	}
	if len(curveIDs) == 0 {
		return nil, fmt.Errorf("all passed TLS groups are unsupported: %v", unsupported)
	}

	return curveIDs, nil
}
