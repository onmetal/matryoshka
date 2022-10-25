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

// TODO: See https://github.com/kubernetes/kubernetes/blob/686379281d9a4207d3c3effe9dc43b5cef29108d/cmd/kube-scheduler/app/options/options.go#L53
// for kubernetes built-in option grouping and adapt types.

// KubeSchedulerFieldManager specifies the field manager used for operations on a KubeScheduler.
var KubeSchedulerFieldManager = fmt.Sprintf("%s/KubeScheduler", GroupVersion.Group)

// KubeSchedulerSpec defines the desired state of KubeScheduler
type KubeSchedulerSpec struct {
	// Replicas specifies the number of kube scheduler replicas to create.
	// This is a pointer to distinguish between not specified and explicit zero.
	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:default=1
	Replicas *int32 `json:"replicas"`
	// Version is the kube scheduler version to use.
	Version string `json:"version"`
	// Selector specifies the label selector to discover managed pods.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// Overlay is the KubeSchedulerPodTemplateOverlay to use to scaffold the deployment.
	Overlay KubeSchedulerPodTemplateOverlay `json:"overlay,omitempty"`
	// ComponentConfig is the base configuration required for any kube scheduler.
	ComponentConfig KubeSchedulerConfiguration `json:"componentConfig"`
	// Authentication is configuration how the kube scheduler handles authentication / authenticating requests.
	Authentication *KubeSchedulerAuthentication `json:"authentication,omitempty"`
	// Authorization is configuration how the kube scheduler handles authorization / authorizing requests.
	Authorization *KubeSchedulerAuthorization `json:"authorization,omitempty"`
	// FeatureGates describe which alpha features should be enabled or beta features disabled
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KubeSchedulerPodTemplateOverlay is the template overlay for pods.
type KubeSchedulerPodTemplateOverlay struct {
	// ObjectMeta specifies additional object metadata to set on the managed pods.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the KubeSchedulerPodOverlay overlay specification for the pod.
	Spec KubeSchedulerPodOverlay `json:"spec,omitempty"`
}

// KubeSchedulerPodOverlay is the PodOverlay with additional ContainerOverlay containers.
type KubeSchedulerPodOverlay struct {
	// PodOverlay is the base managed pod specification.
	PodOverlay `json:",inline,omitempty"`
	// SchedulerContainer is the ContainerOverlay that hosts the kube scheduler.
	SchedulerContainer ContainerOverlay `json:"schedulerContainer,omitempty"`
}

// DefaultKubeSchedulerConfigurationKubeconfigKey is the default key to lookup KubeSchedulerConfiguration kubeconfig.
const DefaultKubeSchedulerConfigurationKubeconfigKey = "kubeconfig"

// KubeSchedulerConfiguration is the kube scheduler configuration required for the kube scheduler to start up.
type KubeSchedulerConfiguration struct {
	// KubeconfigSecret is the reference to the kubeconfig secret.
	KubeconfigSecret SecretSelector `json:"kubeconfigSecret"`
	// TODO: expose other configuration. See https://github.com/kubernetes/kubernetes/blob/429f71d9583af6f2add7496fd8e41ae0f180a832/pkg/scheduler/apis/config/types.go#L41
}

// DefaultKubeSchedulerAuthenticationKubeconfigSecretKey is the default key for
// KubeSchedulerAuthentication.KubeconfigSecret.
const DefaultKubeSchedulerAuthenticationKubeconfigSecretKey = "kubeconfig"

// KubeSchedulerAuthentication is configuration how the kube scheduler handles authentication
// and / or authenticating requests.
type KubeSchedulerAuthentication struct {
	// SkipLookup instructs the kube scheduler to skip looking up additional authentication
	// information from the api server.
	SkipLookup bool `json:"skipLookup,omitempty"`
	// KubeconfigSecret is the kubeconfig secret used for authenticating.
	KubeconfigSecret SecretSelector `json:"kubeconfigSecret"`
}

// DefaultKubeSchedulerAuthorizationKubeconfigSecretKey is the default key for
// KubeSchedulerAuthorization.KubeconfigSecret.
const DefaultKubeSchedulerAuthorizationKubeconfigSecretKey = "kubeconfig"

// KubeSchedulerAuthorization is configuration how the kube scheduler handles authorization
// and / or authenticating requests.
type KubeSchedulerAuthorization struct {
	// KubeconfigSecret is the kugbeconfig secret used for authorizing.
	KubeconfigSecret SecretSelector `json:"kubeconfigSecret"`
}

// KubeSchedulerStatus defines the observed state of KubeScheduler
type KubeSchedulerStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	Replicas            int32              `json:"replicas,omitempty"`
	UpdatedReplicas     int32              `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32              `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32              `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32              `json:"unavailableReplicas,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

// KubeSchedulerConditionType are types of KubeSchedulerCondition.
type KubeSchedulerConditionType string

const (
	// KubeSchedulerAvailable reports whether the kube scheduler is available,
	// meaning the required number of replicas has met the health checks for a certain amount of time.
	KubeSchedulerAvailable KubeSchedulerConditionType = "Available"
	// KubeSchedulerProgressing reports whether the update of a kube scheduler
	// deployment is progressing as expected.
	KubeSchedulerProgressing KubeSchedulerConditionType = "Progressing"
	// KubeSchedulerDeploymentFailure indicates any error that might have occurred when
	// creating the deployment of a kube scheduler.
	KubeSchedulerDeploymentFailure KubeSchedulerConditionType = "DeploymentFailure"
)

// KubeSchedulerCondition reports individual conditions of a KubeScheduler.
type KubeSchedulerCondition struct {
	Type               KubeSchedulerConditionType `json:"type"`
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

// KubeScheduler is the Schema for the kubeschedulers API
type KubeScheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeSchedulerSpec   `json:"spec,omitempty"`
	Status KubeSchedulerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeSchedulerList contains a list of KubeScheduler
type KubeSchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeScheduler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeScheduler{}, &KubeSchedulerList{})
}
