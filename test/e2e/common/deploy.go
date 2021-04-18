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

package common

import (
	"fmt"
	"path"
	"time"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

func DeployOperator(c clientset.Interface) (func(), error) {
	var (
		executor = CmdHelmExecutor{framework.TestContext.KubeConfig}
		chart    = HelmChart{
			name:      "csi-baremetal-operator",
			path:      path.Join(OperatorTestContext.ChartsFolder, "csi-baremetal-operator"),
			namespace: "test-csi-operator",
		}
		installArgs     = fmt.Sprintf("--set image.tag=%s", OperatorTestContext.OperatorVersion)
		waitTime        = 1 * time.Minute
		sleepBeforeWait = 10 * time.Second
	)

	cleanup := func() {
		if err := executor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Operator helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		if err := c.CoreV1().Namespaces().Delete(chart.namespace, nil); err != nil {
			e2elog.Logf("Namespace deletion failed. Namespace: %s", chart.namespace)
		}
	}

	if err := executor.InstallRelease(&chart, installArgs); err != nil {
		return nil, err
	}

	time.Sleep(sleepBeforeWait)

	if err := e2epod.WaitForPodsRunningReady(c, chart.namespace, 0, 0, waitTime, nil); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}

func DeployCSI(f *framework.Framework) (func(), error) {
	var (
		executor = CmdHelmExecutor{framework.TestContext.KubeConfig}
		chart    = HelmChart{
			name:      "csi-baremetal",
			path:      path.Join(OperatorTestContext.ChartsFolder, "csi-baremetal-deployment"),
			namespace: f.Namespace.Name,
		}
		installArgs = fmt.Sprintf("--set image.tag=%s "+
			"--set image.pullPolicy=IfNotPresent "+
			"--set driver.drivemgr.type=loopbackmgr "+
			"--set driver.drivemgr.deployConfig=true "+
			"--set scheduler.patcher.enable=true", OperatorTestContext.CsiVersion)
		podWait         = 3 * time.Minute
		sleepBeforeWait = 10 * time.Second
	)

	cleanup := func() {
		if OperatorTestContext.CompleteUninstall {
			if err := execCmdObj(framework.KubectlCmd("delete", "pvc", "--all")); err != nil {
				e2elog.Logf("PVC deletion failed")
			}

			if err := execCmdObj(framework.KubectlCmd("delete", "volumes", "--all")); err != nil {
				e2elog.Logf("Volumes deletion failed")
			}

			if err := execCmdObj(framework.KubectlCmd("delete", "lvgs", "--all")); err != nil {
				e2elog.Logf("Lvgs deletion failed")
			}

			if err := execCmdObj(framework.KubectlCmd("delete", "csibmnodes", "--all")); err != nil {
				e2elog.Logf("Csibmnodes deletion failed")
			}
		}

		if err := executor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Deployment helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}
	}

	if err := executor.InstallRelease(&chart, installArgs); err != nil {
		return nil, err
	}

	time.Sleep(sleepBeforeWait)

	if err := e2epod.WaitForPodsRunningReady(f.ClientSet, chart.namespace, 0, 0, podWait, nil); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}
