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

// TODO: See https://github.com/kubernetes/kubernetes/blob/686379281d9a4207d3c3effe9dc43b5cef29108d/cmd/kube-controller-manager/app/options/options.go#L60
// for kubernetes built-in option grouping and adapt types.

// KubeControllerManagerFieldManager specifies the field manager used for operations on a KubeControllerManager.
var KubeControllerManagerFieldManager = fmt.Sprintf("%s/KubeControllerManager", GroupVersion.Group)

// KubeControllerManagerSpec defines the desired state of KubeControllerManager
type KubeControllerManagerSpec struct {
	// Replicas specifies the number of kube controller manager replicas to create.
	// This is a pointer to distinguish between not specified and explicit zero.
	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:default=1
	Replicas *int32 `json:"replicas"`
	// Version is the kube controller manager version to use.
	Version string `json:"version"`
	// Selector specifies the label selector to discover managed pods.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// Overlay is the KubeControllerManagerPodTemplateOverlay to use to scaffold the deployment.
	Overlay KubeControllerManagerPodTemplateOverlay `json:"overlay,omitempty"`
	// Generic is the generic / base configuration required for any kube controller manager.
	Generic KubeControllerManagerGenericConfiguration `json:"generic"`
	// Shared is configuration that is shared among any controller or other configuration options of a
	// kube controller manager.
	Shared KubeControllerManagerSharedConfiguration `json:"shared"`
	// CSRSigningController is configuration for the CSR (Certificate Signing Request) signing controller.
	CSRSigningController *KubeControllerManagerCSRSigningControllerConfiguration `json:"csrSigningController,omitempty"`
	// ServiceAccountController is configuration for the service account controller.
	ServiceAccountController *KubeControllerManagerServiceAccountControllerConfiguration `json:"serviceAccountController,omitempty"`
	// Authentication is configuration how the kube controller manager handles authentication / authenticating requests.
	Authentication *KubeControllerManagerAuthentication `json:"authentication,omitempty"`
	// Authorization is configuration how the kube controller manager handles authorization / authorizing requests.
	Authorization *KubeControllerManagerAuthorization `json:"authorization,omitempty"`
}

// KubeControllerManagerPodTemplateOverlay is the template overlay for pods.
type KubeControllerManagerPodTemplateOverlay struct {
	// ObjectMeta specifies additional object metadata to set on the managed pods.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the KubeAPIServerPodOverlay overlay specification for the pod.
	Spec KubeControllerManagerPodOverlay `json:"spec,omitempty"`
}

// KubeControllerManagerPodOverlay is the PodOverlay with additional ContainerOverlay containers.
type KubeControllerManagerPodOverlay struct {
	// PodOverlay is the base managed pod specification.
	PodOverlay `json:",inline,omitempty"`
	// ControllerManagerContainer is the ContainerOverlay that hosts the api server.
	ControllerManagerContainer ContainerOverlay `json:"controllerManagerContainer,omitempty"`
}

// DefaultKubeControllerManagerGenericConfigurationKubeconfigKey is the default key to lookup
// KubeControllerManagerGenericConfiguration kubeconfigs.
const DefaultKubeControllerManagerGenericConfigurationKubeconfigKey = "kubeconfig"

// KubeControllerManagerGenericConfiguration is generic kube controller manager configuration required for
// each kube controller manager to start up.
type KubeControllerManagerGenericConfiguration struct {
	// KubeconfigSecret is the reference to the kubeconfig secret.
	KubeconfigSecret SecretSelector `json:"kubeconfigSecret"`
	// Controllers is the list of controllers to enable or disable
	// '*' means "all enabled by default controllers"
	// 'foo' means "enable 'foo'"
	// '-foo' means "disable 'foo'"
	// first item for a particular name wins.
	//+kubebuilder:default={*}
	Controllers []string `json:"controllers,omitempty"`
}

// KubeControllerManagerControllerCredentials specifies how the controllers managed by kube controller manager should
// handle their credentials towards the Kubernetes API.
type KubeControllerManagerControllerCredentials string

const (
	// KubeControllerManagerGlobalCredentials instructs kube controller manager controllers to use the same set of
	// global credentials as the kube controller manager.
	KubeControllerManagerGlobalCredentials KubeControllerManagerControllerCredentials = "Global"
	// KubeControllerManagerServiceAccountCredentials instructs kube controller manager controllers to each use
	// individual service accounts.
	KubeControllerManagerServiceAccountCredentials KubeControllerManagerControllerCredentials = "ServiceAccount"
)

// KubeControllerManagerSharedConfiguration is configuration that is shared among other components of the
// kube controller manager.
type KubeControllerManagerSharedConfiguration struct {
	// ClusterName is the instance prefix for the cluster.
	//+kubebuilder:default=kubernetes
	ClusterName string `json:"clusterName,omitempty"`
	// ControllerCredentials specifies how the controllers managed by kube controller manager should handle
	// their credentials towards the Kubernetes API.
	//+kubebuilder:default=ServiceAccount
	//+kubebuilder:validation:Enum=Global;ServiceAccount
	ControllerCredentials KubeControllerManagerControllerCredentials `json:"controllerCredentials,omitempty"`
}

// KubeControllerManagerCSRSigningControllerConfiguration is configuration how the CSR (Certificate Signing Request)
// signing controller operates.
type KubeControllerManagerCSRSigningControllerConfiguration struct {
	// ClusterSigningSecret references the secret used for signing.
	// It is expected that this secret contains 'ca.crt' and 'tls.key' as items.
	ClusterSigningSecret *corev1.LocalObjectReference `json:"clusterSigningSecret,omitempty"`
}

const (
	// DefaultKubeControllerManagerServiceAccountControllerConfigurationPrivateKeySecretKey is the default key for
	// KubeControllerManagerServiceAccountControllerConfiguration.PrivateKeySecret.
	DefaultKubeControllerManagerServiceAccountControllerConfigurationPrivateKeySecretKey = "tls.key"
	// DefaultKubeControllerManagerServiceAccountControllerConfigurationRootCertificateSecretKey is the default key for
	// KubeControllerManagerServiceAccountControllerConfiguration.RootCertificateSecret.
	DefaultKubeControllerManagerServiceAccountControllerConfigurationRootCertificateSecretKey = "tls.crt"
)

// KubeControllerManagerServiceAccountControllerConfiguration contains configuration on how the kube
// controller manager service account controller operates.
type KubeControllerManagerServiceAccountControllerConfiguration struct {
	// PrivateKeySecret is the secret containing the private key to sign service accounts with.
	PrivateKeySecret *SecretSelector `json:"privateKeySecret,omitempty"`
	// RootCertificateSecret is the secret containing an optional root certificate to distribute
	// with each service account.
	RootCertificateSecret *SecretSelector `json:"rootCertificateSecret,omitempty"`
}

// DefaultKubeControllerManagerAuthenticationKubeconfigSecretKey is the default key for
// KubeControllerManagerAuthentication.KubeconfigSecret.
const DefaultKubeControllerManagerAuthenticationKubeconfigSecretKey = "kubeconfig"

// KubeControllerManagerAuthentication is configuration how the kube controller manager handles authentication
// and / or authenticating requests.
type KubeControllerManagerAuthentication struct {
	// SkipLookup instructs the kube controller manager to skip looking up additional authentication
	// information from the api server.
	SkipLookup bool `json:"skipLookup,omitempty"`
	// KubeconfigSecret is the kubeconfig secret used for authenticating.
	KubeconfigSecret SecretSelector `json:"kubeconfigSecret"`
}

// DefaultKubeControllerManagerAuthorizationKubeconfigSecretKey is the default key for
// KubeControllerManagerAuthorization.KubeconfigSecret.
const DefaultKubeControllerManagerAuthorizationKubeconfigSecretKey = "kubeconfig"

// KubeControllerManagerAuthorization is configuration how the kube controller manager handles authorization
// and / or authenticating requests.
type KubeControllerManagerAuthorization struct {
	// KubeconfigSecret is the kugbeconfig secret used for authorizing.
	KubeconfigSecret SecretSelector `json:"kubeconfigSecret"`
}

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
