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

import (
	corev1 "k8s.io/api/core/v1"
)

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

// PodOverlay represents a pod specification that is managed by a controller but
// allows for additional configuration to be passed in.
type PodOverlay struct {
	// AdditionalVolumes is a list of additional volumes to mount to the pod.
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"`
	// AdditionalInitContainers is a list of additional initialization containers belonging to the pod.
	AdditionalInitContainers []corev1.Container `json:"additionalInitContainers,omitempty"`
	// Containers is a list of additional containers belonging to the pod.
	AdditionalContainers []corev1.Container `json:"additionalContainers,omitempty"`
	// DNSPolicy sets the DNS policy for the pod.
	// Defaults to "ClusterFirst".
	DNSPolicy *corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	NodeSelector *map[string]string `json:"nodeSelector,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// AutomountServiceAccountToken indicates whether a service account token should be automatically mounted.
	AutomountServiceAccountToken *bool `json:"automountServiceAccountToken,omitempty"`
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	NodeName *string `json:"nodeName,omitempty"`
	// Host networking requested for this pod. Use the host's network namespace.
	// If this option is set, the ports that will be used must be specified.
	HostNetwork *bool `json:"hostNetwork,omitempty"`
	// Use the host's pid namespace.
	// Optional: Default to false.
	HostPID *bool `json:"hostPID,omitempty"`
	// Use the host's ipc namespace.
	// Optional: Default to false.
	HostIPC *bool `json:"hostIPC,omitempty"`
	// Share a single process namespace between all the containers in a pod.
	// When this is set containers will be able to view and signal processes from other containers
	// in the same pod, and the first process in each container will not be assigned PID 1.
	// HostPID and ShareProcessNamespace cannot both be set.
	ShareProcessNamespace *bool `json:"shareProcessNamespace,omitempty"`
	// SecurityContext holds pod-level security attributes and common container settings.
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// AdditionalImagePullSecrets is an additional list of references to secrets in the same namespace to use for
	// pulling any of the images used by this PodOverlay.
	AdditionalImagePullSecrets []corev1.LocalObjectReference `json:"additionalImagePullSecrets,omitempty"`
	// Specifies the hostname of the Pod
	// If not specified, the pod's hostname will be set to a system-defined value.
	Hostname *string `json:"hostname,omitempty"`
	// If specified, the fully qualified Pod hostname will be "<hostname>.<subdomain>.<pod namespace>.svc.<cluster domain>".
	// If not specified, the pod will not have a domain name at all.
	Subdomain *string `json:"subdomain,omitempty"`
	// If specified, the pod's scheduling constraints
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// If specified, the pod will be dispatched by specified scheduler.
	// If not specified, the pod will be dispatched by default scheduler.
	SchedulerName *string `json:"schedulerName,omitempty"`
	// If specified, the pod's tolerations.
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	HostAliases *[]corev1.HostAlias `json:"hostAliases,omitempty"`
	// PriorityClassName indicates the pod's priority.
	PriorityClassName *string `json:"priorityClassName,omitempty"`
	// Priority is the priority value.
	Priority *int32 `json:"priority,omitempty"`
	// DNSConfig Specifies the DNS parameters of a pod.
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
	// AdditionalReadinessGates specifies additional readiness gates that will be evaluated for pod readiness.
	AdditionalReadinessGates []corev1.PodReadinessGate `json:"additionalReadinessGates,omitempty"`
	// RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be used
	// to run this pod. If no RuntimeClass resource matches the named class, the pod will not be run.
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
	// EnableServiceLinks indicates whether information about services should be injected into pod's
	// environment variables, matching the syntax of Docker links.
	EnableServiceLinks *bool `json:"enableServiceLinks,omitempty"`
	// PreemptionPolicy is the Policy for preempting pods with lower priority.
	PreemptionPolicy *corev1.PreemptionPolicy `json:"preemptionPolicy,omitempty"`
	// TopologySpreadConstraints describes constraints of how a group of pods ought to spread across topology domains.
	TopologySpreadConstraints *[]corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// If true the pod's hostname will be configured as the pod's FQDN, rather than the leaf name (the default).
	SetHostnameAsFQDN *bool `json:"setHostnameAsFQDN,omitempty"`
}

// ContainerOverlay represents a container that is managed by a controller but
// allows for additional configuration to be passed in.
type ContainerOverlay struct {
	// AdditionalPorts specifies a list of additional ports to expose.
	AdditionalPorts []corev1.ContainerPort `json:"additionalPorts,omitempty"`
	// AdditionalEnvFrom specifies a list of additional env sources for the pod.
	AdditionalEnvFrom []corev1.EnvFromSource `json:"additionalEnvFrom,omitempty"`
	// AdditionalEnv specifies a list of additional env variables for the pod.
	AdditionalEnv []corev1.EnvVar `json:"additionalEnv,omitempty"`
	// Resources required by this container.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// AdditionalVolumeMounts specifies additional volumes to mount into the container's filesystem.
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
	// AdditionalVolumeDevices is a list of additional block devices to be used by the container.
	AdditionalVolumeDevices []corev1.VolumeDevice `json:"additionalVolumeDevices,omitempty"`
	// ImagePullPolicy specifies the image pull policy.
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// SecurityContext defines the security options the container should be run with.
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}
