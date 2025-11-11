package data_sources

import (
	core "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/ssp-operator/internal/networkpolicies"

	"kubevirt.io/ssp-operator/internal"
)

const (
	ViewRoleName        = "os-images.kubevirt.io:view"
	EditClusterRoleName = "os-images.kubevirt.io:edit"
)

func newDataSource(name string) *cdiv1beta1.DataSource {
	return &cdiv1beta1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: internal.GoldenImagesNamespace,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name,
					Namespace: internal.GoldenImagesNamespace,
				},
			},
		},
	}
}

func newDataSourceReference(name string, referenceName string) *cdiv1beta1.DataSource {
	return &cdiv1beta1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: internal.GoldenImagesNamespace,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				DataSource: &cdiv1beta1.DataSourceRefSourceDataSource{
					Name:      referenceName,
					Namespace: internal.GoldenImagesNamespace,
				},
			},
		},
	}
}

func newGoldenImagesNS(namespace string) *core.Namespace {
	return &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

func newViewRole(namespace string) *rbac.Role {
	return &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ViewRoleName,
			Namespace: namespace,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{core.GroupName},
				Resources: []string{"persistentvolumeclaims", "persistentvolumeclaims/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"datavolumes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"datavolumes/source"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"datasources"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"dataimportcrons"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{core.GroupName},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func newViewRoleBinding(namespace string) *rbac.RoleBinding {
	return &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ViewRoleName,
			Namespace: namespace,
		},
		Subjects: []rbac.Subject{
			{
				Kind:     rbac.GroupKind,
				Name:     "system:authenticated",
				APIGroup: rbac.GroupName,
			},
			{
				Kind:     rbac.GroupKind,
				Name:     "system:serviceaccounts",
				APIGroup: rbac.GroupName,
			},
		},
		RoleRef: rbac.RoleRef{
			Kind:     "Role",
			Name:     ViewRoleName,
			APIGroup: rbac.GroupName,
		},
	}
}

func newEditRole() *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: EditClusterRoleName,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{core.GroupName},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{core.GroupName},
				Resources: []string{"persistentvolumeclaims/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"datavolumes"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"datavolumes/source"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"datasources"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{cdiv1beta1.SchemeGroupVersion.Group},
				Resources: []string{"dataimportcrons"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	}
}

func newNetworkPolicies(namespace string, runningOnOpenShift bool) []*networkv1.NetworkPolicy {
	var g *networkpolicies.Generator
	if runningOnOpenShift {
		g = networkpolicies.NewOpenShiftGenerator()
	} else {
		g = networkpolicies.NewKubernetesGenerator()
	}

	return []*networkv1.NetworkPolicy{
		g.NewEgressToKubeAPIAndDNS(namespace, networkpolicies.LabelCDIKubevirtIo, "importer", "cdi-clone-source"),
		networkpolicies.NewIngressToImporterMetrics(namespace),
		networkpolicies.NewIngressFromCDIUploadServerToCDICloneSource(namespace),
		networkpolicies.NewIngressFromCDIUploadProxyToCDIUploadServer(namespace),
		networkpolicies.NewEgressFromCDICloneSourceToCDIUploadServer(namespace),
		networkpolicies.NewEgressFromImporterToDataSource(namespace),
	}
}
