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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dell/csi-baremetal-operator/api/v1/components"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName={csi,csis}
// CSIBaremetal is the Schema for the csi API
type CSIBaremetal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              components.CSIBaremetalSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// CSIBaremetalList contains a list of CSI
//+kubebuilder:object:generate=true
type CSIBaremetalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CSIBaremetal `json:"items"`
}

func init() {
	SchemeBuilderCSIBaremetal.Register(&CSIBaremetal{}, &CSIBaremetalList{})
}

func (in *CSIBaremetal) DeepCopyInto(out *CSIBaremetal) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}
