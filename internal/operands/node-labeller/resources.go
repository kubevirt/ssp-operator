package node_labeller

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

var commonLabels = map[string]string{
	"app": "kubevirt-node-labeller",
}

var cpuPluginConfigmap = `obsoleteCPUs:
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

var configMapData = map[string]string{
	"cpu-plugin-configmap.yaml": cpuPluginConfigmap,
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
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: namespace,
		},
		Data: configMapData,
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

func initContainerKvmInfoNfdPlugin() *core.Container {
	// Build the KvmInfoNfdPlugin Init Container
	return &core.Container{
		Name:            "kvm-info-nfd-plugin",
		Image:           getNodeLabellerImages().kvmInfoNFD,
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{"cp /usr/bin/kvm-caps-info-nfd-plugin /etc/kubernetes/node-feature-discovery/source.d/;"},
		ImagePullPolicy: core.PullAlways,
		VolumeMounts: []core.VolumeMount{
			{
				Name:      nfdVolumeName,
				MountPath: nfdVolumeMountPath,
			},
		},
	}
}

func initContainerKubevirtCpuNfdPlugin() *core.Container {
	// Build the KubevirtCpuNfdPlugin Init Container
	args := []string{"cp /plugin/dest/cpu-nfd-plugin /etc/kubernetes/node-feature-discovery/source.d/;cp /config/cpu-plugin-configmap.yaml /etc/kubernetes/node-feature-discovery/source.d/cpu-plugin-configmap.yaml;"}
	return &core.Container{
		Name:            "kubevirt-cpu-nfd-plugin",
		Image:           getNodeLabellerImages().cpuNFD,
		Command:         []string{"/bin/sh", "-c"},
		Args:            args,
		ImagePullPolicy: core.PullAlways,
		VolumeMounts: []core.VolumeMount{
			{
				Name:      nfdVolumeName,
				MountPath: nfdVolumeMountPath,
			},
			{
				Name:      configMapVolumeName,
				MountPath: configMapVolumeMountPath,
			},
		},
	}
}

func initContainerLibvirt() *core.Container {
	// Build the Virt Launcher Init Container
	args := []string{"if [ ! -e /dev/kvm ] && [ $(grep '\\<kvm\\>' /proc/misc | wc -l) -eq 0 ]; then echo 'exiting due to missing kvm device'; exit 0; fi; if [ ! -e /dev/kvm ]; then mknod /dev/kvm c 10 $(grep '\\<kvm\\>' /proc/misc | cut -f 1 -d' '); fi; libvirtd -d; chmod o+rw /dev/kvm; virsh domcapabilities --machine q35 --arch x86_64 --virttype kvm > /etc/kubernetes/node-feature-discovery/source.d/virsh_domcapabilities.xml; cp -r /usr/share/libvirt/cpu_map /etc/kubernetes/node-feature-discovery/source.d/"}
	var boolVal = true
	return &core.Container{
		Name:            "libvirt",
		Image:           getNodeLabellerImages().virtLauncher,
		Command:         []string{"/bin/sh", "-c"},
		Args:            args,
		ImagePullPolicy: core.PullAlways,
		VolumeMounts: []core.VolumeMount{
			{
				Name:      nfdVolumeName,
				MountPath: nfdVolumeMountPath,
			},
		},
		SecurityContext: &core.SecurityContext{
			Privileged: &boolVal,
		},
	}
}

func initContainerKubevirtNodeLabeller() *core.Container {
	// Build the KubevirtNodeLabeller Init Container
	args := []string{"if [ ! -e /dev/kvm ] && [ $(grep '\\<kvm\\>' /proc/misc | wc -l) -eq 0 ]; then echo 'exiting due to missing kvm device'; exit 0; fi; if [ ! -e /dev/kvm ]; then mknod /dev/kvm c 10 $(grep '\\<kvm\\>' /proc/misc | cut -f 1 -d' '); fi; ./usr/sbin/node-labeller"}
	var boolVal = true
	return &core.Container{
		Name:    "kubevirt-node-labeller",
		Image:   getNodeLabellerImages().nodeLabeller,
		Command: []string{"/bin/sh", "-c"},
		Args:    args,
		Env: []core.EnvVar{
			{
				Name: "NODE_NAME",
				ValueFrom: &core.EnvVarSource{
					FieldRef: &core.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
		},
		VolumeMounts: []core.VolumeMount{
			{
				Name:      nfdVolumeName,
				MountPath: nfdVolumeMountPath,
			},
		},
		SecurityContext: &core.SecurityContext{
			Privileged: &boolVal,
		},
	}
}

func newDaemonSet(namespace string) *apps.DaemonSet {
	//Build the InitContainers
	initContainers := []core.Container{
		*initContainerKvmInfoNfdPlugin(),
		*initContainerKubevirtCpuNfdPlugin(),
		*initContainerLibvirt(),
		*initContainerKubevirtNodeLabeller(),
	}
	//Build the containers
	containers := []core.Container{
		*kubevirtNodeLabellerSleeperContainer(),
	}
	return &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DaemonSetName,
			Namespace: namespace,
			Labels:    commonLabels,
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: commonLabels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: commonLabels,
				},
				Spec: core.PodSpec{
					ServiceAccountName: ServiceAccountName,
					Containers:         containers,
					InitContainers:     initContainers,
					Volumes: []core.Volume{
						{
							Name: nfdVolumeName,
							VolumeSource: core.VolumeSource{
								EmptyDir: &core.EmptyDirVolumeSource{
									Medium: "",
								},
							},
						},
						{
							Name: configMapVolumeName,
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: ConfigMapName,
									},
								},
							},
						},
					},
				},
			},
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
