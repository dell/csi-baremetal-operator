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

package pmm

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
)

type NodeRemovalController struct {
	clientset kubernetes.Interface
	log       logr.Logger
}

func (nrc *NodeRemovalController) Reconcile(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	nodes, err := common.GetSelectedNodes(ctx, nrc.clientset, csi.Spec.NodeSelector)
	if err != nil {
		return nil
	}

	nrc.log.Info(nodes.String())

	return nil
}
