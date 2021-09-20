package common

import (
	"os"
)

const (
	OperatorVersionKey = "OPERATOR_VERSION"

	TemplateValidatorImageKey = "VALIDATOR_IMAGE"

	defaultOperatorVersion = "devel"
)

func EnvOrDefault(envName string, defVal string) string {
	val := os.Getenv(envName)
	if val == "" {
		return defVal
	}
	return val
}

func GetOperatorVersion() string {
	return EnvOrDefault(OperatorVersionKey, defaultOperatorVersion)
}
