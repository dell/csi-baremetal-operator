package common

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// UpdateConfigMap updates found configmap with Spec from expected, creates one if not found
func UpdateConfigMap(ctx context.Context, client kubernetes.Interface, expected *coreV1.ConfigMap, log logr.Logger) error {
	cfClient := client.CoreV1().ConfigMaps(expected.Namespace)

	// try to find existing one
	found, err := cfClient.Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		log.Error(err, "Failed to get configmap "+expected.Name)
		return err
	}

	// create if not found
	if apiErrors.IsNotFound(err) {
		_, err = cfClient.Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			log.Error(err, "Failed to create configmap "+expected.Name)
			return err
		}
		log.Info("Configmap created successfully: " + expected.Name)
		return nil
	}

	// update with new data
	if !reflect.DeepEqual(found.Data, expected.Data) {
		found.Data = expected.Data
		_, err = cfClient.Update(ctx, found, metav1.UpdateOptions{})
		if err != nil {
			log.Error(err, "Failed to update configmap "+expected.Name)
			return err
		}
		log.Info("Configmap updated successfully: " + expected.Name)
	}

	return nil
}

// UpdateDaemonSet updates found daemonset with Spec from expected, creates one if not found
// nolint
func UpdateDaemonSet(ctx context.Context, client kubernetes.Interface, expected *appsv1.DaemonSet, log logr.Logger) error {
	dsClient := client.AppsV1().DaemonSets(expected.GetNamespace())

	found, err := dsClient.Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if _, err := dsClient.Create(ctx, expected, metav1.CreateOptions{}); err != nil {
				log.Error(err, "Failed to create daemonset "+expected.Name)
				return err
			}

			log.Info("Daemonset created successfully: " + expected.Name)
			return nil
		}

		log.Error(err, "Failed to get daemonset "+expected.Name)
		return err
	}

	if daemonsetChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(ctx, found, metav1.UpdateOptions{}); err != nil {
			log.Error(err, "Failed to update daemonset "+expected.Name)
			return err
		}
		log.Info("Daemonset updated successfully: " + expected.Name)
	}

	return nil
}

// UpdateDeployment updates found deployment with Spec from expected, creates one if not found
// nolint
func UpdateDeployment(ctx context.Context, client kubernetes.Interface, expected *appsv1.Deployment, log logr.Logger) error {
	dsClient := client.AppsV1().Deployments(expected.GetNamespace())

	found, err := dsClient.Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if _, err := dsClient.Create(ctx, expected, metav1.CreateOptions{}); err != nil {
				log.Error(err, "Failed to create deployment "+expected.Name)
				return err
			}

			log.Info("Deployment created successfully: " + expected.Name)
			return nil
		}

		log.Error(err, "Failed to get deployment "+expected.Name)
		return err
	}

	if deploymentChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(ctx, found, metav1.UpdateOptions{}); err != nil {
			log.Error(err, "Failed to update deployment "+expected.Name)
			return err
		}
		log.Info("Deployment updated successfully: " + expected.Name)
	}

	return nil
}

func deploymentChanged(expected *appsv1.Deployment, found *appsv1.Deployment) bool {
	if !equality.Semantic.DeepEqual(expected.Spec.Replicas, found.Spec.Replicas) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Selector, found.Spec.Selector) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Template, found.Spec.Template) {
		return true
	}

	return false
}

func daemonsetChanged(expected *appsv1.DaemonSet, found *appsv1.DaemonSet) bool {
	if !equality.Semantic.DeepEqual(expected.Spec.Selector, found.Spec.Selector) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Template, found.Spec.Template) {
		return true
	}

	return false
}
