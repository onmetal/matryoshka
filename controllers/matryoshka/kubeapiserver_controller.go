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

	"github.com/go-logr/logr"
	"github.com/onmetal/controller-utils/clientutils"
	condition "github.com/onmetal/controller-utils/conditionutils"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubeapiserver"
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

// KubeAPIServerReconciler reconciles a KubeAPIServer object
type KubeAPIServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	resolver *kubeapiserver.Resolver
}

//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeapiservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeapiservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=matryoshka.onmetal.de,resources=kubeapiservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps/v1,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=v1,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=v1,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *KubeAPIServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	apiServer := &matryoshkav1alpha1.KubeAPIServer{}
	if err := r.Get(ctx, req.NamespacedName, apiServer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, apiServer)
}

func (r *KubeAPIServerReconciler) reconcileExists(ctx context.Context, log logr.Logger, server *matryoshkav1alpha1.KubeAPIServer) (ctrl.Result, error) {
	if !server.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, server)
	}
	return r.reconcile(ctx, log, server)
}

func (r *KubeAPIServerReconciler) delete(ctx context.Context, log logr.Logger, server *matryoshkav1alpha1.KubeAPIServer) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *KubeAPIServerReconciler) fetchAndMirrorDeploymentStatus(ctx context.Context, server *matryoshkav1alpha1.KubeAPIServer) error {
	deploy := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(server), deploy); err != nil {
		return err
	}

	r.mirrorDeploymentStatus(server, deploy)
	return nil
}

var deploymentMirrorConditionTypes = map[appsv1.DeploymentConditionType]matryoshkav1alpha1.KubeAPIServerConditionType{
	appsv1.DeploymentAvailable:   matryoshkav1alpha1.KubeAPIServerAvailable,
	appsv1.DeploymentProgressing: matryoshkav1alpha1.KubeAPIServerProgressing,
}

func (r *KubeAPIServerReconciler) mirrorDeploymentStatus(server *matryoshkav1alpha1.KubeAPIServer, deploy *appsv1.Deployment) {
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
			message = "The deployment managed by the api server did not report any information yet"
		} else {
			status = deployCond.Status
			reason = deployCond.Reason
			message = deployCond.Message
		}
		condition.MustUpdateSlice(&server.Status.Conditions, string(ourType),
			condition.UpdateStatus(status),
			condition.UpdateReason(reason),
			condition.UpdateMessage(message),
			condition.UpdateObserved(server),
		)
	}
	server.Status.Replicas = deploy.Status.Replicas
	server.Status.ReadyReplicas = deploy.Status.ReadyReplicas
	server.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	server.Status.UnavailableReplicas = deploy.Status.UnavailableReplicas
	server.Status.UpdatedReplicas = deploy.Status.UpdatedReplicas
	server.Status.ObservedGeneration = server.Generation
}

func (r *KubeAPIServerReconciler) reconcile(ctx context.Context, log logr.Logger, server *matryoshkav1alpha1.KubeAPIServer) (ctrl.Result, error) {
	log.V(1).Info("Building api server deployment")
	deploy, err := r.resolver.Resolve(ctx, server)
	if err != nil {
		condition.MustUpdateSlice(&server.Status.Conditions, string(matryoshkav1alpha1.KubeAPIServerDeploymentFailure),
			condition.UpdateStatus(corev1.ConditionTrue),
			condition.UpdateReason("BuildError"),
			condition.UpdateMessage(fmt.Sprintf("Building the deployment for the api server resulted in an error: %v", err)),
			condition.UpdateObserved(server),
		)
		if err := r.fetchAndMirrorDeploymentStatus(ctx, server); err != nil {
			log.Error(err, "Could not fetch and mirror deployment conditions")
		}
		if err := r.Status().Update(ctx, server); err != nil {
			log.Error(err, "Could not update status")
		}
		return ctrl.Result{}, fmt.Errorf("error building api server deployment: %w", err)
	}

	log.V(1).Info("Applying api server deployment")
	if err := r.Patch(ctx, deploy, client.Apply, client.FieldOwner(matryoshkav1alpha1.KubeAPIServerFieldManager)); err != nil {
		condition.MustUpdateSlice(&server.Status.Conditions, string(matryoshkav1alpha1.KubeAPIServerDeploymentFailure),
			condition.UpdateStatus(corev1.ConditionUnknown),
			condition.UpdateReason("ApplyError"),
			condition.UpdateMessage(fmt.Sprintf("Building the deployment for the api server resulted in an error: %v", err)),
			condition.UpdateObserved(server),
		)
		if err := r.fetchAndMirrorDeploymentStatus(ctx, server); err != nil {
			log.Error(err, "Could not fetch and mirror deployment conditions")
		}
		if err := r.Status().Update(ctx, server); err != nil {
			log.Error(err, "Could not update status")
		}
		return ctrl.Result{}, fmt.Errorf("error applying api server: %w", err)
	}

	log.V(1).Info("Updating api server status")
	condition.MustUpdateSlice(&server.Status.Conditions, string(matryoshkav1alpha1.KubeAPIServerDeploymentFailure),
		condition.UpdateStatus(corev1.ConditionFalse),
		condition.UpdateReason("Applied"),
		condition.UpdateMessage("The api server deployment has been applied successfully."),
		condition.UpdateObserved(server),
	)
	r.mirrorDeploymentStatus(server, deploy)
	if err := r.Status().Update(ctx, server); err != nil {
		return ctrl.Result{}, fmt.Errorf("could not update status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *KubeAPIServerReconciler) enqueueReferencingAPIServers(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	log := ctrl.Log.WithName("kubeapiserver").WithName("enqueueReferencingAPIServers")

	list := &matryoshkav1alpha1.KubeAPIServerList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Could not list api servers in namespace", "namespace", obj.GetNamespace())
		return nil
	}

	var requests []reconcile.Request
	for _, apiServer := range list.Items {
		log := log.WithValues("kubeapiserver", client.ObjectKeyFromObject(&apiServer))
		refs, err := r.resolver.ObjectReferences(&apiServer)
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

		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&apiServer)})
	}
	return requests
}

func (r *KubeAPIServerReconciler) init() error {
	var err error
	r.resolver, err = kubeapiserver.NewResolver(kubeapiserver.ResolverOptions{
		Scheme: r.Scheme,
		Client: r.Client,
	})
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeAPIServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.init(); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&matryoshkav1alpha1.KubeAPIServer{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueReferencingAPIServers),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
