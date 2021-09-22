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
	"context"
	"fmt"

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
	if certificateAuthority := cluster.CertificateAuthority; certificateAuthority != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: certificateAuthority.Secret.Name},
			Object: &corev1.Secret{},
		})
	}
}

func (r *Resolver) addKubeconfigAuthInfoGetRequests(s *clientutils.GetRequestSet, namespace string, authInfo *matryoshkav1alpha1.KubeconfigAuthInfo) {
	if clientCertificate := authInfo.ClientCertificate; clientCertificate != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: clientCertificate.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if clientKey := authInfo.ClientKey; clientKey != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: clientKey.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if token := authInfo.Token; token != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: token.Secret.Name},
			Object: &corev1.Secret{},
		})
	}

	if password := authInfo.Password; password != nil {
		s.Insert(clientutils.GetRequest{
			Key:    client.ObjectKey{Namespace: namespace, Name: password.Secret.Name},
			Object: &corev1.Secret{},
		})
	}
}

// ObjectReferences returns a clientutils.ObjectRefSet of all objects the matryoshkav1alpha1.Kubeconfig references.
func (r *Resolver) ObjectReferences(kubeconfig *matryoshkav1alpha1.Kubeconfig) (clientutils.ObjectRefSet, error) {
	reqs := r.getRequests(kubeconfig)
	return clientutils.ObjectRefSetFromGetRequestSet(r.scheme, reqs)
}

// Resolve resolves a matryoshkav1alpha1.Kubeconfig to its required manifests.
func (r *Resolver) Resolve(ctx context.Context, kubeconfig *matryoshkav1alpha1.Kubeconfig) (*clientcmdapiv1.Config, error) {
	log := ctrl.LoggerFrom(ctx)

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

	log.V(1).Info("Building kubeconfig.")
	cfg, err := r.config(ctx, s, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error resolving kubeconfig to config: %w", err)
	}

	return cfg, nil
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
	for _, context := range kubeconfig.Spec.Contexts {
		contexts = append(contexts, clientcmdapiv1.NamedContext{
			Name: context.Name,
			Context: clientcmdapiv1.Context{
				Cluster:   context.Context.Cluster,
				AuthInfo:  context.Context.AuthInfo,
				Namespace: context.Context.Namespace,
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
	if clientCertificate := authInfo.ClientCertificate; clientCertificate != nil {
		var err error
		clientCertificateData, err = utils.GetSecretSelector(ctx, s, namespace, *clientCertificate.Secret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoClientCertificateKey)
		if err != nil {
			return nil, err
		}
	}

	var clientKeyData []byte
	if clientKey := authInfo.ClientKey; clientKey != nil {
		var err error
		clientKeyData, err = utils.GetSecretSelector(ctx, s, namespace, *clientKey.Secret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoClientKeyKey)
		if err != nil {
			return nil, err
		}
	}

	var token string
	if tok := authInfo.Token; tok != nil {
		var (
			tokenData []byte
			err       error
		)
		tokenData, err = utils.GetSecretSelector(ctx, s, namespace, *tok.Secret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoTokenKey)
		if err != nil {
			return nil, err
		}

		token = string(tokenData)
	}

	var password string
	if pwd := authInfo.Password; pwd != nil {
		var (
			passwordData []byte
			err          error
		)
		passwordData, err = utils.GetSecretSelector(ctx, s, namespace, *pwd.Secret, matryoshkav1alpha1.DefaultKubeconfigAuthInfoPasswordKey)
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
	var certificateAuthorityData []byte
	if certificateAuthority := cluster.CertificateAuthority; certificateAuthority != nil {
		var err error
		certificateAuthorityData, err = utils.GetSecretSelector(ctx, s, namespace, *certificateAuthority.Secret, matryoshkav1alpha1.DefaultKubeconfigClusterCertificateAuthorityKey)
		if err != nil {
			return nil, err
		}
	}

	return &clientcmdapiv1.Cluster{
		Server:                   cluster.Server,
		TLSServerName:            cluster.TLSServerName,
		InsecureSkipTLSVerify:    cluster.InsecureSkipTLSVerify,
		CertificateAuthorityData: certificateAuthorityData,
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
