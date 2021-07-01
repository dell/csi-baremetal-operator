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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOSNameAndVersion(t *testing.T) {
	name, version, err := GetOSNameAndVersion("")
	assert.NotNil(t, err)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("Wrong OS")
	assert.NotNil(t, err)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("12.04")
	assert.NotNil(t, err)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("Ubuntu 18.04.4 LTS")
	assert.Equal(t, err, nil)
	assert.Equal(t, name, "ubuntu")
	assert.Equal(t, version, "18.04")

	name, version, err = GetOSNameAndVersion("Ubuntu 19.10")
	assert.Equal(t, err, nil)
	assert.Equal(t, name, "ubuntu")
	assert.Equal(t, version, "19.10")

	// OpenShift has the following output for OS Image
	name, version, err = GetOSNameAndVersion("Red Hat Enterprise Linux CoreOS 46.82.202101301821-0 (Ootpa)")
	assert.Equal(t, err, nil)
	assert.Equal(t, name, "red")
	assert.Equal(t, version, "46.82")
}

func TestGetKernelVersion(t *testing.T) {
	_, err := GetKernelVersion("")
	assert.NotNil(t, err)

	_, err = GetKernelVersion("bla-bla")
	assert.NotNil(t, err)

	// ubuntu 19
	testSupportedKernel := "5.4"
	testSupportedKernelVersion, err := GetKernelVersion(testSupportedKernel)
	assert.Equal(t, err, nil)

	// ubuntu 19
	newKernel1 := "5.4.0-66-generic"
	newKernel1Version, err := GetKernelVersion(newKernel1)
	assert.Equal(t, err, nil)
	// ubuntu 21
	newKernel2 := "5.12.13-051213-generic"
	newKernel2Version, err := GetKernelVersion(newKernel2)
	assert.Equal(t, err, nil)

	// ubuntu 18
	oldKernel1 := "4.15.0-76-generic"
	oldKernel1Version, err := GetKernelVersion(oldKernel1)
	assert.Equal(t, err, nil)
	// rhel coreos 4.6
	oldKernel2 := "4.18.0-193.41.1.el8_2.x86_64"
	oldKernel2Version, err := GetKernelVersion(oldKernel2)
	assert.Equal(t, err, nil)

	assert.True(t, greaterOrEqual(testSupportedKernelVersion, testSupportedKernelVersion))
	assert.True(t, greaterOrEqual(newKernel1Version, testSupportedKernelVersion))
	assert.True(t, greaterOrEqual(newKernel2Version, testSupportedKernelVersion))

	assert.False(t, greaterOrEqual(oldKernel1Version, testSupportedKernelVersion))
	assert.False(t, greaterOrEqual(oldKernel2Version, testSupportedKernelVersion))
}
