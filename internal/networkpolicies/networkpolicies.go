package networkpolicies

import (
	k8sv1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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

func (g *Generator) NewEgressToKubeAPIAndDNS(namespace, labelKey, labelValue string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		"ssp-operator-allow-egress-to-kube-api-and-dns-"+labelValue,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{labelKey: labelValue},
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
					},
				},
			},
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
