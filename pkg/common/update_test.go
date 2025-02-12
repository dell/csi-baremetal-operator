package common

import (
	"context"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	newConfigMap = &coreV1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Data: map[string]string{"test": "test"},
	}

	targetConfigMap = &coreV1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Data: map[string]string{"test-updated": "test-updated"},
	}

	newDaemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test": "selector-test"},
			},
		},
	}

	targetDaemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test-update",
							Image: "test-update",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test": "selector-test"},
			},
		},
	}

	targetDaemonSetSelector = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test-updated": "selector-test-updated"},
			},
		},
	}

	newDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test": "selector-test"},
			},
			Replicas: getReplicas(2),
		},
	}

	targetDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test-updated",
							Image: "test-updated",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test": "selector-test"},
			},
			Replicas: getReplicas(2),
		},
	}

	targetDeploymentSelector = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test-updated": "selector-test-updated"},
			},
			Replicas: getReplicas(2),
		},
	}

	targetDeploymentReplicas = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"selector-test": "selector-test"},
			},
			Replicas: getReplicas(5),
		},
	}
)

func Test_UpdateConfigMap(t *testing.T) {
	t.Run("Check UpdateConfigMap function", func(t *testing.T) {
		var (
			ctx = context.Background()
			log = logrus.WithField("Test name", "UpdateTests")
		)

		clientSet := prepareFakeNodeClientSet()
		dsClient := clientSet.CoreV1().ConfigMaps(newConfigMap.Namespace)
		_, err := dsClient.Create(ctx, newConfigMap, metav1.CreateOptions{})
		assert.Nil(t, err)

		err = UpdateConfigMap(ctx, clientSet, targetConfigMap, log)
		assert.Nil(t, err)

		foundConfigMap, err := dsClient.Get(ctx, targetConfigMap.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, foundConfigMap)

		if !reflect.DeepEqual(foundConfigMap.Data, targetConfigMap.Data) {
			t.Errorf("Expected deployment spec: %v, but got: %v", foundConfigMap.Data, targetConfigMap.Data)
		}

	})
}

func Test_UpdateDaemonSet(t *testing.T) {
	tests := []struct {
		testTarget      string
		targetDaemonSet *appsv1.DaemonSet
	}{
		{
			testTarget:      "spec",
			targetDaemonSet: targetDaemonSet,
		},
		{
			testTarget:      "selector",
			targetDaemonSet: targetDaemonSetSelector,
		},
	}
	for _, tt := range tests {
		t.Run("Check changed daemonset "+tt.testTarget, func(t *testing.T) {
			var (
				ctx = context.Background()
				log = logrus.WithField("Test name", "UpdateTests")
			)

			clientSet := prepareFakeNodeClientSet()
			dsClient := clientSet.AppsV1().DaemonSets(newDaemonSet.Namespace)
			_, err := dsClient.Create(ctx, newDaemonSet, metav1.CreateOptions{})
			assert.Nil(t, err)

			err = UpdateDaemonSet(ctx, clientSet, tt.targetDaemonSet, log)
			assert.Nil(t, err)

			foundDaemonset, err := dsClient.Get(ctx, tt.targetDaemonSet.Name, metav1.GetOptions{})
			assert.Nil(t, err)
			assert.NotNil(t, foundDaemonset)

			if !reflect.DeepEqual(foundDaemonset.Spec, tt.targetDaemonSet.Spec) {
				t.Errorf("Expected deployment spec: %v, but got: %v", foundDaemonset.Spec, tt.targetDaemonSet.Spec)
			}
		})
	}
}

func Test_UpdateDeployment_Selector(t *testing.T) {
	tests := []struct {
		testTarget       string
		targetDeployment *appsv1.Deployment
	}{
		{
			testTarget:       "spec",
			targetDeployment: targetDeployment,
		},
		{
			testTarget:       "selector",
			targetDeployment: targetDeploymentSelector,
		},
		{
			testTarget:       "replicas",
			targetDeployment: targetDeploymentReplicas,
		},
	}

	for _, tt := range tests {
		t.Run("Check changed deployment "+tt.testTarget, func(t *testing.T) {
			var (
				ctx = context.Background()
				log = logrus.WithField("Test name", "UpdateTests")
			)

			clientSet := prepareFakeNodeClientSet()
			dsClient := clientSet.AppsV1().Deployments(newDeployment.Namespace)
			_, err := dsClient.Create(ctx, newDeployment, metav1.CreateOptions{})
			assert.Nil(t, err)

			err = UpdateDeployment(ctx, clientSet, tt.targetDeployment, log)
			assert.Nil(t, err)

			foundDeployment, err := dsClient.Get(ctx, tt.targetDeployment.Name, metav1.GetOptions{})
			assert.Nil(t, err)
			assert.NotNil(t, foundDeployment)

			if !reflect.DeepEqual(foundDeployment.Spec, tt.targetDeployment.Spec) {
				t.Errorf("Expected deployment spec: %v, but got: %v", foundDeployment.Spec, tt.targetDeployment.Spec)
			}
		})
	}
}
func prepareFakeNodeClientSet(objects ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objects...)
}

func getReplicas(replicas int32) *int32 {
	return &replicas
}
