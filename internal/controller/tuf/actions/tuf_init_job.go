package actions

import (
	"context"
	"fmt"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	"github.com/securesign/operator/internal/controller/tuf/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewInitJobAction() action.Action[*rhtasv1alpha1.Tuf] {
	return &initJobAction{}
}

type initJobAction struct {
	action.BaseAction
}

func (i initJobAction) Name() string {
	return "tuf-init job"
}

func (i initJobAction) CanHandle(_ context.Context, instance *rhtasv1alpha1.Tuf) bool {
	c := meta.FindStatusCondition(instance.GetConditions(), constants.Ready)
	return c.Reason == constants.Creating && !meta.IsStatusConditionTrue(instance.GetConditions(), RepositoryCondition)
}

func (i initJobAction) Handle(ctx context.Context, instance *rhtasv1alpha1.Tuf) *action.Result {
	if job, err := kubernetes.GetJob(ctx, i.Client, instance.Namespace, InitJobName); job != nil {
		i.Logger.Info("Tuf tuf-repository-init is already present.", "Succeeded", job.Status.Succeeded, "Failures", job.Status.Failed)
		if kubernetes.IsCompleted(*job) {
			if !kubernetes.IsFailed(*job) {
				meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
					Type:    RepositoryCondition,
					Status:  metav1.ConditionTrue,
					Reason:  constants.Ready,
					Message: "tuf-repository-init job passed",
				})
				return i.StatusUpdate(ctx, instance)
			} else {
				err = fmt.Errorf("tuf-repository-init job failed")
				meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
					Type:    RepositoryCondition,
					Status:  metav1.ConditionFalse,
					Reason:  constants.Failure,
					Message: err.Error(),
				})
				meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
					Type:    constants.Ready,
					Status:  metav1.ConditionFalse,
					Reason:  constants.Failure,
					Message: err.Error(),
				})
				return i.FailedWithStatusUpdate(ctx, err, instance)
			}
		} else {
			// job not completed yet
			return i.Requeue()
		}
	} else if client.IgnoreNotFound(err) != nil {
		return i.Failed(err)

	}

	job := utils.CreateTufInitJob(instance, InitJobName, RBACName, constants.LabelsForComponent(ComponentName, instance.Name))
	if err := i.Client.Create(ctx, job); err != nil {
		return i.Failed(err)
	}

	return i.Requeue()
}
