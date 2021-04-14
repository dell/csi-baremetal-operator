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
	"time"

	"github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/dell/csi-baremetal-operator/test/e2e/common"
)

// DefineLabeledDeployTestSuite defines label tests
func CSIDeployTestSuite(f *framework.Framework) {
	ginkgo.Context("Deploy csi-baremetal with operator", func() {
		csiDeployTestSuite(f)
	})
}

func csiDeployTestSuite(f *framework.Framework) {
	ginkgo.It("", func() {
		driverCleanup, err := common.DeployCSIDeployment(f.ClientSet)
		if err != nil {
		}
		defer driverCleanup()

		err = e2epod.WaitForPodsRunningReady(f.ClientSet, f.Namespace.Name, 0, 0, 1*time.Minute, nil)
		if err != nil {
			framework.Failf("Pods not ready, error: %s", err.Error())
		}
	})
}
