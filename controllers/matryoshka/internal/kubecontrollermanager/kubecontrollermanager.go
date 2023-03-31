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

// Resolver resolves a matryoshkav1alpha1.KubeControllerManager to its required manifests.
type Resolver struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *Resolver) getRequests(kcm *matryoshkav1alpha1.KubeControllerManager) *clientutils.GetRequestSet {
	s := clientutils.NewGetRequestSet()
	s.Insert(clientutils.GetRequest{
		Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: kcm.Spec.Generic.KubeconfigSecret.Name},
		Object: &corev1.Secret{},
	})

	if serviceAccount := kcm.Spec.ServiceAccountController; serviceAccount != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: serviceAccount.PrivateKeySecret.Name},
			Object: &corev1.Secret{},
		})

		if rootCA := serviceAccount.RootCertificateSecret; rootCA != nil {
			s.Insert(clientutils.GetRequest{
				Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: rootCA.Name},
				Object: &corev1.Secret{},
			})
		}
	}

	if csrSigning := kcm.Spec.CSRSigningController; csrSigning != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: csrSigning.ClusterSigningSecret.Name},
			Object: &corev1.Secret{},
		})
	}

	if authN := kcm.Spec.Authentication; authN != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: authN.KubeconfigSecret.Name},
			Object: &corev1.Secret{},
		})
	}

	if authZ := kcm.Spec.Authorization; authZ != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: kcm.Namespace, Name: authZ.KubeconfigSecret.Name},
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

func (r *Resolver) controllerManagerContainer(_ context.Context, _ *memorystore.Store, kcm *matryoshkav1alpha1.KubeControllerManager) (*corev1.Container, error) {
	container := &corev1.Container{
		Name:         "kube-controller-manager",
		Image:        fmt.Sprintf("registry.k8s.io/kube-controller-manager:v%s", kcm.Spec.Version),
		Command:      r.kubeControllerManagerCommand(kcm),
		VolumeMounts: r.kubeControllerManagerVolumeMounts(kcm),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
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
	}

	if err := common.ApplyContainerOverlay(container, &kcm.Spec.Overlay.Spec.ControllerManagerContainer); err != nil {
		return nil, err
	}

	return container, nil
}

func (r *Resolver) deploymentPodSpec(ctx context.Context, s *memorystore.Store, kcm *matryoshkav1alpha1.KubeControllerManager) (*corev1.PodSpec, error) {
	controllerManagerContainer, err := r.controllerManagerContainer(ctx, s, kcm)
	if err != nil {
		return nil, err
	}

	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			*controllerManagerContainer,
		},
		TerminationGracePeriodSeconds: pointer.Int64(30),
		Volumes:                       r.kubeControllerManagerVolumes(kcm),
	}
	if err := common.ApplyPodOverlay(spec, &kcm.Spec.Overlay.Spec.PodOverlay); err != nil {
		return nil, err
	}

	return spec, nil
}

func (r *Resolver) deployment(ctx context.Context, s *memorystore.Store, kcm *matryoshkav1alpha1.KubeControllerManager) (*appsv1.Deployment, error) {
	baseLabels := map[string]string{
		"matryoshka.onmetal.de/app": fmt.Sprintf("kubecontrollermanager-%s", kcm.Name),
	}
	meta := kcm.Spec.Overlay.ObjectMeta
	meta.Labels = utils.MergeStringStringMaps(meta.Labels, baseLabels)
	selector := kcm.Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: baseLabels,
		}
	}

	spec, err := r.deploymentPodSpec(ctx, s, kcm)
	if err != nil {
		return nil, err
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   kcm.Namespace,
			Name:        kcm.Name,
			Labels:      meta.Labels,
			Annotations: meta.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: kcm.Spec.Replicas,
			Selector: selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec:       *spec,
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

	// ServiceAccountPrivateKeyName is the name used for service account volume name and path.
	ServiceAccountPrivateKeyName = "service-account-private-key"
	// ServiceAccountPrivateKeyVolumeName is the name of the service account volume
	ServiceAccountPrivateKeyVolumeName = VolumePrefix + ServiceAccountPrivateKeyName
	// ServiceAccountPrivateKeyVolumePath is the path of the service account volume.
	ServiceAccountPrivateKeyVolumePath = PathPrefix + ServiceAccountPrivateKeyName

	// ServiceAccountRootCertificateName is the name used for the service account root ca volume and path.
	ServiceAccountRootCertificateName = "service-account-root-ca"
	// ServiceAccountRootCertificateVolumeName is the name of the service account root ca volume.
	ServiceAccountRootCertificateVolumeName = VolumePrefix + ServiceAccountRootCertificateName
	// ServiceAccountRootCertificateVolumePath is the path of the service account root ca volume.
	ServiceAccountRootCertificateVolumePath = PathPrefix + ServiceAccountRootCertificateName

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

	// ClusterSigningName is the name used for the cluster signing volume name and path.
	ClusterSigningName = "cluster-signing"
	// ClusterSigningVolumeName is the name of the cluster signing volume.
	ClusterSigningVolumeName = VolumePrefix + ClusterSigningName
	// ClusterSigningVolumePath is the path of the cluster signing volume.
	ClusterSigningVolumePath = PathPrefix + ClusterSigningName
)

func (r *Resolver) kubeControllerManagerCommand(kcm *matryoshkav1alpha1.KubeControllerManager) []string {
	featureGates := []string{}
	for key, val := range kcm.Spec.FeatureGates {
		featureGates = append(featureGates, fmt.Sprintf("%v=%v", key, val))
	}

	cmd := []string{
		"/usr/local/bin/kube-controller-manager",
		"--bind-address=0.0.0.0",
		"--leader-elect=true",
		"--v=2",

		fmt.Sprintf("--cluster-name=%s", kcm.Spec.Shared.ClusterName),
		fmt.Sprintf("--controllers=%s", strings.Join(kcm.Spec.Generic.Controllers, ",")),
		fmt.Sprintf("--feature-gates=%s", strings.Join(featureGates, ",")),
		fmt.Sprintf("--kubeconfig=%s/%s", KubeconfigVolumePath, utils.StringOrDefault(kcm.Spec.Generic.KubeconfigSecret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerGenericConfigurationKubeconfigKey)),
		fmt.Sprintf("--use-service-account-credentials=%t", kcm.Spec.Shared.ControllerCredentials == matryoshkav1alpha1.KubeControllerManagerServiceAccountCredentials),
	}

	if serviceAccount := kcm.Spec.ServiceAccountController; serviceAccount != nil {
		if privateKey := serviceAccount.PrivateKeySecret; privateKey != nil {
			cmd = append(cmd, fmt.Sprintf("--service-account-private-key-file=%s/%s",
				ServiceAccountPrivateKeyVolumePath,
				utils.StringOrDefault(privateKey.Key, matryoshkav1alpha1.DefaultKubeControllerManagerServiceAccountControllerConfigurationPrivateKeySecretKey),
			))
		}

		if rootCA := serviceAccount.RootCertificateSecret; rootCA != nil {
			cmd = append(cmd, fmt.Sprintf("--root-ca-file=%s/%s",
				ServiceAccountRootCertificateVolumePath,
				utils.StringOrDefault(rootCA.Key, matryoshkav1alpha1.DefaultKubeControllerManagerServiceAccountControllerConfigurationRootCertificateSecretKey),
			))
		}
	}

	if signing := kcm.Spec.CSRSigningController; signing != nil {
		if clusterSigning := signing.ClusterSigningSecret; clusterSigning != nil {
			cmd = append(cmd, fmt.Sprintf("--cluster-signing-cert-file=%s/ca.crt", ClusterSigningVolumePath))
			cmd = append(cmd, fmt.Sprintf("--cluster-signing-key-file=%s/tls.key", ClusterSigningVolumePath))
		}
	}

	if authentication := kcm.Spec.Authentication; authentication != nil {
		cmd = append(cmd,
			fmt.Sprintf("--authentication-kubeconfig=%s/%s",
				AuthenticationKubeconfigVolumePath,
				utils.StringOrDefault(authentication.KubeconfigSecret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerAuthenticationKubeconfigSecretKey),
			),
			fmt.Sprintf("--authentication-skip-lookup=%t", authentication.SkipLookup),
		)
	}

	if authorization := kcm.Spec.Authorization; authorization != nil {
		cmd = append(cmd, fmt.Sprintf("--authorization-kubeconfig=%s/%s",
			AuthorizationKubeconfigVolumePath,
			utils.StringOrDefault(authorization.KubeconfigSecret.Key, matryoshkav1alpha1.DefaultKubeControllerManagerAuthorizationKubeconfigSecretKey),
		))
	}

	return cmd
}

func (r *Resolver) kubeControllerManagerVolumeMounts(kcm *matryoshkav1alpha1.KubeControllerManager) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      KubeconfigVolumeName,
			MountPath: KubeconfigVolumePath,
		},
	}

	if serviceAccount := kcm.Spec.ServiceAccountController; serviceAccount != nil {
		if privateKey := serviceAccount.PrivateKeySecret; privateKey != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      ServiceAccountPrivateKeyVolumeName,
				MountPath: ServiceAccountPrivateKeyVolumePath,
			})
		}

		if rootCA := serviceAccount.RootCertificateSecret; rootCA != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      ServiceAccountRootCertificateVolumeName,
				MountPath: ServiceAccountRootCertificateVolumePath,
			})
		}
	}

	if signing := kcm.Spec.CSRSigningController; signing != nil {
		if signing.ClusterSigningSecret != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      ClusterSigningVolumeName,
				MountPath: ClusterSigningVolumePath,
			})
		}
	}

	if authentication := kcm.Spec.Authentication; authentication != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      AuthenticationKubeconfigVolumeName,
			MountPath: AuthenticationKubeconfigVolumePath,
		})
	}

	if authorization := kcm.Spec.Authorization; authorization != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      AuthorizationKubeconfigVolumeName,
			MountPath: AuthorizationKubeconfigVolumePath,
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
					SecretName: kcm.Spec.Generic.KubeconfigSecret.Name,
				},
			},
		},
	}

	if serviceAccount := kcm.Spec.ServiceAccountController; serviceAccount != nil {
		if privateKey := serviceAccount.PrivateKeySecret; privateKey != nil {
			volumes = append(volumes, corev1.Volume{
				Name: ServiceAccountPrivateKeyVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: privateKey.Name,
					},
				},
			})
		}

		if rootCA := serviceAccount.RootCertificateSecret; rootCA != nil {
			volumes = append(volumes, corev1.Volume{
				Name: ServiceAccountRootCertificateVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: rootCA.Name,
					},
				},
			})
		}
	}

	if signing := kcm.Spec.CSRSigningController; signing != nil {
		if clusterSigning := signing.ClusterSigningSecret; clusterSigning != nil {
			volumes = append(volumes, corev1.Volume{
				Name: ClusterSigningVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: clusterSigning.Name,
					},
				},
			})
		}
	}

	if authentication := kcm.Spec.Authentication; authentication != nil {
		volumes = append(volumes, corev1.Volume{
			Name: AuthenticationKubeconfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: authentication.KubeconfigSecret.Name,
				},
			},
		})
	}

	if authorization := kcm.Spec.Authorization; authorization != nil {
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
