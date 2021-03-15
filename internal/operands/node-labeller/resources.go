package node_labeller

/*
*
* This package is deprecated! Do not add any new code here.
*
 */

import (
	secv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/ssp-operator/internal/common"
)

// Resources from node-labeller
const (
	kubevirtNodeLabeller = "kubevirt-node-labeller"

	ServiceAccountName       = kubevirtNodeLabeller
	DaemonSetName            = kubevirtNodeLabeller
	ConfigMapName            = "kubevirt-cpu-plugin-configmap"
	ClusterRoleName          = kubevirtNodeLabeller
	ClusterRoleBindingName   = kubevirtNodeLabeller
	nfdVolumeName            = "nfd-source"
	nfdVolumeMountPath       = "/etc/kubernetes/node-feature-discovery/source.d/"
	configMapVolumeName      = "cpu-config"
	configMapVolumeMountPath = "/config"
	SecurityContextName      = kubevirtNodeLabeller
)

type nodeLabellerImages struct {
	nodeLabeller string
	sleeper      string
	kvmInfoNFD   string
	cpuNFD       string
	virtLauncher string
}

func getNodeLabellerImages() nodeLabellerImages {
	return nodeLabellerImages{
		nodeLabeller: common.EnvOrDefault(common.KubevirtNodeLabellerImageKey, KubevirtNodeLabellerDefaultImage),
		sleeper:      common.EnvOrDefault(common.KubevirtNodeLabellerImageKey, KubevirtNodeLabellerDefaultImage),
		kvmInfoNFD:   common.EnvOrDefault(common.KvmInfoNfdPluginImageKey, KvmInfoNfdDefaultImage),
		cpuNFD:       common.EnvOrDefault(common.KubevirtCpuNfdPluginImageKey, KvmCpuNfdDefaultImage),
		virtLauncher: common.EnvOrDefault(common.VirtLauncherImageKey, LibvirtDefaultImage),
	}
}

func newClusterRole() *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterRoleName,
		},
		Rules: []rbac.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"nodes"},
			Verbs:     []string{"get", "update", "patch"},
		}},
	}
}

func newServiceAccount(namespace string) *core.ServiceAccount {
	return &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountName,
			Namespace: namespace,
		},
	}
}

func newClusterRoleBinding(namespace string) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterRoleBindingName,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     ClusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbac.Subject{{
			Kind:      "ServiceAccount",
			Name:      ServiceAccountName,
			Namespace: namespace,
		}},
	}
}

func newConfigMap(namespace string) *core.ConfigMap {
	const cpuPluginConfigmap = `obsoleteCPUs:
  - "486"
  - "pentium"
  - "pentium2"
  - "pentium3"
  - "pentiumpro"
  - "coreduo"
  - "n270"
  - "core2duo"
  - "Conroe"
  - "athlon"
  - "phenom"
minCPU: "Penryn"`

	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"cpu-plugin-configmap.yaml": cpuPluginConfigmap,
		},
	}
}

func kubevirtNodeLabellerSleeperContainer() *core.Container {
	// Build the kubevirtNodeLabellerSleeper Container
	return &core.Container{
		Name:    "kubevirt-node-labeller-sleeper",
		Image:   getNodeLabellerImages().sleeper,
		Command: []string{"sleep"},
		Args:    []string{"infinity"},
	}
}

func newDaemonSet(namespace string) *apps.DaemonSet {
	return &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DaemonSetName,
			Namespace: namespace,
		},
	}
}

func newSecurityContextConstraint(namespace string) *secv1.SecurityContextConstraints {
	var usersList []string
	usersList = append(usersList, "system:serviceaccount:"+namespace+":"+ServiceAccountName)
	return &secv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: SecurityContextName,
		},
		AllowPrivilegedContainer: true,
		RunAsUser: secv1.RunAsUserStrategyOptions{
			Type: "RunAsAny",
		},
		SELinuxContext: secv1.SELinuxContextStrategyOptions{
			Type: "RunAsAny",
		},
		Users: usersList,
	}
}
