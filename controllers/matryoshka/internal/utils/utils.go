// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"

	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecretSelector gets the secret referenced by the namespace and matryoshkav1alpha1.SecretSelector and
// uses LookupSecretSelector to retrieve the specified key. If key is empty, defaultKey is used.
func GetSecretSelector(ctx context.Context, c client.Client, namespace string, sel matryoshkav1alpha1.SecretSelector, defaultKey string) ([]byte, error) {
	clientKey := client.ObjectKey{Namespace: namespace, Name: sel.Name}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, clientKey, secret); err != nil {
		return nil, err
	}

	return LookupSecretSelector(secret, sel, defaultKey)
}

// LookupSecretSelector looks up the key referenced by the matryoshkav1alpha1.SecretSelector or the defaultKey if
// key is empty.
func LookupSecretSelector(secret *corev1.Secret, sel matryoshkav1alpha1.SecretSelector, defaultKey string) ([]byte, error) {
	key := StringOrDefault(sel.Key, defaultKey)

	v, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("secret has no data at key %s", key)
	}

	return v, nil
}

// GetConfigMapSelector gets the config map referenced by the namespace and matryoshkav1alpha1.ConfigMapSelector and
// uses LookupConfigMapSelector to retrieve the specified key. If key is empty, defaultKey is used.
func GetConfigMapSelector(ctx context.Context, c client.Client, namespace string, sel matryoshkav1alpha1.ConfigMapSelector, defaultKey string) (string, error) {
	clientKey := client.ObjectKey{Namespace: namespace, Name: sel.Name}
	configMap := &corev1.ConfigMap{}
	if err := c.Get(ctx, clientKey, configMap); err != nil {
		return "", err
	}

	return LookupConfigMapSelector(configMap, sel, defaultKey)
}

// LookupConfigMapSelector looks up the key referenced by the matryoshkav1alpha1.ConfigMapSelector or the defaultKey if
// key is empty.
func LookupConfigMapSelector(configMap *corev1.ConfigMap, sel matryoshkav1alpha1.ConfigMapSelector, defaultKey string) (string, error) {
	key := StringOrDefault(sel.Key, defaultKey)

	v, ok := configMap.Data[key]
	if !ok {
		return "", fmt.Errorf("config map has no data at key %s", key)
	}

	return v, nil
}

// StringOrDefault returns the given string if it's non-empty, otherwise it returns the defaultValue.
func StringOrDefault(s string, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

func mkChecksumKey(kind string, obj client.Object) string {
	return fmt.Sprintf("checksum.%s/%s", kind, obj.GetName())
}

func secretDataChecksum(data map[string][]byte) string {
	h := sha256.New()
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		h.Write([]byte(fmt.Sprintf("%s=%s", key, string(data[key]))))
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func configMapDataChecksum(data map[string]string) string {
	h := sha256.New()
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		h.Write([]byte(fmt.Sprintf("%s=%s", key, data[key])))
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ComputeMountableChecksum computes checksums for secrets and config maps by applying a sha256sum to the ordered items.
func ComputeMountableChecksum(secrets []corev1.Secret, configMaps []corev1.ConfigMap) (map[string]string, error) {
	checksums := make(map[string]string, len(secrets)+len(configMaps))
	for _, secret := range secrets {
		checksums[mkChecksumKey("secret", &secret)] = secretDataChecksum(secret.Data)
	}
	for _, config := range configMaps {
		checksums[mkChecksumKey("config", &config)] = configMapDataChecksum(config.Data)
	}
	return checksums, nil
}

// MergeStringStringMaps returns a new map with the key-value pairs of all given maps merged.
// If all maps are nil, nil is returned.
// If a map is non-nil, a non-nil map is returned.
// The key-value pairs of the last map have the highest priority.
func MergeStringStringMaps(ms ...map[string]string) map[string]string {
	var res map[string]string
	for _, m := range ms {
		if m != nil && res == nil {
			res = make(map[string]string)
		}
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}
