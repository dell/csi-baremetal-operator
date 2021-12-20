package node

import (
	"context"
	"errors"

	nodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/models"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
	rbacmodels "github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

const (
	platformLabel = "nodes.csi-baremetal.dell.com/platform"
)

// Node controls csi-baremetal-node
type Node struct {
	clientset     kubernetes.Interface
	log           *logrus.Entry
	validator     validator.Validator
	eventRecorder events.EventRecorder
	matchPolicies []rbacv1.PolicyRule
}

// NewNode creates a Node object
func NewNode(clientset kubernetes.Interface,
	eventRecorder events.EventRecorder,
	validator validator.Validator,
	matchPolicies []rbacv1.PolicyRule,
	logger *logrus.Entry,
) *Node {
	return &Node{
		clientset:     clientset,
		log:           logger,
		validator:     validator,
		eventRecorder: eventRecorder,
		matchPolicies: matchPolicies,
	}
}

// Update updates csi-baremetal-node or creates if not found
func (n *Node) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	var (
		// need to trying deploy each daemonset
		// return err != nil to request reconcile again if one ore more daemonsets failed
		resultErr error
	)

	// in case of Openshift deployment and non default namespace - validate node service accounts security bindings
	if csi.Spec.Platform == constant.PlatformOpenShift && csi.Namespace != constant.DefaultNamespace {
		var rbacError rbac.Error
		if resultErr = n.validator.ValidateRBAC(ctx, &models.RBACRules{
			Data: &rbacmodels.ServiceAccountIsRoleBoundData{
				ServiceAccountName: csi.Spec.Driver.Node.ServiceAccount,
				Namespace:          csi.Namespace,
				Role: &rbacv1.Role{
					Rules: n.matchPolicies,
				},
			},
			Type: models.ServiceAccountIsRoleBound,
		}); resultErr != nil {
			if errors.As(resultErr, &rbacError) {
				n.eventRecorder.Eventf(csi, eventing.WarningType, "NodeRoleValidationFailed",
					"ServiceAccount %s has insufficient securityContextConstraints, should have privileged",
					csi.Spec.Driver.Node.ServiceAccount)
				n.log.Warning(rbacError, "Node service account has insufficient securityContextConstraints, should have privileged")
				return nil
			}
			n.log.Error(resultErr, "Error occurred while validating node service account security context bindings")
			return resultErr
		}
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
