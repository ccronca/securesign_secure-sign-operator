package utils

import (
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/utils"
	"github.com/securesign/operator/internal/controller/constants"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateTufDeployment(instance *v1alpha1.Tuf, dpName string, sa string, labels map[string]string) *apps.Deployment {
	replicas := int32(1)
	dep := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: core.PodSpec{
					ServiceAccountName: sa,
					Volumes: []core.Volume{
						{
							Name: "tuf-secrets",
							VolumeSource: core.VolumeSource{
								Projected: secretsVolumeProjection(instance.Status.Keys),
							},
						},
						{
							Name: "repository",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: instance.Status.PvcName,
								},
							},
						},
					},
					Containers: []core.Container{
						{
							Name:  "tuf-server",
							Image: constants.TufImage,
							Ports: []core.ContainerPort{
								{
									Protocol:      core.ProtocolTCP,
									ContainerPort: 8080,
								},
							},
							Env: []core.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: instance.Namespace,
								},
							},
							Args: []string{
								"-mode", "serve",
								"-target-dir", "/var/run/target",
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "tuf-secrets",
									MountPath: "/var/run/tuf-secrets",
								},
								{
									Name:      "repository",
									MountPath: "/var/run/target",
									ReadOnly:  true,
								},
							},
							LivenessProbe: &core.Probe{
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      1,
								FailureThreshold:    3,
								SuccessThreshold:    1,
								ProbeHandler: core.ProbeHandler{
									HTTPGet: &core.HTTPGetAction{
										Port: intstr.FromInt32(8080),
										Path: "/",
									},
								},
							},
							ReadinessProbe: &core.Probe{
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      1,
								FailureThreshold:    10,
								SuccessThreshold:    1,
								ProbeHandler: core.ProbeHandler{
									HTTPGet: &core.HTTPGetAction{
										Port: intstr.FromInt32(8080),
										Path: "/root.json",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	utils.SetProxyEnvs(dep)
	return dep
}
