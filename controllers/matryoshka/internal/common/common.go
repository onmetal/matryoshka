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
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"
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
	container.Resources = overlay.Resources
	container.VolumeMounts = append(container.VolumeMounts, overlay.AdditionalVolumeMounts...)
	container.VolumeDevices = append(container.VolumeDevices, overlay.AdditionalVolumeDevices...)
	container.ImagePullPolicy = overlay.ImagePullPolicy
	container.SecurityContext = overlay.SecurityContext
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
	spec.DNSPolicy = overlay.DNSPolicy
	spec.NodeSelector = utils.MergeStringStringMaps(spec.NodeSelector, overlay.NodeSelector)
	spec.ServiceAccountName = overlay.ServiceAccountName
	spec.AutomountServiceAccountToken = overlay.AutomountServiceAccountToken
	spec.NodeName = overlay.NodeName
	spec.HostNetwork = overlay.HostNetwork
	spec.HostPID = overlay.HostPID
	spec.HostIPC = overlay.HostIPC
	spec.ShareProcessNamespace = overlay.ShareProcessNamespace
	spec.SecurityContext = overlay.SecurityContext
	spec.Hostname = overlay.Hostname
	spec.Subdomain = overlay.Subdomain
	spec.Affinity = overlay.Affinity
	spec.SchedulerName = overlay.SchedulerName
	spec.Tolerations = overlay.Tolerations
	spec.HostAliases = overlay.HostAliases
	spec.PriorityClassName = overlay.PriorityClassName
	spec.Priority = overlay.Priority
	spec.DNSConfig = overlay.DNSConfig
	spec.ReadinessGates = append(spec.ReadinessGates, overlay.AdditionalReadinessGates...)
	spec.RuntimeClassName = overlay.RuntimeClassName
	spec.EnableServiceLinks = overlay.EnableServiceLinks
	spec.PreemptionPolicy = overlay.PreemptionPolicy
	spec.TopologySpreadConstraints = overlay.TopologySpreadConstraints
	spec.SetHostnameAsFQDN = overlay.SetHostnameAsFQDN
	return nil
}
