package common

import (
	"context"
	"reflect"

	coreV1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func UpdateConfigMap(ctx context.Context, client kubernetes.Interface, expected *coreV1.ConfigMap) error {
	cfClient := client.CoreV1().ConfigMaps(expected.Namespace)

	// try to find existing one
	found, err := cfClient.Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		return err
	}

	// create if not found
	if apiErrors.IsNotFound(err) {
		_, err = cfClient.Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return nil
	}

	// update with new data
	if !reflect.DeepEqual(found.Data, expected.Data) {
		found.Data = expected.Data
		_, err = cfClient.Update(ctx, found, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
