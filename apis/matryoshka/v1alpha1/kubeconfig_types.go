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

// KubeconfigFieldManager is the field manager used by the kubeconfig controller.
var KubeconfigFieldManager = fmt.Sprintf("%s/Kubeconfig", GroupVersion.Group)

// DefaultKubeconfigKey is the default key to write the kubeconfig into a secret's data.
const DefaultKubeconfigKey = "kubeconfig"

// KubeconfigSpec defines the desired state of Kubeconfig
type KubeconfigSpec struct {
	// SecretName is the name of the secret to create for the Kubeconfig.
	SecretName string `json:"secretName"`

	// KubeconfigKey is the Secret.Data key to write the kubeconfig into.
	// If left empty, DefaultKubeconfigKey will be used.
	KubeconfigKey string `json:"kubeconfigKey,omitempty"`

	Clusters       []KubeconfigNamedCluster  `json:"clusters"`
	AuthInfos      []KubeconfigNamedAuthInfo `json:"users"`
	Contexts       []KubeconfigNamedContext  `json:"contexts"`
	CurrentContext string                    `json:"currentContext"`
}

// KubeconfigNamedCluster is a named KubeconfigCluster.
type KubeconfigNamedCluster struct {
	// Name is the name of the KubeconfigCluster.
	Name string `json:"name"`
	// Cluster is a KubeconfigCluster.
	Cluster KubeconfigCluster `json:"cluster"`
}

// KubeconfigCluster specifies settings on how to connect to a cluster.
type KubeconfigCluster struct {
	// Server is the server's address.
	Server string `json:"server"`
	// TLSServerName name is the server's TLS name.
	TLSServerName string `json:"tlsServerName,omitempty"`
	// InsecureSkipTLSVerify may be used to disable https ca certificate validation.
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
	// CertificateAuthoritySecret is the ca to use for connecting to the server.
	CertificateAuthoritySecret *SecretSelector `json:"certificateAuthoritySecret,omitempty"`
	// ProxyURL is the proxy url to use to connect to the server.
	ProxyURL string `json:"proxyURL,omitempty"`
}

// DefaultKubeconfigClusterCertificateAuthoritySecretKey is the default key to look up for secrets referenced by
// KubeconfigCluster.CertificateAuthoritySecret.
const DefaultKubeconfigClusterCertificateAuthoritySecretKey = "ca.crt"

// KubeconfigNamedAuthInfo is a named KubeconfigAuthInfo.
type KubeconfigNamedAuthInfo struct {
	// Name is the name of the KubeconfigAuthInfo.
	Name string `json:"name"`
	// AuthInfo is a KubeconfigAuthInfo.
	AuthInfo KubeconfigAuthInfo `json:"user"`
}

// KubeconfigAuthInfo is information how to authenticate to the cluster.
type KubeconfigAuthInfo struct {
	// ClientCertificateSecret references a client certificate to present to the server.
	ClientCertificateSecret *SecretSelector `json:"clientCertificateSecret,omitempty"`
	// ClientKeySecret references a client key to present to the server.
	ClientKeySecret *SecretSelector `json:"clientKeySecret,omitempty"`
	// TokenSecret references a token to present to the server.
	TokenSecret *SecretSelector `json:"tokenSecret,omitempty"`
	// Impersonate sets the user to impersonate.
	Impersonate string `json:"as,omitempty"`
	// ImpersonateGroups sets the groups to impersonate.
	ImpersonateGroups []string `json:"asGroups,omitempty"`
	// Username sets the username to use.
	Username string `json:"username,omitempty"`
	// Password references a password.
	PasswordSecret *SecretSelector `json:"passwordSecret,omitempty"`
}

// DefaultKubeconfigAuthInfoClientKeySecretKey is the default key for KubeconfigAuthInfo.ClientKeySecret.
const DefaultKubeconfigAuthInfoClientKeySecretKey = "tls.key"

// DefaultKubeconfigAuthInfoClientCertificateSecretKey is the default key to use for
// KubeconfigAuthInfo.ClientCertificateSecret.
const DefaultKubeconfigAuthInfoClientCertificateSecretKey = "tls.crt"

// DefaultKubeconfigAuthInfoTokenSecretKey is the default key for KubeconfigAuthInfo.TokenSecret.
const DefaultKubeconfigAuthInfoTokenSecretKey = "token"

// DefaultKubeconfigAuthInfoPasswordSecretKey is the default key for KubeconfigAuthInfo.PasswordSecret.
const DefaultKubeconfigAuthInfoPasswordSecretKey = "password"

// KubeconfigNamedContext is a named KubeconfigContext.
type KubeconfigNamedContext struct {
	Name    string            `json:"name"`
	Context KubeconfigContext `json:"context"`
}

// KubeconfigContext bundles together a cluster, auth info and optional namespace to use.
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
