package reconciler

import (
	"context"
	"github.com/securesign/operator/internal/controller/common/action"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type supplier [T any]func () T

type Reconciler[T interface{client.Object}] struct {
	client client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	actions  []action.Action[T]
	supplier supplier[T]
}

func New[T interface{client.Object}](client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, actions []action.Action[T], supplier supplier[T]) *Reconciler[T] {
	return &Reconciler[T]{client, scheme, recorder, actions, supplier}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CTlog object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (t *Reconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := t.supplier()
	rlog := log.FromContext(ctx)
	rlog.V(1).Info("Reconciling CTlog", "request", req)

	if err := t.client.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	target := instance.DeepCopyObject().(T)

	for _, a := range t.actions {
		rlog.V(2).Info("Executing " + a.Name())
		a.InjectClient(t.client)
		a.InjectLogger(rlog.WithName(a.Name()))
		a.InjectRecorder(t.recorder)

		if f, ok := a.(action.HandleFailure[T]); ok {
			if f.CanHandleFailure(ctx, target) {
				result := f.HandleFailure(ctx, target)
				if result != nil {
					return result.Result, result.Err
				}
			}
		}

		if a.CanHandle(ctx, target) {
			rlog.V(1).Info("Executing " + a.Name())
			result := a.Handle(ctx, target)
			if result != nil {
				return result.Result, result.Err
			}
		}
	}
	return reconcile.Result{}, nil
}
