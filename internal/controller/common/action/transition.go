package action

import (
	"context"
	"fmt"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/securesign/operator/internal/controller/constants"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


func NewToPending[T interface{client.Object}]() Action[T] {
	return &transition[T]{"", constants.Pending, metav1.ConditionFalse, BaseAction{}}
}

func NewToCreate[T interface{client.Object}]() Action[T] {
	return &transition[T]{constants.Pending, constants.Creating, metav1.ConditionFalse, BaseAction{}}
}

func NewToInitialize[T interface{client.Object}]() Action[T] {
	return &transition[T]{constants.Creating, constants.Initialize, metav1.ConditionFalse, BaseAction{}}
}

func NewToReady[T interface{client.Object}]() Action[T] {
	return &transition[T]{constants.Initialize, constants.Ready, metav1.ConditionTrue, BaseAction{}}
}

type transition[T interface{client.Object}] struct {
	from string
	to string
	condition metav1.ConditionStatus
	BaseAction
}

func (i *transition[T]) Name() string {
	return fmt.Sprintf("move to %s phase", i.to)
}

func (i *transition[T]) CanHandle(_ context.Context, instance T) bool {
	c := meta.FindStatusCondition(*i.getConditions(instance), constants.Ready)
	if  i.from == "" {
		return c == nil || c.Reason == i.from
	}
	if c == nil {
		return false
	}
	return c.Reason == i.from
}

func (i *transition[T]) Handle(ctx context.Context, instance T) *Result {

	meta.SetStatusCondition(i.getConditions(instance), metav1.Condition{
		Type: constants.Ready,
		Status: i.condition,
		Reason: i.to,
	})

	return i.StatusUpdate(ctx, client.Object(instance))
}

func (i *transition[T]) getConditions(instance T) *[]metav1.Condition{
	// Obtain the reflect.Value of the instance
	valueOfA := reflect.ValueOf(instance).Elem()

	// Access the Status field
	statusField := valueOfA.FieldByName("Status")

	// Ensure the field exists and is a struct
	if !statusField.IsValid() || statusField.Kind() != reflect.Struct {
		panic("Status field is invalid or not a struct")
	}

	// Access the Conditions field within the Status struct
	conditionsField := statusField.FieldByName("Conditions")

	// Ensure the field exists and is a slice
	if !conditionsField.IsValid() || conditionsField.Kind() != reflect.Slice {
		panic("Conditions field is invalid or not a slice")
	}

	// Obtain the pointer to the Conditions field
	conditionsFieldPtr := conditionsField.Addr().Interface()

	return conditionsFieldPtr.(*[]metav1.Condition)
}
