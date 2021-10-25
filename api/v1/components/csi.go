/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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

// DeploymentSpec represent all CSI components need to be deployed by operator
type DeploymentSpec struct {
	Driver         *Driver         `json:"driver,omitempty"`
	NodeController *NodeController `json:"nodeController,omitempty"`
	Scheduler      *Scheduler      `json:"scheduler,omitempty"`

	// +nullable
	// +optional
	GlobalRegistry string `json:"globalRegistry"`
	// +nullable
	// +optional
	RegistrySecret string `json:"registrySecret"`
	// +kubebuilder:validation:Enum=IfNotPresent;Always;Never
	// +kubebuilder:default:=IfNotPresent
	PullPolicy string `json:"pullPolicy"`

	NodeSelector             *NodeSelector `json:"nodeSelector,omitempty"`
	NodeIDAnnotation         bool          `json:"nodeIDAnnotation,omitempty"`
	SequentialLVGReservation bool          `json:"sequentialLVGReservation,omitempty"`

	// +optional
	Selector Selector `json:"selector,omitempty"`

	// +kubebuilder:validation:Enum=rke;openshift;vanilla
	// +kubebuilder:default:=vanilla
	Platform string `json:"platform"`
}

// Selector is an unused field to passing linters since our CRD is not actual Deployment
type Selector struct {
	MatchLabels []string `json:"matchLabels,omitempty"`
}
