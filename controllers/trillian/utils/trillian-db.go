package trillianUtils

import (
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/controllers/constants"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateTrillDb(instance *v1alpha1.Trillian, dpName string, sa string, openshift bool, labels map[string]string) *apps.Deployment {
	replicas := int32(1)
	var secCont *core.PodSecurityContext
	if !openshift {
		uid := int64(1001)
		fsid := int64(1001)
		secCont = &core.PodSecurityContext{
			RunAsUser: &uid,
			FSGroup:   &fsid,
		}
	}
	return &apps.Deployment{
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
					SecurityContext:    secCont,
					Volumes: []core.Volume{
						{
							Name: "storage",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: instance.Status.Db.Pvc.Name,
								},
							},
						},
					},
					Containers: []core.Container{
						{
							Name:  dpName,
							Image: constants.TrillianDbImage,
							ReadinessProbe: &core.Probe{
								ProbeHandler: core.ProbeHandler{
									Exec: &core.ExecAction{
										Command: []string{
											"mysqladmin",
											"ping",
											"-h",
											"localhost",
											"-u",
											"$(MYSQL_USER)",
											"-p$(MYSQL_PASSWORD)",
										},
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      1,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							Ports: []core.ContainerPort{
								{
									Protocol:      core.ProtocolTCP,
									ContainerPort: 3306,
								},
							},
							// Env variables from secret trillian-mysql
							Env: []core.EnvVar{
								{
									Name: "MYSQL_USER",
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											Key:                  "mysql-user",
											LocalObjectReference: *instance.Status.Db.DatabaseSecretRef,
										},
									},
								},
								{
									Name: "MYSQL_PASSWORD",
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											Key:                  "mysql-password",
											LocalObjectReference: *instance.Status.Db.DatabaseSecretRef,
										},
									},
								},
								{
									Name: "MYSQL_ROOT_PASSWORD",
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											Key:                  "mysql-root-password",
											LocalObjectReference: *instance.Status.Db.DatabaseSecretRef,
										},
									},
								},
								{
									Name: "MYSQL_PORT",
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											Key:                  "mysql-port",
											LocalObjectReference: *instance.Status.Db.DatabaseSecretRef,
										},
									},
								},
								{
									Name: "MYSQL_DATABASE",
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											Key:                  "mysql-database",
											LocalObjectReference: *instance.Status.Db.DatabaseSecretRef,
										},
									},
								},
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "storage",
									MountPath: "/var/lib/mysql",
								},
							},
						},
					},
				},
			},
		},
	}
}
