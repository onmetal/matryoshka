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

	"github.com/onmetal/controller-utils/clientutils"
	"github.com/onmetal/controller-utils/memorystore"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/pkg/utils"
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
	MountPrefix = "matryoshka-onmetal-de-"
	PathPrefix  = "/srv/kubernetes/"

	ServiceAccountName       = "service-account"
	ServiceAccountVolumeName = MountPrefix + ServiceAccountName
	ServiceAccountVolumePath = PathPrefix + ServiceAccountName

	TLSName       = "tls"
	TLSVolumeName = MountPrefix + TLSName
	TLSVolumePath = PathPrefix + TLSName

	TokenName       = "token"
	TokenVolumeName = MountPrefix + TokenName
	TokenVolumePath = PathPrefix + TokenName

	ETCDCAName       = "etcd-ca"
	ETCDCAVolumeName = MountPrefix + ETCDCAName
	ETCDCAVolumePath = PathPrefix + ETCDCAName

	ETCDKeyName       = "etcd-key"
	ETCDKeyVolumeName = MountPrefix + ETCDKeyName
	ETCDKeyVolumePath = PathPrefix + ETCDKeyName
)

type Resolver struct {
	scheme *runtime.Scheme
	client client.Client
}

func (r *Resolver) getRequests(server *matryoshkav1alpha1.KubeAPIServer) *clientutils.GetRequestSet {
	s := clientutils.NewGetRequestSet()

	s.Insert(clientutils.GetRequest{
		Key:    client.ObjectKey{Namespace: server.Namespace, Name: server.Spec.ServiceAccount.Secret.Name},
		Object: &corev1.Secret{},
	})

	if key := server.Spec.ETCD.Key; key != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: key.Secret.Name},
			Object: &corev1.Secret{},
		})
	}
	if ca := server.Spec.ETCD.CertificateAuthority; ca != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: ca.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if token := server.Spec.Authentication.Token; token != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: token.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if tls := server.Spec.TLS; tls != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: server.Namespace, Name: tls.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	return s
}

func (r *Resolver) ObjectReferences(server *matryoshkav1alpha1.KubeAPIServer) (clientutils.ObjectRefSet, error) {
	getRequests := r.getRequests(server)
	return clientutils.ObjectRefSetFromGetRequestSet(r.scheme, getRequests)
}

func (r *Resolver) apiServerVolumes(server *matryoshkav1alpha1.KubeAPIServer) []corev1.Volume {
	var volumes []corev1.Volume
	volumes = append(volumes, corev1.Volume{
		Name: ServiceAccountVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: server.Spec.ServiceAccount.Secret.Name},
		},
	})
	if tls := server.Spec.TLS; tls != nil {
		volumes = append(volumes, corev1.Volume{
			Name: TLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: tls.Secret.Name},
			},
		})
	}
	if token := server.Spec.Authentication.Token; token != nil {
		volumes = append(volumes, corev1.Volume{
			Name: TokenVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: token.Secret.Name},
			},
		})
	}
	if etcdCA := server.Spec.ETCD.CertificateAuthority; etcdCA != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ETCDCAVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: etcdCA.Secret.Name},
			},
		})
	}
	if etcdKey := server.Spec.ETCD.Key; etcdKey != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ETCDKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: etcdKey.Secret.Name},
			},
		})
	}
	return volumes
}

func (r *Resolver) apiServerVolumeMounts(server *matryoshkav1alpha1.KubeAPIServer) []corev1.VolumeMount {
	var mounts []corev1.VolumeMount
	mounts = append(mounts, corev1.VolumeMount{
		Name:      ServiceAccountVolumeName,
		MountPath: ServiceAccountVolumePath,
	})
	if tls := server.Spec.TLS; tls != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSVolumePath,
		})
	}
	if token := server.Spec.Authentication.Token; token != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      TokenVolumeName,
			MountPath: TokenVolumePath,
		})
	}
	if etcdCA := server.Spec.ETCD.CertificateAuthority; etcdCA != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      ETCDCAVolumeName,
			MountPath: ETCDCAVolumePath,
		})
	}
	if etcdKey := server.Spec.ETCD.Key; etcdKey != nil {
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

		fmt.Sprintf("--enable-bootstrap-token-auth=%t", server.Spec.Authentication.BootstrapToken != nil),
		fmt.Sprintf("--anonymous-auth=%t", server.Spec.Authentication.Anonymous != nil),

		fmt.Sprintf("--service-account-issuer=%s", server.Spec.ServiceAccount.Issuer),
		fmt.Sprintf("--service-account-key-file=%s/tls.key", ServiceAccountVolumePath),
		fmt.Sprintf("--service-account-signing-key-file=%s/tls.key", ServiceAccountVolumePath),
	}

	if tls := server.Spec.TLS; tls != nil {
		cmd = append(cmd,
			fmt.Sprintf("--tls-cert-file=%s/tls.crt", TLSVolumePath),
			fmt.Sprintf("--tls-private-key-file=%s/tls.key", TLSVolumePath),
		)
	}
	if token := server.Spec.Authentication.Token; token != nil {
		cmd = append(cmd,
			fmt.Sprintf("--token-auth-file=%s/%s", TokenVolumePath, utils.StringOrDefault(token.Secret.Key, matryoshkav1alpha1.DefaultKubeAPIServerTokenAuthenticationKey)),
		)
	}
	if etcdCA := server.Spec.ETCD.CertificateAuthority; etcdCA != nil {
		cmd = append(cmd,
			fmt.Sprintf("--etcd-cafile=%s/%s", ETCDCAVolumePath, utils.StringOrDefault(etcdCA.Secret.Key, matryoshkav1alpha1.DefaultKubeAPIServerETCDCertificateAuthorityKey)),
		)
	}
	if etcdKey := server.Spec.ETCD.Key; etcdKey != nil {
		cmd = append(cmd,
			fmt.Sprintf("--etcd-certfile=%s/tls.crt", ETCDKeyVolumePath),
			fmt.Sprintf("--etcd-certfile=%s/tls.key", ETCDKeyVolumePath),
		)
	}

	return cmd
}

type AuthToken struct {
	Token  string
	User   string
	UID    string
	Groups []string
}

func ParseAuthTokens(data []byte) ([]AuthToken, error) {
	r := csv.NewReader(bytes.NewReader(data))
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
	if token := server.Spec.Authentication.Token; token != nil {
		tokensData, err := utils.GetSecretSelector(ctx, s, server.Namespace, token.Secret, matryoshkav1alpha1.DefaultKubeAPIServerTokenAuthenticationKey)
		if err != nil {
			return nil, err
		}

		tokens, err := ParseAuthTokens(tokensData)
		if err != nil {
			return nil, err
		}

		if len(tokens) == 0 {
			return nil, fmt.Errorf("no authentication token to use for health checks")
		}

		headers = append(headers, corev1.HTTPHeader{
			Name:  "Authorization",
			Value: fmt.Sprintf("Bearer %s", tokens[0].Token),
		})
	}
	return headers, nil
}

func (r *Resolver) probeForPath(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer, path string) (*corev1.Probe, error) {
	scheme := corev1.URISchemeHTTP
	if server.Spec.TLS != nil {
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

func (r *Resolver) deployment(ctx context.Context, s *memorystore.Store, server *matryoshkav1alpha1.KubeAPIServer) (*appsv1.Deployment, error) {
	selector := map[string]string{
		"matryoshka.onmetal.de/app": fmt.Sprintf("kubeapiserver-%s", server.Name),
	}
	labels := utils.MergeStringStringMaps(server.Spec.Labels, selector)

	livenessProbe, err := r.probeForPath(ctx, s, server, "/livez")
	if err != nil {
		return nil, fmt.Errorf("could not create liveness probe: %w", err)
	}

	readinessProbe, err := r.probeForPath(ctx, s, server, "/readyz")
	if err != nil {
		return nil, fmt.Errorf("could not create readiness probe: %w", err)
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   server.Namespace,
			Name:        server.Name,
			Labels:      labels,
			Annotations: server.Spec.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(server.Spec.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: server.Spec.Annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      "kube-apiserver",
							Image:     fmt.Sprintf("k8s.gcr.io/kube-apiserver:v%s", server.Spec.Version),
							Resources: server.Spec.Resources,
							Command:   r.apiServerCommand(server),
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
						},
					},
					TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
					Volumes:                       r.apiServerVolumes(server),
				},
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

	for k, v := range checksums {
		metav1.SetMetaDataAnnotation(&deployment.Spec.Template.ObjectMeta, k, v)
	}
	return nil
}

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
	deployment, err := r.deployment(ctx, s, server)
	if err != nil {
		return nil, fmt.Errorf("error building api server deployment: %w", err)
	}

	log.V(2).Info("Updating deployment with checksums.")
	if err := r.updateDeploymentChecksums(ctx, s, deployment); err != nil {
		return nil, fmt.Errorf("error updating deployment checksums")
	}

	return deployment, nil
}

type ResolverOptions struct {
	Scheme *runtime.Scheme
	Client client.Client
}

func (o *ResolverOptions) Validate() error {
	if o.Scheme == nil {
		return fmt.Errorf("scheme needs to be set")
	}
	if o.Client == nil {
		return fmt.Errorf("client needs to be set")
	}
	return nil
}

func NewResolver(opts ResolverOptions) (*Resolver, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	return &Resolver{
		scheme: opts.Scheme,
		client: opts.Client,
	}, nil
}
