/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/blang/semver/v4"
	gyaml "github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands/node-labeller"
)

type generatorFlags struct {
	file              string
	dumpCRDs          bool
	removeCerts       bool
	webhookPort       int32
	csvVersion        string
	namespace         string
	operatorVersion   string
	validatorImage    string
	kvmInfoImage      string
	virtLauncher      string
	nodeLabellerImage string
	cpuPlugin         string
	operatorImage     string
}

var (
	f       generatorFlags
	rootCmd = &cobra.Command{
		Use:   "csv-generator",
		Short: "csv-generator for ssp operator",
		Long:  `csv-generator generates deploy manifest for ssp operator`,
		Run: func(cmd *cobra.Command, args []string) {
			err := runGenerator()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
)

func main() {
	rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVar(&f.file, "file", "data/olm-catalog/ssp-operator.clusterserviceversion.yaml", "Location of the CSV yaml to modify")
	rootCmd.Flags().StringVar(&f.csvVersion, "csv-version", "", "Version of csv manifest (required)")
	rootCmd.Flags().StringVar(&f.namespace, "namespace", "", "Namespace in which ssp operator will be deployed (required)")
	rootCmd.Flags().StringVar(&f.operatorImage, "operator-image", "", "Link to operator image (required)")
	rootCmd.Flags().StringVar(&f.operatorVersion, "operator-version", "", "Operator version (required)")
	rootCmd.Flags().StringVar(&f.validatorImage, "validator-image", "", "Link to template-validator image")
	rootCmd.Flags().StringVar(&f.nodeLabellerImage, "node-labeller-image", "", "Link to node-labeller image")
	rootCmd.Flags().StringVar(&f.kvmInfoImage, "kvm-info-image", "", "Link to kvm-info-nfd-plugin image")
	rootCmd.Flags().StringVar(&f.virtLauncher, "virt-launcher-image", "", "Link to virt-launcher image")
	rootCmd.Flags().StringVar(&f.cpuPlugin, "cpu-plugin-image", "", "Link to cpu-nfd-plugin image")
	rootCmd.Flags().Int32Var(&f.webhookPort, "webhook-port", 0, "Container port for the admission webhook")
	rootCmd.Flags().BoolVar(&f.removeCerts, "webhook-remove-certs", false, "Remove the webhook certificate volume and mount")
	rootCmd.Flags().BoolVar(&f.dumpCRDs, "dump-crds", false, "Dump crds to stdout")

	rootCmd.MarkFlagRequired("csv-version")
	rootCmd.MarkFlagRequired("namespace")
	rootCmd.MarkFlagRequired("operator-image")
	rootCmd.MarkFlagRequired("operator-version")
}

func runGenerator() error {
	csvFile, err := ioutil.ReadFile(f.file)
	if err != nil {
		return err
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(csvFile), 1024)
	csv := csvv1.ClusterServiceVersion{}
	err = decoder.Decode(&csv)
	if err != nil {
		return err
	}

	err = replaceVariables(f, &csv)
	if err != nil {
		return err
	}

	if f.removeCerts {
		removeCerts(f, &csv)
	}

	relatedImages, err := buildRelatedImages(f)
	if err != nil {
		return err
	}

	err = marshallObject(csv, relatedImages, os.Stdout)
	if err != nil {
		return err
	}
	if f.dumpCRDs {
		files, err := ioutil.ReadDir("data/crd")
		if err != nil {
			return err
		}
		for _, file := range files {
			crdFile, err := ioutil.ReadFile(fmt.Sprintf("data/crd/%s", file.Name()))
			if err != nil {
				return err
			}

			crd := extv1beta1.CustomResourceDefinition{}
			decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(crdFile), 1024)

			err = decoder.Decode(&crd)
			if err != nil {
				return err
			}

			err = marshallObject(crd, relatedImages, os.Stdout)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func buildRelatedImage(imageDesc string, imageName string) (map[string]interface{}, error) {
	ri := make(map[string]interface{})
	ri["name"] = imageName
	ri["image"] = imageDesc

	return ri, nil
}

func buildRelatedImages(flags generatorFlags) ([]interface{}, error) {
	var relatedImages []interface{} = make([]interface{}, 0)

	if flags.validatorImage != "" {
		relatedImage, err := buildRelatedImage(flags.validatorImage, "template-validator")
		if err != nil {
			return nil, err
		}
		relatedImages = append(relatedImages, relatedImage)
	}

	img := node_labeller.KubevirtNodeLabellerDefaultImage
	if flags.nodeLabellerImage != "" {
		img = flags.nodeLabellerImage
	}
	relatedImage, err := buildRelatedImage(img, "node-labeller")
	if err != nil {
		return nil, err
	}
	relatedImages = append(relatedImages, relatedImage)

	img = node_labeller.KvmInfoNfdDefaultImage
	if flags.kvmInfoImage != "" {
		img = flags.kvmInfoImage
	}
	relatedImage, err = buildRelatedImage(img, "kvm-info-nfd-plugin")
	if err != nil {
		return nil, err
	}
	relatedImages = append(relatedImages, relatedImage)

	img = node_labeller.KvmCpuNfdDefaultImage
	if flags.cpuPlugin != "" {
		img = flags.cpuPlugin
	}
	relatedImage, err = buildRelatedImage(img, "cpu-nfd-plugin")
	if err != nil {
		return nil, err
	}
	relatedImages = append(relatedImages, relatedImage)

	img = node_labeller.LibvirtDefaultImage
	if flags.virtLauncher != "" {
		img = flags.virtLauncher
	}
	relatedImage, err = buildRelatedImage(img, "virt-launcher")
	if err != nil {
		return nil, err
	}
	relatedImages = append(relatedImages, relatedImage)

	return relatedImages, nil
}

func replaceVariables(flags generatorFlags, csv *csvv1.ClusterServiceVersion) error {
	csv.Name = "ssp-operator.v" + flags.csvVersion
	v, err := semver.New(flags.csvVersion)
	if err != nil {
		return err
	}
	csv.Spec.Version = version.OperatorVersion{
		Version: *v,
	}

	templateSpec := &csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec
	for i, container := range templateSpec.Containers {
		updatedContainer := container
		if container.Name == "manager" {
			updatedContainer.Image = flags.operatorImage
			updatedVariables := make([]v1.EnvVar, 0)
			for _, envVariable := range container.Env {

				if envVariable.Name == common.KvmInfoNfdPluginImageKey {
					envVariable.Value = flags.kvmInfoImage
				}
				if envVariable.Name == common.TemplateValidatorImageKey {
					envVariable.Value = flags.validatorImage
				}
				if envVariable.Name == common.VirtLauncherImageKey {
					envVariable.Value = flags.virtLauncher
				}
				if envVariable.Name == common.KubevirtNodeLabellerImageKey {
					envVariable.Value = flags.nodeLabellerImage
				}
				if envVariable.Name == common.KubevirtCpuNfdPluginImageKey {
					envVariable.Value = flags.cpuPlugin
				}
				if envVariable.Name == common.OperatorVersionKey {
					envVariable.Value = flags.operatorVersion
				}
				updatedVariables = append(updatedVariables, envVariable)
			}
			updatedContainer.Env = updatedVariables
			templateSpec.Containers[i] = updatedContainer
			break
		}
	}

	if flags.webhookPort > 0 {
		csv.Spec.WebhookDefinitions[0].ContainerPort = flags.webhookPort
	}

	return nil
}

func removeCerts(flags generatorFlags, csv *csvv1.ClusterServiceVersion) {
	// Remove the certs mount from the manager container
	templateSpec := &csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec
	for i, container := range templateSpec.Containers {
		if container.Name == "manager" {
			updatedVolumeMounts := templateSpec.Containers[i].VolumeMounts
			for j, volumeMount := range templateSpec.Containers[i].VolumeMounts {
				if volumeMount.Name == "cert" {
					updatedVolumeMounts = append(templateSpec.Containers[i].VolumeMounts[:j], templateSpec.Containers[i].VolumeMounts[j+1:]...)
					break
				}
			}
			templateSpec.Containers[i].VolumeMounts = updatedVolumeMounts
			break
		}
	}

	// Remove the cert volume definition
	updatedVolumes := templateSpec.Volumes
	for i, volume := range templateSpec.Volumes {
		if volume.Name == "cert" {
			updatedVolumes = append(templateSpec.Volumes[:i], templateSpec.Volumes[i+1:]...)
		}
	}
	templateSpec.Volumes = updatedVolumes
}

func marshallObject(obj interface{}, relatedImages []interface{}, writer io.Writer) error {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		return err
	}

	// remove status and metadata.creationTimestamp
	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "spec", "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "status")

	deployments, exists, err := unstructured.NestedSlice(r.Object, "spec", "install", "spec", "deployments")
	if err != nil {
		return err
	}

	if exists {
		for _, obj := range deployments {
			deployment := obj.(map[string]interface{})
			unstructured.RemoveNestedField(deployment, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(deployment, "spec", "template", "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(deployment, "status")
		}
		unstructured.SetNestedSlice(r.Object, deployments, "spec", "install", "spec", "deployments")
	}

	unstructured.SetNestedSlice(r.Object, relatedImages, "spec", "relatedImages")

	jsonBytes, err = json.Marshal(r.Object)
	if err != nil {
		return err
	}

	yamlBytes, err := gyaml.JSONToYAML(jsonBytes)
	if err != nil {
		return err
	}

	// fix templates by removing unneeded single quotes...
	s := string(yamlBytes)
	s = strings.Replace(s, "'{{", "{{", -1)
	s = strings.Replace(s, "}}'", "}}", -1)

	// fix double quoted strings by removing unneeded single quotes...
	s = strings.Replace(s, " '\"", " \"", -1)
	s = strings.Replace(s, "\"'\n", "\"\n", -1)

	yamlBytes = []byte(s)

	_, err = writer.Write([]byte("---\n"))
	if err != nil {
		return err
	}

	_, err = writer.Write(yamlBytes)
	if err != nil {
		return err
	}

	return nil
}
