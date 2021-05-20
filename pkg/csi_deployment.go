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
	node           *node.Node
	controller     Controller
	extender       SchedulerExtender
	patcher        SchedulerPatcher
	nodeController NodeController
}

func NewCSIDeployment(clientSet kubernetes.Clientset, client client.Client, log logr.Logger) CSIDeployment {
	return CSIDeployment{
		node: node.NewNode(
			&clientSet,
			log.WithValues(constant.CSIName, "node"),
		),
		controller: Controller{
			Clientset: clientSet,
			Logger:    log.WithValues(constant.CSIName, "controller"),
		},
		extender: SchedulerExtender{
			Clientset: clientSet,
			Logger:    log.WithValues(constant.CSIName, "extender"),
		},
		patcher: SchedulerPatcher{
			Clientset: clientSet,
			Client:    client,
			Logger:    log.WithValues(constant.CSIName, "patcher"),
		},
		nodeController: NodeController{
			Clientset: clientSet,
			Logger:    log.WithValues(constant.CSIName, "nodeController"),
		},
	}
}

func (c *CSIDeployment) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	errs, ctx := errgroup.WithContext(ctx)

	errs.Go(func() error { return c.nodeController.Update(ctx, csi, scheme) })
	errs.Go(func() error { return c.controller.Update(ctx, csi, scheme) })
	errs.Go(func() error { return c.node.Update(ctx, csi, scheme) })
	errs.Go(func() error { return c.extender.Update(ctx, csi, scheme) })
	errs.Go(func() error { return c.patchPlatform(ctx, csi, scheme) })

	return errs.Wait()
}

// patchPlatform is patching method for the scheduler depends on the platform
func (c *CSIDeployment) patchPlatform(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.PatchOpenShift(ctx, scheme)
	default:
		return c.patcher.Update(ctx, csi, scheme)
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

func (c *CSIDeployment) CleanLabels(ctx context.Context) error {
	return c.node.CleanLabels(ctx)
}
