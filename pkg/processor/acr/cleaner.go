package acr

import (
	"context"
	"strings"
	"time"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ctxTimeout = 30 * time.Second

// cleaner implements a watcher, which has to check
// all existing ACRs and remove ones, if they are outdated
// (pods for these ACRs were removed). Stacked volumes may
// lead to races, if they are in RESERVED state (block other
// reservations) or new created pods have the same name.
// It's the workaround until we use scheduler-extender

// Cleaner is the watcher to remove outdated ACRs
type Cleaner interface {
	Handle(ctx context.Context)
}

type cleaner struct {
	client client.Client
	log    *logrus.Entry
}

func (c *cleaner) Handle(ctx context.Context) {
	ctx, cancelFn := context.WithTimeout(ctx, ctxTimeout)
	defer cancelFn()

	acrs := &acrcrd.AvailableCapacityReservationList{}
	err := c.client.List(ctx, acrs)
	if err != nil {
		c.log.Errorf("failed to get ACR List: %s", err.Error())
		return
	}

	for i, acr := range acrs.Items {
		if c.needToRemoveACR(ctx, &acrs.Items[i]) {
			c.log.Infof("Try to delete ACR %s", acr.GetName())
			err := c.client.Delete(ctx, &acrs.Items[i])
			if err != nil {
				c.log.Errorf("failed to delete ACR %s: %s", acr.GetName(), err.Error())
			} else {
				c.log.Infof("ACR %s was successfully deleted", acr.GetName())
			}
		}
	}
}

func (c *cleaner) needToRemoveACR(ctx context.Context, acr *acrcrd.AvailableCapacityReservation) bool {
	ns, podName := getPodName(acr)

	pod := &corev1.Pod{}
	err := c.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: ns}, pod)
	if err != nil && !k8serrors.IsNotFound(err) {
		c.log.Errorf("failed to get pod %s in %s namespace: %s", podName, ns, err.Error())
		return false
	}

	// Check if pod was deleted
	if k8serrors.IsNotFound(err) {
		c.log.Warnf("ACR %s is no longer actual. Pod %s in %s ns was removed", acr.GetName(), podName, ns)
		return true
	}

	// Check if pod is Running
	if pod.Status.Phase == corev1.PodRunning {
		c.log.Warnf("ACR %s is no longer actual. Pod %s in %s ns is Running", acr.GetName(), podName, ns)
		return true
	}

	return false
}

// getPodName returns namespace and pod names for passed acr
// must be synced with https://github.com/dell/csi-baremetal/blob/4c0c38da3cdb57a214e63c8ef1373bff8841db49/pkg/scheduler/extender/extender.go#L356
func getPodName(acr *acrcrd.AvailableCapacityReservation) (string, string) {
	namespace := acr.Spec.Namespace
	pod := strings.Replace(acr.GetName(), namespace+"-", "", 1)

	return namespace, pod
}

// NewACRCleaner creates new ACR cleaner
func NewACRCleaner(client client.Client, log *logrus.Entry) Cleaner {
	return &cleaner{
		client: client,
		log:    log,
	}
}
