package pkg

import (
	"context"
	"golang.org/x/sync/errgroup"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/node"
)

type CSIDeployment struct {
	ctx            context.Context
	node           *node.Node
	controller     Controller
	extender       SchedulerExtender
	patcher        SchedulerPatcher
	nodeController NodeController
}

func NewCSIDeployment(ctx context.Context, clientSet kubernetes.Clientset,
	client client.Client, log logr.Logger) CSIDeployment {
	return CSIDeployment{
		ctx: ctx,
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

func (c *CSIDeployment) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	errs, _ := errgroup.WithContext(c.ctx)

	errs.Go(func() error { return c.nodeController.Update(csi, scheme) })
	errs.Go(func() error { return c.controller.Update(csi, scheme) })
	errs.Go(func() error { return c.node.Update(csi, scheme) })
	errs.Go(func() error { return c.extender.Update(csi, scheme) })
	errs.Go(func() error { return c.patchPlatform(csi, scheme) })

	return errs.Wait()
}

// patchPlatform is patching method for the scheduler depends on the platform
func (c *CSIDeployment) patchPlatform(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.PatchOpenShift(c.ctx, scheme)
	default:
		return c.patcher.Update(csi, scheme)
	}
}

func (c *CSIDeployment) UninstallPatcher(csi csibaremetalv1.Deployment) error {
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.UnPatchOpenShift(c.ctx)
	default:
		return nil
	}
}

func (c *CSIDeployment) CleanLabels() error {
	return c.node.CleanLabels()
}
