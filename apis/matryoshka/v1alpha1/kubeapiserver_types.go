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

// TODO: See https://github.com/kubernetes/kubernetes/blob/686379281d9a4207d3c3effe9dc43b5cef29108d/cmd/kube-apiserver/app/options/options.go#L47
// for kubernetes built-in option grouping and adapt types.

// KubeAPIServerFieldManager specifies the field manager used for operations on an KubeAPIServer.
var KubeAPIServerFieldManager = fmt.Sprintf("%s/KubeAPIServer", GroupVersion.Group)

// KubeAPIServerSpec defines the desired state of KubeAPIServer
type KubeAPIServerSpec struct {
	// Replicas specifies the desired amount of replicas for the api server deployment.
	// This is a pointer to distinguish between not specified and explicit zero.
	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:default=1
	Replicas *int32 `json:"replicas"`
	// Version is the api server version to use.
	Version string `json:"version"`
	// Selector specifies the label selector to discover managed pods.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// Overlay is the KubeAPIServerPodTemplateOverlay to use to scaffold the deployment.
	Overlay KubeAPIServerPodTemplateOverlay `json:"overlay,omitempty"`
	// ETCD specifies the KubeAPIServerETCD connection settings.
	ETCD KubeAPIServerETCD `json:"etcd"`
	// Authentication specifies how users can authenticate to the api server.
	Authentication KubeAPIServerAuthentication `json:"authentication"`
	// SecureServing optionally defines how to secure the api server.
	SecureServing *KubeAPIServerSecureServing `json:"secureServing,omitempty"`
	// ServiceAccount are service account settings for the api server.
	ServiceAccount KubeAPIServerServiceAccount `json:"serviceAccount"`
	// FeatureGates describe which alpha features should be enabled or beta features disabled
	FeatureGates map[string]bool `json:"featureGates"`
}

// KubeAPIServerPodTemplateOverlay is the template overlay for pods.
type KubeAPIServerPodTemplateOverlay struct {
	// ObjectMeta specifies additional object metadata to set on the managed pods.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the KubeAPIServerPodOverlay overlay specification for the pod.
	Spec KubeAPIServerPodOverlay `json:"spec,omitempty"`
}

// KubeAPIServerPodOverlay is the PodOverlay with additional ContainerOverlay containers.
type KubeAPIServerPodOverlay struct {
	// PodOverlay is the base managed pod specification.
	PodOverlay `json:",inline,omitempty"`
	// APIServerContainer is the ContainerOverlay that hosts the api server.
	APIServerContainer ContainerOverlay `json:"apiServerContainer,omitempty"`
}

// KubeAPIServerServiceAccount is the specification how service accounts
// are issued / signed.
type KubeAPIServerServiceAccount struct {
	// Issuer is the service account issuer.
	Issuer string `json:"issuer"`
	// KeySecret references a secret containing a key 'tls.key' that contains the
	// key to sign and verify service accounts with.
	KeySecret SecretSelector `json:"keySecret"`
	// SigningKeySecret references a secret containing a key 'tls.key' that contains the
	// key to sign and verify service accounts with.
	SigningKeySecret SecretSelector `json:"signingKeySecret"`
}

// KubeAPIServerETCD contains settings on how the api server connects to an etcd.
type KubeAPIServerETCD struct {
	// Servers is the list of etcd servers for the api server to connect to.
	//+kubebuilder:validation:MinItems=1
	Servers []string `json:"servers"`
	// CertificateAuthoritySecret is an optional specification of the certificate authority to use
	// when connecting to the etcd.
	CertificateAuthoritySecret *SecretSelector `json:"certificateAuthoritySecret,omitempty"`
	// KeySecret is an optional specification of the key to use when connecting to the etcd.
	KeySecret *SecretSelector `json:"keySecret,omitempty"`
}

// DefaultKubeAPIServerETCDCertificateAuthoritySecretKey is the default key that will be used to look up the certificate
// authority in a secret referenced by KubeAPIServerETCD.CertificateAuthoritySecret.
const DefaultKubeAPIServerETCDCertificateAuthoritySecretKey = "ca.crt"

// KubeAPIServerETCDCertificateAuthority specifies how to obtain the certificate authority to use when connecting to etcd.
type KubeAPIServerETCDCertificateAuthority struct {
	// Secret is a SecretSelector specifying where to retrieve the ca certificate.
	// If key is left blank, DefaultKubeAPIServerETCDCertificateAuthoritySecretKey is used.
	Secret SecretSelector `json:"secret"`
}

// KubeAPIServerETCDKey specifies how to obtain the etcd key used when connecting to etcd.
type KubeAPIServerETCDKey struct {
	// Secret references a secret containing the etcd key under 'tls.key'.
	Secret corev1.LocalObjectReference `json:"secret"`
}

const (
	// DefaultKubeAPIServerAuthenticationTokenSecretKey is the default key to look up for tokens when the key in
	// KubeAPIServerAuthentication is blank.
	DefaultKubeAPIServerAuthenticationTokenSecretKey = "token.csv"
	// DefaultKubeAPIServerAuthenticationClientCertificateSecretKey is the default key to look up for client
	// certificates when the key in KubeAPIServerAuthentication.ClientCertificateSecret is blank.
	DefaultKubeAPIServerAuthenticationClientCertificateSecretKey = "ca.crt"
)

// KubeAPIServerAuthentication specifies how users may authenticate to the api server.
type KubeAPIServerAuthentication struct {
	// BootstrapToken specifies whether bootstrap token authentication is enabled.
	BootstrapToken bool `json:"bootstrapToken,omitempty"`
	// Anonymous specifies whether anonymous authentication is enabled.
	Anonymous bool `json:"anonymous,omitempty"`
	// TokenSecret specifies whether token authentication is enabled and where these tokens are located at.
	TokenSecret *SecretSelector `json:"tokenSecret,omitempty"`
	// ClientCertificateSecret makes any request presenting a client certificate signed by one of the authorities in
	// the client-ca-file to be authenticated with an identity corresponding to the CommonName of the
	// client certificate.
	ClientCertificateSecret *SecretSelector `json:"clientCertificateSecret,omitempty"`
}

// KubeAPIServerSecureServing specifies where tls configuration for the api server is found.
type KubeAPIServerSecureServing struct {
	// Secret references a secret containing 'tls.crt' and 'tls.key' to use
	// for TLS-securing the API server.
	Secret corev1.LocalObjectReference `json:"secret"`
}

// KubeAPIServerStatus defines the observed state of KubeAPIServer
type KubeAPIServerStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	Replicas            int32              `json:"replicas,omitempty"`
	UpdatedReplicas     int32              `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32              `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32              `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32              `json:"unavailableReplicas,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

// KubeAPIServerConditionType are types of KubeAPIServerCondition.
type KubeAPIServerConditionType string

const (
	// KubeAPIServerAvailable reports whether the api server is available,
	// meaning the required number of replicas has met the health checks for a certain amount of time.
	KubeAPIServerAvailable KubeAPIServerConditionType = "Available"
	// KubeAPIServerProgressing reports whether the update of an api server
	// deployment is progressing as expected.
	KubeAPIServerProgressing KubeAPIServerConditionType = "Progressing"
	// KubeAPIServerDeploymentFailure indicates any error that might have occurred when
	// creating the deployment of an api server.
	KubeAPIServerDeploymentFailure KubeAPIServerConditionType = "DeploymentFailure"
)

// KubeAPIServerCondition reports individual conditions of an KubeAPIServer.
type KubeAPIServerCondition struct {
	Type               KubeAPIServerConditionType `json:"type"`
	Status             corev1.ConditionStatus     `json:"status"`
	ObservedGeneration int64                      `json:"observedGeneration,omitempty"`
	LastUpdateTime     metav1.Time                `json:"lastUpdateTime"`
	LastTransitionTime metav1.Time                `json:"lastTransitionTime"`
	Reason             string                     `json:"reason"`
	Message            string                     `json:"message"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.readyReplicas`,description="number of ready replicas"
//+kubebuilder:printcolumn:name="Up-To-Date",type="number",JSONPath=`.status.updatedReplicas`,description="number of updated replicas"
//+kubebuilder:printcolumn:name="Available",type="number",JSONPath=`.status.availableReplicas`,description="number of available replicas"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// KubeAPIServer is the Schema for the KubeAPIServers API
type KubeAPIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeAPIServerSpec   `json:"spec,omitempty"`
	Status KubeAPIServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeAPIServerList contains a list of KubeAPIServer
type KubeAPIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeAPIServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeAPIServer{}, &KubeAPIServerList{})
}
