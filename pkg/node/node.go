package node

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
)

const (
	platformLabel    = "nodes.csi-baremetal.dell.com/platform"
	nodeIDAnnotation = "nodes.csi-baremetal.dell.com/uuid"
)

type Node struct {
	clientset kubernetes.Interface
	log       logr.Logger
}

func NewNode(clientset kubernetes.Interface, logger logr.Logger) *Node {
	return &Node{
		clientset: clientset,
		log:       logger,
	}
}

func (n *Node) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	var (
		// need to trying deploy each daemonset
		// return err != nil to request reconcile again if one ore more daemonsets failed
		resultErr error
		namespace = common.GetNamespace(csi)
	)

	isReady, err := n.isAnnotationsReady(ctx, csi.Spec.NodeSelector)
	if err != nil {
		return err
	}
	if !isReady {
		return errors.New("nodes are not annotated yet")
	}

	needToDeploy, err := n.updateNodeLabels(ctx, csi.Spec.NodeSelector)
	if err != nil {
		return err
	}

	for platformName, isDeploying := range needToDeploy {
		if isDeploying {
			expected := createNodeDaemonSet(csi, platforms[platformName])
			if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
				n.log.Error(err, "Failed to set controller reference "+expected.Name)
				continue
			}

			if err = n.updateDaemonset(ctx, expected, namespace); err != nil {
				n.log.Error(err, "Failed to update daemonset "+expected.Name)
				resultErr = err
			}
		}
	}

	return resultErr
}

// CleanLabels deletes platform-label on each node in cluster
func (n *Node) CleanLabels(ctx context.Context) error {
	return n.cleanNodeLabels(ctx)
}

func (n *Node) updateDaemonset(ctx context.Context, expected *v1.DaemonSet, namespace string) error {
	dsClient := n.clientset.AppsV1().DaemonSets(namespace)

	found, err := dsClient.Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if _, err := dsClient.Create(ctx, expected, metav1.CreateOptions{}); err != nil {
				n.log.Error(err, "Failed to create daemonset "+expected.Name)
				return err
			}

			n.log.Info("Daemonset created successfully: " + expected.Name)
			return nil
		}

		n.log.Error(err, "Failed to get daemonset "+expected.Name)
		return err
	}

	if common.DaemonsetChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(ctx, found, metav1.UpdateOptions{}); err != nil {
			n.log.Error(err, "Failed to update daemonset "+expected.Name)
			return err
		}

		n.log.Info("Daemonset updated successfully: " + expected.Name)
		return nil
	}

	return nil
}

func (n *Node) isAnnotationsReady(ctx context.Context, selector *components.NodeSelector) (bool, error) {
	nodes, err := n.getNodes(ctx, selector)
	if err != nil {
		return false, err
	}

	for _, node := range nodes.Items {
		if _, ok := node.Annotations[nodeIDAnnotation]; !ok {
			return false, nil
		}
	}

	return true, nil
}

// updateNodeLabels gets list of all nodes in cluster,
// selects fit platform for each one and add/update node platform-label
// returns a Set of platforms, which will be deployed
func (n *Node) updateNodeLabels(ctx context.Context, selector *components.NodeSelector) (Set, error) {
	// need to trying getKernelVersion and update label on each node
	// return err != nil to request reconcile again if one ore more nodes failed
	var (
		resultErr error
	)

	needToDeploy := createPlatformsSet()

	nodes, err := n.getNodes(ctx, selector)
	if err != nil {
		return needToDeploy, err
	}

	for _, node := range nodes.Items {
		kernelVersion, err := GetNodeKernelVersion(node)
		if err != nil {
			n.log.Error(err, "Failed to get kernel version for "+node.Name)
			resultErr = err
			continue
		}

		platformName := findPlatform(kernelVersion)
		needToDeploy[platformName] = true

		node.Labels[platformLabel] = platforms[platformName].labeltag
		if _, err := n.clientset.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{}); err != nil {
			n.log.Error(err, "Failed to update label on "+node.Name)
			resultErr = err
		}
	}

	return needToDeploy, resultErr
}

func (n *Node) cleanNodeLabels(ctx context.Context) error {
	nodes, err := n.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		if _, ok := node.Labels[platformLabel]; ok {
			delete(node.Labels, platformLabel)
			if _, err := n.clientset.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{}); err != nil {
				n.log.Error(err, "Failed to delete label on "+node.Name)
			}
		}
	}

	return nil
}

func (n *Node) getNodes(ctx context.Context, selector *components.NodeSelector) (*corev1.NodeList, error) {
	var listOptions = metav1.ListOptions{}

	if selector != nil {
		labelSelector := metav1.LabelSelector{MatchLabels: common.MakeNodeSelectorMap(selector)}
		listOptions.LabelSelector = labels.Set(labelSelector.MatchLabels).String()
	}

	nodes, err := n.clientset.CoreV1().Nodes().List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

// findPlatform calls checkVersion for all platforms in list,
// returns first found platform-name or "default" if no one passed
func findPlatform(kernelVersion string) string {
	for key, value := range platforms {
		if value.checkVersion(kernelVersion) {
			return key
		}
	}

	return "default"
}

// Set is needed to check if one type of platform is exists in current cluster
type Set map[string]bool

// createNeedToDeploySet returns set of platform-names
func createPlatformsSet() Set {
	var result = Set{}

	for key := range platforms {
		result[key] = false
	}
	return result
}
