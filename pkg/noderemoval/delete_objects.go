package noderemoval

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

func (c *Controller) deleteCSIResources(ctx context.Context, csibmnode *nodecrd.Node) error {
	var (
		errors []string
		nodeID = csibmnode.Spec.UUID
	)

	if err := c.deleteDrives(ctx, nodeID); err != nil {
		errors = append(errors, err.Error())
	}
	if err := c.deleteACs(ctx, nodeID); err != nil {
		errors = append(errors, err.Error())
	}
	if err := c.deleteLVGs(ctx, nodeID); err != nil {
		errors = append(errors, err.Error())
	}
	if err := c.deleteVolumes(ctx, nodeID); err != nil {
		errors = append(errors, err.Error())
	}

	// we don't clean csibmnode after getting some errors to retry on next reconcile
	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	if err := c.deleteObject(ctx, csibmnode, "csibmnode", false); err != nil {
		return err
	}

	return nil
}

func (c *Controller) deleteDrives(ctx context.Context, nodeID string) error {
	// Field selectors for CRDs' spec is not supported https://github.com/kubernetes/kubernetes/issues/53459
	// fieldSelector := fields.SelectorFromSet(map[string]string{"spec.NodeId": nodeID})

	drives := &drivecrd.DriveList{}
	err := c.client.List(ctx, drives)
	if err != nil {
		return err
	}

	var errors []string

	for i, drive := range drives.Items {
		if drive.Spec.NodeId == nodeID {
			if err = c.deleteObject(ctx, &drives.Items[i], "drive", false); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func (c *Controller) deleteACs(ctx context.Context, nodeID string) error {
	acs := &accrd.AvailableCapacityList{}
	err := c.client.List(ctx, acs)
	if err != nil {
		return err
	}

	var errors []string

	for i, ac := range acs.Items {
		if ac.Spec.NodeId == nodeID {
			if err = c.deleteObject(ctx, &acs.Items[i], "ac", false); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func (c *Controller) deleteLVGs(ctx context.Context, nodeID string) error {
	lvgs := &lvgcrd.LogicalVolumeGroupList{}
	err := c.client.List(ctx, lvgs)
	if err != nil {
		return err
	}

	var errors []string

	for i, lvg := range lvgs.Items {
		if lvg.Spec.Node == nodeID {
			if err = c.deleteObject(ctx, &lvgs.Items[i], "lvg", true); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func (c *Controller) deleteVolumes(ctx context.Context, nodeID string) error {
	volumes := &volumecrd.VolumeList{}
	err := c.client.List(ctx, volumes)
	if err != nil {
		return err
	}

	var errors []string

	for i, volume := range volumes.Items {
		if volume.Spec.NodeId == nodeID {
			if err = c.deleteObject(ctx, &volumes.Items[i], "volume", true); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func (c *Controller) deleteObject(ctx context.Context, obj client.Object, objType string, patchFinalizer bool) error {
	var errors []string

	if patchFinalizer {
		if len(obj.GetFinalizers()) != 0 {
			obj.SetFinalizers([]string{})
			if err := c.client.Update(ctx, obj); err != nil {
				c.log.Error(err, fmt.Sprintf("Failed to update obj, type: %s, name: %s", objType, obj.GetName()))
				errors = append(errors, err.Error())
			}

			if err := c.client.Get(ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
				c.log.Error(err, fmt.Sprintf("Failed to get obj, type: %s, name: %s", objType, obj.GetName()))
				errors = append(errors, err.Error())
			}
		}
	}

	// remove object
	if err := c.client.Delete(ctx, obj); err != nil {
		c.log.Error(err, fmt.Sprintf("Failed to delete obj, type: %s, name: %s", objType, obj.GetName()))
		errors = append(errors, err.Error())
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}
