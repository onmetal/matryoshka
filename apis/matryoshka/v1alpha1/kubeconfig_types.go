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
	SecretName string `json:"secretName"`

	Clusters       []KubeconfigNamedCluster  `json:"clusters"`
	AuthInfos      []KubeconfigNamedAuthInfo `json:"users"`
	Contexts       []KubeconfigNamedContext  `json:"contexts"`
	CurrentContext string                    `json:"currentContext"`
}

// KubeconfigNamedCluster is a named KubeconfigCluster.
type KubeconfigNamedCluster struct {
	Name    string            `json:"name"`
	Cluster KubeconfigCluster `json:"cluster"`
}

// KubeconfigCluster specifies settings on how to connect to a cluster.
type KubeconfigCluster struct {
	// Server is the server's address.
	Server string `json:"server"`
	// TLSServer name is the server's TLS name.
	TLSServerName string `json:"tlsServerName,omitempty"`
	// InsecureSkipTLSVerify may be used to disable https ca certificate validation.
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
	// CertificateAuthority is the ca to use for connecting to the server.
	CertificateAuthority *KubeconfigClusterCertificateAuthority `json:"certificateAuthority,omitempty"`
	// ProxyURL is the proxy url to use to connect to the server.
	ProxyURL string `json:"proxyURL,omitempty"`
}

// DefaultKubeconfigClusterCertificateAuthorityKey is the default key to look up for secrets referenced by
// KubeconfigClusterCertificateAuthority.
const DefaultKubeconfigClusterCertificateAuthorityKey = "ca.crt"

// KubeconfigClusterCertificateAuthority references a secret that contains the kubeconfig cluster ca.
// If key is unset, DefaultKubeconfigClusterCertificateAuthorityKey will be used as default.
type KubeconfigClusterCertificateAuthority struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

// KubeconfigNamedAuthInfo is a named KubeconfigAuthInfo.
type KubeconfigNamedAuthInfo struct {
	Name     string             `json:"name"`
	AuthInfo KubeconfigAuthInfo `json:"user"`
}

// KubeconfigAuthInfo is information how to authenticate to the cluster.
type KubeconfigAuthInfo struct {
	// ClientCertificate references a client certificate to present to the server.
	ClientCertificate *KubeconfigAuthInfoClientCertificate `json:"clientCertificate,omitempty"`
	// ClientKey references a client key to present to the server.
	ClientKey *KubeconfigAuthInfoClientKey `json:"clientKey,omitempty"`
	// Token references a token to present to the server.
	Token *KubeconfigAuthInfoToken `json:"token,omitempty"`
	// Impersonate sets the user to impersonate.
	Impersonate string `json:"as,omitempty"`
	// ImpersonateGroups sets the groups to impersonate.
	ImpersonateGroups []string `json:"asGroups,omitempty"`
	// Username sets the username to use.
	Username string `json:"username,omitempty"`
	// Password references a password.
	Password *KubeconfigAuthInfoPassword `json:"password,omitempty"`
}

// DefaultKubeconfigAuthInfoClientKeyKey is the default key for KubeconfigAuthInfoClientKey.
const DefaultKubeconfigAuthInfoClientKeyKey = "tls.key"

// KubeconfigAuthInfoClientKey references an entity containing the client key used for authentication.
// If key is unset, DefaultKubeconfigAuthInfoClientKeyKey will be used as default.
type KubeconfigAuthInfoClientKey struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

// DefaultKubeconfigAuthInfoClientCertificateKey is the default key to use for KubeconfigAuthInfoClientCertificate.
const DefaultKubeconfigAuthInfoClientCertificateKey = "tls.crt"

// KubeconfigAuthInfoClientCertificate references an entity containing the client certificate used for authentication.
// If key is unset, DefaultKubeconfigAuthInfoClientCertificateKey will be used as default.
type KubeconfigAuthInfoClientCertificate struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

// DefaultKubeconfigAuthInfoTokenKey is the default key for KubeconfigAuthInfoToken.
const DefaultKubeconfigAuthInfoTokenKey = "token"

// KubeconfigAuthInfoToken references an entity containing a token used for authentication.
// If key is unset, DefaultKubeconfigAuthInfoTokenKey will be used as default.
type KubeconfigAuthInfoToken struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

// DefaultKubeconfigAuthInfoPasswordKey is the default key for KubeconfigAuthInfoPassword.
const DefaultKubeconfigAuthInfoPasswordKey = "password"

// KubeconfigAuthInfoPassword references an entity containing a password used for authentication.
// If key is unset, DefaultKubeconfigAuthInfoPasswordKey will be used as default.
type KubeconfigAuthInfoPassword struct {
	Secret *SecretSelector `json:"secret,omitempty"`
}

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
