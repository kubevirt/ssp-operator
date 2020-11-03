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

package v1alpha1

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("SSP Validation", func() {
	var (
		client  client.Client
		objects = make([]runtime.Object, 0)
	)

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		// add our own scheme
		SchemeBuilder.AddToScheme(scheme)
		// add more schemes
		v1.AddToScheme(scheme)

		client = fake.NewFakeClientWithScheme(scheme, objects...)
	})

	Context("creating SSP CR", func() {
		Context("when one is already present", func() {
			BeforeEach(func() {
				// add an SSP CR to fake client
				sspExisting := &SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ssp",
						Namespace: "test-ns",
					},
					Spec: SSPSpec{},
				}
				objects = append(objects, sspExisting)
			})

			AfterEach(func() {
				objects = make([]runtime.Object, 0)
			})

			It("should be rejected", func() {
				ssp := SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ssp2",
						Namespace: "test-ns2",
					},
					Spec: SSPSpec{},
				}
				ssp.ForceCltValue(client)
				err := ssp.ValidateCreate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists in namespace test-ns: test-ssp"))
			})
		})
	})
})

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
