package node

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	securityverifier "github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier"
	"github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier/models"
	verifierModels "github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier/models"
)

// Node controls csi-baremetal-node
type Node struct {
	clientset                          kubernetes.Interface
	log                                *logrus.Entry
	podSecurityPolicyVerifier          securityverifier.SecurityVerifier
	securityContextConstraintsVerifier securityverifier.SecurityVerifier
}

// NewNode creates a Node object
func NewNode(clientset kubernetes.Interface,
	podSecurityPolicyVerifier securityverifier.SecurityVerifier,
	securityContextConstraintsVerifier securityverifier.SecurityVerifier,
	logger *logrus.Entry,
) *Node {
	return &Node{
		clientset:                          clientset,
		log:                                logger,
		podSecurityPolicyVerifier:          podSecurityPolicyVerifier,
		securityContextConstraintsVerifier: securityContextConstraintsVerifier,
	}
}

// Update updates csi-baremetal-node or creates if not found
func (n *Node) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// in case of Openshift deployment and non default namespace - validate node service accounts security bindings
	if csi.Spec.Platform == constant.PlatformOpenShift && csi.Namespace != constant.DefaultNamespace {
		if err := n.securityContextConstraintsVerifier.Verify(ctx, csi, verifierModels.Node); err != nil {
			var verifierError securityverifier.Error
			err = n.securityContextConstraintsVerifier.HandleError(ctx, csi, csi.Spec.Driver.Node.ServiceAccount, err)
			if errors.As(err, &verifierError) {
				return nil
			}
			return err
		}
	}

	// in case of podSecurityPolicy feature enabled - validate node service accounts security bindings
	if csi.Spec.Driver.Node.PodSecurityPolicy != nil && csi.Spec.Driver.Node.PodSecurityPolicy.Enable {
		if err := n.podSecurityPolicyVerifier.Verify(ctx, csi, models.Node); err != nil {
			var verifierError securityverifier.Error
			err = n.podSecurityPolicyVerifier.HandleError(ctx, csi, csi.Spec.Driver.Node.ServiceAccount, err)
			if errors.As(err, &verifierError) {
				return nil
			}
			return err
		}
	}

	expected := createNodeDaemonSet(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		n.log.Error(err, "Failed to set controller reference "+expected.Name)
		return err
	}

	if err := common.UpdateDaemonSet(ctx, n.clientset, expected, n.log); err != nil {
		n.log.Error(err, "Failed to update daemonset "+expected.Name)
		return err
	}

	return nil
}

// Uninstall deletes uuid-label on each node in cluster
func (n *Node) Uninstall(ctx context.Context, _ *csibaremetalv1.Deployment) error {
	return n.cleanNodeLabels(ctx)
}

func (n *Node) cleanNodeLabels(ctx context.Context) error {
	nodes, err := n.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		nodeIns := node.DeepCopy()
		// delete label with NodeID
		// workaround to work with csi-node-driver-registrar sidecar internal logic
		// implemented in this method to decrease Kubernetes API calls
		if _, ok := nodeIns.Labels[nodeconst.NodeIDTopologyLabelKey]; ok {
			delete(nodeIns.Labels, nodeconst.NodeIDTopologyLabelKey)
			if _, err := n.clientset.CoreV1().Nodes().Update(ctx, nodeIns, metav1.UpdateOptions{}); err != nil {
				n.log.Error(err, "Failed to delete label on "+nodeIns.Name)
			}
		}
	}

	return nil
}
