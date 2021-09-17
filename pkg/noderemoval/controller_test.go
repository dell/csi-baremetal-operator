package noderemoval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/dell/csi-baremetal-operator/pkg/common"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

var (
	ctx = context.Background()

	testNode1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{},
		},
	}
	testNode2 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-2",
			Labels: map[string]string{},
		},
	}

	testCSIBMNode1 = nodecrd.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csibmnode-1",
		},
		Spec: api.Node{
			UUID: "ffff-aaaa-bbbb",
			Addresses: map[string]string{
				string(corev1.NodeHostName):   "node-1",
				string(corev1.NodeInternalIP): "10.10.10.1",
			},
		},
	}
	testCSIBMNode2 = nodecrd.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csibmnode-2",
		},
		Spec: api.Node{
			UUID: "1111-2222-3333",
			Addresses: map[string]string{
				string(corev1.NodeHostName):   "node-2",
				string(corev1.NodeInternalIP): "10.10.10.2",
			},
		},
	}
)

func Test_getTaintedNodes(t *testing.T) {
	t.Run("Should return info about nodes with taint", func(t *testing.T) {
		var (
			node1 = testNode1.DeepCopy()
			node2 = testNode2.DeepCopy()
		)

		node1.Spec.Taints = []corev1.Taint{
			{
				Key:    nodeRemovalTaintKey,
				Value:  nodeRemovalTaintValue,
				Effect: nodeRemovalTaintEffect,
			},
		}

		taintedNodes := getTaintedNodes([]corev1.Node{*node1, *node2})
		assert.True(t, taintedNodes[node1.Name])
		assert.False(t, taintedNodes[node2.Name])
	})
}

func Test_reconcileNodes(t *testing.T) {
	t.Run("Should label csibmnode", func(t *testing.T) {
		var (
			csibmnode1   = testCSIBMNode1.DeepCopy()
			taintedNodes = map[string]bool{
				getNodeName(csibmnode1): true,
			}
		)

		c := prepareController(csibmnode1)

		removingNodes, err := c.reconcileNodes(ctx, []nodecrd.Node{*csibmnode1}, taintedNodes)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(removingNodes))

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, csibmnode1)
		assert.Nil(t, err)

		value := csibmnode1.GetLabels()[nodeRemovalTaintKey]
		assert.Equal(t, nodeRemovalTaintValue, value)
	})

	t.Run("Should remove label on csibmnode", func(t *testing.T) {
		var (
			csibmnode1   = testCSIBMNode1.DeepCopy()
			taintedNodes = map[string]bool{
				getNodeName(csibmnode1): false,
			}
		)

		csibmnode1.Labels = map[string]string{nodeRemovalTaintKey: nodeRemovalTaintValue}

		c := prepareController(csibmnode1)

		removingNodes, err := c.reconcileNodes(ctx, []nodecrd.Node{*csibmnode1}, taintedNodes)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(removingNodes))

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, csibmnode1)
		assert.Nil(t, err)

		value, ok := csibmnode1.GetLabels()[nodeRemovalTaintKey]
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	t.Run("Should remove node", func(t *testing.T) {
		var (
			csibmnode1   = testCSIBMNode1.DeepCopy()
			taintedNodes = map[string]bool{}
		)

		csibmnode1.Labels = map[string]string{nodeRemovalTaintKey: nodeRemovalTaintValue}

		c := prepareController(csibmnode1)

		removingNodes, err := c.reconcileNodes(ctx, []nodecrd.Node{*csibmnode1}, taintedNodes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(removingNodes))
		assert.Equal(t, csibmnode1.Name, removingNodes[0].Name)
	})
}

func Test_removeNodes(t *testing.T) {
	t.Run("Should delete resources", func(t *testing.T) {
		var (
			csibmnode1 = testCSIBMNode1.DeepCopy()
			csibmnode2 = testCSIBMNode2.DeepCopy()
			drive1     = drivecrd.Drive{
				ObjectMeta: metav1.ObjectMeta{Name: "drive1"},
				Spec:       api.Drive{NodeId: csibmnode1.Spec.UUID},
			}
			drive2 = drivecrd.Drive{
				ObjectMeta: metav1.ObjectMeta{Name: "drive2"},
				Spec:       api.Drive{NodeId: csibmnode2.Spec.UUID},
			}
			ac1 = accrd.AvailableCapacity{
				ObjectMeta: metav1.ObjectMeta{Name: "ac1"},
				Spec:       api.AvailableCapacity{NodeId: csibmnode1.Spec.UUID},
			}
			ac2 = accrd.AvailableCapacity{
				ObjectMeta: metav1.ObjectMeta{Name: "ac2"},
				Spec:       api.AvailableCapacity{NodeId: csibmnode2.Spec.UUID},
			}
			lvg1 = lvgcrd.LogicalVolumeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "lvg1"},
				Spec:       api.LogicalVolumeGroup{Node: csibmnode1.Spec.UUID},
			}
			lvg2 = lvgcrd.LogicalVolumeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "lvg2"},
				Spec:       api.LogicalVolumeGroup{Node: csibmnode2.Spec.UUID},
			}
			volume1 = volumecrd.Volume{
				ObjectMeta: metav1.ObjectMeta{Name: "volume1"},
				Spec:       api.Volume{NodeId: csibmnode1.Spec.UUID},
			}
			volume2 = volumecrd.Volume{
				ObjectMeta: metav1.ObjectMeta{Name: "volume2"},
				Spec:       api.Volume{NodeId: csibmnode2.Spec.UUID},
			}
		)

		c := prepareController(csibmnode1, csibmnode2, &drive1, &drive2, &ac1, &ac2, &lvg1, &lvg2, &volume1, &volume2)

		err := c.removeNodes(ctx, []nodecrd.Node{*csibmnode1})
		assert.Nil(t, err)

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, csibmnode1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: drive1.Name}, &drive1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: ac1.Name}, &ac1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: lvg1.Name}, &lvg1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: volume1.Name}, &volume1)
		assert.True(t, k8serrors.IsNotFound(err))

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode2.Name}, csibmnode2)
		assert.Nil(t, err)
		err = c.client.Get(ctx, client.ObjectKey{Name: drive2.Name}, &drive2)
		assert.Nil(t, err)
		err = c.client.Get(ctx, client.ObjectKey{Name: ac2.Name}, &ac2)
		assert.Nil(t, err)
		err = c.client.Get(ctx, client.ObjectKey{Name: lvg2.Name}, &lvg2)
		assert.Nil(t, err)
		err = c.client.Get(ctx, client.ObjectKey{Name: volume2.Name}, &volume2)
		assert.Nil(t, err)
	})

	t.Run("Should wait running pod", func(t *testing.T) {
		var (
			csibmnode1 = testCSIBMNode1.DeepCopy()
			pod        = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "csi-namespace",
					Labels:    map[string]string{"name": "csi-baremetal-node"},
				},
				Spec: corev1.PodSpec{
					NodeName: getNodeName(csibmnode1),
				},
			}
		)

		c := prepareController(csibmnode1, &pod)

		err := c.removeNodes(ctx, []nodecrd.Node{*csibmnode1})
		assert.NotNil(t, err)
	})
}

func prepareController(objects ...client.Object) *Controller {
	scheme, _ := common.PrepareScheme()
	builder := fake.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	client := builderWithScheme.WithObjects(objects...).Build()
	controller := NewNodeRemovalController(
		nil,
		client,
		ctrl.Log.WithName("NodeRemovalTest"))

	return controller
}
