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

package kubecontrollermanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"

	"github.com/onmetal/controller-utils/clientutils"

	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/onmetal/controller-utils/memorystore"

	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolver resolves a matryoshkav1alpha1.KubeControllerManager to its required manifests.
type Resolver struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *Resolver) getRequests(kcm *matryoshkav1alpha1.KubeControllerManager) *clientutils.GetRequestSet {
	s := clientutils.NewGetRequestSet()
	s.Insert(
		clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: kcm.Spec.KubernetesAPI.Kubeconfig.Secret.Name},
			Object: &corev1.Secret{},
		},
		clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: kcm.Spec.ServiceAccount.PrivateKey.Secret.Name},
			Object: &corev1.Secret{},
		},
	)

	if signing := kcm.Spec.Cluster.Signing; signing != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: signing.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if kubeconfig := kcm.Spec.KubernetesAPI.AuthorizationKubeconfig; kubeconfig != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: kubeconfig.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if kubeconfig := kcm.Spec.KubernetesAPI.AuthenticationKubeconfig; kubeconfig != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: kubeconfig.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if rootCertificateAuthority := kcm.Spec.ServiceAccount.RootCertificateAuthority; rootCertificateAuthority != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: rootCertificateAuthority.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	return s
}

// ObjectReferences returns a clientutils.ObjectRefSet of objects a *matryoshkav1alpha1.KubeControllerManager
// references.
func (r *Resolver) ObjectReferences(kcm *matryoshkav1alpha1.KubeControllerManager) (clientutils.ObjectRefSet, error) {
	reqs := r.getRequests(kcm)
	return clientutils.ObjectRefSetFromGetRequestSet(r.scheme, reqs)
}

// Resolve resolves a matryoshkav1alpha1.KubeControllerManager to its required manifests.
func (r *Resolver) Resolve(ctx context.Context, kcm *matryoshkav1alpha1.KubeControllerManager) (*appsv1.Deployment, error) {
	log := ctrl.LoggerFrom(ctx)

	reqs := r.getRequests(kcm).List()
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
	deployment, err := r.deployment(ctx, s, kcm)
	if err != nil {
		return nil, fmt.Errorf("error building api server deployment: %w", err)
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

func (r *Resolver) deployment(ctx context.Context, s *memorystore.Store, kcm *matryoshkav1alpha1.KubeControllerManager) (*appsv1.Deployment, error) {
	selector := map[string]string{
		"matryoshka.onmetal.de/app": fmt.Sprintf("kube-controller-manager-%s", kcm.Name),
	}
	labels := utils.MergeStringStringMaps(kcm.Spec.Labels, selector)

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   kcm.Namespace,
			Name:        kcm.Name,
			Labels:      labels,
			Annotations: kcm.Spec.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(kcm.Spec.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: kcm.Spec.Annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "kube-controller-manager",
							Image:        fmt.Sprintf("k8s.gcr.io/kube-controller-manager:v%s", kcm.Spec.Version),
							Resources:    kcm.Spec.Resources,
							Command:      r.kubeControllerManagerCommand(kcm),
							VolumeMounts: r.kubeControllerManagerVolumeMounts(kcm),
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10257),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      15,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    2,
							},
						},
					},
					TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
					Volumes:                       r.kubeControllerManagerVolumes(kcm),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(kcm, deploy, r.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference: %w", err)
	}
	return deploy, nil
}

const (
	// VolumePrefix is the prefix of all kube controller manager volumes.
	VolumePrefix = "matryoshka-onmetal-de-"
	// PathPrefix is the prefix of all kube controller manager volume path mounts.
	PathPrefix = "/srv/kubernetes/"

	// ServiceAccountName is the name used for service account volume name and path.
	ServiceAccountName = "service-account"
	// ServiceAccountVolumeName is the name of the service account volume
	ServiceAccountVolumeName = VolumePrefix + ServiceAccountName
	// ServiceAccountVolumePath is the path of the service account volume.
	ServiceAccountVolumePath = PathPrefix + ServiceAccountName

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

	// SigningName is the name used for the signing volume name and path.
	SigningName = "signing"
	// SigningVolumeName is the name of the signing volume.
	SigningVolumeName = VolumePrefix + SigningName
	// SigningVolumePath is the path of the signing volume.
	SigningVolumePath = PathPrefix + SigningName

	// ServiceAccountRootCertficateName is the name used for the service account root ca volume and path.
	ServiceAccountRootCertficateName = "service-account-root-ca"
	// ServiceAccountRootCertficateVolumeName is the name of the service account root ca volume.
	ServiceAccountRootCertficateVolumeName = VolumePrefix + ServiceAccountRootCertficateName
	// ServiceAccountRootCertficateVolumePath is the path of the service account root ca volume.
	ServiceAccountRootCertficateVolumePath = PathPrefix + ServiceAccountRootCertficateName
)

func (r *Resolver) kubeControllerManagerCommand(kcm *matryoshkav1alpha1.KubeControllerManager) []string {
	cmd := []string{
		"/usr/local/bin/kube-controller-manager",
		"--bind-address=0.0.0.0",
		"--leader-elect=true",
		"--v=2",

		fmt.Sprintf("--cluster-name=%s", kcm.Spec.Cluster.Name),
		fmt.Sprintf("--controllers=%s", strings.Join(kcm.Spec.Controllers.List, ",")),
		fmt.Sprintf("--kubeconfig=%s/%s", KubeconfigVolumePath, utils.StringOrDefault(kcm.Spec.KubernetesAPI.Kubeconfig.Secret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerKubeconfigKey)),
		fmt.Sprintf("--service-account-private-key-file=%s/%s", ServiceAccountVolumePath, utils.StringOrDefault(kcm.Spec.ServiceAccount.PrivateKey.Secret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerServiceAccountPrivateKeyKey)),
	}

	if signing := kcm.Spec.Cluster.Signing; signing != nil {
		cmd = append(cmd, fmt.Sprintf("--cluster-signing-cert-file=%s/ca.crt", SigningVolumePath))
		cmd = append(cmd, fmt.Sprintf("--cluster-signing-key-file=%s/tls.key", SigningVolumePath))
	}

	if kubeconfig := kcm.Spec.KubernetesAPI.AuthorizationKubeconfig; kubeconfig != nil {
		cmd = append(cmd, fmt.Sprintf("--authorization-kubeconfig=%s/%s", AuthorizationKubeconfigVolumePath, utils.StringOrDefault(kubeconfig.Secret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerAuthorizationKubeconfigKey)))
	}
	if kubeconfig := kcm.Spec.KubernetesAPI.AuthenticationKubeconfig; kubeconfig != nil {
		cmd = append(cmd, fmt.Sprintf("--authentication-kubeconfig=%s/%s", AuthenticationKubeconfigVolumePath, utils.StringOrDefault(kubeconfig.Secret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerAuthenticationKubeconfigKey)))
	}

	if rootCertificateAuthority := kcm.Spec.ServiceAccount.RootCertificateAuthority; rootCertificateAuthority != nil {
		cmd = append(cmd, fmt.Sprintf("--root-ca-file=%s/%s", ServiceAccountRootCertficateVolumePath, utils.StringOrDefault(rootCertificateAuthority.Secret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerServiceAccountRootCertificateAuthorityKey)))
	}

	return cmd
}

func (r *Resolver) kubeControllerManagerVolumeMounts(kcm *matryoshkav1alpha1.KubeControllerManager) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      KubeconfigVolumeName,
			MountPath: KubeconfigVolumePath,
		},
		{
			Name:      ServiceAccountVolumeName,
			MountPath: ServiceAccountVolumePath,
		},
	}

	if kcm.Spec.Cluster.Signing != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      SigningVolumeName,
			MountPath: SigningVolumePath,
		})
	}

	if kcm.Spec.KubernetesAPI.AuthorizationKubeconfig != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      AuthorizationKubeconfigVolumeName,
			MountPath: AuthorizationKubeconfigVolumePath,
		})
	}

	if kcm.Spec.KubernetesAPI.AuthenticationKubeconfig != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      AuthenticationKubeconfigVolumeName,
			MountPath: AuthenticationKubeconfigVolumePath,
		})
	}

	if kcm.Spec.ServiceAccount.RootCertificateAuthority != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      ServiceAccountRootCertficateVolumeName,
			MountPath: ServiceAccountRootCertficateVolumePath,
		})
	}

	return mounts
}

func (r *Resolver) kubeControllerManagerVolumes(kcm *matryoshkav1alpha1.KubeControllerManager) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: KubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kcm.Spec.KubernetesAPI.Kubeconfig.Secret.Name,
				},
			},
		},
		{
			Name: ServiceAccountVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kcm.Spec.ServiceAccount.PrivateKey.Secret.Name,
				},
			},
		},
	}

	if signing := kcm.Spec.Cluster.Signing; signing != nil {
		volumes = append(volumes, corev1.Volume{
			Name: SigningVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: signing.Secret.Name,
				},
			},
		})
	}

	if kubeconfig := kcm.Spec.KubernetesAPI.AuthorizationKubeconfig; kubeconfig != nil {
		volumes = append(volumes, corev1.Volume{
			Name: AuthorizationKubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kubeconfig.Secret.Name,
				},
			},
		})
	}

	if kubeconfig := kcm.Spec.KubernetesAPI.AuthenticationKubeconfig; kubeconfig != nil {
		volumes = append(volumes, corev1.Volume{
			Name: AuthenticationKubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kubeconfig.Secret.Name,
				},
			},
		})
	}

	if rootCertificateAuthority := kcm.Spec.ServiceAccount.RootCertificateAuthority; rootCertificateAuthority != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ServiceAccountRootCertficateVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: rootCertificateAuthority.Secret.Name,
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
