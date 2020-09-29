package common_templates

import (
	"bytes"
	templatev1 "github.com/openshift/api/template/v1"
	"io"
	"io/ioutil"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	GoldenImagesNSname  = "kubevirt-os-images"
	bundleDir           = "data/common-templates-bundle/"
	ViewRoleName        = "os-images.kubevirt.io:view"
	EditClusterRoleName = "os-images.kubevirt.io:edit"
	Version             = "v0.12.2"
)

func readTemplates(filename string) ([]templatev1.Template, error) {
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
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims", "persistentvolumeclaims/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"cdi.kubevirt.io"},
				Resources: []string{"datavolumes/source"},
				Verbs:     []string{"create"},
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
				Kind:     "Group",
				Name:     "system:authenticated",
				APIGroup: "rbac.authorization.k8s.io",
			},
			{
				Kind:     "Group",
				Name:     "system:serviceaccounts",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: rbac.RoleRef{
			Kind:     "Role",
			Name:     ViewRoleName,
			APIGroup: "rbac.authorization.k8s.io",
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
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"cdi.kubevirt.io"},
				Resources: []string{"datavolumes"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"cdi.kubevirt.io"},
				Resources: []string{"datavolumes/source"},
				Verbs:     []string{"create"},
			},
		},
	}
}
