package kubeconfig

import (
	"context"
	"fmt"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/pkg/utils"
	"github.com/onmetal/matryoshka/pkg/utils/multigetter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resolver struct {
	scheme *runtime.Scheme
	client client.Client
	mg     *multigetter.Multigetter
}

func (r *Resolver) UsesObject(kubeconfig *matryoshkav1alpha1.Kubeconfig, obj client.Object) (bool, error) {
	acc := multigetter.NewAccumulator(r.scheme)
	if err := registerKubeconfigRequests(acc, kubeconfig); err != nil {
		return false, err
	}
	return acc.Has(client.ObjectKeyFromObject(obj), obj)
}

func registerKubeconfigRequests(acc *multigetter.RequestAccumulator, kubeconfig *matryoshkav1alpha1.Kubeconfig) error {
	for _, authInfo := range kubeconfig.Spec.AuthInfos {
		if err := registerAuthInfoRequests(acc, kubeconfig.Namespace, &authInfo.AuthInfo); err != nil {
			return err
		}
	}
	for _, cluster := range kubeconfig.Spec.Clusters {
		if err := registerClusterRequests(acc, kubeconfig.Namespace, &cluster.Cluster); err != nil {
			return err
		}
	}
	return nil
}

func registerClusterRequests(acc *multigetter.RequestAccumulator, namespace string, cluster *matryoshkav1alpha1.Cluster) error {
	if certificateAuthority := cluster.CertificateAuthority; certificateAuthority != nil {
		if err := acc.Add(client.ObjectKey{Namespace: namespace, Name: certificateAuthority.Secret.Name}, &corev1.Secret{}); err != nil {
			return err
		}
	}

	return nil
}

func registerAuthInfoRequests(acc *multigetter.RequestAccumulator, namespace string, authInfo *matryoshkav1alpha1.AuthInfo) error {
	if clientCertificate := authInfo.ClientCertificate; clientCertificate != nil {
		if err := acc.Add(client.ObjectKey{Namespace: namespace, Name: clientCertificate.Secret.Name}, &corev1.Secret{}); err != nil {
			return err
		}
	}

	if clientKey := authInfo.ClientKey; clientKey != nil {
		if err := acc.Add(client.ObjectKey{Namespace: namespace, Name: clientKey.Secret.Name}, &corev1.Secret{}); err != nil {
			return err
		}
	}

	if token := authInfo.Token; token != nil {
		if err := acc.Add(client.ObjectKey{Namespace: namespace, Name: token.Secret.Name}, &corev1.Secret{}); err != nil {
			return err
		}
	}

	if password := authInfo.Password; password != nil {
		if err := acc.Add(client.ObjectKey{Namespace: namespace, Name: password.Secret.Name}, &corev1.Secret{}); err != nil {
			return err
		}
	}

	return nil
}

func (r *Resolver) clientForKubeconfig(ctx context.Context, kubeconfig *matryoshkav1alpha1.Kubeconfig) (client.Client, error) {
	acc := multigetter.NewAccumulator(r.scheme)
	if err := registerKubeconfigRequests(acc, kubeconfig); err != nil {
		return nil, fmt.Errorf("error registering kubeconfig requests: %w", err)
	}

	objs, err := acc.Resolve(ctx, r.mg)
	if err != nil {
		return nil, err
	}

	return multigetter.ObjectsClientOverlay(r.client, objs), nil
}

func (r *Resolver) Resolve(ctx context.Context, kubeconfig *matryoshkav1alpha1.Kubeconfig) (*clientcmdapiv1.Config, error) {
	c, err := r.clientForKubeconfig(ctx, kubeconfig)
	if err != nil {
		return nil, err
	}

	return resolveKubeconfig(ctx, c, kubeconfig)
}

func resolveKubeconfig(ctx context.Context, c client.Client, kubeconfig *matryoshkav1alpha1.Kubeconfig) (*clientcmdapiv1.Config, error) {
	authInfos := make([]clientcmdapiv1.NamedAuthInfo, 0, len(kubeconfig.Spec.AuthInfos))
	for _, authInfo := range kubeconfig.Spec.AuthInfos {
		resolved, err := resolveAuthInfo(ctx, c, kubeconfig.Namespace, &authInfo.AuthInfo)
		if err != nil {
			return nil, err
		}

		authInfos = append(authInfos, clientcmdapiv1.NamedAuthInfo{Name: authInfo.Name, AuthInfo: *resolved})
	}

	clusters := make([]clientcmdapiv1.NamedCluster, 0, len(kubeconfig.Spec.Clusters))
	for _, cluster := range kubeconfig.Spec.Clusters {
		resolved, err := resolveCluster(ctx, c, kubeconfig.Namespace, &cluster.Cluster)
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

func resolveAuthInfo(
	ctx context.Context,
	c client.Client,
	namespace string,
	authInfo *matryoshkav1alpha1.AuthInfo,
) (*clientcmdapiv1.AuthInfo, error) {
	var clientCertificateData []byte
	if clientCertificate := authInfo.ClientCertificate; clientCertificate != nil {
		var err error
		clientCertificateData, err = utils.GetSecretSelector(ctx, c, namespace, *clientCertificate.Secret, matryoshkav1alpha1.DefaultAuthInfoClientCertificateKey)
		if err != nil {
			return nil, err
		}
	}

	var clientKeyData []byte
	if clientKey := authInfo.ClientKey; clientKey != nil {
		var err error
		clientKeyData, err = utils.GetSecretSelector(ctx, c, namespace, *clientKey.Secret, matryoshkav1alpha1.DefaultAuthInfoClientKeyKey)
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
		tokenData, err = utils.GetSecretSelector(ctx, c, namespace, *tok.Secret, matryoshkav1alpha1.DefaultAuthInfoTokenKey)
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
		passwordData, err = utils.GetSecretSelector(ctx, c, namespace, *pwd.Secret, matryoshkav1alpha1.DefaultAuthInfoPasswordKey)
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

func resolveCluster(ctx context.Context, c client.Client, namespace string, cluster *matryoshkav1alpha1.Cluster) (*clientcmdapiv1.Cluster, error) {
	var certificateAuthorityData []byte
	if certificateAuthority := cluster.CertificateAuthority; certificateAuthority != nil {
		var err error
		certificateAuthorityData, err = utils.GetSecretSelector(ctx, c, namespace, *certificateAuthority.Secret, matryoshkav1alpha1.DefaultClusterCertificateAuthorityKey)
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

type ResolverOptions struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (o *ResolverOptions) Validate() error {
	if o.Client == nil {
		return fmt.Errorf("client needs to be set")
	}
	if o.Scheme == nil {
		return fmt.Errorf("scheme needs to be set")
	}
	return nil
}

func NewResolver(opts ResolverOptions) (*Resolver, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	mg, err := multigetter.New(multigetter.Options{
		Scheme:    opts.Scheme,
		Client:    opts.Client,
		Threshold: multigetter.DefaultThreshold,
	})
	if err != nil {
		return nil, err
	}

	return &Resolver{
		scheme: opts.Scheme,
		client: opts.Client,
		mg:     mg,
	}, nil
}
