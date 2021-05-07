package common_templates

import (
	"bytes"
	"io"
	"io/ioutil"

	templatev1 "github.com/openshift/api/template/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	ViewRoleName        = "os-images.kubevirt.io:view"
	EditClusterRoleName = "os-images.kubevirt.io:edit"
)

// ReadTemplates from the combined yaml file and return the list of its templates
func ReadTemplates(filename string) ([]templatev1.Template, error) {
	var bundle []templatev1.Template
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(file), 1024)
	for {
		template := templatev1.Template{}
		err = decoder.Decode(&template)
		if err == io.EOF {
			return bundle, nil
		}
		if err != nil {
			return nil, err
		}
		if template.Name != "" {
			bundle = append(bundle, template)
		}
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
				APIGroups: []string{CdiApiGroup},
				Resources: []string{"datavolumes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{CdiApiGroup},
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
				APIGroups: []string{CdiApiGroup},
				Resources: []string{"datavolumes"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{CdiApiGroup},
				Resources: []string{"datavolumes/source"},
				Verbs:     []string{"create"},
			},
		},
	}
}
