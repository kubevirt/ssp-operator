package common

import "os"

const (
	templateValidatorImage = "VALIDATOR_IMAGE"
)

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
	return envOrDefault(templateValidatorImage, defaultImage + ":" +defaultVersion)
}
