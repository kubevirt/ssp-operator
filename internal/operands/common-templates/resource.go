package common_templates

import (
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

const (
	ViewRoleName        = "os-images.kubevirt.io:view"
	EditClusterRoleName = "os-images.kubevirt.io:edit"
)

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
		},
	}
}
