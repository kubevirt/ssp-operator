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
	OperatorVersionKey        = "OPERATOR_VERSION"
	TemplateValidatorImageKey = "VALIDATOR_IMAGE"
	CleanupVMImageKey         = "CLEANUP_VM_IMG"
	CopyTemplateImageKey      = "COPY_TEMPLATE_IMG"
	ModifyDataObjectImageKey  = "MODIFY_DATA_OBJECT_IMG"
	CreateVMImageKey          = "CREATE_VM_IMG"
	DiskVirtCustomizeImageKey = "DISK_VIRT_CUSTOMIZE_IMG"
	DiskVirtSysprepImageKey   = "DISK_VIRT_SYSPREP_IMG"
	ModifyVMTemplateImageKey  = "MODIFY_VM_TEMPLATE_IMG"
	WaitForVMISTatusImageKey  = "WAIT_FOR_VMI_STATUS_IMG"
	VirtioImageKey            = "VIRTIO_IMG"
	GenerateSSHKeysImageKey   = "GENERATE_SSH_KEYS_IMG"

	DefaultWaitForVMIStatusIMG  = "quay.io/kubevirt/tekton-task-wait-for-vmi-status:" + TektonTasksVersion
	DeafultModifyVMTemplateIMG  = "quay.io/kubevirt/tekton-task-modify-vm-template:" + TektonTasksVersion
	DeafultDiskVirtSysprepIMG   = "quay.io/kubevirt/tekton-task-disk-virt-sysprep:" + TektonTasksVersion
	DeafultDiskVirtCustomizeIMG = "quay.io/kubevirt/tekton-task-disk-virt-customize:" + TektonTasksVersion
	DeafultCreateVMIMG          = "quay.io/kubevirt/tekton-task-create-vm:" + TektonTasksVersion
	DeafultModifyDataObjectIMG  = "quay.io/kubevirt/tekton-task-modify-data-object:" + TektonTasksVersion
	DeafultCopyTemplateIMG      = "quay.io/kubevirt/tekton-task-copy-template:" + TektonTasksVersion
	DeafultCleanupVMIMG         = "quay.io/kubevirt/tekton-task-execute-in-vm:" + TektonTasksVersion
	GenerateSSHKeysIMG          = "quay.io/kubevirt/tekton-task-generate-ssh-keys:" + TektonTasksVersion
	DefaultVirtioIMG            = "quay.io/kubevirt/virtio-container-disk:v0.59.0"

	defaultOperatorVersion = "devel"
)

// GetSSHKeysStatusImage returns generate-ssh-keys task image url
func GetSSHKeysStatusImage() string {
	return EnvOrDefault(GenerateSSHKeysImageKey, GenerateSSHKeysIMG)
}

// GetWaitForVMIStatusImage returns wait-for-vmi-status task image url
func GetWaitForVMIStatusImage() string {
	return EnvOrDefault(WaitForVMISTatusImageKey, DefaultWaitForVMIStatusIMG)
}

// GetModifyVMTemplateImage returns modify-vm-template task image url
func GetModifyVMTemplateImage() string {
	return EnvOrDefault(ModifyVMTemplateImageKey, DeafultModifyVMTemplateIMG)
}

// GetDiskVirtSysprepImage returns disk-virt-sysprep task image url
func GetDiskVirtSysprepImage() string {
	return EnvOrDefault(DiskVirtSysprepImageKey, DeafultDiskVirtSysprepIMG)
}

// GetDiskVirtCustomizeImage returns disk-virt-customize task image url
func GetDiskVirtCustomizeImage() string {
	return EnvOrDefault(DiskVirtCustomizeImageKey, DeafultDiskVirtCustomizeIMG)
}

// GetCreateVMImage returns create-vm-from-manifest task image url
func GetCreateVMImage() string {
	return EnvOrDefault(CreateVMImageKey, DeafultCreateVMIMG)
}

// GetModifyDataObjectImage returns modify-data-object task image url
func GetModifyDataObjectImage() string {
	return EnvOrDefault(ModifyDataObjectImageKey, DeafultModifyDataObjectIMG)
}

// GetCopyTemplatemage returns copy-template task image url
func GetCopyTemplateImage() string {
	return EnvOrDefault(CopyTemplateImageKey, DeafultCopyTemplateIMG)
}

// GetCleanupVMImage returns cleanup-vm task image url
func GetCleanupVMImage() string {
	return EnvOrDefault(CleanupVMImageKey, DeafultCleanupVMIMG)
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
