package node

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"

	nodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"
)

const (
	platformLabel = "nodes.csi-baremetal.dell.com/platform"
)

// Node controls csi-baremetal-node
type Node struct {
	clientset kubernetes.Interface
	log       logr.Logger
}

// NewNode creates a Node object
func NewNode(clientset kubernetes.Interface, logger logr.Logger) *Node {
	return &Node{
		clientset: clientset,
		log:       logger,
	}
}

// Update updates csi-baremetal-node or creates if not found
func (n *Node) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	var (
		// need to trying deploy each daemonset
		// return err != nil to request reconcile again if one ore more daemonsets failed
		resultErr error
	)

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

			if err = common.UpdateDaemonSet(ctx, n.clientset, expected, n.log); err != nil {
				n.log.Error(err, "Failed to update daemonset "+expected.Name)
				resultErr = err
			}
		}
	}

	return resultErr
}

// Uninstall deletes platform-label on each node in cluster
func (n *Node) Uninstall(ctx context.Context, _ *csibaremetalv1.Deployment) error {
	return n.cleanNodeLabels(ctx)
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

	nodes, err := common.GetSelectedNodes(ctx, n.clientset, selector)
	if err != nil {
		return needToDeploy, err
	}

	for i, node := range nodes.Items {
		kernelVersion, err := GetNodeKernelVersion(&nodes.Items[i])
		if err != nil {
			n.log.Error(err, "Failed to get kernel version for "+node.Name)
			resultErr = err
			continue
		}

		platformName := findPlatform(kernelVersion)
		needToDeploy[platformName] = true

		// skip updating label if exists
		if value, ok := node.Labels[platformLabel]; ok && (value == platforms[platformName].labeltag) {
			continue
		}

		node.Labels[platformLabel] = platforms[platformName].labeltag
		if _, err := n.clientset.CoreV1().Nodes().Update(ctx, &nodes.Items[i], metav1.UpdateOptions{}); err != nil {
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
		nodeIns := node
		toUpdate := false

		// delete platform label
		if _, ok := node.Labels[platformLabel]; ok {
			delete(node.Labels, platformLabel)
			toUpdate = true
		}

		// delete label with NodeID
		// workaround to work with csi-node-driver-registrar sidecar internal logic
		// implemented in this method to decrease Kubernetes API calls
		if _, ok := node.Labels[nodeconst.NodeIDTopologyLabelKey]; ok {
			delete(node.Labels, nodeconst.NodeIDTopologyLabelKey)
			toUpdate = true
		}

		if toUpdate {
			if _, err := n.clientset.CoreV1().Nodes().Update(ctx, &nodeIns, metav1.UpdateOptions{}); err != nil {
				n.log.Error(err, "Failed to delete label on "+node.Name)
			}
		}
	}

	return nil
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
