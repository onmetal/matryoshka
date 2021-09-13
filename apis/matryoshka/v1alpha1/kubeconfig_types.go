/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var KubeconfigFieldManager = fmt.Sprintf("%s/Kubeconfig", GroupVersion.Group)

const DefaultKubeconfigKey = "kubeconfig"

// KubeconfigSpec defines the desired state of Kubeconfig
type KubeconfigSpec struct {
	SecretName string `json:"secretName"`

	Clusters       []NamedCluster  `json:"clusters"`
	AuthInfos      []NamedAuthInfo `json:"users"`
	Contexts       []NamedContext  `json:"contexts"`
	CurrentContext string          `json:"currentContext"`
}

type NamedCluster struct {
	Name    string  `json:"name"`
	Cluster Cluster `json:"cluster"`
}

type Cluster struct {
	Server                string                       `json:"server"`
	TLSServerName         string                       `json:"tlsServerName,omitempty"`
	InsecureSkipTLSVerify bool                         `json:"insecureSkipTLSVerify,omitempty"`
	CertificateAuthority  *ClusterCertificateAuthority `json:"certificateAuthority,omitempty"`
	ProxyURL              string                       `json:"proxyURL,omitempty"`
}

const DefaultClusterCertificateAuthorityKey = "ca.crt"

type ClusterCertificateAuthority struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

type ConfigMapSelector struct {
	corev1.LocalObjectReference `json:",inline"`
	Key                         string `json:"key,omitempty"`
}

type SecretSelector struct {
	corev1.LocalObjectReference `json:",inline"`
	Key                         string `json:"key,omitempty"`
}

type NamedAuthInfo struct {
	Name     string   `json:"name"`
	AuthInfo AuthInfo `json:"user"`
}

type AuthInfo struct {
	ClientCertificate *AuthInfoClientCertificate `json:"clientCertificate,omitempty"`
	ClientKey         *AuthInfoClientKey         `json:"clientKey,omitempty"`
	Token             *AuthInfoToken             `json:"token,omitempty"`
	Impersonate       string                     `json:"as,omitempty"`
	ImpersonateGroups []string                   `json:"asGroups,omitempty"`
	Username          string                     `json:"username,omitempty"`
	Password          *AuthInfoPassword          `json:"password,omitempty"`
}

const DefaultAuthInfoClientKeyKey = "tls.key"

type AuthInfoClientKey struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

const DefaultAuthInfoClientCertificateKey = "tls.crt"

type AuthInfoClientCertificate struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

const DefaultAuthInfoTokenKey = "token"

type AuthInfoToken struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

const DefaultAuthInfoPasswordKey = "password"

type AuthInfoPassword struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

type NamedContext struct {
	Name    string  `json:"name"`
	Context Context `json:"context"`
}

type Context struct {
	Cluster   string `json:"cluster"`
	AuthInfo  string `json:"user"`
	Namespace string `json:"namespace,omitempty"`
}

// KubeconfigStatus defines the observed state of Kubeconfig
type KubeconfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Kubeconfig is the Schema for the kubeconfigs API
type Kubeconfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeconfigSpec   `json:"spec,omitempty"`
	Status KubeconfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeconfigList contains a list of Kubeconfig
type KubeconfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kubeconfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kubeconfig{}, &KubeconfigList{})
}
