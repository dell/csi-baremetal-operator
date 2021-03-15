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

// Package csicrd contains API Schema definitions for the csi v1 API group
// +groupName=csi-baremetal.dell.com
// +versionName=v1
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	crScheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersionCSIBaremetal is group version used to register these objects
	GroupVersionCSIBaremetal = schema.GroupVersion{Group: "csi-baremetal.dell.com", Version: "v1"}

	// SchemeBuilderCSIBaremetal is used to add go types to the GroupVersionKind scheme
	SchemeBuilderCSIBaremetal = &crScheme.Builder{GroupVersion: GroupVersionCSIBaremetal}

	// AddToSchemeCSIBaremetal adds the types in this group-version to the given scheme.
	AddToSchemeCSIBaremetal = SchemeBuilderCSIBaremetal.AddToScheme
)
