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

package kubeapiserver

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/common"

	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"

	"github.com/onmetal/controller-utils/clientutils"
	"github.com/onmetal/controller-utils/memorystore"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// VolumePrefix is the prefix used for all volume names.
	VolumePrefix = "matryoshka-onmetal-de-"
	// PathPrefix is the prefix used for all volume paths.
	PathPrefix = "/srv/kubernetes/"

	// ServiceAccountKeyName is the name used for the service account volume name and path.
	ServiceAccountKeyName = "service-account-key"
	// ServiceAccountKeyVolumeName is the name of the service account volume.
	ServiceAccountKeyVolumeName = VolumePrefix + ServiceAccountKeyName
	// ServiceAccountKeyVolumePath is the path of the service account volume.
	ServiceAccountKeyVolumePath = PathPrefix + ServiceAccountKeyName

	// ServiceAccountSigningKeyName is the name used for the service account volume name and path.
	ServiceAccountSigningKeyName = "service-account-skey"
	// ServiceAccountSigningKeyVolumeName is the name of the service account volume.
	ServiceAccountSigningKeyVolumeName = VolumePrefix + ServiceAccountSigningKeyName
	// ServiceAccountSigningKeyVolumePath is the path of the service account volume.
	ServiceAccountSigningKeyVolumePath = PathPrefix + ServiceAccountSigningKeyName

	// TLSName is the name used for the tls account volume name and path.
	TLSName = "tls"
	// TLSVolumeName is the name of the tls volume.
	TLSVolumeName = VolumePrefix + TLSName
	// TLSVolumePath is the path of the tls volume.
	TLSVolumePath = PathPrefix + TLSName

	// TokenName is the name used for the token volume name and path.
	TokenName = "token"
	// TokenVolumeName is the name of the token volume.
	TokenVolumeName = VolumePrefix + TokenName
	// TokenVolumePath is the path of the token volume.
	TokenVolumePath = PathPrefix + TokenName

	// ETCDCAName is the name used for the etcd ca volume name and path.
	ETCDCAName = "etcd-ca"
	// ETCDCAVolumeName is the name of the etcd ca volume.
	ETCDCAVolumeName = VolumePrefix + ETCDCAName
	// ETCDCAVolumePath is the path of the etcd ca volume.
	ETCDCAVolumePath = PathPrefix + ETCDCAName

	// ETCDKeyName is the name used for the etcd key volume name and path.
	ETCDKeyName = "etcd-key"
	// ETCDKeyVolumeName is the name of the etcd key volume.
	ETCDKeyVolumeName = VolumePrefix + ETCDKeyName
	// ETCDKeyVolumePath is the path of the etcd key volume.
	ETCDKeyVolumePath = PathPrefix + ETCDKeyName
)

// Resolver resolves matryoshkav1alpha1.KubeAPIServer to its manifests.
type Resolver struct {
	scheme *runtime.Scheme
	client client.Client
}

func (r *Resolver) getRequests(server *matryoshkav1alpha1.KubeAPIServer) *clientutils.GetRequestSet {
	s := clientutils.NewGetRequestSet()

	s.Insert(clientutils.GetRequest{
		Key:    client.ObjectKey{Namespace: server.Namespace, Name: server.Spec.ServiceAccount.KeySecret.Name},
		Object: &corev1.Secret{},
	})
	s.Insert(clientutils.GetRequest{
		Key:    client.ObjectKey{Namespace: server.Namespace, Name: server.Spec.ServiceAccount.SigningKeySecret.Name},
		Object: &corev1.Secret{},
	})

	if key := server.Spec.ETCD.KeySecret; key != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: key.Name},
			Object: &corev1.Secret{},
		})
	}
	if ca := server.Spec.ETCD.CertificateAuthoritySecret; ca != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: ca.Name},
			Object: &corev1.Secret{},
		})
	}

	if token := server.Spec.Authentication.TokenSecret; token != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: token.Name},
			Object: &corev1.Secret{},
		})
	}

	if tls := server.Spec.SecureServing; tls != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: tls.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	return s
}

// ObjectReferences returns all object references of a matryoshkav1alpha1.KubeAPIServer.
func (r *Resolver) ObjectReferences(server *matryoshkav1alpha1.KubeAPIServer) (clientutils.ObjectRefSet, error) {
	getRequests := r.getRequests(server)
	return clientutils.ObjectRefSetFromGetRequestSet(r.scheme, getRequests)
}

func (r *Resolver) apiServerVolumes(server *matryoshkav1alpha1.KubeAPIServer) []corev1.Volume {
	var volumes []corev1.Volume
	volumes = append(volumes, corev1.Volume{
		Name: ServiceAccountKeyVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: server.Spec.ServiceAccount.KeySecret.Name},
		},
	})
	volumes = append(volumes, corev1.Volume{
		Name: ServiceAccountSigningKeyVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: server.Spec.ServiceAccount.SigningKeySecret.Name},
		},
	})
	if tls := server.Spec.SecureServing; tls != nil {
		volumes = append(volumes, corev1.Volume{
			Name: TLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: tls.Secret.Name},
			},
		})
	}
	if token := server.Spec.Authentication.TokenSecret; token != nil {
		volumes = append(volumes, corev1.Volume{
			Name: TokenVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: token.Name},
			},
		})
	}
	if etcdCA := server.Spec.ETCD.CertificateAuthoritySecret; etcdCA != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ETCDCAVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: etcdCA.Name},
			},
		})
	}
	if etcdKey := server.Spec.ETCD.KeySecret; etcdKey != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ETCDKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: etcdKey.Name},
			},
		})
	}
	return volumes
}

func (r *Resolver) apiServerVolumeMounts(server *matryoshkav1alpha1.KubeAPIServer) []corev1.VolumeMount {
	var mounts []corev1.VolumeMount
	mounts = append(mounts, corev1.VolumeMount{
		Name:      ServiceAccountKeyVolumeName,
		MountPath: ServiceAccountKeyVolumePath,
	})
	mounts = append(mounts, corev1.VolumeMount{
		Name:      ServiceAccountSigningKeyVolumeName,
		MountPath: ServiceAccountSigningKeyVolumePath,
	})
	if tls := server.Spec.SecureServing; tls != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSVolumePath,
		})
	}
	if token := server.Spec.Authentication.TokenSecret; token != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      TokenVolumeName,
			MountPath: TokenVolumePath,
		})
	}
	if etcdCA := server.Spec.ETCD.CertificateAuthoritySecret; etcdCA != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      ETCDCAVolumeName,
			MountPath: ETCDCAVolumePath,
		})
	}
	if etcdKey := server.Spec.ETCD.KeySecret; etcdKey != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      ETCDKeyVolumeName,
			MountPath: ETCDKeyVolumePath,
		})
	}
	return mounts
}

func (r *Resolver) apiServerCommand(server *matryoshkav1alpha1.KubeAPIServer) []string {
	cmd := []string{
		"/usr/local/bin/kube-apiserver",
		"--enable-admission-plugins=NamespaceLifecycle,NodeRestriction,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota",
		"--allow-privileged=false",
		"--authorization-mode=Node,RBAC",
		"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
		"--event-ttl=1h",
		"--profiling=false",
		"--secure-port=443",
		"--bind-address=0.0.0.0",
		"--service-cluster-ip-range=100.64.0.0/24",
		fmt.Sprintf("--etcd-servers=%s", strings.Join(server.Spec.ETCD.Servers, ",")),

		fmt.Sprintf("--enable-bootstrap-token-auth=%t", server.Spec.Authentication.BootstrapToken),
		fmt.Sprintf("--anonymous-auth=%t", server.Spec.Authentication.Anonymous),

		fmt.Sprintf("--service-account-issuer=%s", server.Spec.ServiceAccount.Issuer),
		fmt.Sprintf("--service-account-key-file=%s/tls.key", ServiceAccountKeyVolumePath),
		fmt.Sprintf("--service-account-signing-key-file=%s/tls.key", ServiceAccountSigningKeyVolumePath),
	}

	if tls := server.Spec.SecureServing; tls != nil {
		cmd = append(cmd,
			fmt.Sprintf("--tls-cert-file=%s/tls.crt", TLSVolumePath),
			fmt.Sprintf("--tls-private-key-file=%s/tls.key", TLSVolumePath),
		)
	}
	if tokenSecret := server.Spec.Authentication.TokenSecret; tokenSecret != nil {
		cmd = append(cmd,
			fmt.Sprintf("--token-auth-file=%s/%s", TokenVolumePath, utils.StringOrDefault(tokenSecret.Key, matryoshkav1alpha1.DefaultKubeAPIServerAuthenticationTokenSecretKey)),
		)
	}
	if etcdCA := server.Spec.ETCD.CertificateAuthoritySecret; etcdCA != nil {
		cmd = append(cmd,
			fmt.Sprintf("--etcd-cafile=%s/%s", ETCDCAVolumePath, utils.StringOrDefault(etcdCA.Key, matryoshkav1alpha1.DefaultKubeAPIServerETCDCertificateAuthoritySecretKey)),
		)
	}
	if etcdKey := server.Spec.ETCD.KeySecret; etcdKey != nil {
		cmd = append(cmd,
			fmt.Sprintf("--etcd-certfile=%s/tls.crt", ETCDKeyVolumePath),
			fmt.Sprintf("--etcd-keyfile=%s/tls.key", ETCDKeyVolumePath),
		)
	}

	return cmd
}

// AuthToken is a token and the user, uid and groups it should be bound to.
type AuthToken struct {
	Token  string
	User   string
	UID    string
	Groups []string
}

// ParseAuthTokens parses the given data into a slice of AuthToken.
func ParseAuthTokens(in io.Reader) ([]AuthToken, error) {
	r := csv.NewReader(in)
	var tokens []AuthToken
	for {
		record, err := r.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("error reading csv: %w", err)
			}
			return tokens, nil
		}

		if len(record) < 3 {
			return nil, fmt.Errorf("malformed token record %v - has to provide at least three columns", record)
		}

		token := AuthToken{
			Token: record[0],
			User:  record[1],
			UID:   record[2],
		}
		if len(record) > 3 {
			token.Groups = strings.Split(record[3], ",")
		}

		tokens = append(tokens, token)
	}
}

func (r *Resolver) probeHTTPHeaders(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer) ([]corev1.HTTPHeader, error) {
	var headers []corev1.HTTPHeader
	if token := server.Spec.Authentication.TokenSecret; token != nil && !server.Spec.Authentication.Anonymous {
		tokensData, err := utils.GetSecretSelector(ctx, s, server.Namespace, *token, matryoshkav1alpha1.DefaultKubeAPIServerAuthenticationTokenSecretKey)
		if err != nil {
			return nil, err
		}

		tokens, err := ParseAuthTokens(bytes.NewReader(tokensData))
		if err != nil {
			return nil, err
		}

		if len(tokens) == 0 {
			return nil, fmt.Errorf("no authentication token to use for health checks")
		}

		headers = append(headers, corev1.HTTPHeader{
			Name: "Authorization",
			// TODO: Just picking any token is not a good idea as
			// * We're exposing the token in the deployment manifest
			// * The user referenced in the token might not even have access to health / liveness checks.
			// We should think of a better concept of health checks in the future.
			Value: fmt.Sprintf("Bearer %s", tokens[0].Token),
		})
	}
	return headers, nil
}

func (r *Resolver) probeForPath(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer, path string) (*corev1.Probe, error) {
	scheme := corev1.URISchemeHTTP
	if server.Spec.SecureServing != nil {
		scheme = corev1.URISchemeHTTPS
	}

	headers, err := r.probeHTTPHeaders(ctx, s, server)
	if err != nil {
		return nil, fmt.Errorf("error getting probe http headers: %w", err)
	}

	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Scheme:      scheme,
				Path:        path,
				Port:        intstr.FromInt(443),
				HTTPHeaders: headers,
			},
		},
		InitialDelaySeconds: 15,
		TimeoutSeconds:      15,
		PeriodSeconds:       30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}, nil
}

func (r *Resolver) apiServerContainer(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer) (*corev1.Container, error) {
	livenessProbe, err := r.probeForPath(ctx, s, server, "/livez")
	if err != nil {
		return nil, fmt.Errorf("could not create liveness probe: %w", err)
	}

	readinessProbe, err := r.probeForPath(ctx, s, server, "/readyz")
	if err != nil {
		return nil, fmt.Errorf("could not create readiness probe: %w", err)
	}

	container := &corev1.Container{
		Name:    "kube-apiserver",
		Image:   fmt.Sprintf("k8s.gcr.io/kube-apiserver:v%s", server.Spec.Version),
		Command: r.apiServerCommand(server),
		Ports: []corev1.ContainerPort{
			{
				Name:          "https",
				ContainerPort: 443,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts:   r.apiServerVolumeMounts(server),
		LivenessProbe:  livenessProbe,
		ReadinessProbe: readinessProbe,
	}

	if err := common.ApplyContainerOverlay(container, &server.Spec.Overlay.Spec.APIServerContainer); err != nil {
		return nil, err
	}

	return container, nil
}

func (r *Resolver) deploymentPodSpec(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer) (*corev1.PodSpec, error) {
	apiServerContainer, err := r.apiServerContainer(ctx, s, server)
	if err != nil {
		return nil, err
	}

	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			*apiServerContainer,
		},
		TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
		Volumes:                       r.apiServerVolumes(server),
	}

	if err := common.ApplyPodOverlay(spec, &server.Spec.Overlay.Spec.PodOverlay); err != nil {
		return nil, err
	}

	return spec, nil
}

func (r *Resolver) deployment(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer) (*appsv1.Deployment, error) {
	baseLabels := map[string]string{
		"matryoshka.onmetal.de/app": fmt.Sprintf("kubeapiserver-%s", server.Name),
	}
	meta := server.Spec.Overlay.ObjectMeta
	meta.Labels = utils.MergeStringStringMaps(meta.Labels, baseLabels)
	selector := server.Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: baseLabels,
		}
	}

	spec, err := r.deploymentPodSpec(ctx, s, server)
	if err != nil {
		return nil, err
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   server.Namespace,
			Name:        server.Name,
			Labels:      meta.Labels,
			Annotations: meta.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: server.Spec.Replicas,
			Selector: selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec:       *spec,
			},
		},
	}

	if err := ctrl.SetControllerReference(server, deploy, r.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference: %w", err)
	}
	return deploy, nil
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

	deployment.Spec.Template.Annotations = utils.MergeStringStringMaps(deployment.Spec.Template.Annotations, checksums)
	return nil
}

// Resolve resolves a matryoshkav1alpha1.KubeAPIServer into its required manifests.
func (r *Resolver) Resolve(ctx context.Context, server *matryoshkav1alpha1.KubeAPIServer) (*appsv1.Deployment, error) {
	log := ctrl.LoggerFrom(ctx)

	reqs := r.getRequests(server).List()
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
	deployment, err := r.deployment(ctx, s, server.DeepCopy())
	if err != nil {
		return nil, fmt.Errorf("error building api server deployment: %w", err)
	}

	log.V(2).Info("Updating deployment with checksums.")
	if err := r.updateDeploymentChecksums(ctx, s, deployment); err != nil {
		return nil, fmt.Errorf("error updating deployment checksums")
	}

	return deployment, nil
}

// NewResolver creates a new Resolver with the given runtime.Scheme and client.Client.
func NewResolver(scheme *runtime.Scheme, c client.Client) *Resolver {
	return &Resolver{
		scheme: scheme,
		client: c,
	}
}
