package pkg

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/node"
)

type CSIDeployment struct {
	node           *node.Node
	controller     Controller
	extender       SchedulerExtender
	patcher        SchedulerPatcher
	nodeController NodeController
}

func NewCSIDeployment(ctx context.Context, clientSet kubernetes.Clientset,
	client client.Client, log logr.Logger) CSIDeployment {
	return CSIDeployment{
		node: node.NewNode(
			ctx,
			&clientSet,
			log.WithValues(constant.CSIName, "node"),
		),
		controller: Controller{
			ctx:       ctx,
			Clientset: clientSet,
			Logger:    log.WithValues(constant.CSIName, "controller"),
		},
		extender: SchedulerExtender{
			ctx:       ctx,
			Clientset: clientSet,
			Logger:    log.WithValues(constant.CSIName, "extender"),
		},
		patcher: SchedulerPatcher{
			ctx:       ctx,
			Clientset: clientSet,
			Client:    client,
			Logger:    log.WithValues(constant.CSIName, "patcher"),
		},
		nodeController: NodeController{
			ctx:       ctx,
			Clientset: clientSet,
			Logger:    log.WithValues(constant.CSIName, "nodeController"),
		},
	}
}

func (c *CSIDeployment) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	if err := c.node.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.controller.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.extender.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.nodeController.Update(csi, scheme); err != nil {
		return err
	}

	// Patching method for the scheduler depends on the platform
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.PatchOpenShift(ctx, scheme)
	default:
		return c.patcher.Update(csi, scheme)

	}
}

func (c *CSIDeployment) UninstallPatcher(ctx context.Context, csi csibaremetalv1.Deployment) error {
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.UnPatchOpenShift(ctx)
	default:
		return nil
	}
}

func (c *CSIDeployment) CleanLabels() error {
	return c.node.CleanLabels()
}
