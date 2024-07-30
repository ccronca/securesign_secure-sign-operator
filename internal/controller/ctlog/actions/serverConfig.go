package actions

import (
	"context"
	"errors"
	"fmt"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	utils "github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	ctlogUtils "github.com/securesign/operator/internal/controller/ctlog/utils"
	"github.com/securesign/operator/internal/controller/rekor/actions"
	trillian "github.com/securesign/operator/internal/controller/trillian/actions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	CTLPubLabel = constants.LabelNamespace + "/ctfe.pub"
)

func NewServerConfigAction() action.Action[*rhtasv1alpha1.CTlog] {
	return &serverConfig{}
}

type serverConfig struct {
	action.BaseAction
}

func (i serverConfig) Name() string {
	return "create server config"
}

func (i serverConfig) CanHandle(_ context.Context, instance *rhtasv1alpha1.CTlog) bool {
	c := meta.FindStatusCondition(instance.Status.Conditions, constants.Ready)
	return c.Reason == constants.Creating && instance.Status.ServerConfigRef == nil
}

func (i serverConfig) Handle(ctx context.Context, instance *rhtasv1alpha1.CTlog) *action.Result {
	var (
		err error
	)
	switch {
	case instance.Status.TreeID == nil:
		return i.Error(errors.New("reference to Trillian TreeID not set"))
	case instance.Status.PrivateKeyRef == nil:
		return i.Error(errors.New("status reference to private key not set"))
	}

	labels := constants.LabelsFor(ComponentName, DeploymentName, instance.Name)

	trillUrl, err := utils.GetInternalUrl(ctx, i.Client, instance.Namespace, trillian.LogserverDeploymentName)
	if err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    constants.Ready,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Creating,
			Message: "Waiting for Trillian logserver",
		})
		i.StatusUpdate(ctx, instance)
		return i.Requeue()
	}

	rootCerts, err := i.handleRootCertificates(instance)
	if err != nil {
		return i.Error(err)
	}

	certConfig, err := i.handlePrivateKey(instance)
	if err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    constants.Ready,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Creating,
			Message: "Waiting for Ctlog private key secret",
		})
		i.StatusUpdate(ctx, instance)
		return i.Requeue()
	}

	var cfg map[string][]byte
	if cfg, err = ctlogUtils.CreateCtlogConfig(trillUrl+":8091", *instance.Status.TreeID, rootCerts, certConfig); err != nil {
		return i.ErrorWithStatusUpdate(ctx, fmt.Errorf("could not create CTLog configuration: %w", err), instance)
	}

	newConfig := utils.CreateImmutableSecret(fmt.Sprintf("ctlog-config-%s", instance.Name), instance.Namespace, cfg, labels)

	if err = controllerutil.SetControllerReference(instance, newConfig, i.Client.Scheme()); err != nil {
		return i.Error(fmt.Errorf("could not set controller reference for Secret: %w", err))
	}

	_, err = i.Ensure(ctx, newConfig)
	if err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    actions.ServerCondition,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Failure,
			Message: err.Error(),
		})
		return i.ErrorWithStatusUpdate(ctx, err, instance)
	}

	instance.Status.ServerConfigRef = &rhtasv1alpha1.LocalObjectReference{Name: newConfig.Name}

	i.Recorder.Event(instance, corev1.EventTypeNormal, "CTLogConfigUpdated", "CTLog config updated")
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:    ServerCondition,
		Status:  metav1.ConditionFalse,
		Reason:  constants.Creating,
		Message: "Server config created",
	})
	return i.StatusUpdate(ctx, instance)
}

func (i serverConfig) handlePrivateKey(instance *rhtasv1alpha1.CTlog) (*ctlogUtils.PrivateKeyConfig, error) {
	private, err := utils.GetSecretData(i.Client, instance.Namespace, instance.Status.PrivateKeyRef)
	if err != nil {
		return nil, err
	}
	public, err := utils.GetSecretData(i.Client, instance.Namespace, instance.Status.PublicKeyRef)
	if err != nil {
		return nil, err
	}
	password, err := utils.GetSecretData(i.Client, instance.Namespace, instance.Status.PrivateKeyPasswordRef)
	if err != nil {
		return nil, err
	}

	return &ctlogUtils.PrivateKeyConfig{
		PrivateKey:     private,
		PublicKey:      public,
		PrivateKeyPass: password,
	}, nil
}

func (i serverConfig) handleRootCertificates(instance *rhtasv1alpha1.CTlog) ([]ctlogUtils.RootCertificate, error) {
	certs := make([]ctlogUtils.RootCertificate, 0)

	for _, selector := range instance.Status.RootCertificates {
		data, err := utils.GetSecretData(i.Client, instance.Namespace, &selector)
		if err != nil {
			return nil, err
		}
		certs = append(certs, data)
	}

	return certs, nil
}

func (i serverConfig) CanHandleError(_ context.Context, instance *rhtasv1alpha1.CTlog) bool {
	return !meta.IsStatusConditionTrue(instance.GetConditions(), actions.ServerCondition) && instance.Status.ServerConfigRef != nil
}

func (i serverConfig) HandleError(ctx context.Context, instance *rhtasv1alpha1.CTlog) *action.Result {
	deployment := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Status.ServerConfigRef.Name,
			Namespace: instance.Namespace,
		},
	}
	if err := i.Client.Delete(ctx, deployment); err != nil {
		i.Logger.V(1).Info("Can't delete server configuration", "error", err.Error())
	}

	instance.Status.ServerConfigRef = nil

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:    ServerCondition,
		Status:  metav1.ConditionFalse,
		Reason:  constants.Recovering,
		Message: "server configuration will be recreated",
	})

	return i.StatusUpdate(ctx, instance)
}
