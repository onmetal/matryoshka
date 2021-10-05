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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/onmetal/controller-utils/clientutils"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	Scheme   *runtime.Scheme
	resolver *kubeconfig.Resolver
}

//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KubeconfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	kc := &matryoshkav1alpha1.Kubeconfig{}
	if err := r.Get(ctx, req.NamespacedName, kc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, kc)
}

func (r *KubeconfigReconciler) reconcileExists(ctx context.Context, log logr.Logger, kc *matryoshkav1alpha1.Kubeconfig) (ctrl.Result, error) {
	if !kc.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, kc)
	}
	return r.reconcile(ctx, log, kc)
}

func (r *KubeconfigReconciler) reconcile(ctx context.Context, log logr.Logger, kc *matryoshkav1alpha1.Kubeconfig) (ctrl.Result, error) {
	log.V(1).Info("Resolving kubeconfig manifests")
	secret, err := r.resolver.Resolve(ctx, kc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not resolve kubeconfig: %w", err)
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
	r.resolver = kubeconfig.NewResolver(r.Scheme, r.Client)
	return nil
}

func (r *KubeconfigReconciler) enqueueReferencingKubeconfigs(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	log := ctrl.Log.WithName("kubeconfig").WithName("enqueueReferencingKubeconfigs")
	kubeconfigs := &matryoshkav1alpha1.KubeconfigList{}
	if err := r.List(ctx, kubeconfigs, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Could not list kubeconfigs in namespace", "namespace", obj.GetNamespace())
		return nil
	}

	var requests []reconcile.Request
	for _, kc := range kubeconfigs.Items {
		key := client.ObjectKeyFromObject(&kc)
		log := log.WithValues("kubeconfig", key)

		refs, err := r.resolver.ObjectReferences(&kc)
		if err != nil {
			log.Error(err, "Error determining references")
			continue
		}

		if ok, err := clientutils.ObjectRefSetReferencesObject(r.Scheme, refs, obj); err != nil || !ok {
			if err != nil {
				log.Error(err, "Error determining object is referenced")
			}
			continue
		}

		requests = append(requests, reconcile.Request{NamespacedName: key})
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
			handler.EnqueueRequestsFromMapFunc(r.enqueueReferencingKubeconfigs),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueReferencingKubeconfigs),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&corev1.Secret{}).
		Complete(r)
}
