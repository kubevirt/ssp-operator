package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func main() {
	if err := newCmd().Execute(); err != nil {
		// Ignoring returned error: no reasonable way to handle it.
		_, _ = fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}

func newCmd() *cobra.Command {
	g := &Generator{}
	cmd := &cobra.Command{
		Use:   "network-policy-generator",
		Short: "network policy generator for ssp operator",
		Long:  "network-policy-generator generates NetworkPolicies for ssp operator",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, v := range g.Generate() {
				if err := marshal(v, cmd.OutOrStdout()); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&g.Namespace, "namespace", "kubevirt", "Namespace in which ssp operator will be deployed (for OKD/OCP set to \"openshift-cnv\")")
	cmd.Flags().StringVar(&g.APINamespace, "api-namespace", "kube-system", "kube-apiserver namespace for the api network policy (for OKD/OCP set to \"openshift-kube-apiserver\")")
	cmd.Flags().StringVar(&g.APILabelKey, "api-pod-selector-label", "component", "kube-apiserver pod selector label key for the api network policy (for OKD/OCP set to \"app\")")
	cmd.Flags().StringVar(&g.APILabelValue, "api-pod-selector-value", "kube-apiserver", "kube-apiserver pod selector label value for the api network policy (for OKD/OCP set to \"openshift-kube-apiserver\")")
	cmd.Flags().Int32Var(&g.APIPort, "api-pod-port", 6443, "kube-apiserver pod port value for the api network policy")
	cmd.Flags().StringVar(&g.DNSNamespace, "dns-namespace", "kube-system", "DNS namespace for the DNS network policy (for OKD/OCP set to \"openshift-dns\")")
	cmd.Flags().StringVar(&g.DNSLabelKey, "dns-pod-selector-label", "k8s-app", "DNS pod selector label key for the DNS network policy (for OKD/OCP set to \"dns.operator.openshift.io/daemonset-dns\")")
	cmd.Flags().StringVar(&g.DNSLabelValue, "dns-pod-selector-value", "kube-dns", "DNS pod selector label value for the DNS network policy (for OKD/OCP set to \"default\")")
	cmd.Flags().Int32Var(&g.DNSPort, "dns-pod-port", 53, "DNS pod port value for the DNS network policy (for OKD/OCP set to \"5353\")")

	return cmd
}

func marshal(obj any, w io.Writer) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	var r unstructured.Unstructured
	if err := yaml.Unmarshal(data, &r.Object); err != nil {
		return err
	}

	// remove metadata.creationTimestamp
	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")

	data, err = yaml.Marshal(r.Object)
	if err != nil {
		return err
	}

	if _, err := w.Write([]byte("---\n")); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}

	return nil
}
