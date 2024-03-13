package vm_console_proxy_bundle

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	vmConsoleProxyBundleDir = "data/vm-console-proxy-bundle/"
)

const (
	clusterRoleKind         = "ClusterRole"
	clusterRoleBindingsKind = "ClusterRoleBinding"
	roleBindingKind         = "RoleBinding"
	serviceKind             = "Service"
	deploymentKind          = "Deployment"
	configMapKind           = "ConfigMap"
	apiServiceKind          = "APIService"
)

type Bundle struct {
	ServiceAccount     *core.ServiceAccount
	ClusterRoles       []rbac.ClusterRole
	ClusterRoleBinding *rbac.ClusterRoleBinding
	RoleBinding        *rbac.RoleBinding
	Service            *core.Service
	Deployment         *apps.Deployment
	ConfigMap          *core.ConfigMap
	ApiService         *apiregv1.APIService
}

func ReadBundle(path string) (*Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	bundle, err := loadBundleFromBytes(data)
	if err != nil {
		return nil, err
	}

	if err = validateBundle(bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

func GetBundlePath() string {
	return filepath.Join(vmConsoleProxyBundleDir, "vm-console-proxy.yaml")
}

func loadBundleFromBytes(data []byte) (*Bundle, error) {
	bundle := &Bundle{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(&obj)
		if err == io.EOF {
			return bundle, nil
		}
		if err != nil {
			return nil, err
		}

		kind := obj.GetKind()

		var destObj any
		switch kind {
		case rbac.ServiceAccountKind:
			destObj = &(bundle.ServiceAccount)
		case clusterRoleKind:
			// Add a dummy object to the slice, it will be filled later
			bundle.ClusterRoles = append(bundle.ClusterRoles, rbac.ClusterRole{})
			destObj = &(bundle.ClusterRoles[len(bundle.ClusterRoles)-1])
		case clusterRoleBindingsKind:
			destObj = &(bundle.ClusterRoleBinding)
		case roleBindingKind:
			destObj = &(bundle.RoleBinding)
		case serviceKind:
			destObj = &(bundle.Service)
		case deploymentKind:
			destObj = &(bundle.Deployment)
		case configMapKind:
			destObj = &(bundle.ConfigMap)
		case apiServiceKind:
			destObj = &(bundle.ApiService)
		case "":
			return nil, fmt.Errorf("empty Kind found in vm-console-proxy bundle")
		default:
			return nil, fmt.Errorf("unsupported Kind found in vm-console-proxy bundle: %s", kind)
		}

		if kind != clusterRoleKind && !reflect.ValueOf(destObj).Elem().IsNil() {
			return nil, fmt.Errorf("duplicate Kind found in vm-console-proxy bundle: %s", kind)
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), destObj)
		if err != nil {
			return nil, err
		}
	}
}

func validateBundle(bundle *Bundle) error {
	missingFields := make([]string, 0, 8)
	if bundle.ServiceAccount == nil {
		missingFields = append(missingFields, "ServiceAccount")
	}
	if len(bundle.ClusterRoles) == 0 {
		missingFields = append(missingFields, "ClusterRole")
	}
	if bundle.ClusterRoleBinding == nil {
		missingFields = append(missingFields, "ClusterRoleBinding")
	}
	if bundle.RoleBinding == nil {
		missingFields = append(missingFields, "RoleBinding")
	}
	if bundle.Service == nil {
		missingFields = append(missingFields, "Service")
	}
	if bundle.Deployment == nil {
		missingFields = append(missingFields, "Deployment")
	}
	if bundle.ConfigMap == nil {
		missingFields = append(missingFields, "ConfigMap")
	}
	if bundle.ApiService == nil {
		missingFields = append(missingFields, "ApiService")
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("bundle is missing these objects: %v", missingFields)
	}

	return nil
}
