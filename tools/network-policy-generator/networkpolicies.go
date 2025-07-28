package main

import (
	k8sv1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

type Generator struct {
	Namespace     string
	APINamespace  string
	APILabelKey   string
	APILabelValue string
	APIPort       int32
	DNSNamespace  string
	DNSLabelKey   string
	DNSLabelValue string
	DNSPort       int32
}

func (g *Generator) Generate() []*networkv1.NetworkPolicy {
	return []*networkv1.NetworkPolicy{
		g.newEgressToKubeAPI(),
		g.newEgressToKubeDNS(),
		g.newIngressToSSPOperatorWebhook(),
		g.newIngressToSSPOperatorMetrics(),
		g.newIngressToVirtTemplateValidatorWebhookAndMetrics(),
		g.newIngressToVMConsoleProxyAPI(),
	}
}

func newNetworkPolicy(namespace, name string, spec *networkv1.NetworkPolicySpec) *networkv1.NetworkPolicy {
	return &networkv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *spec,
	}
}

func (g *Generator) newEgressToKubeAPI() *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		g.Namespace,
		"ssp-operator-allow-egress-to-kube-api",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "name",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"ssp-operator",
							"virt-template-validator",
							"vm-console-proxy",
						},
					},
				},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": g.APINamespace},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{g.APILabelKey: g.APILabelValue},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(g.APIPort)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func (g *Generator) newEgressToKubeDNS() *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		g.Namespace,
		"ssp-operator-allow-egress-to-kube-dns",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "name",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"ssp-operator",
							"virt-template-validator",
							"vm-console-proxy",
						},
					},
				},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": g.DNSNamespace},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{g.DNSLabelKey: g.DNSLabelValue},
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(g.DNSPort)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
						{
							Port:     ptr.To(intstr.FromInt32(g.DNSPort)),
							Protocol: ptr.To(k8sv1.ProtocolUDP),
						},
					},
				},
			},
		},
	)
}

func (g *Generator) newIngressToSSPOperatorWebhook() *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		g.Namespace,
		"ssp-operator-allow-ingress-to-ssp-operator-webhook",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "ssp-operator"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
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

func (g *Generator) newIngressToSSPOperatorMetrics() *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		g.Namespace,
		"ssp-operator-allow-ingress-to-ssp-operator-metrics",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "ssp-operator"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
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

func (g *Generator) newIngressToVirtTemplateValidatorWebhookAndMetrics() *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		g.Namespace,
		"ssp-operator-allow-ingress-to-virt-template-validator-webhook-and-metrics",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "virt-template-validator"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
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

func (g *Generator) newIngressToVMConsoleProxyAPI() *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		g.Namespace,
		"ssp-operator-allow-ingress-to-vm-console-proxy-api",
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "vm-console-proxy"},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8768)),
							Protocol: ptr.To(k8sv1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}
