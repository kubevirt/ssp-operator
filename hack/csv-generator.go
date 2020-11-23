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
	"github.com/spf13/cobra"
)

type generatorFlags struct {
	bumpCRDs          bool
	csvVersion        string
	namespace         string
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
			runGenerator()
		},
	}
)

func main() {
	rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVar(&f.csvVersion, "csv-version", "", "Version of csv manifest (required)")
	rootCmd.Flags().StringVar(&f.namespace, "namespace", "", "Version of csv manifest (required)")
	rootCmd.Flags().StringVar(&f.operatorImage, "operator-image", "", "Link to operator image (required)")
	rootCmd.Flags().StringVar(&f.validatorImage, "validator-image", "", "Link to template-validator image")
	rootCmd.Flags().StringVar(&f.nodeLabellerImage, "node-labeller-image", "", "Link to node-labeller image")
	rootCmd.Flags().StringVar(&f.kvmInfoImage, "kvm-info-image", "", "Link to kvm-info-nfd-plugin image")
	rootCmd.Flags().StringVar(&f.virtLauncher, "virt-launcher", "", "Link to virt-launcher image")
	rootCmd.Flags().StringVar(&f.cpuPlugin, "cpu-plugin-image", "", "Link to cpu-nfd-plugin image")
	rootCmd.Flags().BoolVar(&f.bumpCRDs, "dump-crds", false, "Dump crds to stdout")

	rootCmd.MarkFlagRequired("csv-version")
	rootCmd.MarkFlagRequired("namespace")
	rootCmd.MarkFlagRequired("operator-image")
}

func runGenerator() {

}
