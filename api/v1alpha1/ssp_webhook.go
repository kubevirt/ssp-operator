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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var ssplog = logf.Log.WithName("ssp-resource")
var clt client.Client

func (r *SSP) SetupWebhookWithManager(mgr ctrl.Manager) error {
	clt = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ssp-kubevirt-io-v1alpha1-ssp,mutating=false,failurePolicy=fail,groups=ssp.kubevirt.io,resources=ssps,versions=v1alpha1,name=vssp.kb.io

var _ webhook.Validator = &SSP{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SSP) ValidateCreate() error {
	var ssps SSPList

	ssplog.Info("validate create", "name", r.Name)
	err := clt.List(context.TODO(), &ssps, &client.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not list SSPs for validation, please try again: %v", err)
	}
	if len(ssps.Items) > 0 {
		return fmt.Errorf("creation failed, an SSP CR already exists in namespace %v: %v", ssps.Items[0].ObjectMeta.Namespace, ssps.Items[0].ObjectMeta.Name)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SSP) ValidateUpdate(old runtime.Object) error {
	ssplog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SSP) ValidateDelete() error {
	ssplog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// Forces the value of clt, to be used in unit tests
func (r *SSP) ForceCltValue(c client.Client) {
	clt = c
}
