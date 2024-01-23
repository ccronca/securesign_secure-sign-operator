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

package tuf

import (
	"context"
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/controllers/common/utils/kubernetes"
	"github.com/securesign/operator/controllers/constants"
	"github.com/securesign/operator/controllers/ctlog"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"maps"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("TUF controller", func() {
	Context("TUF controller test", func() {

		const (
			TufName      = "test-tuf"
			TufNamespace = "default"
		)

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TufName,
				Namespace: TufNamespace,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: TufName, Namespace: TufNamespace}
		tuf := &v1alpha1.Tuf{}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("removing the custom resource for the Kind Tuf")
			found := &v1alpha1.Tuf{}
			err := k8sClient.Get(ctx, typeNamespaceName, found)
			Expect(err).To(Not(HaveOccurred()))

			Eventually(func() error {
				return k8sClient.Delete(context.TODO(), found)
			}, 2*time.Minute, time.Second).Should(Succeed())

			// TODO(user): Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations.
			// More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for Tuf", func() {
			By("creating the custom resource for the Kind Tuf")
			err := k8sClient.Get(ctx, typeNamespaceName, tuf)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				tuf := &v1alpha1.Tuf{
					ObjectMeta: metav1.ObjectMeta{
						Name:      TufName,
						Namespace: TufNamespace,
					},
					Spec: v1alpha1.TufSpec{
						ExternalAccess: v1alpha1.ExternalAccess{
							Host:    "tuf.localhost",
							Enabled: true,
						},
						Port: 8181,
						Keys: []v1alpha1.TufKey{
							{
								Name: "fulcio_v1.crt.pem",
								SecretRef: &v1alpha1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "fulcio-pub-key",
									},
									Key: "cert",
								},
							},
							{
								Name: "ctfe.pub",
							},
							{
								Name: "rekor.pub",
								SecretRef: &v1alpha1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "rekor-pub-key",
									},
									Key: "public",
								},
							},
						},
					},
				}
				err = k8sClient.Create(ctx, tuf)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &v1alpha1.Tuf{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Pending phase untill ctlog public key is resolved")
			Eventually(func() v1alpha1.Phase {
				found := &v1alpha1.Tuf{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return found.Status.Phase
			}, time.Minute, time.Second).Should(Equal(v1alpha1.PhasePending))

			By("Creating ctlog secret with public key")
			secretLabels := map[string]string{
				constants.TufLabelNamespace + "/ctfe.pub": "public",
			}
			maps.Copy(secretLabels, constants.LabelsFor(ctlog.ComponentName, ctlog.ComponentName, ctlog.ComponentName))
			_ = k8sClient.Create(ctx, kubernetes.CreateSecret("ctlog", typeNamespaceName.Namespace, map[string][]byte{
				"public": []byte("secret"),
			}, secretLabels))

			By("Waiting until Tuf instance is Initialization")
			Eventually(func() v1alpha1.Phase {
				found := &v1alpha1.Tuf{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return found.Status.Phase
			}, time.Minute, time.Second).Should(Equal(v1alpha1.PhaseInitialize))

			deployment := &appsv1.Deployment{}
			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: TufNamespace}, deployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Move to Ready phase")
			// Workaround to succeed condition for Ready phase
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.ReadyReplicas = *deployment.Spec.Replicas
			Expect(k8sClient.Status().Update(ctx, deployment)).Should(Succeed())

			By("Waiting until Tuf instance is Ready")
			Eventually(func() v1alpha1.Phase {
				found := &v1alpha1.Tuf{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return found.Status.Phase
			}, time.Minute, time.Second).Should(Equal(v1alpha1.PhaseReady))

			By("Checking if Service was successfully created in the reconciliation")
			service := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: TufNamespace}, service)
			}, time.Minute, time.Second).Should(Succeed())
			Expect(service.Spec.Ports[0].Port).Should(Equal(int32(8181)))

			By("Checking if Ingress was successfully created in the reconciliation")
			ingress := &v1.Ingress{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: TufNamespace}, ingress)
			}, time.Minute, time.Second).Should(Succeed())
			Expect(ingress.Spec.Rules[0].Host).Should(Equal("tuf.localhost"))
			Expect(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name).Should(Equal(service.Name))
			Expect(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Name).Should(Equal("tuf"))

			By("Checking the latest Status Condition added to the Tuf instance")
			Eventually(func() error {
				found := &v1alpha1.Tuf{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				rekorCondition := findCondition(found.Status.Conditions, "rekor.pub")
				Expect(rekorCondition).Should(Not(BeNil()))
				Expect(rekorCondition.Status).Should(Equal(metav1.ConditionTrue))
				Expect(rekorCondition.Reason).Should(Equal("Ready"))
				ctlogCondition := findCondition(found.Status.Conditions, "ctfe.pub")
				Expect(ctlogCondition).Should(Not(BeNil()))
				Expect(ctlogCondition.Status).Should(Equal(metav1.ConditionTrue))
				Expect(ctlogCondition.Reason).Should(Equal("Ready"))
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if controller will return deployment to desired state")
			deployment = &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: TufNamespace}, deployment)
			}, time.Minute, time.Second).Should(Succeed())
			replicas := int32(99)
			deployment.Spec.Replicas = &replicas
			Expect(k8sClient.Status().Update(ctx, deployment)).Should(Succeed())
			Eventually(func() int32 {
				deployment = &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: TufNamespace}, deployment)).Should(Succeed())
				return *deployment.Spec.Replicas
			}, time.Minute, time.Second).Should(Equal(int32(1)))
		})
	})
})

func findCondition(conditions []metav1.Condition, ctype string) *metav1.Condition {
	if conditions != nil && len(conditions) != 0 {
		for _, c := range conditions {
			if c.Type == ctype {
				return &c
			}
		}
	}
	return nil
}