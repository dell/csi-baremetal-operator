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
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
)

type KubectlExecutor interface {
	ApplyManifest(arg string) error
	DeleteManifest(arg string) error
}

type KubectlCmdExecutor struct{}

func ApplyManifest(arg string) error {
	cmdStr := fmt.Sprintf("kubectl apply -f %s", arg)
	return execCmdStr(cmdStr)
}

func DeleteManifest(arg string) error {
	cmdStr := fmt.Sprintf("kubectl delete -f %s", arg)
	return execCmdStr(cmdStr)
}

func execCmdStr(cmdStr string) error {
	var stdout, stderr bytes.Buffer

	e2elog.Logf("Exec: %s", cmdStr)

	cmdStrSplit := strings.Split(cmdStr, " ")
	cmd := exec.Command(cmdStrSplit[0], cmdStrSplit[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	e2elog.Logf("Stdout: %s", stdout.String())
	e2elog.Logf("Stderr: %s", stderr.String())

	if err != nil {
		return err
	}
	return nil
}
