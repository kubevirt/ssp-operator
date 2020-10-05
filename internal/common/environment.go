package common

import (
	"os"
)

const (
	templateValidatorImage    = "VALIDATOR_IMAGE"
	kubevirtNodeLabellerImage = "NODE_LABELLER_IMAGE"
	kvmInfoNfdPluginImage     = "KVM_INFO_IMAGE"
	kubevirtCpuNfdPluginImage = "CPU_PLUGIN_IMAGE"
	virtLauncherImage         = "VIRT_LAUNCHER_IMAGE"
)

type NodeLabellerImages struct {
	NodeLabeller string
	Sleeper      string
	KVMInfoNFD   string
	CPUNFD       string
	VirtLauncher string
}

func envOrDefault(envName string, defVal string) string {
	val := os.Getenv(envName)
	if val == "" {
		return defVal
	}
	return val
}

func GetTemplateValidatorImage() string {
	const defaultVersion = "v0.7.0"
	const defaultImage = "quay.io/kubevirt/kubevirt-template-validator"
	return envOrDefault(templateValidatorImage, defaultImage+":"+defaultVersion)
}

func GetNodeLabellerImages() NodeLabellerImages {

	const kubevirtNodeLabellerDefaultVersion = "v0.2.0"
	const kubevirtNodeLabellerDefaultImage = "quay.io/kubevirt/node-labeller"

	const kvmInfoNfdDefaultVersion = "v0.5.8"
	const kvmInfoNfdDefaultImage = "quay.io/kubevirt/kvm-info-nfd-plugin"

	const kvmCpuNfdDefaultVersion = "v0.1.1"
	const kvmCpuNfdDefaultImage = "quay.io/kubevirt/cpu-nfd-plugin"

	const libvirtDefaultVersion = "v0.21.0"
	const libvirtDefaultImage = "kubevirt/virt-launcher"

	return NodeLabellerImages{
		NodeLabeller: envOrDefault(kubevirtNodeLabellerImage, kubevirtNodeLabellerDefaultImage+":"+kubevirtNodeLabellerDefaultVersion),
		Sleeper:      envOrDefault(kubevirtNodeLabellerImage, kubevirtNodeLabellerDefaultImage+":"+kubevirtNodeLabellerDefaultVersion),
		KVMInfoNFD:   envOrDefault(kvmInfoNfdPluginImage, kvmInfoNfdDefaultImage+":"+kvmInfoNfdDefaultVersion),
		CPUNFD:       envOrDefault(kubevirtCpuNfdPluginImage, kvmCpuNfdDefaultImage+":"+kvmCpuNfdDefaultVersion),
		VirtLauncher: envOrDefault(virtLauncherImage, libvirtDefaultImage+":"+libvirtDefaultVersion),
	}
}
