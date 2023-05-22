package tekton_bundle

import (
	"bytes"
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
	tektonTasksKubernetesBundleDir     = "/data/tekton-tasks/kubernetes/"
	tektonTasksOKDBundleDir            = "/data/tekton-tasks/okd/"
	tektonPipelinesKubernetesBundleDir = "/data/tekton-pipelines/kubernetes/"
	tektonPipelinesOKDBundleDir        = "/data/tekton-pipelines/okd/"
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
	Tasks           []pipeline.Task
	ServiceAccounts []v1.ServiceAccount
	RoleBindings    []rbac.RoleBinding
	ClusterRoles    []rbac.ClusterRole
	Pipelines       []pipeline.Pipeline
	ConfigMaps      []v1.ConfigMap
}

func ReadTasksBundle(isOpenshift bool) (*Bundle, error) {
	var files [][]byte
	path := getTasksBundlePath(isOpenshift)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	files = append(files, data)

	tektonObjs, err := decodeObjectsFromFiles(files)
	if err != nil {
		return nil, err
	}

	return tektonObjs, nil
}

func ReadPipelineBundle(isOpenshift bool) (*Bundle, error) {
	path := getPipelineBundlePath(isOpenshift)
	files, err := readFolder(path)
	if err != nil {
		return nil, err
	}

	tektonObjs, err := decodeObjectsFromFiles(files)
	if err != nil {
		return nil, err
	}

	return tektonObjs, nil
}

func getPipelineBundlePath(isOpenshift bool) string {
	if isOpenshift {
		return tektonPipelinesOKDBundleDir
	}
	return tektonPipelinesKubernetesBundleDir
}

func getTasksBundlePath(isOpenshift bool) string {
	if isOpenshift {
		return filepath.Join(tektonTasksOKDBundleDir, "kubevirt-tekton-tasks-okd.yaml")
	}
	return filepath.Join(tektonTasksKubernetesBundleDir, "kubevirt-tekton-tasks-kubernetes.yaml")
}

func readFolder(folderPath string) ([][]byte, error) {
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}
	filesBytes := make([][]byte, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		f, err := os.ReadFile(filepath.Join(folderPath, file.Name()))
		if err != nil {
			return nil, err
		}
		filesBytes = append(filesBytes, f)
	}

	return filesBytes, nil
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
				case tasksString:
					task := pipeline.Task{}
					err = getObject(obj, &task)
					if err != nil {
						return nil, err
					}
					bundle.Tasks = append(bundle.Tasks, task)
				case pipelineKindString:
					p := pipeline.Pipeline{}
					err = getObject(obj, &p)
					if err != nil {
						return nil, err
					}
					bundle.Pipelines = append(bundle.Pipelines, p)
				case serviceAccountKind:
					sa := v1.ServiceAccount{}
					err = getObject(obj, &sa)
					if err != nil {
						return nil, err
					}
					bundle.ServiceAccounts = append(bundle.ServiceAccounts, sa)
				case roleBindingKind:
					rb := rbac.RoleBinding{}
					err = getObject(obj, &rb)
					if err != nil {
						return nil, err
					}
					bundle.RoleBindings = append(bundle.RoleBindings, rb)
				case clusterRoleKind:
					cr := rbac.ClusterRole{}
					err = getObject(obj, &cr)
					if err != nil {
						return nil, err
					}
					bundle.ClusterRoles = append(bundle.ClusterRoles, cr)
				case configMapKind:
					cm := v1.ConfigMap{}
					err = getObject(obj, &cm)
					if err != nil {
						return nil, err
					}
					bundle.ConfigMaps = append(bundle.ConfigMaps, cm)
				default:
					continue
				}
			}
		}
	}
	return bundle, nil
}

func getObject(obj map[string]interface{}, newObj interface{}) error {
	o, err := json.Marshal(&obj)
	if err != nil {
		return err
	}

	return json.Unmarshal(o, newObj)
}
