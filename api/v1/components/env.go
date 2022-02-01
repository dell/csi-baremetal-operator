/*
Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package components

import "k8s.io/apimachinery/pkg/util/intstr"

// EnvVars contains additional env variables passed to sidecar containers trough helm
type EnvVars struct {
	// +kubebuilder:validation:Pattern:="[A-Z0-9_]"
	Name  string             `json:"name"`
	Value intstr.IntOrString `json:"value"`
}
