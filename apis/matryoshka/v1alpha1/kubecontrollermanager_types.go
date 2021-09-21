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

// KubeControllerManagerFieldManager specifies the field manager used for operations on a KubeControllerManager.
var KubeControllerManagerFieldManager = fmt.Sprintf("%s/KubeControllerManager", GroupVersion.Group)

// KubeControllerManagerSpec defines the desired state of KubeControllerManager
type KubeControllerManagerSpec struct {
	// Replicas specifies the number of kube controller manager replicas to create.
	//+kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
	// Version is the kube controller manager version to use.
	//+kubebuilder:validation:Pattern=^[0-9]+\.[0-9]+\.[0-9]+$
	Version string `json:"version"`
	// Resources specifies the resources the kube controller manager container requires.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Labels sets labels on resources managed by this.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations sets annotations on resources managed by this.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Cluster specifies cluster specific settings.
	Cluster KubeControllerManagerCluster `json:"cluster"`
	// Controllers are controller settings for the kube controller manager.
	// +optional
	Controllers KubeControllerManagerControllers `json:"controllers,omitempty"`
	// KubernetesAPI are settings on how to interface with the Kubernetes API.
	KubernetesAPI KubeControllerManagerKubernetesAPI `json:"kubernetesAPI"`
	// ServiceAccount specifies how the kube controller manager should handle service accounts.
	ServiceAccount KubeControllerManagerServiceAccount `json:"serviceAccount"`
}

// DefaultKubeControllerManagerKubeconfigKey is the default key to lookup KubeControllerManagerKubeconfig kubeconfigs.
const DefaultKubeControllerManagerKubeconfigKey = "kubeconfig"

// KubeControllerManagerKubeconfig specifies the kubeconfig for the kube controller manager to use.
type KubeControllerManagerKubeconfig struct {
	// Secret references the secret for the kube controller manager to lookup. If key is unset,
	// DefaultKubeControllerManagerKubeconfigKey is used.
	Secret SecretSelector `json:"secret"`
}

// DefaultKubeControllerManagerAuthorizationKubeconfigKey is the default key to lookup
// KubeControllerManagerAuthorizationKubeconfig kubeconfigs.
const DefaultKubeControllerManagerAuthorizationKubeconfigKey = "kubeconfig"

// KubeControllerManagerAuthorizationKubeconfig specifies the authorization kubeconfig for the kube controller manager
// to use.
type KubeControllerManagerAuthorizationKubeconfig struct {
	// Secret references the secret for the kube controller manager to lookup. If key is unset,
	// DefaultKubeControllerManagerAuthorizationKubeconfigKey is used.
	Secret SecretSelector `json:"secret"`
}

// DefaultKubeControllerManagerAuthenticationKubeconfigKey is the default key to lookup
// KubeControllerManagerAuthenticationKubeconfig kubeconfigs.
const DefaultKubeControllerManagerAuthenticationKubeconfigKey = "kubeconfig"

// KubeControllerManagerAuthenticationKubeconfig specifies the authentication kubeconfig for the kube controller manager
// to use.
type KubeControllerManagerAuthenticationKubeconfig struct {
	// Secret references the secret for the kube controller manager to lookup. If key is unset,
	// DefaultKubeControllerManagerAuthenticationKubeconfigKey is used.
	Secret SecretSelector `json:"secret"`
}

// KubeControllerManagerKubernetesAPI specifies how the kube controller manager accesses the Kubernetes API.
type KubeControllerManagerKubernetesAPI struct {
	// Kubeconfig specifies the kubeconfig for the kube controller manager to use.
	Kubeconfig KubeControllerManagerKubeconfig `json:"kubeconfig"`
	// AuthorizationKubeconfig specifies the authorization kubeconfig for the kube controller manager to use.
	AuthorizationKubeconfig *KubeControllerManagerAuthorizationKubeconfig `json:"authorizationKubeconfig,omitempty"`
	// AuthenticationKubeconfig specifies the authentication kubeconfig for the kube controller manager to use.
	AuthenticationKubeconfig *KubeControllerManagerAuthenticationKubeconfig `json:"authenticationKubeconfig,omitempty"`
}

// KubeControllerManagerCluster specifies cluster settings for the kube controller manager.
type KubeControllerManagerCluster struct {
	// Name is the cluster name for the kube controller manager to use.
	Name string `json:"name"`
	// Signing specifies signing settings for the cluster.
	Signing *KubeControllerManagerClusterSigning `json:"signing,omitempty"`
}

// KubeControllerManagerClusterSigning specifies signing settings for the cluster.
type KubeControllerManagerClusterSigning struct {
	// Secret specifies the secret the kube controller manager uses for signing.
	// It is expected that the secret contains 'ca.crt' and 'tls.key'.
	Secret corev1.LocalObjectReference `json:"secret"`
}

// DefaultKubeControllerManagerServiceAccountPrivateKeyKey is the default key for
// KubeControllerManagerServiceAccountPrivateKey to look up.
const DefaultKubeControllerManagerServiceAccountPrivateKeyKey = "tls.key"

// KubeControllerManagerServiceAccountPrivateKey specifies the private key for the kube controller manager to use.
type KubeControllerManagerServiceAccountPrivateKey struct {
	// Secret specifies the secret to look up for the private key of the kube controller manager.
	// If key is left empty, DefaultKubeControllerManagerServiceAccountPrivateKeyKey will be used as default.
	Secret SecretSelector `json:"secret"`
}

// DefaultKubeControllerManagerServiceAccountRootCertificateAuthorityKey is the default key for
// KubeControllerManagerServiceAccountRootCertificateAuthority to look up.
const DefaultKubeControllerManagerServiceAccountRootCertificateAuthorityKey = "ca.crt"

// KubeControllerManagerServiceAccountRootCertificateAuthority specifies the root certificate authority for service
// accounts for the kube controller manager to use.
type KubeControllerManagerServiceAccountRootCertificateAuthority struct {
	// Secret specifies the secret to look up for the root ca of service accounts of the kube controller manager.
	// If key is left empty, DefaultKubeControllerManagerServiceAccountRootCertificateAuthorityKey will be used as
	// default.
	Secret SecretSelector `json:"secret"`
}

// KubeControllerManagerServiceAccount specifies settings on how the kube controller manager manages service accounts.
type KubeControllerManagerServiceAccount struct {
	// RootCertificateAuthority specifies an optional root certificate authority to distribute as part of each
	// service account.
	RootCertificateAuthority *KubeControllerManagerServiceAccountRootCertificateAuthority `json:"rootCertificateAuthority,omitempty"`
	// Private key specifies the private key for the kube controller manager to use.
	PrivateKey KubeControllerManagerServiceAccountPrivateKey `json:"privateKey,omitempty"`
}

// KubeControllerManagerAllDefaultOnControllers can be used in KubeControllerManagerControllers to enable all
// 'on-by-default' controllers of the kube controller manager.
const KubeControllerManagerAllDefaultOnControllers = "*"

// KubeControllerManagerControllers specifies controller specific settings.
type KubeControllerManagerControllers struct {
	// List is the list of all enabled controllers.
	//+kubebuilder:default={*}
	List []string `json:"list,omitempty"`
	// Credentials specifies how the controllers managed by kube controller manager should handle
	// their credentials towards the Kubernetes API.
	//+kubebuilder:default=ServiceAccount
	//+kubebuilder:validation:Enum=Global;ServiceAccount
	Credentials KubeControllerManagerControllersCredentials `json:"credentials,omitempty"`
}

// KubeControllerManagerControllersCredentials specifies how the controllers managed by kube controller manager should
// handle their credentials towards the Kubernetes API.
type KubeControllerManagerControllersCredentials string

const (
	// KubeControllerManagerGlobalCredentials instructs kube controller manager controllers to use the same set of
	// global credentials as the kube controller manager.
	KubeControllerManagerGlobalCredentials KubeControllerManagerControllersCredentials = "Global"
	// KubeControllerManagerServiceAccountCredentials instructs kube controller manager controllers to each use
	// individual service accounts.
	KubeControllerManagerServiceAccountCredentials KubeControllerManagerControllersCredentials = "ServiceAccount"
)

// KubeControllerManagerStatus defines the observed state of KubeControllerManager
type KubeControllerManagerStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	Replicas            int32              `json:"replicas,omitempty"`
	UpdatedReplicas     int32              `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32              `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32              `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32              `json:"unavailableReplicas,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

// KubeControllerManagerConditionType are types of KubeControllerManagerCondition.
type KubeControllerManagerConditionType string

const (
	// KubeControllerManagerAvailable reports whether the kube controller manager is available,
	// meaning the required number of replicas has met the health checks for a certain amount of time.
	KubeControllerManagerAvailable KubeControllerManagerConditionType = "Available"
	// KubeControllerManagerProgressing reports whether the update of a kube controller manager
	// deployment is progressing as expected.
	KubeControllerManagerProgressing KubeControllerManagerConditionType = "Progressing"
	// KubeControllerManagerDeploymentFailure indicates any error that might have occurred when
	// creating the deployment of a kube controller manager.
	KubeControllerManagerDeploymentFailure KubeControllerManagerConditionType = "DeploymentFailure"
)

// KubeControllerManagerCondition reports individual conditions of a KubeControllerManager.
type KubeControllerManagerCondition struct {
	Type               KubeControllerManagerConditionType `json:"type"`
	Status             corev1.ConditionStatus             `json:"status"`
	ObservedGeneration int64                              `json:"observedGeneration,omitempty"`
	LastUpdateTime     metav1.Time                        `json:"lastUpdateTime"`
	LastTransitionTime metav1.Time                        `json:"lastTransitionTime"`
	Reason             string                             `json:"reason"`
	Message            string                             `json:"message"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.readyReplicas`,description="number of ready replicas"
//+kubebuilder:printcolumn:name="Up-To-Date",type="number",JSONPath=`.status.updatedReplicas`,description="number of updated replicas"
//+kubebuilder:printcolumn:name="Available",type="number",JSONPath=`.status.availableReplicas`,description="number of available replicas"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// KubeControllerManager is the Schema for the kubecontrollermanagers API
type KubeControllerManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeControllerManagerSpec   `json:"spec,omitempty"`
	Status KubeControllerManagerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeControllerManagerList contains a list of KubeControllerManager
type KubeControllerManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeControllerManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeControllerManager{}, &KubeControllerManagerList{})
}
