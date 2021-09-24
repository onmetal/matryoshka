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

package common

import (
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ApplyContainerOverlay applies all properties from the v1alpha1.ContainerOverlay to the container.
//
// Applying the overlay consists of merging properties that have an 'Additional' prefix and overriding all other
// properties.
func ApplyContainerOverlay(container *corev1.Container, overlay *matryoshkav1alpha1.ContainerOverlay) error {
	container.Ports = append(container.Ports, overlay.AdditionalPorts...)
	container.EnvFrom = append(container.EnvFrom, overlay.AdditionalEnvFrom...)
	container.Env = append(container.Env, overlay.AdditionalEnv...)
	if overlay.Resources != nil {
		container.Resources = *overlay.Resources
	}
	container.VolumeMounts = append(container.VolumeMounts, overlay.AdditionalVolumeMounts...)
	container.VolumeDevices = append(container.VolumeDevices, overlay.AdditionalVolumeDevices...)
	if overlay.ImagePullPolicy != nil {
		container.ImagePullPolicy = *overlay.ImagePullPolicy
	}
	if overlay.SecurityContext != nil {
		container.SecurityContext = overlay.SecurityContext
	}
	return nil
}

// ApplyPodOverlay applies the v1alpha1.PodOverlay to the pod.
//
// Applying the overlay consists of merging properties that have an 'Additional' prefix and overriding all other
// properties.
func ApplyPodOverlay(spec *corev1.PodSpec, overlay *matryoshkav1alpha1.PodOverlay) error {
	spec.Volumes = append(spec.Volumes, overlay.AdditionalVolumes...)
	spec.InitContainers = append(spec.InitContainers, overlay.AdditionalInitContainers...)
	spec.Containers = append(spec.Containers, overlay.AdditionalContainers...)
	if overlay.DNSPolicy != nil {
		spec.DNSPolicy = *overlay.DNSPolicy
	}
	if overlay.NodeSelector != nil {
		spec.NodeSelector = *overlay.NodeSelector
	}
	if overlay.ServiceAccountName != nil {
		spec.ServiceAccountName = *overlay.ServiceAccountName
	}
	if overlay.AutomountServiceAccountToken != nil {
		spec.AutomountServiceAccountToken = overlay.AutomountServiceAccountToken
	}
	if overlay.NodeName != nil {
		spec.NodeName = *overlay.NodeName
	}
	if overlay.HostNetwork != nil {
		spec.HostNetwork = *overlay.HostNetwork
	}
	if overlay.HostPID != nil {
		spec.HostPID = *overlay.HostPID
	}
	if overlay.HostIPC != nil {
		spec.HostIPC = *overlay.HostIPC
	}
	if overlay.ShareProcessNamespace != nil {
		spec.ShareProcessNamespace = overlay.ShareProcessNamespace
	}
	if overlay.SecurityContext != nil {
		spec.SecurityContext = overlay.SecurityContext
	}
	if overlay.Hostname != nil {
		spec.Hostname = *overlay.Hostname
	}
	if overlay.Subdomain != nil {
		spec.Subdomain = *overlay.Subdomain
	}
	if overlay.Affinity != nil {
		spec.Affinity = overlay.Affinity
	}
	if overlay.SchedulerName != nil {
		spec.SchedulerName = *overlay.SchedulerName
	}
	if overlay.Tolerations != nil {
		spec.Tolerations = *overlay.Tolerations
	}
	if overlay.HostAliases != nil {
		spec.HostAliases = *overlay.HostAliases
	}
	if overlay.PriorityClassName != nil {
		spec.PriorityClassName = *overlay.PriorityClassName
	}
	if overlay.Priority != nil {
		spec.Priority = overlay.Priority
	}
	if overlay.DNSConfig != nil {
		spec.DNSConfig = overlay.DNSConfig
	}
	spec.ReadinessGates = append(spec.ReadinessGates, overlay.AdditionalReadinessGates...)
	if overlay.RuntimeClassName != nil {
		spec.RuntimeClassName = overlay.RuntimeClassName
	}
	if overlay.EnableServiceLinks != nil {
		spec.EnableServiceLinks = overlay.EnableServiceLinks
	}
	if overlay.PreemptionPolicy != nil {
		spec.PreemptionPolicy = overlay.PreemptionPolicy
	}
	if overlay.TopologySpreadConstraints != nil {
		spec.TopologySpreadConstraints = *overlay.TopologySpreadConstraints
	}
	if overlay.SetHostnameAsFQDN != nil {
		spec.SetHostnameAsFQDN = overlay.SetHostnameAsFQDN
	}
	return nil
}
