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

package matryoshka

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/onmetal/controller-utils/clientutils"
	condition "github.com/onmetal/controller-utils/conditionutils"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubescheduler"
	appsv1 "k8s.io/api/apps/v1"
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

// KubeSchedulerReconciler reconciles a KubeScheduler object
type KubeSchedulerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	resolver *kubescheduler.Resolver
}

//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeschedulers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeschedulers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeschedulers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *KubeSchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ks := &matryoshkav1alpha1.KubeScheduler{}
	if err := r.Get(ctx, req.NamespacedName, ks); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, ks)
}

func (r *KubeSchedulerReconciler) reconcileExists(ctx context.Context, log logr.Logger, ks *matryoshkav1alpha1.KubeScheduler) (ctrl.Result, error) {
	if !ks.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, ks)
	}
	return r.reconcile(ctx, log, ks)
}

func (r *KubeSchedulerReconciler) init() error {
	r.resolver = kubescheduler.NewResolver(r.Scheme, r.Client)
	return nil
}

func (r *KubeSchedulerReconciler) delete(ctx context.Context, log logr.Logger, ks *matryoshkav1alpha1.KubeScheduler) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *KubeSchedulerReconciler) fetchAndMirrorDeploymentStatus(ctx context.Context, ks *matryoshkav1alpha1.KubeScheduler) error {
	deploy := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(ks), deploy); err != nil {
		return err
	}

	r.mirrorDeploymentStatus(ks, deploy)
	return nil
}

func (r *KubeSchedulerReconciler) mirrorDeploymentStatus(ks *matryoshkav1alpha1.KubeScheduler, deploy *appsv1.Deployment) {
	for deployType, ourType := range deploymentMirrorConditionTypes {
		var (
			status  corev1.ConditionStatus
			reason  string
			message string
		)
		deployCond := &appsv1.DeploymentCondition{}
		if !condition.MustFindSlice(deploy.Status.Conditions, string(deployType), deployCond) {
			status = corev1.ConditionUnknown
			reason = "Unknown"
			message = "The deployment managed by the kube scheduler did not report any information yet"
		} else {
			status = deployCond.Status
			reason = deployCond.Reason
			message = deployCond.Message
		}
		condition.MustUpdateSlice(&ks.Status.Conditions, string(ourType),
			condition.UpdateStatus(status),
			condition.UpdateReason(reason),
			condition.UpdateMessage(message),
			condition.UpdateObserved(ks),
		)
	}
	ks.Status.Replicas = deploy.Status.Replicas
	ks.Status.ReadyReplicas = deploy.Status.ReadyReplicas
	ks.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	ks.Status.UnavailableReplicas = deploy.Status.UnavailableReplicas
	ks.Status.UpdatedReplicas = deploy.Status.UpdatedReplicas
	ks.Status.ObservedGeneration = ks.Generation
}

func (r *KubeSchedulerReconciler) reconcile(ctx context.Context, log logr.Logger, ks *matryoshkav1alpha1.KubeScheduler) (ctrl.Result, error) {
	log.V(1).Info("Building deployment")
	deploy, err := r.resolver.Resolve(ctx, ks)
	if err != nil {
		condition.MustUpdateSlice(&ks.Status.Conditions, string(matryoshkav1alpha1.KubeSchedulerDeploymentFailure),
			condition.UpdateStatus(corev1.ConditionTrue),
			condition.UpdateReason("BuildError"),
			condition.UpdateMessage(fmt.Sprintf("Building the deployment for the kube scheduler resulted in an error: %v", err)),
			condition.UpdateObserved(ks),
		)
		if err := r.fetchAndMirrorDeploymentStatus(ctx, ks); err != nil {
			log.Error(err, "Could not fetch and mirror deployment conditions")
		}
		if err := r.Status().Update(ctx, ks); err != nil {
			log.Error(err, "Could not update status")
		}
		return ctrl.Result{}, fmt.Errorf("error building kube scheduler deployment: %w", err)
	}

	log.V(1).Info("Applying kube scheduler deployment")
	if err := r.Patch(ctx, deploy, client.Apply, client.FieldOwner(matryoshkav1alpha1.KubeSchedulerFieldManager)); err != nil {
		condition.MustUpdateSlice(&ks.Status.Conditions, string(matryoshkav1alpha1.KubeSchedulerDeploymentFailure),
			condition.UpdateStatus(corev1.ConditionUnknown),
			condition.UpdateReason("ApplyError"),
			condition.UpdateMessage(fmt.Sprintf("Building the deployment for the kube scheduler resulted in an error: %v", err)),
			condition.UpdateObserved(ks),
		)
		if err := r.fetchAndMirrorDeploymentStatus(ctx, ks); err != nil {
			log.Error(err, "Could not fetch and mirror deployment conditions")
		}
		if err := r.Status().Update(ctx, ks); err != nil {
			log.Error(err, "Could not update status")
		}
		return ctrl.Result{}, fmt.Errorf("error applying kube scheduler: %w", err)
	}

	log.V(1).Info("Updating kube scheduler status")
	condition.MustUpdateSlice(&ks.Status.Conditions, string(matryoshkav1alpha1.KubeSchedulerDeploymentFailure),
		condition.UpdateStatus(corev1.ConditionFalse),
		condition.UpdateReason("Applied"),
		condition.UpdateMessage("The kube scheduler deployment has been applied successfully."),
		condition.UpdateObserved(ks),
	)
	r.mirrorDeploymentStatus(ks, deploy)
	if err := r.Status().Update(ctx, ks); err != nil {
		return ctrl.Result{}, fmt.Errorf("could not update status: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeSchedulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.init(); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&matryoshkav1alpha1.KubeScheduler{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueReferencingKubeSchedulers),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func (r *KubeSchedulerReconciler) enqueueReferencingKubeSchedulers(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	log := ctrl.Log.WithName("kubescheduler").WithName("enqueueReferencingKubeSchedulers")

	list := &matryoshkav1alpha1.KubeSchedulerList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Could not list objects in namespace", "namespace", obj.GetNamespace())
		return nil
	}

	var requests []reconcile.Request
	for _, ks := range list.Items {
		log := log.WithValues("kubescheduler", client.ObjectKeyFromObject(&ks))
		refs, err := r.resolver.ObjectReferences(&ks)
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

		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&ks)})
	}
	return requests
}
