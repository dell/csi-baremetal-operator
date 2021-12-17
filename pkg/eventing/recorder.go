package eventing

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// tickStepForEventCreating is the time step between retries of creating k8s Event
var tickStepForEventCreating = 10 * time.Second

// timeoutForEventCreating is the timeout for Event creating operation
var timeoutForEventCreating = time.Minute

// Recorder is a recorder for sending events
type Recorder interface {
	Eventf(ctx context.Context, object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})
}

type recorder struct {
	client client.Client
	scheme *runtime.Scheme
	source v1.EventSource
	log    *logrus.Entry
}

func (r *recorder) Eventf(ctx context.Context, object runtime.Object,
	eventtype, reason, messageFmt string, args ...interface{},
) {
	message := fmt.Sprintf(messageFmt, args...)

	reference, err := ref.GetReference(r.scheme, object)
	if err != nil {
		r.log.Errorf("Could not construct reference to: '%#v' due to: '%v'. "+
			"Will not report event: '%v' '%v' '%v'", object, err, eventtype, reason, message)
		return
	}

	event := r.makeEvent(reference, nil, eventtype, reason, message)
	event.Source = r.source
	go createK8sEvent(ctx, event, r.client, r.log)
	return
}

// makeEvent is helper to build v1.Event according parameters
func (r *recorder) makeEvent(ref *v1.ObjectReference, labels map[string]string, eventtype, reason, message string) *v1.Event {
	t := metav1.NewTime(time.Now())
	namespace := ref.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			// make uniq name
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: namespace,
			Labels:    labels,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        message,
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           eventtype,
	}
}

// createK8sEvent creates k8s Event with retries and timeout
func createK8sEvent(ctx context.Context, k8sEvent *v1.Event, client client.Client, log *logrus.Entry) {
	childCtx, cancelFunc := context.WithTimeout(ctx, timeoutForEventCreating)
	defer cancelFunc()
	ticker := time.NewTicker(tickStepForEventCreating)
	defer ticker.Stop()

	for {
		select {
		case <-childCtx.Done():
			log.Errorf("Failed to create event %v: timeout was reached", k8sEvent)
			return
		case <-ticker.C:
			// To support correct retry mechanism its needed to create new context with timeout for each call.
			// Otherwise we can stuck at first cs.Client.Create() call and exit from function after timeout without
			// retries.
			eventCtx, eventCtxCancelFunc := context.WithTimeout(childCtx, tickStepForEventCreating)
			err := client.Create(eventCtx, k8sEvent)
			eventCtxCancelFunc()
			if err == nil {
				return
			}
		}
	}
}

// NewRecorder is a constructor for eventing recorder
func NewRecorder(client client.Client, scheme *runtime.Scheme, source v1.EventSource, log *logrus.Entry) Recorder {
	return &recorder{
		client: client,
		scheme: scheme,
		source: source,
		log:    log,
	}
}
