package node

import (
	"context"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

const (
	masterNodeLabel = "node-role.kubernetes.io/master"
	label           = "nodes.csi-baremetal.dell.com/platform"
)

type Node struct {
	ctx       context.Context
	clientset kubernetes.Clientset
	log       logr.Logger
}

func NewNode(ctx context.Context, clientset kubernetes.Clientset, logger logr.Logger) *Node {
	return &Node{
		ctx:       ctx,
		clientset: clientset,
		log:       logger,
	}
}

func (n *Node) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	var (
		resultErr error
		namespace = common.GetNamespace(csi)
	)

	needToDeploy, err := n.updateNodeLabels()
	if err != nil {
		return err
	}

	for platformName, isDeploying := range needToDeploy {
		if isDeploying {
			expected := createNodeDaemonSet(csi, platforms[platformName])
			if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
				n.log.Error(err, "Failed to set controller reference")
				continue
			}

			if err = n.updateDaemonset(expected, namespace); err != nil {
				n.log.Error(err, "Failed to update daemonset")
				resultErr = err
			}
		}
	}

	return resultErr
}

func (n *Node) updateDaemonset(expected *v1.DaemonSet, namespace string) error {

	dsClient := n.clientset.AppsV1().DaemonSets(namespace)

	found, err := dsClient.Get(n.ctx, expected.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, err := dsClient.Create(n.ctx, expected, metav1.CreateOptions{}); err != nil {
				n.log.Error(err, "Failed to create daemonset")
				return err
			}

			n.log.Info("Daemonset created successfully")
			return nil
		}

		n.log.Error(err, "Failed to get daemonset")
		return err
	}

	if common.DaemonsetChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(n.ctx, found, metav1.UpdateOptions{}); err != nil {
			n.log.Error(err, "Failed to update daemonset")
			return err
		}

		n.log.Info("Daemonset updated successfully")
		return nil
	}

	return nil
}

func (n *Node) CleanLabels() error {
	return n.cleanNodeLabels()
}

func (n *Node) cleanNodeLabels() error {
	nodes, err := n.clientset.CoreV1().Nodes().List(n.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		if _, ok := node.Labels[label]; ok {
			delete(node.Labels, label)
			if _, err := n.clientset.CoreV1().Nodes().Update(n.ctx, &node, metav1.UpdateOptions{}); err != nil {
				n.log.Error(err, "Failed to delete label on "+node.Name)
			}
		}
	}

	return nil
}

type Set map[string]bool

func (n *Node) updateNodeLabels() (Set, error) {
	needToDeploy := createNeedToDeploySet()

	nodes, err := n.clientset.CoreV1().Nodes().List(n.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		kernelVersion, err := GetNodeKernelVersion(node)
		if err != nil {
			n.log.Error(err, "Failed to get kernel version for "+node.Name)
			continue
		}

		platformName := findPlatform(kernelVersion)
		needToDeploy[platformName] = true

		node.Labels[label] = platforms[platformName].labeltag
		if _, err := n.clientset.CoreV1().Nodes().Update(n.ctx, &node, metav1.UpdateOptions{}); err != nil {
			n.log.Error(err, "Failed to update label on "+node.Name)
		}
	}

	return needToDeploy, nil
}

func findPlatform(kernelVersion string) string {
	for key, value := range platforms {
		if value.checkVersion(kernelVersion) {
			return key
		}
	}

	return "default"
}

func createNeedToDeploySet() Set {
	var result = Set{}

	for key, _ := range platforms {
		result[key] = false
	}
	return result
}
