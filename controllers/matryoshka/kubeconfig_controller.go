/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package matryoshka

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/pkg/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KubeconfigReconciler reconciles a Kubeconfig object
type KubeconfigReconciler struct {
	client.Client
	resolver *kubeconfig.Resolver
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kubeconfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *KubeconfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	kubeconfig := &matryoshkav1alpha1.Kubeconfig{}
	if err := r.Get(ctx, req.NamespacedName, kubeconfig); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, kubeconfig)
}

func (r *KubeconfigReconciler) reconcileExists(ctx context.Context, log logr.Logger, kubeconfig *matryoshkav1alpha1.Kubeconfig) (ctrl.Result, error) {
	if !kubeconfig.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, kubeconfig)
	}
	return r.reconcile(ctx, log, kubeconfig)
}

func (r *KubeconfigReconciler) reconcile(ctx context.Context, log logr.Logger, kubeconfig *matryoshkav1alpha1.Kubeconfig) (ctrl.Result, error) {
	log.V(1).Info("Resolving kubeconfig")
	config, err := r.resolver.Resolve(ctx, kubeconfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not resolve kubeconfig: %w", err)
	}

	log.V(1).Info("Encoding config")
	var b bytes.Buffer
	if err := latest.Codec.Encode(config, &b); err != nil {
		return ctrl.Result{}, fmt.Errorf("error encoding config: %w", err)
	}

	log.V(1).Info("Writing secret")
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
			matryoshkav1alpha1.DefaultKubeconfigKey: b.Bytes(),
		},
	}
	if err := ctrl.SetControllerReference(kubeconfig, secret, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("error setting secret owner reference: %w", err)
	}
	if err := r.Patch(ctx, secret, client.Apply, client.FieldOwner(matryoshkav1alpha1.KubeconfigFieldManager)); err != nil {
		return ctrl.Result{}, fmt.Errorf("error applying secret: %w", err)
	}

	log.V(1).Info("Successfully applied secret")
	return ctrl.Result{}, nil
}

func (r *KubeconfigReconciler) delete(ctx context.Context, log logr.Logger, kubeconfig *matryoshkav1alpha1.Kubeconfig) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *KubeconfigReconciler) init() error {
	var err error
	r.resolver, err = kubeconfig.NewResolver(kubeconfig.ResolverOptions{
		Client: r.Client,
		Scheme: r.Scheme,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *KubeconfigReconciler) enqueueRequestsForUsingKubeconfigs(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	log := ctrl.LoggerFrom(ctx, "triggerGVK", obj.GetObjectKind().GroupVersionKind(), "triggerKey", client.ObjectKeyFromObject(obj))
	kubeconfigs := &matryoshkav1alpha1.KubeconfigList{}
	if err := r.List(ctx, kubeconfigs, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, kubeconfig := range kubeconfigs.Items {
		kubeconfigKey := client.ObjectKeyFromObject(&kubeconfig)
		log := log.WithValues("kubeconfig", kubeconfigKey)

		ok, err := r.resolver.UsesObject(&kubeconfig, obj)
		if err != nil {
			log.Error(err, "Error determining uses")
			continue
		}

		if ok {
			requests = append(requests, reconcile.Request{NamespacedName: kubeconfigKey})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeconfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.init(); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&matryoshkav1alpha1.Kubeconfig{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRequestsForUsingKubeconfigs),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRequestsForUsingKubeconfigs),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&corev1.Secret{}).
		Complete(r)
}
