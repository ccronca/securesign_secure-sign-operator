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
	"maps"
	"time"

	k8sTest "github.com/securesign/operator/internal/testing/kubernetes"

	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	actions2 "github.com/securesign/operator/internal/controller/ctlog/actions"
	"github.com/securesign/operator/internal/controller/tuf/actions"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeCli "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TUF update test", func() {
	Context("TUF update test", func() {

		const (
			TufName      = "test-tuf"
			TufNamespace = "update"
		)

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: TufNamespace,
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
									LocalObjectReference: v1alpha1.LocalObjectReference{
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
									LocalObjectReference: v1alpha1.LocalObjectReference{
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
			}).Should(Succeed())

			By("Status conditions are initialized")
			Eventually(func(g Gomega) bool {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return meta.IsStatusConditionPresentAndEqual(found.Status.Conditions, constants.Ready, metav1.ConditionFalse)
			}).Should(BeTrue())

			By("Creating ctlog secret with public key")
			secretLabels := map[string]string{
				constants.LabelNamespace + "/ctfe.pub": "public",
			}
			maps.Copy(secretLabels, constants.LabelsFor(actions2.ComponentName, actions2.ComponentName, TufName))
			_ = k8sClient.Create(ctx, kubernetes.CreateSecret("ctlog-test", typeNamespaceName.Namespace, map[string][]byte{
				"public": []byte("secret"),
			}, secretLabels))

			By("Waiting until Tuf instance is Initialization")
			Eventually(func(g Gomega) string {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return meta.FindStatusCondition(found.Status.Conditions, constants.Ready).Reason
			}).Should(Equal(constants.Initialize))

			deployment := &appsv1.Deployment{}
			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: TufNamespace}, deployment)
			}).Should(Succeed())

			By("Move to Ready phase")
			// Workaround to succeed condition for Ready phase
			Expect(k8sTest.SetDeploymentToReady(ctx, k8sClient, deployment)).To(Succeed())

			By("Waiting until Tuf instance is Ready")
			Eventually(func(g Gomega) bool {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return meta.IsStatusConditionTrue(found.Status.Conditions, constants.Ready)
			}).Should(BeTrue())

			By("Checking the latest Status Condition added to the Tuf instance")
			Eventually(func(g Gomega) error {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				rekorCondition := meta.FindStatusCondition(found.Status.Conditions, "rekor.pub")
				g.Expect(rekorCondition).Should(Not(BeNil()))
				g.Expect(rekorCondition.Status).Should(Equal(metav1.ConditionTrue))
				g.Expect(rekorCondition.Reason).Should(Equal("Ready"))
				ctlogCondition := meta.FindStatusCondition(found.Status.Conditions, "ctfe.pub")
				g.Expect(ctlogCondition).Should(Not(BeNil()))
				g.Expect(ctlogCondition.Status).Should(Equal(metav1.ConditionTrue))
				g.Expect(ctlogCondition.Reason).Should(Equal("Ready"))
				return nil
			}).Should(Succeed())

			By("Recreate ctlog secret")
			Expect(k8sClient.DeleteAllOf(ctx, &corev1.Secret{}, runtimeCli.InNamespace(TufNamespace), runtimeCli.MatchingLabels(secretLabels))).To(Succeed())

			By("Pending phase until ctlog public key is resolved")
			Eventually(func(g Gomega) string {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return meta.FindStatusCondition(found.Status.Conditions, constants.Ready).Reason
			}).Should(Equal(constants.Pending))

			Expect(k8sClient.Create(ctx, kubernetes.CreateSecret("ctlog-update", typeNamespaceName.Namespace, map[string][]byte{
				"public": []byte("update"),
			}, secretLabels))).To(Succeed())

			By("Recreate ctlog secret")
			Eventually(func(g Gomega) bool {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return meta.IsStatusConditionTrue(found.Status.Conditions, "ctfe.pub")
			}).Should(BeTrue())

			Eventually(func(g Gomega) []v1alpha1.TufKey {
				found := &v1alpha1.Tuf{}
				g.Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return found.Status.Keys
			}).Should(ContainElements(WithTransform(func(k v1alpha1.TufKey) string { return k.SecretRef.Name }, Equal("ctlog-update"))))

			By("CTL deployment is updated")
			Eventually(func(g Gomega) bool {
				updated := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: TufNamespace}, updated)).To(Succeed())
				return equality.Semantic.DeepDerivative(deployment.Spec.Template.Spec.Volumes, updated.Spec.Template.Spec.Volumes)
			}).Should(BeFalse())

			By("Move to Ready phase")
			deployment = &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: TufNamespace}, deployment)).To(Succeed())
			Expect(k8sTest.SetDeploymentToReady(ctx, k8sClient, deployment)).To(Succeed())
		})
	})
})
