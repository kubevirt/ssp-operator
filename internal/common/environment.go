package common

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	osconfv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OperatorVersionKey          = "OPERATOR_VERSION"
	TemplateValidatorImageKey   = "VALIDATOR_IMAGE"
	TektonTasksImageKey         = "TEKTON_TASKS_IMAGE"
	TektonTasksDiskVirtImageKey = "TEKTON_TASKS_DISK_VIRT_IMAGE"
	VirtioImageKey              = "VIRTIO_IMG"
	VmConsoleProxyImageKey      = "VM_CONSOLE_PROXY_IMAGE"

	DefaultTektonTasksIMG         = "quay.io/kubevirt/tekton-tasks:" + TektonTasksVersion
	DeafultTektonTasksDiskVirtIMG = "quay.io/kubevirt/tekton-tasks-disk-virt:" + TektonTasksVersion
	DefaultVirtioIMG              = "quay.io/kubevirt/virtio-container-disk:v0.59.0"

	defaultOperatorVersion = "devel"
)

// GetSSHKeysStatusImage returns generate-ssh-keys task image url
func GetTektonTasksImage() string {
	return EnvOrDefault(TektonTasksImageKey, DefaultTektonTasksIMG)
}

// GetDiskVirtSysprepImage returns disk-virt-sysprep task image url
func GetTektonTasksDiskVirtImage() string {
	return EnvOrDefault(TektonTasksDiskVirtImageKey, DeafultTektonTasksDiskVirtIMG)
}

// GetVirtioImage returns virtio image url
func GetVirtioImage() string {
	return EnvOrDefault(VirtioImageKey, DefaultVirtioIMG)
}

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

func RunningOnOpenshift(ctx context.Context, cl client.Reader) (bool, error) {
	clusterVersion := &osconfv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(clusterVersion), clusterVersion); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
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

func GetOperatorNamespace(logger logr.Logger) (string, error) {
	nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", fmt.Errorf("in getOperatorNamespace failed in call to downward API: %w", err)
	}
	ns := strings.TrimSpace(string(nsBytes))
	logger.Info("Found namespace", "Namespace", ns)
	return ns, nil
}
