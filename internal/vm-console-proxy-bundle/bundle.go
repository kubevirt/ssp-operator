package vm_console_proxy_bundle

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	vmConsoleProxyBundleDir = "data/vm-console-proxy-bundle/"
)

const (
	clusterRoleKind         = "ClusterRole"
	clusterRoleBindingsKind = "ClusterRoleBinding"
	serviceKind             = "Service"
	deploymentKind          = "Deployment"
	configMapKind           = "ConfigMap"
)

type Bundle struct {
	ServiceAccount     core.ServiceAccount
	ClusterRole        rbac.ClusterRole
	ClusterRoleBinding rbac.ClusterRoleBinding
	Service            core.Service
	Deployment         apps.Deployment
	ConfigMap          core.ConfigMap
}

func ReadBundle(path string) (*Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return loadBundleFromBytes(data)
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
			destObj = &(bundle.ClusterRole)
		case clusterRoleBindingsKind:
			destObj = &(bundle.ClusterRoleBinding)
		case serviceKind:
			destObj = &(bundle.Service)
		case deploymentKind:
			destObj = &(bundle.Deployment)
		case configMapKind:
			destObj = &(bundle.ConfigMap)
		case "":
			return nil, fmt.Errorf("empty Kind found in vm-console-proxy bundle")
		default:
			return nil, fmt.Errorf("unsupported Kind found in vm-console-proxy bundle: %s", kind)
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), destObj)
		if err != nil {
			return nil, err
		}
	}
}
