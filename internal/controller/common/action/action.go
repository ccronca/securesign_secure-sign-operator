package action

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Result struct {
	Result reconcile.Result
	Err    error
}

type Action[T interface{client.Object}] interface {
	InjectClient(client client.Client)
	InjectRecorder(recorder record.EventRecorder)
	InjectLogger(logger logr.Logger)

	// a user friendly name for the action
	Name() string

	// returns true if the action can handle the integration
	CanHandle(context.Context, T) bool

	// executes the handling function
	Handle(context.Context, T) *Result
}


type HandleFailure[T interface{}] interface {
	CanHandleFailure(context.Context, T) bool
	HandleFailure(context.Context, T) *Result
}