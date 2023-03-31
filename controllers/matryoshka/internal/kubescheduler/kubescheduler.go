// Copyright 2022 OnMetal authors
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

package kubescheduler

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onmetal/controller-utils/clientutils"
	"github.com/onmetal/controller-utils/memorystore"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/common"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"
)

// Resolver resolves a matryoshkav1alpha1.KubeScheduler to its required manifests.
type Resolver struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *Resolver) getRequests(ks *matryoshkav1alpha1.KubeScheduler) *clientutils.GetRequestSet {
	s := clientutils.NewGetRequestSet()
	s.Insert(clientutils.GetRequest{
		Key:    client.ObjectKey{Namespace: ks.Namespace, Name: ks.Spec.ComponentConfig.KubeconfigSecret.Name},
		Object: &corev1.Secret{},
	})

	if authN := ks.Spec.Authentication; authN != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: ks.Namespace, Name: authN.KubeconfigSecret.Name},
			Object: &corev1.Secret{},
		})
	}

	if authZ := ks.Spec.Authorization; authZ != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: ks.Namespace, Name: authZ.KubeconfigSecret.Name},
			Object: &corev1.Secret{},
		})
	}

	return s
}

// ObjectReferences returns a clientutils.ObjectRefSet of objects a *matryoshkav1alpha1.KubeScheduler references.
func (r *Resolver) ObjectReferences(ks *matryoshkav1alpha1.KubeScheduler) (clientutils.ObjectRefSet, error) {
	reqs := r.getRequests(ks)
	return clientutils.ObjectRefSetFromGetRequestSet(r.scheme, reqs)
}

// Resolve resolves a matryoshkav1alpha1.KubeScheduler to its required manifests.
func (r *Resolver) Resolve(ctx context.Context, ks *matryoshkav1alpha1.KubeScheduler) (*appsv1.Deployment, error) {
	log := ctrl.LoggerFrom(ctx)

	reqs := r.getRequests(ks).List()
	log.V(1).Info("Retrieving source objects.")
	if err := clientutils.GetMultiple(ctx, r.client, reqs); err != nil {
		return nil, fmt.Errorf("error retrieving source objects: %w", err)
	}

	log.V(2).Info("Building cache from retrieved source objects.")
	s := memorystore.New(r.scheme)
	for _, obj := range clientutils.ObjectsFromGetRequests(reqs) {
		if err := s.Create(ctx, obj); err != nil {
			return nil, fmt.Errorf("error storing object %s in cache: %w",
				client.ObjectKeyFromObject(obj),
				err,
			)
		}
	}

	log.V(1).Info("Building deployment.")
	deployment, err := r.deployment(ctx, s, ks)
	if err != nil {
		return nil, fmt.Errorf("error building deployment: %w", err)
	}

	log.V(2).Info("Updating deployment with checksums.")
	if err := r.updateDeploymentChecksums(ctx, s, deployment); err != nil {
		return nil, fmt.Errorf("error updating deployment checksums")
	}

	return deployment, nil
}

func (r *Resolver) updateDeploymentChecksums(ctx context.Context, s *memorystore.Store, deployment *appsv1.Deployment) error {
	secrets := &corev1.SecretList{}
	if err := s.List(ctx, secrets); err != nil {
		return fmt.Errorf("error retrieving referenced secrets: %w", err)
	}

	configMaps := &corev1.ConfigMapList{}
	if err := s.List(ctx, configMaps); err != nil {
		return fmt.Errorf("error retrieving referenced configmaps: %w", err)
	}

	checksums, err := utils.ComputeMountableChecksum(secrets.Items, configMaps.Items)
	if err != nil {
		return fmt.Errorf("error computing mountable checksum: %w", err)
	}

	for k, v := range checksums {
		metav1.SetMetaDataAnnotation(&deployment.Spec.Template.ObjectMeta, k, v)
	}
	return nil
}

func (r *Resolver) schedulerContainer(_ context.Context, _ *memorystore.Store, ks *matryoshkav1alpha1.KubeScheduler) (*corev1.Container, error) {
	container := &corev1.Container{
		Name:         "kube-scheduler",
		Image:        fmt.Sprintf("registry.k8s.io/kube-scheduler:v%s", ks.Spec.Version),
		Command:      r.kubeSchedulerCommand(ks),
		VolumeMounts: r.kubeSchedulerVolumeMounts(ks),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt(10259),
					Scheme: corev1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    2,
		},
	}

	if err := common.ApplyContainerOverlay(container, &ks.Spec.Overlay.Spec.SchedulerContainer); err != nil {
		return nil, err
	}

	return container, nil
}

func (r *Resolver) deploymentPodSpec(ctx context.Context, s *memorystore.Store, ks *matryoshkav1alpha1.KubeScheduler) (*corev1.PodSpec, error) {
	schedulerContainer, err := r.schedulerContainer(ctx, s, ks)
	if err != nil {
		return nil, err
	}

	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			*schedulerContainer,
		},
		TerminationGracePeriodSeconds: pointer.Int64(30),
		Volumes:                       r.kubeSchedulerVolumes(ks),
	}
	if err := common.ApplyPodOverlay(spec, &ks.Spec.Overlay.Spec.PodOverlay); err != nil {
		return nil, err
	}

	return spec, nil
}

func (r *Resolver) deployment(ctx context.Context, s *memorystore.Store, ks *matryoshkav1alpha1.KubeScheduler) (*appsv1.Deployment, error) {
	baseLabels := map[string]string{
		"matryoshka.onmetal.de/app": fmt.Sprintf("kubescheduler-%s", ks.Name),
	}
	meta := ks.Spec.Overlay.ObjectMeta
	meta.Labels = utils.MergeStringStringMaps(meta.Labels, baseLabels)
	selector := ks.Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: baseLabels,
		}
	}

	spec, err := r.deploymentPodSpec(ctx, s, ks)
	if err != nil {
		return nil, err
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ks.Namespace,
			Name:        ks.Name,
			Labels:      meta.Labels,
			Annotations: meta.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ks.Spec.Replicas,
			Selector: selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec:       *spec,
			},
		},
	}

	if err := ctrl.SetControllerReference(ks, deploy, r.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference: %w", err)
	}
	return deploy, nil
}

const (
	// VolumePrefix is the prefix of all kube scheduler volumes.
	VolumePrefix = "matryoshka-onmetal-de-"
	// PathPrefix is the prefix of all kube scheduler volume path mounts.
	PathPrefix = "/srv/kubernetes/"

	// KubeconfigName is the name used for kubeconfig volume name and path.
	KubeconfigName = "kubeconfig"
	// KubeconfigVolumeName is the name of the kubeconfig volume.
	KubeconfigVolumeName = VolumePrefix + KubeconfigName
	// KubeconfigVolumePath is the path of the kubeconfig volume
	KubeconfigVolumePath = PathPrefix + KubeconfigName

	// AuthorizationKubeconfigName is the name used for the authorization kubeconfig volume name and path.
	AuthorizationKubeconfigName = "authorization-kubeconfig"
	// AuthorizationKubeconfigVolumeName is the name of the authorization kubeconfig volume.
	AuthorizationKubeconfigVolumeName = VolumePrefix + AuthorizationKubeconfigName
	// AuthorizationKubeconfigVolumePath is the path of the authorization kubeconfig volume.
	AuthorizationKubeconfigVolumePath = PathPrefix + AuthorizationKubeconfigName

	// AuthenticationKubeconfigName is the name used for the authentication kubeconfig volume name and path.
	AuthenticationKubeconfigName = "authentication-kubeconfig"
	// AuthenticationKubeconfigVolumeName is the name of the authentication kubeconfig volume.
	AuthenticationKubeconfigVolumeName = VolumePrefix + AuthenticationKubeconfigName
	// AuthenticationKubeconfigVolumePath is the path of the authentication kubeconfig volume.
	AuthenticationKubeconfigVolumePath = PathPrefix + AuthenticationKubeconfigName
)

func (r *Resolver) kubeSchedulerCommand(ks *matryoshkav1alpha1.KubeScheduler) []string {
	featureGates := []string{}
	for key, val := range ks.Spec.FeatureGates {
		featureGates = append(featureGates, fmt.Sprintf("%v=%v", key, val))
	}

	cmd := []string{
		"/usr/local/bin/kube-scheduler",
		"--bind-address=0.0.0.0",
		"--leader-elect=true",
		"--v=2",
		fmt.Sprintf("--feature-gates=%s", strings.Join(featureGates, ",")),
		fmt.Sprintf("--kubeconfig=%s/%s", KubeconfigVolumePath, utils.StringOrDefault(ks.Spec.ComponentConfig.KubeconfigSecret.Key, matryoshkav1alpha1.DefaultKubeSchedulerConfigurationKubeconfigKey)),
	}

	if authentication := ks.Spec.Authentication; authentication != nil {
		cmd = append(cmd,
			fmt.Sprintf("--authentication-kubeconfig=%s/%s",
				AuthenticationKubeconfigVolumePath,
				utils.StringOrDefault(authentication.KubeconfigSecret.Key, matryoshkav1alpha1.DefaultKubeSchedulerAuthenticationKubeconfigSecretKey),
			),
			fmt.Sprintf("--authentication-skip-lookup=%t", authentication.SkipLookup),
		)
	}

	if authorization := ks.Spec.Authorization; authorization != nil {
		cmd = append(cmd, fmt.Sprintf("--authorization-kubeconfig=%s/%s",
			AuthorizationKubeconfigVolumePath,
			utils.StringOrDefault(authorization.KubeconfigSecret.Key, matryoshkav1alpha1.DefaultKubeSchedulerAuthorizationKubeconfigSecretKey),
		))
	}

	return cmd
}

func (r *Resolver) kubeSchedulerVolumeMounts(ks *matryoshkav1alpha1.KubeScheduler) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      KubeconfigVolumeName,
			MountPath: KubeconfigVolumePath,
		},
	}

	if authentication := ks.Spec.Authentication; authentication != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      AuthenticationKubeconfigVolumeName,
			MountPath: AuthenticationKubeconfigVolumePath,
		})
	}

	if authorization := ks.Spec.Authorization; authorization != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      AuthorizationKubeconfigVolumeName,
			MountPath: AuthorizationKubeconfigVolumePath,
		})
	}

	return mounts
}

func (r *Resolver) kubeSchedulerVolumes(ks *matryoshkav1alpha1.KubeScheduler) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: KubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: ks.Spec.ComponentConfig.KubeconfigSecret.Name,
				},
			},
		},
	}

	if authentication := ks.Spec.Authentication; authentication != nil {
		volumes = append(volumes, corev1.Volume{
			Name: AuthenticationKubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: authentication.KubeconfigSecret.Name,
				},
			},
		})
	}

	if authorization := ks.Spec.Authorization; authorization != nil {
		volumes = append(volumes, corev1.Volume{
			Name: AuthorizationKubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: authorization.KubeconfigSecret.Name,
				},
			},
		})
	}

	return volumes
}

// NewResolver creates a new Resolver.
func NewResolver(scheme *runtime.Scheme, c client.Client) *Resolver {
	return &Resolver{scheme: scheme, client: c}
}
