package tekton_bundle

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	tektonTasksKubernetesBundleDir = "/data/tekton-tasks/kubernetes/"
	tektonTasksOKDBundleDir        = "/data/tekton-tasks/okd/"
)

var (
	tasksString        = string(pipeline.NamespacedTaskKind)
	pipelineKindString = "Pipeline"
	serviceAccountKind = rbac.ServiceAccountKind
	roleBindingKind    = "RoleBinding"
	clusterRoleKind    = "ClusterRole"
	configMapKind      = "ConfigMap"
)

type Bundle struct {
	// TODO: Update to v1.Task
	Tasks []pipeline.Task //nolint:staticcheck

	ServiceAccounts []v1.ServiceAccount
	RoleBindings    []rbac.RoleBinding
	ClusterRoles    []rbac.ClusterRole

	// TODO: Update to v1.Pipeline
	Pipelines []pipeline.Pipeline //nolint:staticcheck

	ConfigMaps []v1.ConfigMap
}

func GetTektonTasksBundlePath(isOpenShift bool) string {
	if isOpenShift {
		return filepath.Join(tektonTasksOKDBundleDir, "kubevirt-tekton-tasks-okd.yaml")
	}
	return filepath.Join(tektonTasksKubernetesBundleDir, "kubevirt-tekton-tasks-kubernetes.yaml")
}

func ReadBundle(paths []string) (*Bundle, error) {
	bundle := &Bundle{}
	for _, path := range paths {
		err := decodeObjects(path, bundle)
		if err != nil {
			return nil, err
		}
	}
	return bundle, nil
}

func decodeObjects(path string, bundle *Bundle) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewYAMLOrJSONDecoder(file, 1024)
	for {
		var obj map[string]interface{}
		err := decoder.Decode(&obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if kind, ok := obj["kind"].(string); ok {
			if kind == "" {
				continue
			}

			switch kind {
			case tasksString:
				task := pipeline.Task{} //nolint:staticcheck
				err = getObject(obj, &task)
				if err != nil {
					return err
				}
				bundle.Tasks = append(bundle.Tasks, task)
			case pipelineKindString:
				p := pipeline.Pipeline{} //nolint:staticcheck
				err = getObject(obj, &p)
				if err != nil {
					return err
				}
				bundle.Pipelines = append(bundle.Pipelines, p)
			case serviceAccountKind:
				sa := v1.ServiceAccount{}
				err = getObject(obj, &sa)
				if err != nil {
					return err
				}
				bundle.ServiceAccounts = append(bundle.ServiceAccounts, sa)
			case roleBindingKind:
				rb := rbac.RoleBinding{}
				err = getObject(obj, &rb)
				if err != nil {
					return err
				}
				bundle.RoleBindings = append(bundle.RoleBindings, rb)
			case clusterRoleKind:
				cr := rbac.ClusterRole{}
				err = getObject(obj, &cr)
				if err != nil {
					return err
				}
				bundle.ClusterRoles = append(bundle.ClusterRoles, cr)
			case configMapKind:
				cm := v1.ConfigMap{}
				err = getObject(obj, &cm)
				if err != nil {
					return err
				}
				bundle.ConfigMaps = append(bundle.ConfigMaps, cm)
			default:
				continue
			}
		}
	}
	return nil
}

func getObject(obj map[string]interface{}, newObj interface{}) error {
	o, err := json.Marshal(&obj)
	if err != nil {
		return err
	}

	return json.Unmarshal(o, newObj)
}
