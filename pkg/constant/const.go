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
	// CSIName - default prefix
	CSIName = "csi-baremetal"

	// PrometheusPort - default prometeus port
	PrometheusPort = 8787
	// LivenessPort - default liveness port
	LivenessPort = "liveness-port"

	// TerminationGracePeriodSeconds - default termination timeout
	TerminationGracePeriodSeconds = 10

	// LogsVolume - default volume for logs
	LogsVolume = "logs"
	// CSISocketDirVolume - default volume of CSI socket
	CSISocketDirVolume = "csi-socket-dir"

	// TerminationMessagePath - default path for saving termination message
	TerminationMessagePath = "/var/log/termination-log"
	// TerminationMessagePolicy - default policy
	TerminationMessagePolicy = corev1.TerminationMessageReadFile

	// ProvisionerName - name of csi-provisioner sidecar
	ProvisionerName = "csi-provisioner"
	// ResizerName - name of csi-resizer sidecar
	ResizerName = "csi-resizer"
	// DriverRegistrarName - name of csi-node-driver-registrar sidecar
	DriverRegistrarName = "csi-node-driver-registrar"
	// LivenessProbeName - name of livenessprobe sidecar
	LivenessProbeName = "livenessprobe"
)

var (
	// CrashVolume - the volume for crush dumps
	CrashVolume = corev1.Volume{
		Name: "crash-dump",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}}

	// CrashMountVolume - the mount point for CrashVolume
	CrashMountVolume = corev1.VolumeMount{
		Name: "crash-dump", MountPath: "/crash-dump",
	}
)
