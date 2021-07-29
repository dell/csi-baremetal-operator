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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"

	"github.com/dell/csi-baremetal-operator/api/v1/nodecrd"
)

const (
	nodeRemovalTaintKey    = "node.dell.com/drain"
	nodeRemovalTaintValue  = "drain"
	nodeRemovalTaintEffect = "NoSchedule"
)

type NodeRemovalController struct {
	clientset kubernetes.Interface
	client    client.Client
	log       logr.Logger
}

func NewNodeRemovalController(clientset kubernetes.Interface, client client.Client, log logr.Logger) *NodeRemovalController {
	return &NodeRemovalController{
		clientset: clientset,
		client:    client,
		log:       log,
	}
}

func (nrc *NodeRemovalController) Reconcile(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	nodes, err := common.GetSelectedNodes(ctx, nrc.clientset, csi.Spec.NodeSelector)
	if err != nil {
		return nil
	}

	csibmnodes := &nodecrd.NodeList{}
	nrc.client.List(ctx, csibmnodes, &client.ListOptions{})

	nodesWithTaint, err := getTaintedNodes(nodes.Items)
	if err != nil {
		return nil
	}

	err = nrc.reconcileNodes(ctx, csibmnodes.Items, nodesWithTaint)
	if err != nil {
		return err
	}

	return nil
}

func (nrc *NodeRemovalController) reconcileNodes(ctx context.Context, csibmnodes []nodecrd.Node, nodesWithTaint map[string]bool) error {
	var errors []string

	for _, csibmnode := range csibmnodes {
		hasLabel := false
		hasTaint := false
		hasNode := false
		needUpdate := false

		csibmnodeIns := csibmnode

		if value, ok := csibmnodeIns.GetLabels()[nodeRemovalTaintKey]; ok && value == nodeRemovalTaintValue {
			hasLabel = true
		}

		hasTaint, hasNode = nodesWithTaint[getNodeName(&csibmnodeIns)]

		// perform node removal
		if hasLabel && !hasNode {
			deleteCSIResources()
			continue
		}

		if hasNode && !hasLabel && hasTaint {
			addNodeRemovalLabel(&csibmnodeIns)
			nrc.log.Info(fmt.Sprintf("Csibmnode %s has labeled with %s=%s", csibmnodeIns.Name, nodeRemovalTaintKey, nodeRemovalTaintValue))
			needUpdate = true
		}

		if hasNode && hasLabel && !hasTaint {
			deleteNodeRemovalLabel(&csibmnodeIns)
			nrc.log.Info(fmt.Sprintf("Csibmnode %s has unlabeled (%s)", csibmnodeIns.Name, nodeRemovalTaintKey))
			needUpdate = true
		}

		if needUpdate {
			if err := nrc.client.Update(ctx, &csibmnodeIns, &client.UpdateOptions{}); err != nil {
				nrc.log.Error(err, "Failed to update csibmnode")
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func getTaintedNodes(nodes []corev1.Node) (map[string]bool, error) {
	nodesWithTaint := map[string]bool{}

	for _, node := range nodes {
		taints := node.Spec.Taints
		if len(taints) == 0 {
			nodesWithTaint[node.Name] = false
			continue
		}

		hasTaint := false
		for _, taint := range taints {
			if taint.Key == nodeRemovalTaintKey &&
				taint.Value == nodeRemovalTaintValue &&
				taint.Effect == nodeRemovalTaintEffect {
				hasTaint = true
				continue
			}
		}

		nodesWithTaint[node.Name] = hasTaint
	}

	return nodesWithTaint, nil
}

func getNodeName(csibmnode *nodecrd.Node) string {
	return csibmnode.Spec.Addresses["Hostname"]
}

func addNodeRemovalLabel(csibmnode *nodecrd.Node) {
	if csibmnode.Labels == nil {
		csibmnode.Labels = map[string]string{}
	}
	csibmnode.Labels[nodeRemovalTaintKey] = nodeRemovalTaintValue
}

func deleteNodeRemovalLabel(csibmnode *nodecrd.Node) {
	delete(csibmnode.Labels, nodeRemovalTaintKey)
}
