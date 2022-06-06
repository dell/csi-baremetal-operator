package volume

import (
	"context"
	"time"

	v1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ctxTimeout = 30 * time.Second

// Actualizer is the watcher to update volume statuses
type Actualizer interface {
	Handle(ctx context.Context)
}

type actualizer struct {
	client client.Client
	log    *logrus.Entry
}

func (a *actualizer) Handle(ctx context.Context) {
	ctx, cancelFn := context.WithTimeout(ctx, ctxTimeout)
	defer cancelFn()

	volumes := &volumecrd.VolumeList{}
	if err := a.client.List(ctx, volumes); err != nil {
		a.log.Errorf("failed to get Volume List: %s", err.Error())
		return
	}

	for i := 0; i < len(volumes.Items); i++ {
		if volumes.Items[i].Spec.CSIStatus == v1.Published && a.ownerPodsAreRemoved(ctx, &volumes.Items[i]) {
			volumes.Items[i].Spec.CSIStatus = v1.Created
			if err := a.client.Update(ctx, &volumes.Items[i]); err != nil {
				a.log.Errorf("failed to actualize Volume %s: %s", volumes.Items[i].GetName(), err.Error())
				continue
			}
			a.log.Infof("Volume %s was successfully actualized", volumes.Items[i].GetName())
		}
	}
}

func (a *actualizer) ownerPodsAreRemoved(ctx context.Context, volume *volumecrd.Volume) bool {
	ownerPods := volume.Spec.GetOwners()

	pod := &corev1.Pod{}
	for i := 0; i < len(ownerPods); i++ {
		err := a.client.Get(ctx, client.ObjectKey{Name: ownerPods[i], Namespace: volume.Namespace}, pod)
		if err != nil && !k8serrors.IsNotFound(err) {
			a.log.Errorf("failed to get pod %s in %s namespace: %s", ownerPods[i], volume.Namespace, err.Error())
			return false
		}

		// Check if pod was deleted
		if k8serrors.IsNotFound(err) {
			a.log.Infof("Pod %s with Volume %s in %s ns was removed", ownerPods[i], volume.Namespace, volume.GetName())
			continue
		}

		// In case either of owner's pods have not deleted - just return false
		return false
	}

	return true
}

// NewVolumeActualizer creates new Volume actualizer
func NewVolumeActualizer(client client.Client, log *logrus.Entry) Actualizer {
	return &actualizer{
		client: client,
		log:    log,
	}
}
