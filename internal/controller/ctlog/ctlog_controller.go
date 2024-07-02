/*
Copyright 2023.

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

package ctlog

import (
	"context"
	olpredicate "github.com/operator-framework/operator-lib/predicate"
	"github.com/securesign/operator/internal/controller/annotations"
	"github.com/securesign/operator/internal/controller/constants"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/securesign/operator/internal/controller/ctlog/actions"
	actions2 "github.com/securesign/operator/internal/controller/fulcio/actions"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/securesign/operator/internal/controller/common/action"
	v1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
)


var Actions = []action.Action[*rhtasv1alpha1.CTlog]{
	// register error handler
	actions.NewHandleErrorAction(),

	action.NewToPending[*rhtasv1alpha1.CTlog](),

	actions.NewPendingAction(),

	action.NewToCreate[*rhtasv1alpha1.CTlog](),

	actions.NewHandleFulcioCertAction(),
	actions.NewHandleKeysAction(),
	actions.NewCreateTrillianTreeAction(),
	actions.NewServerConfigAction(),

	actions.NewRBACAction(),
	actions.NewDeployAction(),
	actions.NewServiceAction(),
	actions.NewCreateMonitorAction(),

	action.NewToInitialize[*rhtasv1alpha1.CTlog](),

	actions.NewInitializeAction(),

	action.NewToReady[*rhtasv1alpha1.CTlog](),
}

//+kubebuilder:rbac:groups=rhtas.redhat.com,resources=ctlogs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rhtas.redhat.com,resources=ctlogs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rhtas.redhat.com,resources=ctlogs/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(r reconcile.Reconciler, mgr ctrl.Manager) error {
	// Filter out with the pause annotation.
	pause, err := olpredicate.NewPause(annotations.PausedReconciliation)
	if err != nil {
		return err
	}

	secretPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{
			Key:      actions2.FulcioCALabel,
			Operator: metav1.LabelSelectorOpExists,
		},
	}})
	if err != nil {
		return err
	}

	partialSecret := &metav1.PartialObjectMetadata{}
	partialSecret.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Secret",
	})

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pause).
		For(&rhtasv1alpha1.CTlog{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.Funcs{UpdateFunc: func(event event.UpdateEvent) bool {
			// do not requeue failed object updates
			instance, ok := event.ObjectNew.(*rhtasv1alpha1.CTlog)
			if !ok {
				return false
			}
			if c := meta.FindStatusCondition(instance.Status.Conditions, constants.Ready); c != nil {
				return c.Reason != constants.Failure
			}
			return true
		}}))).
		Owns(&v1.Deployment{}).
		Owns(&v12.Service{}).
		WatchesMetadata(partialSecret, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			val, ok := object.GetLabels()["app.kubernetes.io/instance"]
			if ok {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: object.GetNamespace(),
							Name:      val,
						},
					},
				}
			}

			list := &rhtasv1alpha1.CTlogList{}
			mgr.GetClient().List(ctx, list, client.InNamespace(object.GetNamespace()))
			requests := make([]reconcile.Request, len(list.Items))
			for i, k := range list.Items {
				requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: k.Name}}
			}
			return requests

		}), builder.WithPredicates(secretPredicate)).
		Complete(r)
}
