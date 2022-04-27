package node

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	securityverifier "github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier"
	"github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier/models"
	verifierModels "github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier/models"
)

const (
	platformLabel = "nodes.csi-baremetal.dell.com/platform"
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
