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

package kubeconfig

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api/latest"

	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"

	"github.com/onmetal/controller-utils/clientutils"
	"github.com/onmetal/controller-utils/memorystore"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolver resolves matryoshkav1alpha1.Kubeconfig to its manifests.
type Resolver struct {
	scheme *runtime.Scheme
	client client.Client
}

func (r *Resolver) getRequests(kubeconfig *matryoshkav1alpha1.Kubeconfig) *clientutils.GetRequestSet {
	s := clientutils.NewGetRequestSet()
	for _, authInfo := range kubeconfig.Spec.AuthInfos {
		r.addKubeconfigAuthInfoGetRequests(s, kubeconfig.Namespace, &authInfo.AuthInfo)
	}
	for _, cluster := range kubeconfig.Spec.Clusters {
		r.addKubeconfigClusterReferencesGetRequests(s, kubeconfig.Namespace, &cluster.Cluster)
	}
	return s
}

func (r *Resolver) addKubeconfigClusterReferencesGetRequests(s *clientutils.GetRequestSet, namespace string, cluster *matryoshkav1alpha1.KubeconfigCluster) {
	if caSecret := cluster.CertificateAuthoritySecret; caSecret != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: caSecret.Name},
			Object: &corev1.Secret{},
		})
	}
}

func (r *Resolver) addKubeconfigAuthInfoGetRequests(s *clientutils.GetRequestSet, namespace string, authInfo *matryoshkav1alpha1.KubeconfigAuthInfo) {
	if clientCertSecret := authInfo.ClientCertificateSecret; clientCertSecret != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: clientCertSecret.Name},
			Object: &corev1.Secret{},
		})
	}

	if clientKeySecret := authInfo.ClientKeySecret; clientKeySecret != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: clientKeySecret.Name},
			Object: &corev1.Secret{},
		})
	}

	if tokenSecret := authInfo.TokenSecret; tokenSecret != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: tokenSecret.Name},
			Object: &corev1.Secret{},
		})
	}

	if passwordSecret := authInfo.PasswordSecret; passwordSecret != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: passwordSecret.Name},
			Object: &corev1.Secret{},
		})
	}
}

// ObjectReferences returns a clientutils.ObjectRefSet of all objects the matryoshkav1alpha1.Kubeconfig references.
func (r *Resolver) ObjectReferences(kubeconfig *matryoshkav1alpha1.Kubeconfig) (clientutils.ObjectRefSet, error) {
	reqs := r.getRequests(kubeconfig)
	return clientutils.ObjectRefSetFromGetRequestSet(r.scheme, reqs)
}

// resolveConfig resolves a matryoshkav1alpha1.Kubeconfig to a clientcmdapiv1.Config.
func (r *Resolver) resolveConfig(ctx context.Context, log logr.Logger, kubeconfig *matryoshkav1alpha1.Kubeconfig) (*clientcmdapiv1.Config, error) {
	reqs := r.getRequests(kubeconfig).List()
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

	log.V(1).Info("Building client config.")
	cfg, err := r.config(ctx, s, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error resolving kubeconfig to config: %w", err)
	}

	return cfg, nil
}

// Resolve resolves a matryoshkav1alpha1.Kubeconfig to its required manifests.
func (r *Resolver) Resolve(ctx context.Context, kubeconfig *matryoshkav1alpha1.Kubeconfig) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Resolving client client config")
	cfg, err := r.resolveConfig(ctx, log, kubeconfig)
	if err != nil {
		return nil, err
	}

	log.V(1).Info("Encoding config")
	var b bytes.Buffer
	if err := latest.Codec.Encode(cfg, &b); err != nil {
		return nil, fmt.Errorf("error encoding config: %w", err)
	}

	dataKey := kubeconfig.Spec.KubeconfigKey
	if dataKey == "" {
		dataKey = matryoshkav1alpha1.DefaultKubeconfigKey
	}

	log.V(1).Info("Resolving secret")
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeconfig.Namespace,
			Name:      kubeconfig.Spec.SecretName,
		},
		Data: map[string][]byte{
			dataKey: b.Bytes(),
		},
	}

	if err := ctrl.SetControllerReference(kubeconfig, secret, r.scheme); err != nil {
		return nil, fmt.Errorf("error setting secret owner reference: %w", err)
	}
	return secret, nil
}

func (r *Resolver) config(ctx context.Context, s *memorystore.Store, kubeconfig *matryoshkav1alpha1.Kubeconfig) (*clientcmdapiv1.Config, error) {
	authInfos := make([]clientcmdapiv1.NamedAuthInfo, 0, len(kubeconfig.Spec.AuthInfos))
	for _, authInfo := range kubeconfig.Spec.AuthInfos {
		resolved, err := r.resolveAuthInfo(ctx, s, kubeconfig.Namespace, &authInfo.AuthInfo)
		if err != nil {
			return nil, err
		}

		authInfos = append(authInfos, clientcmdapiv1.NamedAuthInfo{Name: authInfo.Name, AuthInfo: *resolved})
	}

	clusters := make([]clientcmdapiv1.NamedCluster, 0, len(kubeconfig.Spec.Clusters))
	for _, cluster := range kubeconfig.Spec.Clusters {
		resolved, err := r.resolveCluster(ctx, s, kubeconfig.Namespace, &cluster.Cluster)
		if err != nil {
			return nil, err
		}

		clusters = append(clusters, clientcmdapiv1.NamedCluster{Name: cluster.Name, Cluster: *resolved})
	}

	contexts := make([]clientcmdapiv1.NamedContext, 0, len(kubeconfig.Spec.Contexts))
	for _, kubeconfigCtx := range kubeconfig.Spec.Contexts {
		contexts = append(contexts, clientcmdapiv1.NamedContext{
			Name: kubeconfigCtx.Name,
			Context: clientcmdapiv1.Context{
				Cluster:   kubeconfigCtx.Context.Cluster,
				AuthInfo:  kubeconfigCtx.Context.AuthInfo,
				Namespace: kubeconfigCtx.Context.Namespace,
			},
		})
	}

	return &clientcmdapiv1.Config{
		Clusters:       clusters,
		AuthInfos:      authInfos,
		Contexts:       contexts,
		CurrentContext: kubeconfig.Spec.CurrentContext,
	}, nil
}

func (r *Resolver) resolveAuthInfo(
	ctx context.Context,
	s *memorystore.Store,
	namespace string,
	authInfo *matryoshkav1alpha1.KubeconfigAuthInfo,
) (*clientcmdapiv1.AuthInfo, error) {
	var clientCertificateData []byte
	if clientCertSecret := authInfo.ClientCertificateSecret; clientCertSecret != nil {
		var err error
		clientCertificateData, err = utils.GetSecretSelector(ctx, s, namespace, *clientCertSecret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoClientCertificateSecretKey)
		if err != nil {
			return nil, err
		}
	}

	var clientKeyData []byte
	if clientKeySecret := authInfo.ClientKeySecret; clientKeySecret != nil {
		var err error
		clientKeyData, err = utils.GetSecretSelector(ctx, s, namespace, *clientKeySecret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoClientKeySecretKey)
		if err != nil {
			return nil, err
		}
	}

	var token string
	if tokenSecret := authInfo.TokenSecret; tokenSecret != nil {
		var (
			tokenData []byte
			err       error
		)
		tokenData, err = utils.GetSecretSelector(ctx, s, namespace, *tokenSecret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoTokenSecretKey)
		if err != nil {
			return nil, err
		}

		token = string(tokenData)
	}

	var password string
	if passwordSecret := authInfo.PasswordSecret; passwordSecret != nil {
		var (
			passwordData []byte
			err          error
		)
		passwordData, err = utils.GetSecretSelector(ctx, s, namespace, *passwordSecret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoPasswordSecretKey)
		if err != nil {
			return nil, err
		}

		password = string(passwordData)
	}

	return &clientcmdapiv1.AuthInfo{
		ClientCertificateData: clientCertificateData,
		ClientKeyData:         clientKeyData,
		Token:                 token,
		Impersonate:           authInfo.Impersonate,
		ImpersonateGroups:     authInfo.ImpersonateGroups,
		Username:              authInfo.Username,
		Password:              password,
	}, nil
}

func (r *Resolver) resolveCluster(ctx context.Context, s *memorystore.Store, namespace string, cluster *matryoshkav1alpha1.KubeconfigCluster) (*clientcmdapiv1.Cluster, error) {
	var caSecretData []byte
	if caSecret := cluster.CertificateAuthoritySecret; caSecret != nil {
		var err error
		caSecretData, err = utils.GetSecretSelector(ctx, s, namespace, *caSecret, matryoshkav1alpha1.DefaultKubeconfigClusterCertificateAuthoritySecretKey)
		if err != nil {
			return nil, err
		}
	}

	return &clientcmdapiv1.Cluster{
		Server:                   cluster.Server,
		TLSServerName:            cluster.TLSServerName,
		InsecureSkipTLSVerify:    cluster.InsecureSkipTLSVerify,
		CertificateAuthorityData: caSecretData,
		ProxyURL:                 cluster.ProxyURL,
	}, nil
}

// NewResolver creates a new Resolver.
func NewResolver(scheme *runtime.Scheme, c client.Client) *Resolver {
	return &Resolver{
		scheme: scheme,
		client: c,
	}
}
