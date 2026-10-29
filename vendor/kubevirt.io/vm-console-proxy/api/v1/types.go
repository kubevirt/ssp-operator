package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TokenResponse is the response object from /token endpoint.
//
// +k8s:openapi-gen=true
type TokenResponse struct {
	Token               string      `json:"token"`
	ExpirationTimestamp metav1.Time `json:"expirationTimestamp"`
}

// TlsProfile is the TLS configuration for the proxy.
type TlsProfile struct {
	Ciphers       []string           `json:"ciphers,omitempty"`
	MinTLSVersion TLSProtocolVersion `json:"minTLSVersion,omitempty"`

	// Groups is a list of key exchange groups used for TLS connections.
	// The values are the IANA-assigned codes, matching the tls.CurveID
	// constants from the standard library "crypto/tls" package.
	Groups []uint16 `json:"groups,omitempty"`
}

// TLSProtocolVersion is a way to specify the protocol version used for TLS connections.
type TLSProtocolVersion string

const (
	// VersionTLS10 is version 1.0 of the TLS security protocol.
	VersionTLS10 TLSProtocolVersion = "VersionTLS10"
	// VersionTLS11 is version 1.1 of the TLS security protocol.
	VersionTLS11 TLSProtocolVersion = "VersionTLS11"
	// VersionTLS12 is version 1.2 of the TLS security protocol.
	VersionTLS12 TLSProtocolVersion = "VersionTLS12"
	// VersionTLS13 is version 1.3 of the TLS security protocol.
	VersionTLS13 TLSProtocolVersion = "VersionTLS13"
)
