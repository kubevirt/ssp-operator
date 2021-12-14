package template_bundle

import (
	"bytes"
	"io"
	"io/ioutil"

	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	kubevirtv1 "kubevirt.io/client-go/apis/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type Bundle struct {
	Templates   []templatev1.Template
	DataSources []cdiv1beta1.DataSource
}

func ReadBundle(filename string) (Bundle, error) {
	templates, err := readTemplates(filename)
	if err != nil {
		return Bundle{}, err
	}

	sources, err := extractDataSources(templates)
	if err != nil {
		return Bundle{}, err
	}

	return Bundle{
		Templates:   templates,
		DataSources: sources,
	}, nil
}

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

func extractDataSources(templates []templatev1.Template) ([]cdiv1beta1.DataSource, error) {
	const dataSourceName = "SRC_PVC_NAME"
	const dataSourceNamespace = "SRC_PVC_NAMESPACE"

	var dataSources []cdiv1beta1.DataSource
	for i := range templates {
		template := &templates[i]

		vm := &kubevirtv1.VirtualMachine{}
		err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(template.Objects[0].Raw), 1024).Decode(vm)
		if err != nil {
			return nil, err
		}

		usesDataSources := false
		for _, volumeTemplate := range vm.Spec.DataVolumeTemplates {
			if volumeTemplate.Spec.SourceRef != nil {
				usesDataSources = true
				break
			}
		}
		if !usesDataSources {
			continue
		}

		name, exists := findParameterValue(dataSourceName, template)
		if !exists {
			continue
		}

		namespace, exists := findParameterValue(dataSourceNamespace, template)
		if !exists {
			continue
		}

		dataSources = append(dataSources, createDataSource(name, namespace))
	}

	return dataSources, nil
}

func findParameterValue(name string, template *templatev1.Template) (string, bool) {
	for i := range template.Parameters {
		if template.Parameters[i].Name == name {
			return template.Parameters[i].Value, true
		}
	}
	return "", false
}

func createDataSource(name, namespace string) cdiv1beta1.DataSource {
	return cdiv1beta1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name,
					Namespace: namespace,
				},
			},
		},
	}
}
