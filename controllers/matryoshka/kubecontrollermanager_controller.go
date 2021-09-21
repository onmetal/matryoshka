/*
 * Copyright (c) 2021 by the OnMetal authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package matryoshka

import (
	"context"
	"fmt"

	"github.com/onmetal/controller-utils/clientutils"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	condition "github.com/onmetal/controller-utils/conditionutils"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubecontrollermanager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"

	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeControllerManagerReconciler reconciles a KubeControllerManager object
type KubeControllerManagerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	resolver *kubecontrollermanager.Resolver
}

//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubecontrollermanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubecontrollermanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubecontrollermanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps/v1,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=v1,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=v1,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *KubeControllerManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	kcm := &matryoshkav1alpha1.KubeControllerManager{}
	if err := r.Get(ctx, req.NamespacedName, kcm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, kcm)
}

func (r *KubeControllerManagerReconciler) reconcileExists(ctx context.Context, log logr.Logger, kcm *matryoshkav1alpha1.KubeControllerManager) (ctrl.Result, error) {
	if !kcm.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, kcm)
	}
	return r.reconcile(ctx, log, kcm)
}

func (r *KubeControllerManagerReconciler) init() error {
	r.resolver = kubecontrollermanager.NewResolver(r.Scheme, r.Client)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeControllerManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.init(); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&matryoshkav1alpha1.KubeControllerManager{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueReferencingKubeControllerManagers),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func (r *KubeControllerManagerReconciler) delete(ctx context.Context, log logr.Logger, kcm *matryoshkav1alpha1.KubeControllerManager) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *KubeControllerManagerReconciler) fetchAndMirrorDeploymentStatus(ctx context.Context, kcm *matryoshkav1alpha1.KubeControllerManager) error {
	deploy := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(kcm), deploy); err != nil {
		return err
	}

	r.mirrorDeploymentStatus(kcm, deploy)
	return nil
}

func (r *KubeControllerManagerReconciler) mirrorDeploymentStatus(kcm *matryoshkav1alpha1.KubeControllerManager, deploy *appsv1.Deployment) {
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
			message = "The deployment managed by the kube controller manager did not report any information yet"
		} else {
			status = deployCond.Status
			reason = deployCond.Reason
			message = deployCond.Message
		}
		condition.MustUpdateSlice(&kcm.Status.Conditions, string(ourType),
			condition.UpdateStatus(status),
			condition.UpdateReason(reason),
			condition.UpdateMessage(message),
			condition.UpdateObserved(kcm),
		)
	}
	kcm.Status.Replicas = deploy.Status.Replicas
	kcm.Status.ReadyReplicas = deploy.Status.ReadyReplicas
	kcm.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	kcm.Status.UnavailableReplicas = deploy.Status.UnavailableReplicas
	kcm.Status.UpdatedReplicas = deploy.Status.UpdatedReplicas
	kcm.Status.ObservedGeneration = kcm.Generation
}

func (r *KubeControllerManagerReconciler) reconcile(ctx context.Context, log logr.Logger, kcm *matryoshkav1alpha1.KubeControllerManager) (ctrl.Result, error) {
	log.V(1).Info("Building deployment")
	deploy, err := r.resolver.Resolve(ctx, kcm)
	if err != nil {
		condition.MustUpdateSlice(&kcm.Status.Conditions, string(matryoshkav1alpha1.KubeControllerManagerDeploymentFailure),
			condition.UpdateStatus(corev1.ConditionTrue),
			condition.UpdateReason("BuildError"),
			condition.UpdateMessage(fmt.Sprintf("Building the deployment for the kube controller manager resulted in an error: %v", err)),
			condition.UpdateObserved(kcm),
		)
		if err := r.fetchAndMirrorDeploymentStatus(ctx, kcm); err != nil {
			log.Error(err, "Could not fetch and mirror deployment conditions")
		}
		if err := r.Status().Update(ctx, kcm); err != nil {
			log.Error(err, "Could not update status")
		}
		return ctrl.Result{}, fmt.Errorf("error building kube controller manager deployment: %w", err)
	}

	log.V(1).Info("Applying kube controller manager deployment")
	if err := r.Patch(ctx, deploy, client.Apply, client.FieldOwner(matryoshkav1alpha1.KubeControllerManagerFieldManager)); err != nil {
		condition.MustUpdateSlice(&kcm.Status.Conditions, string(matryoshkav1alpha1.KubeControllerManagerDeploymentFailure),
			condition.UpdateStatus(corev1.ConditionUnknown),
			condition.UpdateReason("ApplyError"),
			condition.UpdateMessage(fmt.Sprintf("Building the deployment for the kube controller manager resulted in an error: %v", err)),
			condition.UpdateObserved(kcm),
		)
		if err := r.fetchAndMirrorDeploymentStatus(ctx, kcm); err != nil {
			log.Error(err, "Could not fetch and mirror deployment conditions")
		}
		if err := r.Status().Update(ctx, kcm); err != nil {
			log.Error(err, "Could not update status")
		}
		return ctrl.Result{}, fmt.Errorf("error applying kube controller manager: %w", err)
	}

	log.V(1).Info("Updating kube controller manager status")
	condition.MustUpdateSlice(&kcm.Status.Conditions, string(matryoshkav1alpha1.KubeControllerManagerDeploymentFailure),
		condition.UpdateStatus(corev1.ConditionFalse),
		condition.UpdateReason("Applied"),
		condition.UpdateMessage("The kube controller manager deployment has been applied successfully."),
		condition.UpdateObserved(kcm),
	)
	r.mirrorDeploymentStatus(kcm, deploy)
	if err := r.Status().Update(ctx, kcm); err != nil {
		return ctrl.Result{}, fmt.Errorf("could not update status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *KubeControllerManagerReconciler) enqueueReferencingKubeControllerManagers(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	log := ctrl.Log.WithName("kubecontrollermanager").WithName("enqueueReferencingKubeControllerManagers")

	list := &matryoshkav1alpha1.KubeControllerManagerList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Could not list objects in namespace", "namespace", obj.GetNamespace())
		return nil
	}

	var requests []reconcile.Request
	for _, kcm := range list.Items {
		log := log.WithValues("kubecontrollermanager", client.ObjectKeyFromObject(&kcm))
		refs, err := r.resolver.ObjectReferences(&kcm)
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

		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&kcm)})
	}
	return requests
}
