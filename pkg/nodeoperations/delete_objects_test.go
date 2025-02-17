package nodeoperations

import (
	"context"
	"testing"

	"github.com/dell/csi-baremetal-operator/pkg/common"
	api "github.com/dell/csi-baremetal/api/generated/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	newCSIDrive = &drivecrd.Drive{
		ObjectMeta: metav1.ObjectMeta{
			Name: "222",
		},
		Spec: api.Drive{
			NodeId: "111-111-111",
		},
	}

	newCSIAC = &accrd.AvailableCapacity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "222",
		},
		Spec: api.AvailableCapacity{
			NodeId: "111-111-111",
		},
	}

	newCSILVG = &lvgcrd.LogicalVolumeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "222",
			Finalizers: []string{"finalizer.dell.io/disk"},
		},

		Spec: api.LogicalVolumeGroup{
			Node: "111-111-111",
		},
	}

	newCSIVolume = &volumecrd.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "222",
			Finalizers: []string{"finalizer.dell.io/disk"},
		},
		Spec: api.Volume{
			NodeId: "111-111-111",
		},
	}
)

func Test_DeleteDrives(t *testing.T) {
	t.Run("Check deleteDrives", func(t *testing.T) {
		var (
			ctx = context.Background()
			log = logrus.WithField("Test name", "deleteDrives")
		)

		scheme, _ := common.PrepareScheme()
		clientSet := prepareFakeNodeClientSet()
		clientCl := prepareFakeClient(scheme)

		controller := NewNodeOperationsController(clientSet, clientCl, log)
		assert.NotNil(t, controller)

		err := clientCl.Create(ctx, newCSIDrive)
		assert.Nil(t, err)

		err = controller.deleteDrives(ctx, "111-111-111")
		assert.Nil(t, err)

		remCSIDrive := drivecrd.Drive{}
		err = clientCl.Get(ctx, client.ObjectKey{Name: newCSIDrive.Name}, &remCSIDrive)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func Test_DeleteACs(t *testing.T) {
	t.Run("Check deleteACs", func(t *testing.T) {
		var (
			ctx = context.Background()
			log = logrus.WithField("Test name", "deleteACs")
		)

		scheme, _ := common.PrepareScheme()
		clientSet := prepareFakeNodeClientSet()
		clientCl := prepareFakeClient(scheme)

		controller := NewNodeOperationsController(clientSet, clientCl, log)
		assert.NotNil(t, controller)

		err := clientCl.Create(ctx, newCSIAC)
		assert.Nil(t, err)

		err = controller.deleteACs(ctx, "111-111-111")
		assert.Nil(t, err)

		remCSIAC := accrd.AvailableCapacity{}
		err = clientCl.Get(ctx, client.ObjectKey{Name: newCSIAC.Name}, &remCSIAC)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func Test_DeleteLVGs(t *testing.T) {
	t.Run("Check deleteLVGs", func(t *testing.T) {
		var (
			ctx = context.Background()
			log = logrus.WithField("Test name", "deleteLVGs")
		)

		scheme, _ := common.PrepareScheme()
		clientSet := prepareFakeNodeClientSet()
		clientCl := prepareFakeClient(scheme)

		controller := NewNodeOperationsController(clientSet, clientCl, log)
		assert.NotNil(t, controller)

		err := clientCl.Create(ctx, newCSILVG)
		assert.Nil(t, err)

		err = controller.deleteLVGs(ctx, "111-111-111")
		assert.Nil(t, err)

		remCSILVG := lvgcrd.LogicalVolumeGroup{}
		err = clientCl.Get(ctx, client.ObjectKey{Name: newCSILVG.Name}, &remCSILVG)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func Test_DeleteVolumes(t *testing.T) {
	t.Run("Check deleteVolumes", func(t *testing.T) {
		var (
			ctx = context.Background()
			log = logrus.WithField("Test name", "deleteVolumes")
		)

		scheme, _ := common.PrepareScheme()
		clientSet := prepareFakeNodeClientSet()
		clientCl := prepareFakeClient(scheme)

		controller := NewNodeOperationsController(clientSet, clientCl, log)
		assert.NotNil(t, controller)

		err := clientCl.Create(ctx, newCSIVolume)
		assert.Nil(t, err)

		err = controller.deleteVolumes(ctx, "111-111-111")
		assert.Nil(t, err)

		remCSIVolume := volumecrd.Volume{}
		err = clientCl.Get(ctx, client.ObjectKey{Name: newCSIVolume.Name}, &remCSIVolume)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func prepareFakeNodeClientSet(objects ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objects...)
}

func prepareFakeClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	builder := fakeClient.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	return builderWithScheme.WithObjects(objects...).Build()
}
