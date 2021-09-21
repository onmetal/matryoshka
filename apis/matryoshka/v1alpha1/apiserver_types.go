/*
 * Copyright (c) 2021 by the OnMetal authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// APIServerFieldManager specifies the field manager used for operations on an APIServer.
var APIServerFieldManager = fmt.Sprintf("%s/APIServer", GroupVersion.Group)

// APIServerSpec defines the desired state of APIServer
type APIServerSpec struct {
	// Replicas specifies the desired amount of replicas for the api server deployment.
	//+kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
	// Version is the api server version to use.
	//+kubebuilder:validation:Pattern=^[0-9]+\.[0-9]+\.[0-9]+$
	Version string `json:"version"`
	// Resources specifies the resources the api server container requires.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Labels sets labels on resources managed by this.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations sets annotations on resources managed by this.
	Annotations map[string]string `json:"annotations,omitempty"`
	// ETCD specifies etcd configuration for the api server to use.
	ETCD APIServerETCD `json:"etcd"`
	// Authentication specifies how users can authenticate to the api server.
	Authentication APIServerAuthentication `json:"authentication"`
	// TLS optionally defines how to secure the api server.
	TLS *APIServerTLS `json:"tls,omitempty"`
	// ServiceAccount are service account settings for the api server.
	ServiceAccount APIServerServiceAccount `json:"serviceAccount"`
}

// APIServerServiceAccount is the specification how service accounts
// are issued / signed.
type APIServerServiceAccount struct {
	// Issuer is the service account issuer.
	Issuer string `json:"issuer"`
	// Secret references a secret containing a key 'tls.key' that contains the
	// key to sign and verify service accounts with.
	Secret corev1.LocalObjectReference `json:"secret"`
}

// APIServerETCD contains settings on how the api server connects to an etcd.
type APIServerETCD struct {
	// Servers is the list of etcd servers for the api server to connect to.
	//+kubebuilder:validation:MinItems=1
	Servers []string `json:"servers"`
	// CertificateAuthority is an optional specification of the certificate authority to use
	// when connecting to the etcd.
	CertificateAuthority *APIServerETCDCertificateAuthority `json:"certificateAuthority,omitempty"`
	// Key is an optional specification of the key to use when connecting to the etcd.
	Key *APIServerETCDKey `json:"key,omitempty"`
}

// DefaultAPIServerETCDCertificateAuthorityKey is the default key that will be used to look up the certificate
// authority in a secret referenced by APIServerETCDCertificateAuthority.
const DefaultAPIServerETCDCertificateAuthorityKey = "ca.crt"

// APIServerETCDCertificateAuthority specifies how to obtain the certificate authority to use when connecting to etcd.
type APIServerETCDCertificateAuthority struct {
	// Secret is a SecretSelector specifying where to retrieve the ca certificate.
	// If key is left blank, DefaultAPIServerETCDCertificateAuthorityKey is used.
	Secret SecretSelector `json:"secret"`
}

// APIServerETCDKey specifies how to obtain the etcd key used when connecting to etcd.
type APIServerETCDKey struct {
	// Secret references a secret containing the etcd key under 'tls.key'.
	Secret corev1.LocalObjectReference `json:"secret"`
}

// APIServerAuthentication specifies how users may authenticate to the api server.
type APIServerAuthentication struct {
	// BootstrapToken specifies whether bootstrap token authentication is enabled.
	BootstrapToken *APIServerBootstrapTokenAuthentication `json:"bootstrapToken,omitempty"`
	// Anonymous specifies whether anonymous authentication is enabled.
	Anonymous *APIServerAnonymousAuthentication `json:"anonymous,omitempty"`
	// Token specifies whether token authentication is enabled and where these tokens are located at.
	Token *APIServerTokenAuthentication `json:"token,omitempty"`
}

// APIServerBootstrapTokenAuthentication specifies how api server bootstrap token authentication shall happen.
type APIServerBootstrapTokenAuthentication struct {
}

// APIServerAnonymousAuthentication specifies how anonymous authentication shall happen.
type APIServerAnonymousAuthentication struct {
}

// DefaultAPIServerTokenAuthenticationKey is the default key to look up for tokens when the key in
// APIServerTokenAuthentication is blank.
const DefaultAPIServerTokenAuthenticationKey = "token.csv"

// APIServerTokenAuthentication specifies how to retrieve the tokens used for api server token authentication.
type APIServerTokenAuthentication struct {
	// Secret specifies a secret containing the tokens.
	// If key is left blank, DefaultAPIServerTokenAuthenticationKey is used as default key.
	Secret SecretSelector `json:"secret"`
}

// APIServerTLS specifies where tls configuration for the api server is found.
type APIServerTLS struct {
	// Secret references a secret containing 'tls.crt' and 'tls.key' to use
	// for TLS-securing the API server.
	Secret corev1.LocalObjectReference `json:"secret"`
}

// APIServerStatus defines the observed state of APIServer
type APIServerStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	Replicas            int32              `json:"replicas,omitempty"`
	UpdatedReplicas     int32              `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32              `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32              `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32              `json:"unavailableReplicas,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

// APIServerConditionType are types of APIServerCondition.
type APIServerConditionType string

const (
	// APIServerAvailable reports whether the api server is available,
	// meaning the required number of replicas has met the health checks for a certain amount of time.
	APIServerAvailable APIServerConditionType = "Available"
	// APIServerProgressing reports whether the update of an api server
	// deployment is progressing as expected.
	APIServerProgressing APIServerConditionType = "Progressing"
	// APIServerDeploymentFailure indicates any error that might have occurred when
	// creating the deployment of an api server.
	APIServerDeploymentFailure APIServerConditionType = "DeploymentFailure"
)

// APIServerCondition reports individual conditions of an APIServer.
type APIServerCondition struct {
	Type               APIServerConditionType `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	ObservedGeneration int64                  `json:"observedGeneration,omitempty"`
	LastUpdateTime     metav1.Time            `json:"lastUpdateTime"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime"`
	Reason             string                 `json:"reason"`
	Message            string                 `json:"message"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.readyReplicas`,description="number of ready replicas"
//+kubebuilder:printcolumn:name="Up-To-Date",type="number",JSONPath=`.status.updatedReplicas`,description="number of updated replicas"
//+kubebuilder:printcolumn:name="Available",type="number",JSONPath=`.status.availableReplicas`,description="number of available replicas"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// APIServer is the Schema for the apiservers API
type APIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIServerSpec   `json:"spec,omitempty"`
	Status APIServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// APIServerList contains a list of APIServer
type APIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APIServer{}, &APIServerList{})
}
