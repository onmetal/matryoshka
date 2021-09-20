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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var KubeconfigFieldManager = fmt.Sprintf("%s/Kubeconfig", GroupVersion.Group)

const DefaultKubeconfigKey = "kubeconfig"

// KubeconfigSpec defines the desired state of Kubeconfig
type KubeconfigSpec struct {
	SecretName string `json:"secretName"`

	Clusters       []KubeconfigNamedCluster  `json:"clusters"`
	AuthInfos      []KubeconfigNamedAuthInfo `json:"users"`
	Contexts       []KubeconfigNamedContext  `json:"contexts"`
	CurrentContext string                    `json:"currentContext"`
}

type KubeconfigNamedCluster struct {
	Name    string            `json:"name"`
	Cluster KubeconfigCluster `json:"cluster"`
}

type KubeconfigCluster struct {
	Server                string                                 `json:"server"`
	TLSServerName         string                                 `json:"tlsServerName,omitempty"`
	InsecureSkipTLSVerify bool                                   `json:"insecureSkipTLSVerify,omitempty"`
	CertificateAuthority  *KubeconfigClusterCertificateAuthority `json:"certificateAuthority,omitempty"`
	ProxyURL              string                                 `json:"proxyURL,omitempty"`
}

const DefaultKubeconfigClusterCertificateAuthorityKey = "ca.crt"

type KubeconfigClusterCertificateAuthority struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

type KubeconfigNamedAuthInfo struct {
	Name     string             `json:"name"`
	AuthInfo KubeconfigAuthInfo `json:"user"`
}

type KubeconfigAuthInfo struct {
	ClientCertificate *KubeconfigAuthInfoClientCertificate `json:"clientCertificate,omitempty"`
	ClientKey         *KubeconfigAuthInfoClientKey         `json:"clientKey,omitempty"`
	Token             *KubeconfigAuthInfoToken             `json:"token,omitempty"`
	Impersonate       string                               `json:"as,omitempty"`
	ImpersonateGroups []string                             `json:"asGroups,omitempty"`
	Username          string                               `json:"username,omitempty"`
	Password          *KubeconfigAuthInfoPassword          `json:"password,omitempty"`
}

const DefaultKubeconfigAuthInfoClientKeyKey = "tls.key"

type KubeconfigAuthInfoClientKey struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

const DefaultKubeconfigAuthInfoClientCertificateKey = "tls.crt"

type KubeconfigAuthInfoClientCertificate struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

const DefaultKubeconfigAuthInfoTokenKey = "token"

type KubeconfigAuthInfoToken struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

const DefaultKubeconfigAuthInfoPasswordKey = "password"

type KubeconfigAuthInfoPassword struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

type KubeconfigNamedContext struct {
	Name    string            `json:"name"`
	Context KubeconfigContext `json:"context"`
}

type KubeconfigContext struct {
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
