package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/node"
	"github.com/dell/csi-baremetal-operator/pkg/nodeoperations"
	"github.com/dell/csi-baremetal-operator/pkg/patcher"
)

// CSIDeployment contains controllers of CSI resources
type CSIDeployment struct {
	node                     *node.Node
	controller               Controller
	extender                 SchedulerExtender
	patcher                  patcher.SchedulerPatcher
	nodeController           NodeController
	nodeOperationsController *nodeoperations.Controller
}

// NewCSIDeployment creates CSIDeployment
func NewCSIDeployment(clientSet kubernetes.Clientset, client client.Client, log *logrus.Logger) CSIDeployment {
	return CSIDeployment{
		node: node.NewNode(
			&clientSet,
			log.WithField(constant.CSIName, "node"),
		),
		controller: Controller{
			Clientset: &clientSet,
			Entry:     log.WithField(constant.CSIName, "controller"),
		},
		extender: SchedulerExtender{
			Clientset: &clientSet,
			Entry:     log.WithField(constant.CSIName, "extender"),
		},
		patcher: patcher.SchedulerPatcher{
			Clientset: &clientSet,
			Client:    client,
			Log:       log.WithField(constant.CSIName, "patcher"),
		},
		nodeController: NodeController{
			Clientset: &clientSet,
			Entry:     log.WithField(constant.CSIName, "nodeController"),
		},
		nodeOperationsController: nodeoperations.NewNodeOperationsController(
			&clientSet,
			client,
			log.WithField(constant.CSIName, "nodeRemovalController"),
		),
	}
}

// Update performs Update functions of contained resources
func (c *CSIDeployment) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	if err := c.nodeController.Update(ctx, csi, scheme); err != nil {
		return err
	}

	if err := c.node.Update(ctx, csi, scheme); err != nil {
		return err
	}

	if err := c.controller.Update(ctx, csi, scheme); err != nil {
		return err
	}

	if err := c.extender.Update(ctx, csi, scheme); err != nil {
		return err
	}

	if err := c.patcher.Update(ctx, csi, scheme); err != nil {
		return err
	}

	return nil
}

// ReconcileNodes performs node removal procedure
func (c *CSIDeployment) ReconcileNodes(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	if err := c.nodeOperationsController.Reconcile(ctx, csi); err != nil {
		return err
	}

	return nil
}

// Uninstall cleans CSI
func (c *CSIDeployment) Uninstall(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	var errMsgs []string

	err := c.patcher.Uninstall(ctx, csi)
	if err != nil {
		errMsgs = append(errMsgs, err.Error())
	}

	err = c.node.Uninstall(ctx, csi)
	if err != nil {
		errMsgs = append(errMsgs, err.Error())
	}

	if len(errMsgs) != 0 {
		return fmt.Errorf(strings.Join(errMsgs, "\n"))
	}

	return nil
}
