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
	"time"

	"github.com/securesign/operator/controllers/ctlog/utils"

	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/controllers/common/utils/kubernetes"
	"github.com/securesign/operator/controllers/constants"
	"github.com/securesign/operator/controllers/ctlog/actions"
	fulcio "github.com/securesign/operator/controllers/fulcio/actions"
	trillian "github.com/securesign/operator/controllers/trillian/actions"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("CTlog update test", func() {
	Context("CTlog update test", func() {

		const (
			Name      = "test"
			Namespace = "update"
		)

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: Namespace,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: Name, Namespace: Namespace}
		instance := &v1alpha1.CTlog{}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("removing the custom resource for the Kind CTlog")
			found := &v1alpha1.CTlog{}
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

		It("should successfully reconcile a custom resource for CTlog", func() {
			By("creating the custom resource for the Kind CTlog")
			err := k8sClient.Get(ctx, typeNamespaceName, instance)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				ptr := int64(1)
				instance := &v1alpha1.CTlog{
					ObjectMeta: metav1.ObjectMeta{
						Name:      Name,
						Namespace: Namespace,
					},

					Spec: v1alpha1.CTlogSpec{
						TreeID: &ptr,
					},
				}
				err = k8sClient.Create(ctx, instance)
				Expect(err).To(Not(HaveOccurred()))

			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &v1alpha1.CTlog{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Creating trillian service")
			Expect(k8sClient.Create(ctx, kubernetes.CreateService(Namespace, trillian.LogserverDeploymentName, 8091, constants.LabelsForComponent(trillian.LogServerComponentName, instance.Name)))).To(Succeed())

			By("Creating fulcio root cert")
			fulcioCa := kubernetes.CreateSecret("test", Namespace,
				map[string][]byte{"cert": []byte("fakeCert")},
				map[string]string{fulcio.FulcioCALabel: "cert"},
			)
			Expect(k8sClient.Create(ctx, fulcioCa)).To(Succeed())

			deployment := &appsv1.Deployment{}
			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: Namespace}, deployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Move to Ready phase")
			// Workaround to succeed condition for Ready phase
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Status: corev1.ConditionTrue, Type: appsv1.DeploymentAvailable, Reason: constants.Ready}}
			Expect(k8sClient.Status().Update(ctx, deployment)).Should(Succeed())

			By("Waiting until CTlog instance is Ready")
			Eventually(func() bool {
				found := &v1alpha1.CTlog{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return meta.IsStatusConditionTrue(found.Status.Conditions, constants.Ready)
			}, time.Minute, time.Second).Should(BeTrue())

			By("Fulcio CA has changed")
			Expect(k8sClient.Delete(ctx, fulcioCa)).To(Succeed())
			fulcioCa = kubernetes.CreateSecret("test2", Namespace,
				map[string][]byte{"cert": []byte("fakeCert2")},
				map[string]string{fulcio.FulcioCALabel: "cert"},
			)
			Expect(k8sClient.Create(ctx, fulcioCa)).To(Succeed())

			By("CA has changed in status field")
			Eventually(func() []v1alpha1.SecretKeySelector {
				found := &v1alpha1.CTlog{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return found.Status.RootCertificates
			}, time.Minute, time.Second).Should(HaveExactElements(WithTransform(func(ks v1alpha1.SecretKeySelector) string {
				return ks.Name
			}, Equal("test2"))))

			By("CTL deployment is updated")
			Eventually(func() bool {
				updated := &appsv1.Deployment{}
				k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: Namespace}, updated)
				return equality.Semantic.DeepDerivative(deployment.Spec.Template.Spec.Volumes, updated.Spec.Template.Spec.Volumes)
			}, time.Minute, time.Second).Should(BeFalse())

			By("Private key has changed")
			key, err := utils.CreatePrivateKey()
			Expect(err).To(Not(HaveOccurred()))
			Expect(k8sClient.Create(ctx, kubernetes.CreateSecret("key-secret", Namespace,
				map[string][]byte{"private": key.PrivateKey}, constants.LabelsFor(actions.ComponentName, Name, instance.Name)))).To(Succeed())

			k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: Namespace}, deployment)
			found := &v1alpha1.CTlog{}
			Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
			found.Spec.PrivateKeyRef = &v1alpha1.SecretKeySelector{
				LocalObjectReference: v1alpha1.LocalObjectReference{
					Name: "key-secret",
				},
				Key: "private",
			}
			Expect(k8sClient.Update(ctx, found)).To(Succeed())

			By("CTLog status field changed")
			Eventually(func() string {
				found := &v1alpha1.CTlog{}
				Expect(k8sClient.Get(ctx, typeNamespaceName, found)).Should(Succeed())
				return found.Status.PrivateKeyRef.Name
			}, time.Minute, time.Second).Should(Equal("key-secret"))

			By("CTL deployment is updated")
			Eventually(func() bool {
				updated := &appsv1.Deployment{}
				k8sClient.Get(ctx, types.NamespacedName{Name: actions.DeploymentName, Namespace: Namespace}, updated)
				return equality.Semantic.DeepDerivative(deployment.Spec.Template.Spec.Volumes, updated.Spec.Template.Spec.Volumes)
			}, 2*time.Minute, time.Second).Should(BeFalse())
		})
	})
})
