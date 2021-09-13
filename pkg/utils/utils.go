package utils

import (
	"context"
	"fmt"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetSecretSelector(ctx context.Context, c client.Client, namespace string, sel matryoshkav1alpha1.SecretSelector, defaultKey string) ([]byte, error) {
	clientKey := client.ObjectKey{Namespace: namespace, Name: sel.Name}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, clientKey, secret); err != nil {
		return nil, err
	}

	return LookupSecretSelector(secret, sel, defaultKey)
}

func LookupSecretSelector(secret *corev1.Secret, sel matryoshkav1alpha1.SecretSelector, defaultKey string) ([]byte, error) {
	key := sel.Key
	if key == "" {
		key = defaultKey
	}

	v, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("secret has no data at key %s", key)
	}

	return v, nil
}

func GetConfigMapSelector(ctx context.Context, c client.Client, namespace string, sel matryoshkav1alpha1.ConfigMapSelector, defaultKey string) (string, error) {
	clientKey := client.ObjectKey{Namespace: namespace, Name: sel.Name}
	configMap := &corev1.ConfigMap{}
	if err := c.Get(ctx, clientKey, configMap); err != nil {
		return "", err
	}

	return LookupConfigMapSelector(configMap, sel, defaultKey)
}

func LookupConfigMapSelector(configMap *corev1.ConfigMap, sel matryoshkav1alpha1.ConfigMapSelector, defaultKey string) (string, error) {
	key := sel.Key
	if key == "" {
		key = defaultKey
	}

	v, ok := configMap.Data[key]
	if !ok {
		return "", fmt.Errorf("config map has no data at key %s", key)
	}

	return v, nil
}
