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

package noderemoval

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	nodepkg "github.com/dell/csi-baremetal-operator/pkg/node"

	"github.com/dell/csi-baremetal/api/v1/nodecrd"
)

const (
	nodeRemovalTaintKey    = "node.dell.com/drain"
	nodeRemovalTaintValue  = "drain"
	nodeRemovalTaintEffect = "NoSchedule"
)

// Controller performs node removal procedure
type Controller struct {
	clientset kubernetes.Interface
	client    client.Client
	log       logr.Logger
}

// NewNodeRemovalController returns Controller object
func NewNodeRemovalController(clientset kubernetes.Interface, client client.Client, log logr.Logger) *Controller {
	return &Controller{
		clientset: clientset,
		client:    client,
		log:       log,
	}
}

// Reconcile checks node removal conditions and deletes CSI resources if csibmnode is labeled and k8sNode is deleted
func (c *Controller) Reconcile(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	var nodeSelector *components.NodeSelector

	if csi != nil {
		nodeSelector = csi.Spec.NodeSelector
	}

	nodes, err := common.GetSelectedNodes(ctx, c.clientset, nodeSelector)
	if err != nil {
		return nil
	}

	csibmnodes := &nodecrd.NodeList{}
	err = c.client.List(ctx, csibmnodes)
	if err != nil {
		return nil
	}

	nodesWithTaint := getTaintedNodes(nodes.Items)

	removingNodes, err := c.reconcileNodes(ctx, csibmnodes.Items, nodesWithTaint)
	if err != nil {
		return err
	}

	if len(removingNodes) != 0 {
		if err := c.removeNodes(ctx, removingNodes); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) reconcileNodes(ctx context.Context, csibmnodes []nodecrd.Node, nodesWithTaint map[string]bool) ([]nodecrd.Node, error) {
	var (
		errors        []string
		removingNodes []nodecrd.Node
	)

	for i, csibmnode := range csibmnodes {
		hasLabel := false
		hasTaint := false
		hasNode := false
		needUpdate := false

		if value, ok := csibmnode.GetLabels()[nodeRemovalTaintKey]; ok && value == nodeRemovalTaintValue {
			hasLabel = true
		}

		hasTaint, hasNode = nodesWithTaint[getNodeName(&csibmnodes[i])]

		// perform node removal
		if hasLabel && !hasNode {
			removingNodes = append(removingNodes, csibmnode)
			continue
		}

		if hasNode && !hasLabel && hasTaint {
			addNodeRemovalLabel(&csibmnodes[i])
			c.log.Info(fmt.Sprintf("Csibmnode %s has labeled with %s=%s", csibmnode.Name, nodeRemovalTaintKey, nodeRemovalTaintValue))
			needUpdate = true
		}

		if hasNode && hasLabel && !hasTaint {
			deleteNodeRemovalLabel(&csibmnodes[i])
			c.log.Info(fmt.Sprintf("Csibmnode %s has unlabeled (%s)", csibmnode.Name, nodeRemovalTaintKey))
			needUpdate = true
		}

		if needUpdate {
			if err := c.client.Update(ctx, &csibmnodes[i], &client.UpdateOptions{}); err != nil {
				c.log.Error(err, "Failed to update csibmnode")
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) != 0 {
		return removingNodes, fmt.Errorf(strings.Join(errors, "\n"))
	}

	return removingNodes, nil
}

func (c *Controller) removeNodes(ctx context.Context, csibmnodes []nodecrd.Node) error {
	var (
		errors []string
	)

	for i := range csibmnodes {
		isRunning, err := c.checkDaemonsetPodRunning(ctx, getNodeName(&csibmnodes[i]))
		if err != nil {
			c.log.Error(err, "Failed to check running pods on node")
			errors = append(errors, err.Error())
			continue
		}
		if isRunning {
			err = fmt.Errorf("csi-baremetal-node pod is still running on node %s", getNodeName(&csibmnodes[i]))
			c.log.Error(err, "Failed to clean related resources")
			errors = append(errors, err.Error())
			continue
		}

		if err := c.deleteCSIResources(ctx, &csibmnodes[i]); err != nil {
			c.log.Error(err, "Failed to clean related resources")
			errors = append(errors, err.Error())
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func getTaintedNodes(nodes []corev1.Node) map[string]bool {
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

	return nodesWithTaint
}

func (c *Controller) checkDaemonsetPodRunning(ctx context.Context, nodeName string) (bool, error) {
	fieldSelector := fields.SelectorFromSet(map[string]string{"spec.nodeName": nodeName})
	labelSelector := nodepkg.GetNodeDaemonsetPodsSelector()

	var pods corev1.PodList
	err := c.client.List(ctx, &pods, &client.ListOptions{FieldSelector: fieldSelector, LabelSelector: labelSelector})
	if err != nil {
		return false, err
	}

	if len(pods.Items) != 0 {
		for _, pod := range pods.Items {
			c.log.Info(fmt.Sprintf("%s is still running", pod.Name))
		}
		return true, nil
	}

	return false, nil
}

func getNodeName(csibmnode *nodecrd.Node) string {
	return csibmnode.Spec.Addresses[string(corev1.NodeHostName)]
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
