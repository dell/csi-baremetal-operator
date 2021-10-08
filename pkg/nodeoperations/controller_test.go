package nodeoperations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	ctx                    = context.Background()
	node1, node2           corev1.Node
	csibmnode1, csibmnode2 nodecrd.Node
	drive1, drive2         drivecrd.Drive
	ac1, ac2               accrd.AvailableCapacity
	lvg1, lvg2             lvgcrd.LogicalVolumeGroup
	volume1, volume2       volumecrd.Volume
	pod                    corev1.Pod
	podnode1, podnode2     corev1.Pod
	podcontroller          corev1.Pod
)

func Init() {
	node1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{},
		},
	}
	node2 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-2",
			Labels: map[string]string{},
		},
	}

	csibmnode1 = nodecrd.Node{
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

	csibmnode2 = nodecrd.Node{
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

	pod = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "csi-namespace",
			Labels:    map[string]string{"name": "csi-baremetal-node"},
		},
		Spec: corev1.PodSpec{
			NodeName: node1.Name,
		},
	}

	drive1 = drivecrd.Drive{
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

	podnode1 = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-baremetal-node-1",
			Namespace: "csi-namespace",
			Labels:    common.ConstructLabelAppMap(),
			OwnerReferences: []metav1.OwnerReference{
				{Name: "pod", Kind: "DaemonSet"},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: node1.Name,
		},
	}

	podnode2 = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-baremetal-node-2",
			Namespace: "csi-namespace",
			Labels:    common.ConstructLabelAppMap(),
			OwnerReferences: []metav1.OwnerReference{
				{Name: "pod", Kind: "DaemonSet"},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: node2.Name,
		},
	}

	podcontroller = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-baremetal-controller",
			Namespace: "csi-namespace",
			Labels:    common.ConstructLabelAppMap(),
			OwnerReferences: []metav1.OwnerReference{
				{Name: "pod", Kind: "Deployment"},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: node1.Name,
		},
	}
}

func Test_getMapIsNodesTainted(t *testing.T) {
	t.Run("Should return info about nodes with taint", func(t *testing.T) {
		Init()
		badTaint := rTaint
		badTaint.Effect = "BadEffect"
		node1.Spec.Taints = []corev1.Taint{rTaint}
		node2.Spec.Taints = []corev1.Taint{badTaint}

		taintedNodes := getMapIsNodesTainted([]corev1.Node{node1, node2}, rTaint)
		assert.True(t, taintedNodes[node1.Name])
		assert.False(t, taintedNodes[node2.Name])
	})
}

func Test_removeNodes(t *testing.T) {
	t.Run("Should delete resources", func(t *testing.T) {
		Init()
		c := prepareController(&csibmnode1, &csibmnode2, &drive1, &drive2, &ac1, &ac2, &lvg1, &lvg2, &volume1, &volume2)

		err := c.removeNodes(ctx, []nodecrd.Node{csibmnode1})
		assert.Nil(t, err)

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, &csibmnode1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: drive1.Name}, &drive1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: ac1.Name}, &ac1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: lvg1.Name}, &lvg1)
		assert.True(t, k8serrors.IsNotFound(err))
		err = c.client.Get(ctx, client.ObjectKey{Name: volume1.Name}, &volume1)
		assert.True(t, k8serrors.IsNotFound(err))

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode2.Name}, &csibmnode2)
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
		Init()
		c := prepareController(&csibmnode1, &csibmnode2, &pod)

		err := c.removeNodes(ctx, []nodecrd.Node{csibmnode1, csibmnode2})
		assert.NotNil(t, err)
	})
}

func Test_handleNodeRemoval(t *testing.T) {
	t.Run("Should label csibmnode", func(t *testing.T) {
		Init()
		node1.Spec.Taints = []corev1.Taint{rTaint}
		c := prepareController(&csibmnode1, &csibmnode2)

		err := c.handleNodeRemoval(ctx, []nodecrd.Node{csibmnode1}, []corev1.Node{node1})
		assert.Nil(t, err)

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, &csibmnode1)
		assert.Nil(t, err)

		value := csibmnode1.GetLabels()[rTaint.Key]
		assert.Equal(t, rTaint.Value, value)
	})

	t.Run("Should remove label on csibmnode", func(t *testing.T) {
		Init()
		csibmnode1.Labels = map[string]string{rTaint.Key: rTaint.Value}
		c := prepareController(&csibmnode1)

		err := c.handleNodeRemoval(ctx, []nodecrd.Node{csibmnode1}, []corev1.Node{node1})
		assert.Nil(t, err)

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, &csibmnode1)
		assert.Nil(t, err)

		value, ok := csibmnode1.GetLabels()[rTaint.Key]
		assert.False(t, ok)
		assert.Equal(t, value, "")
	})

	t.Run("Should remove node", func(t *testing.T) {
		Init()
		node1.Spec.Taints = []corev1.Taint{rTaint}
		csibmnode1.Labels = map[string]string{rTaint.Key: rTaint.Value}
		c := prepareController(&csibmnode1)

		err := c.handleNodeRemoval(ctx, []nodecrd.Node{csibmnode1}, []corev1.Node{})
		assert.Nil(t, err)

		err = c.client.Get(ctx, client.ObjectKey{Name: csibmnode1.Name}, &csibmnode1)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func Test_handleNodeMaintenance(t *testing.T) {
	t.Run("Should remove csi pods from tainted nodes (ecxept DaemonSets)", func(t *testing.T) {
		Init()
		node1.Spec.Taints = []corev1.Taint{mTaint}
		c := prepareController(&csibmnode1, &csibmnode2, &podnode1, &podnode2, &podcontroller)

		err := c.handleNodeMaintenance(ctx, []corev1.Node{node1, node2})
		assert.Nil(t, err)

		// Expected Deployment pod was deleted from tainted node
		err = c.client.Get(ctx, client.ObjectKey{Namespace: podcontroller.Namespace, Name: podcontroller.Name}, &podcontroller)
		assert.True(t, k8serrors.IsNotFound(err))

		// Expected not tainted node wasn't affected: Deployment pod still alive
		err = c.client.Get(ctx, client.ObjectKey{Namespace: podnode1.Namespace, Name: podnode1.Name}, &podnode1)
		assert.Nil(t, err)

		// Expected DaemonSet pod wasn't deleted from tainted node
		err = c.client.Get(ctx, client.ObjectKey{Namespace: podnode2.Namespace, Name: podnode2.Name}, &podnode2)
		assert.Nil(t, err)
	})
}

func prepareController(objects ...runtime.Object) *Controller {
	scheme, _ := common.PrepareScheme()
	client := fake.NewFakeClientWithScheme(scheme, objects...)
	controller := NewNodeOperationsController(
		nil,
		client,
		ctrl.Log.WithName("NodeOperationsTest"))

	return controller
}
