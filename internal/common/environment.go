package common

import (
	"context"
	"os"

	osconfv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func GetInfrastructureTopology(c client.Reader) (osconfv1.TopologyMode, error) {
	infraConfig := &osconfv1.Infrastructure{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, infraConfig); err != nil {
		return "", err
	}

	return infraConfig.Status.InfrastructureTopology, nil
}
