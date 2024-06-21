package env

import (
	"context"
	"errors"
	"fmt"
	"os"

	osconfv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal"
)

const (
	OperatorVersionKey        = "OPERATOR_VERSION"
	TemplateValidatorImageKey = "VALIDATOR_IMAGE"
	VmConsoleProxyImageKey    = "VM_CONSOLE_PROXY_IMAGE"

	podNamespaceKey = "POD_NAMESPACE"
)

func EnvOrDefault(envName string, defVal string) string {
	val := os.Getenv(envName)
	if val == "" {
		return defVal
	}
	return val
}

func GetOperatorVersion() string {
	return EnvOrDefault(OperatorVersionKey, internal.DefaultOperatorVersion)
}

func RunningOnOpenshift(ctx context.Context, cl client.Reader) (bool, error) {
	clusterVersion := &osconfv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(clusterVersion), clusterVersion); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) || errors.Is(err, &discovery.ErrGroupDiscoveryFailed{}) {
			// Not on OpenShift
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func GetInfrastructureTopology(ctx context.Context, c client.Reader) (osconfv1.TopologyMode, error) {
	infraConfig := &osconfv1.Infrastructure{}
	if err := c.Get(ctx, types.NamespacedName{Name: "cluster"}, infraConfig); err != nil {
		return "", err
	}

	return infraConfig.Status.InfrastructureTopology, nil
}

func GetOperatorNamespace() (string, error) {
	namespace, exists := os.LookupEnv(podNamespaceKey)
	if !exists {
		return "", fmt.Errorf("environment variable %s is not specified", podNamespaceKey)
	}
	return namespace, nil
}
