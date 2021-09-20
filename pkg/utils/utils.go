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
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"io"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sort"
	"strings"
)

func IgnoreAlreadyExists(err error) error {
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

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

func ConvertAndSetList(scheme *runtime.Scheme, list runtime.Object, objs []client.Object) error {
	elemType, err := ListElementType(list)
	if err != nil {
		return err
	}

	var converted []runtime.Object
	for _, obj := range objs {
		into := reflect.New(elemType).Interface()
		if err := scheme.Convert(obj, into, nil); err != nil {
			return err
		}

		converted = append(converted, into.(runtime.Object))
	}
	return meta.SetList(list, converted)
}

func GVKForList(scheme *runtime.Scheme, list runtime.Object) (schema.GroupVersionKind, error) {
	gvk, err := apiutil.GVKForObject(list, scheme)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	if strings.HasSuffix(gvk.Kind, "List") {
		// if this was a list, treat it as a request for the item's resource
		gvk.Kind = gvk.Kind[:len(gvk.Kind)-4]
	}

	return gvk, nil
}

func ListElementType(list runtime.Object) (reflect.Type, error) {
	itemsPtr, err := meta.GetItemsPtr(list)
	if err != nil {
		return nil, err
	}

	v := reflect.ValueOf(itemsPtr)
	return v.Type().Elem().Elem(), nil
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

func ReadObjectsFile(filename string) ([]unstructured.Unstructured, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		utilruntime.HandleError(f.Close())
	}()

	rd := yaml.NewYAMLReader(bufio.NewReader(f))
	var objs []unstructured.Unstructured
	for {
		data, err := rd.Read()
		if err != nil {
			if !errors.Is(io.EOF, err) {
				return nil, err
			}
			return objs, nil
		}

		if strings.TrimSpace(string(data)) == "" {
			continue
		}

		obj := &unstructured.Unstructured{}
		if _, _, err := scheme.Codecs.UniversalDeserializer().Decode(data, nil, obj); err != nil {
			return nil, fmt.Errorf("invalid object: %w", err)
		}

		objs = append(objs, *obj)
	}
}

func ApplyFile(ctx context.Context, c client.Client, filename string, namespace, fieldOwner string) ([]unstructured.Unstructured, error) {
	objs, err := ReadObjectsFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	for i := range objs {
		obj := &objs[i]
		obj.SetNamespace(namespace)
		if err := c.Patch(ctx, obj, client.Apply, client.FieldOwner(fieldOwner)); err != nil {
			return nil, err
		}
	}
	return objs, nil
}

func DeleteFile(ctx context.Context, c client.Client, filename string, namespace string) error {
	objs, err := ReadObjectsFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	for i := range objs {
		obj := &objs[i]
		obj.SetNamespace(namespace)
		if err := c.Delete(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}
