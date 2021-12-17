package node

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/eventing"
	"github.com/dell/csi-baremetal-operator/pkg/eventing/mocks"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
)

const (
	kernelVersion    = "4.15"
	newKernelVersion = "5.4"
)

var (
	nodeSelector *components.NodeSelector

	logEntry = logrus.WithField("Test name", "NodeTest")

	testNode1 = coreV1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{}},
		Status: coreV1.NodeStatus{
			NodeInfo: coreV1.NodeSystemInfo{
				KernelVersion: kernelVersion,
			},
		},
	}
	testNode2 = coreV1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-2",
			Labels: map[string]string{}},
		Status: coreV1.NodeStatus{
			NodeInfo: coreV1.NodeSystemInfo{
				KernelVersion: kernelVersion,
			},
		},
	}
)

func TestNewNode(t *testing.T) {
	t.Run("Create Node", func(t *testing.T) {
		// preparing clientset
		clientSet := fake.NewSimpleClientset()
		// preparing client
		scheme, _ := common.PrepareScheme()
		builder := fakeClient.ClientBuilder{}
		builderWithScheme := builder.WithScheme(scheme)
		cl := builderWithScheme.WithObjects().Build()

		node := NewNode(clientSet, logEntry,
			validator.NewValidator(rbac.NewValidator(cl, logEntry, rbac.NewMatcher())),
			new(mocks.Recorder),
		)
		assert.NotNil(t, node.clientset)
		assert.NotNil(t, node.log)
		assert.NotNil(t, node.validator)
		assert.NotNil(t, node.eventRecorder)
	})
}

func Test_updateNodeLabels(t *testing.T) {
	t.Run("Should deploy default platform and label nodes", func(t *testing.T) {
		var (
			ctx   = context.Background()
			node1 = testNode1.DeepCopy()
			node2 = testNode2.DeepCopy()
		)

		eventRecorder := new(mocks.Recorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, node1, node2)

		needToDeploy, err := node.updateNodeLabels(ctx, nodeSelector)
		assert.Nil(t, err)
		assert.True(t, needToDeploy["default"])
		assert.False(t, needToDeploy["kernel-5.4"])

		updatedNode, err := node.clientset.CoreV1().Nodes().Get(ctx, node1.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["default"].labeltag, updatedNode.Labels[platformLabel])

		updatedNode, err = node.clientset.CoreV1().Nodes().Get(ctx, node2.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["default"].labeltag, updatedNode.Labels[platformLabel])

	})

	t.Run("Should deploy specific platform and label nodes", func(t *testing.T) {
		var (
			ctx   = context.Background()
			node1 = testNode1.DeepCopy()
			node2 = testNode2.DeepCopy()
		)

		node1.Status.NodeInfo = coreV1.NodeSystemInfo{KernelVersion: newKernelVersion}
		node2.Status.NodeInfo = coreV1.NodeSystemInfo{KernelVersion: newKernelVersion}

		eventRecorder := new(mocks.Recorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, node1, node2)

		needToDeploy, err := node.updateNodeLabels(ctx, nodeSelector)
		assert.Nil(t, err)
		assert.True(t, needToDeploy["kernel-5.4"])
		assert.False(t, needToDeploy["default"])

		updatedNode, err := node.clientset.CoreV1().Nodes().Get(ctx, node1.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["kernel-5.4"].labeltag, updatedNode.Labels[platformLabel])

		updatedNode, err = node.clientset.CoreV1().Nodes().Get(ctx, node2.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["kernel-5.4"].labeltag, updatedNode.Labels[platformLabel])
	})

	t.Run("Should deploy multi platform and label nodes", func(t *testing.T) {
		var (
			ctx   = context.Background()
			node1 = testNode1.DeepCopy()
			node2 = testNode2.DeepCopy()
		)

		node1.Status.NodeInfo = coreV1.NodeSystemInfo{KernelVersion: newKernelVersion}

		eventRecorder := new(mocks.Recorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, node1, node2)

		needToDeploy, err := node.updateNodeLabels(ctx, nodeSelector)
		assert.Nil(t, err)
		assert.True(t, needToDeploy["kernel-5.4"])
		assert.True(t, needToDeploy["default"])

		updatedNode, err := node.clientset.CoreV1().Nodes().Get(ctx, node1.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["kernel-5.4"].labeltag, updatedNode.Labels[platformLabel])

		updatedNode, err = node.clientset.CoreV1().Nodes().Get(ctx, node2.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["default"].labeltag, updatedNode.Labels[platformLabel])
	})

	t.Run("Error when node kernel version not readable", func(t *testing.T) {
		var (
			ctx           = context.Background()
			corruptedNode = testNode1.DeepCopy()
		)

		corruptedNode.Status.NodeInfo = coreV1.NodeSystemInfo{KernelVersion: "corrupted_version"}

		eventRecorder := new(mocks.Recorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, corruptedNode)

		needToDeploy, err := node.updateNodeLabels(ctx, nodeSelector)
		assert.NotNil(t, err)
		assert.False(t, needToDeploy["kernel-5.4"])
		assert.False(t, needToDeploy["default"])

		updatedNode, err := node.clientset.CoreV1().Nodes().Get(ctx, corruptedNode.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, map[string]string{}, updatedNode.Labels)
	})

	t.Run("Should label nodes only with selector", func(t *testing.T) {
		var (
			ctx           = context.Background()
			node1         = testNode1.DeepCopy()
			node2         = testNode2.DeepCopy()
			selectorLabel = "label"
			selectorTag   = "tag"
		)

		nodeSelector = &components.NodeSelector{Key: selectorLabel, Value: selectorTag}
		node1.Labels[selectorLabel] = selectorTag

		eventRecorder := new(mocks.Recorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, node1, node2)

		needToDeploy, err := node.updateNodeLabels(ctx, nodeSelector)
		assert.Nil(t, err)
		assert.True(t, needToDeploy["default"])

		updatedNode, err := node.clientset.CoreV1().Nodes().Get(ctx, node1.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, platforms["default"].labeltag, updatedNode.Labels[platformLabel])

		updatedNode, err = node.clientset.CoreV1().Nodes().Get(ctx, node2.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		_, ok := updatedNode.Labels[selectorLabel]
		assert.False(t, ok)
	})
}

func Test_cleanNodeLabels(t *testing.T) {
	t.Run("Should clean labels", func(t *testing.T) {
		var (
			ctx   = context.Background()
			node1 = testNode1.DeepCopy()
			node2 = testNode2.DeepCopy()
		)

		node1.Labels[platformLabel] = "default"
		node2.Labels[platformLabel] = "default"

		eventRecorder := new(mocks.Recorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, node1, node2)

		err := node.cleanNodeLabels(ctx)
		assert.Nil(t, err)

		updatedNode, err := node.clientset.CoreV1().Nodes().Get(ctx, node1.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, map[string]string{}, updatedNode.Labels)

		updatedNode, err = node.clientset.CoreV1().Nodes().Get(ctx, node2.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, map[string]string{}, updatedNode.Labels)
	})
}

func prepareNode(eventRecorder eventing.Recorder, objects ...client.Object) *Node {
	// preparing clientSet
	runTimeObjects := make([]runtime.Object, len(objects))
	for i := 0; i < len(objects); i++ {
		runTimeObjects[i] = objects[i].DeepCopyObject()
	}
	clientSet := fake.NewSimpleClientset(runTimeObjects...)

	// preparing client
	scheme, _ := common.PrepareScheme()
	builder := fakeClient.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	cl := builderWithScheme.WithObjects(objects...).Build()

	// prepare event recorder
	node := NewNode(clientSet, logEntry,
		validator.NewValidator(rbac.NewValidator(cl, logEntry, rbac.NewMatcher())),
		eventRecorder,
	)

	return node
}
