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

package node

import (
	"errors"
	"regexp"
	"strings"

	"github.com/masterminds/semver"
	corev1 "k8s.io/api/core/v1"
)

// GetOSNameAndVersion receives string with the OS information in th following format:
// "<OS name> <OS version> <Extra information>". For example, "Ubuntu 18.04.4 LTS"
// returns os name with the lower case and major and minor version. For example, "ubuntu", "18.04"
func GetOSNameAndVersion(osInfo string) (name, version string, err error) {
	// check input parameter
	if len(osInfo) == 0 {
		return "", "", errors.New("errorEmptyParameter")
	}

	// extract OS name
	name = regexp.MustCompile(`^[A-Za-z]+`).FindString(osInfo)
	if len(name) == 0 {
		return "", "", errors.New("errorEmptyParameter")
	}

	// extract OS version
	version = regexp.MustCompile(`[0-9]+\.[0-9]+`).FindString(osInfo)
	if len(version) == 0 {
		return "", "", errors.New("errorEmptyParameter")
	}

	return strings.ToLower(name), version, nil
}

// GetKernelVersion receives string with the kernel version information in the following format:
// "X.Y.Z-<Number>-<Description>". For example, "5.4.0-66-generic"
// returns kernel version - major and minor. For example, "5.4"
func GetKernelVersion(kernelVersion string) (version *semver.Version, err error) {
	if len(kernelVersion) == 0 {
		return nil, errors.New("errorEmptyParameter")
	}

	// extract kernel version - x.y.z
	versionStr := regexp.MustCompile(`^[0-9]+\.[0-9]+`).FindString(kernelVersion)
	return semver.NewVersion(versionStr)
}

// GetNodeKernelVersion returns kernel version of Node
func GetNodeKernelVersion(node *corev1.Node) (version *semver.Version, err error) {
	return GetKernelVersion(node.Status.NodeInfo.KernelVersion)
}
