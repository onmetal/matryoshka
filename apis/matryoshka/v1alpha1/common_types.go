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

package v1alpha1

import corev1 "k8s.io/api/core/v1"

// ConfigMapSelector selects a config map by name and a field in its data property by the key.
type ConfigMapSelector struct {
	corev1.LocalObjectReference `json:",inline"`
	// Key is the key to look up in the config map data. Some types use a default if this value is unset.
	Key string `json:"key,omitempty"`
}

// SecretSelector selects a secret by name and a field in its data property by the key.
type SecretSelector struct {
	corev1.LocalObjectReference `json:",inline"`
	// Key is the key to look up in the config map data. Some types use a default if this value is unset.
	Key string `json:"key,omitempty"`
}
