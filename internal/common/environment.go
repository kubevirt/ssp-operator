package common

import (
	"os"
)

const (
	OperatorVersionKey = "OPERATOR_VERSION"

	TemplateValidatorImageKey    = "VALIDATOR_IMAGE"
	KubevirtNodeLabellerImageKey = "NODE_LABELLER_IMAGE"
	KvmInfoNfdPluginImageKey     = "KVM_INFO_IMAGE"
	KubevirtCpuNfdPluginImageKey = "CPU_PLUGIN_IMAGE"
	VirtLauncherImageKey         = "VIRT_LAUNCHER_IMAGE"
)

func EnvOrDefault(envName string, defVal string) string {
	val := os.Getenv(envName)
	if val == "" {
		return defVal
	}
	return val
}
