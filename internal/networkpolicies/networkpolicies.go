package networkpolicies

import (
	"strings"

	k8sv1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	LabelVMConsoleProxyKubevirtIo = "vm-console-proxy.kubevirt.io"
	LabelCDIKubevirtIo            = "cdi.kubevirt.io"
)

type Generator struct {
	apiNamespace  string
	apiLabelKey   string
	apiLabelValue string
	apiPort       int32
	dnsNamespace  string
	dnsLabelKey   string
	dnsLabelValue string
	dnsPort       int32
}

func NewKubernetesGenerator() *Generator {
	return &Generator{
		apiNamespace:  "kube-system",
		apiLabelKey:   "component",
		apiLabelValue: "kube-apiserver",
		apiPort:       6443,
		dnsNamespace:  "kube-system",
		dnsLabelKey:   "k8s-app",
		dnsLabelValue: "kube-dns",
		dnsPort:       53,
	}
}

func NewOpenShiftGenerator() *Generator {
	return &Generator{
		apiNamespace:  "openshift-kube-apiserver",
		apiLabelKey:   "app",
		apiLabelValue: "openshift-kube-apiserver",
		apiPort:       6443,
		dnsNamespace:  "openshift-dns",
		dnsLabelKey:   "dns.operator.openshift.io/daemonset-dns",
		dnsLabelValue: "default",
		dnsPort:       5353,
	}
}

func (g *Generator) NewEgressToKubeAPIAndDNS(namespace, labelKey string, labelValues ...string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-egress-to-kube-api-and-dns-"+strings.Join(labelValues, "-"),
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      labelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   labelValues,
					},
				},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": g.apiNamespace},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{g.apiLabelKey: g.apiLabelValue},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(g.apiPort)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": g.dnsNamespace},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{g.dnsLabelKey: g.dnsLabelValue},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(g.dnsPort)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
						{
							Port:     ptr.To(intstr.FromInt32(g.dnsPort)),
							Protocol: ptr.To(k8sv1.ProtocolUDP),
						},
					},
				},
			},
		},
	)
}

func NewIngressToVirtTemplateValidatorWebhookAndMetrics(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-ingress-to-virt-template-validator-webhook-and-metrics",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"kubevirt.io": "virt-template-validator"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
						{
							Port:     ptr.To(intstr.FromInt32(9443)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func NewIngressToVMConsoleProxyAPI(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-ingress-to-vm-console-proxy-api",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{LabelVMConsoleProxyKubevirtIo: "vm-console-proxy"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromString("api")),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func NewIngressToImporterMetrics(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-ingress-to-importer-metrics",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					LabelCDIKubevirtIo:           "importer",
					"prometheus.cdi.kubevirt.io": "true",
				},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					From: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{LabelCDIKubevirtIo: "cdi-deployment"},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func NewIngressFromCDIUploadServerToCDICloneSource(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-ingress-from-cdi-upload-server-to-cdi-clone-source",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{LabelCDIKubevirtIo: "cdi-upload-server"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					From: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{LabelCDIKubevirtIo: "cdi-clone-source"},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func NewEgressFromCDICloneSourceToCDIUploadServer(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-egress-from-cdi-clone-source-to-cdi-upload-server",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{LabelCDIKubevirtIo: "cdi-clone-source"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{LabelCDIKubevirtIo: "cdi-upload-server"},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func NewEgressFromImporterToDataSource(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-egress-from-importer-to-datasource",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{LabelCDIKubevirtIo: "importer"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress:      []networkv1.NetworkPolicyEgressRule{{}},
		},
	)
}

func newNetworkPolicy(namespace, name string, spec *networkv1.NetworkPolicySpec) *networkv1.NetworkPolicy {
	return &networkv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *spec,
	}
}
