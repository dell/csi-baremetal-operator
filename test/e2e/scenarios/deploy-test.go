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

package scenarios

import (
	"github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/dell/csi-baremetal-operator/test/e2e/common"
)

// CSIDeployTestSuite checks CSI Deployment installing with operator
func CSIDeployTestSuite() {
	ginkgo.Context("Install csi-baremetal-deployment with operator", func() {
		csiDeployTestSuite()
	})
}

func csiDeployTestSuite() {
	var (
		f = framework.NewDefaultFramework("csi-deployment")
	)

	ginkgo.It("All pods are ready", func() {
		driverCleanup, err := common.DeployCSI(f)
		framework.ExpectNoError(err)

		driverCleanup()
	})
}
