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

package constant

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	CSIName = "csi-baremetal"

	// ports
	PrometheusPort = 8787
	LivenessPort   = "liveness-port"

	// timeouts
	TerminationGracePeriodSeconds = 10

	// volumes
	LogsVolume         = "logs"
	CSISocketDirVolume = "csi-socket-dir"

	// termination settings
	TerminationMessagePath   = "/var/log/termination-log"
	TerminationMessagePolicy = corev1.TerminationMessageReadFile

	// sidecars
	ProvisionerName     = "csi-provisioner"
	ResizerName         = "csi-resizer"
	DriverRegistrarName = "csi-node-driver-registrar"
	LivenessProbeName   = "livenessprobe"
)
