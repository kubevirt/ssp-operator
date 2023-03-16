package vm_console_proxy_bundle

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	yamlv2 "gopkg.in/yaml.v2"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	vmConsoleProxyBundleDir = "data/vm-console-proxy-bundle/"
)

var (
	serviceAccountKind      = rbac.ServiceAccountKind
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

func ReadBundle(path string) (Bundle, error) {
	files, err := readFile(path)
	if err != nil {
		return Bundle{}, err
	}

	bundleObj, err := decodeObjectsFromFiles(files)
	if err != nil {
		return Bundle{}, err
	}

	return *bundleObj, nil
}

func GetBundlePath() string {
	return filepath.Join(vmConsoleProxyBundleDir, "vm-console-proxy.yaml")
}

func readFile(fileName string) ([][]byte, error) {
	file, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	return [][]byte{file}, nil
}

func decodeObjectsFromFiles(files [][]byte) (*Bundle, error) {
	bundle := &Bundle{}
	for _, file := range files {
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(file), 1024)
		for {
			var obj map[string]interface{}
			err := decoder.Decode(&obj)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if kind, ok := obj["kind"].(string); ok {
				if kind == "" {
					continue
				}

				switch kind {
				case serviceAccountKind:
					serviceAccount := core.ServiceAccount{}
					err = getObject(obj, &serviceAccount)
					if err != nil {
						return nil, err
					}
					bundle.ServiceAccount = serviceAccount
				case clusterRoleKind:
					clusterRole := rbac.ClusterRole{}
					err = getObject(obj, &clusterRole)
					if err != nil {
						return nil, err
					}
					bundle.ClusterRole = clusterRole
				case clusterRoleBindingsKind:
					clusterRoleBinding := rbac.ClusterRoleBinding{}
					err = getObject(obj, &clusterRoleBinding)
					if err != nil {
						return nil, err
					}
					bundle.ClusterRoleBinding = clusterRoleBinding
				case serviceKind:
					service := core.Service{}
					err = getObject(obj, &service)
					if err != nil {
						return nil, err
					}
					bundle.Service = service
				case deploymentKind:
					deployment := apps.Deployment{}
					err = getObject(obj, &deployment)
					if err != nil {
						return nil, err
					}
					bundle.Deployment = deployment
				case configMapKind:
					configMap := core.ConfigMap{}
					err = getObject(obj, &configMap)
					if err != nil {
						return nil, err
					}
					bundle.ConfigMap = configMap
				default:
					return nil, fmt.Errorf("unsupported Kind found in vm-console-proxy bundle: %s", kind)
				}
			}
		}
	}

	return bundle, nil
}

func getObject(obj map[string]interface{}, newObj interface{}) error {
	o, err := yamlv2.Marshal(&obj)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(o, newObj)
}
