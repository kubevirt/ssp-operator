package metrics

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

const (
	MonitorNamespace                    = "openshift-monitoring"
	defaultRunbookURLTemplate           = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv               = "RUNBOOK_URL_TEMPLATE"
	PrometheusLabelKey                  = "prometheus.ssp.kubevirt.io"
	PrometheusLabelValue                = "true"
	PrometheusClusterRoleName           = "prometheus-k8s-ssp"
	PrometheusServiceAccountName        = "prometheus-k8s"
	MetricsPortName                     = "http-metrics"
	CertFilename                        = "tls.crt"
	DefaultCertsDirectory               = "/tmp/k8s-webhook-server/serving-certs"
	TemplateValidatorMetricsServiceName = "template-validator-metrics"
	MetricsServiceName                  = "ssp-operator-metrics"
	MetricsServiceKey                   = "metrics.ssp.kubevirt.io"
)

// Variable to store OLM deployment info (set from main)
var isOLMDeployment bool

func SetOLMDeployment(isOLM bool) {
	isOLMDeployment = isOLM
}

func newMonitoringClusterRole() *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: PrometheusClusterRoleName,
		},
		Rules: []rbac.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
}

func newMonitoringClusterRoleBinding() *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: PrometheusClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      PrometheusServiceAccountName,
				Namespace: MonitorNamespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     PrometheusClusterRoleName,
			APIGroup: rbac.GroupName,
		},
	}
}

func ServiceMonitorLabels() map[string]string {
	return map[string]string{
		"openshift.io/cluster-monitoring": "true",
		PrometheusLabelKey:                PrometheusLabelValue,
		"k8s-app":                         "kubevirt",
	}
}

func extractHostnameFromCert(certPath string) (string, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate file: %w", err)
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return "", fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName, nil
	}

	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0], nil
	}

	return "", fmt.Errorf("no hostname found in certificate")
}

// getCAConfigForServiceMonitor returns the appropriate CA configuration
func getCAConfigForServiceMonitor() *promv1.SecretOrConfigMap {
	if isOLMDeployment {
		// OLM served certificate: use ssp-operator-service-cert secret with olmCAKey
		return &promv1.SecretOrConfigMap{
			Secret: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "ssp-operator-service-cert",
				},
				Key: "olmCAKey",
			},
		}
	}

	// Service-CA served certificate: use openshift-service-ca.crt configmap
	return &promv1.SecretOrConfigMap{
		ConfigMap: &v1.ConfigMapKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: "openshift-service-ca.crt",
			},
			Key: "service-ca.crt",
		},
	}
}

func newValidatorServiceMonitor(namespace string) *promv1.ServiceMonitor {
	tlsConfig := &promv1.TLSConfig{
		SafeTLSConfig: promv1.SafeTLSConfig{
			InsecureSkipVerify: ptr.To(false),
			CA: promv1.SecretOrConfigMap{
				ConfigMap: &v1.ConfigMapKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "openshift-service-ca.crt",
					},
					Key: "service-ca.crt",
				},
			},
		},
	}
	tlsConfig.ServerName = ptr.To("virt-template-validator.kubevirt.svc")

	serviceMonitor := newServiceMonitor(TemplateValidatorMetricsServiceName, namespace, tlsConfig, metav1.LabelSelector{
		MatchLabels: map[string]string{
			MetricsServiceKey: TemplateValidatorMetricsServiceName,
		},
	})
	return &serviceMonitor
}

func newSspServiceMonitor(namespace string) *promv1.ServiceMonitor {
	certPath := filepath.Join(DefaultCertsDirectory, CertFilename)
	hostname, _ := extractHostnameFromCert(certPath)

	tlsConfig := &promv1.TLSConfig{
		SafeTLSConfig: promv1.SafeTLSConfig{
			InsecureSkipVerify: ptr.To(false),
			// Use appropriate CA based on deployment type
			CA: *getCAConfigForServiceMonitor(),
		},
	}

	if hostname != "" {
		tlsConfig.ServerName = &hostname
	}

	serviceMonitor := newServiceMonitor(rules.RuleName, namespace, tlsConfig, metav1.LabelSelector{
		MatchLabels: map[string]string{
			MetricsServiceKey: MetricsServiceName,
		},
	})
	return &serviceMonitor
}

func newServiceMonitor(name,
	namespace string,
	tlsConfig *promv1.TLSConfig,
	selector metav1.LabelSelector) promv1.ServiceMonitor {
	return promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    ServiceMonitorLabels(),
		},
		Spec: promv1.ServiceMonitorSpec{
			NamespaceSelector: promv1.NamespaceSelector{
				Any: true,
			},
			Selector: selector,
			Endpoints: []promv1.Endpoint{
				{
					Port:        MetricsPortName,
					Scheme:      "https",
					TLSConfig:   tlsConfig,
					HonorLabels: true,
				},
			},
		},
	}
}
