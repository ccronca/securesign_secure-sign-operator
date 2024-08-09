package trillian

import (
	"context"

	. "github.com/onsi/gomega"
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	"github.com/securesign/operator/internal/controller/trillian/actions"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Verify(ctx context.Context, cli client.Client, namespace string, name string, dbPresent bool) {
	Eventually(func(g Gomega) bool {
		instance := &v1alpha1.Trillian{}
		g.Expect(cli.Get(ctx, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, instance)).To(Succeed())
		return meta.IsStatusConditionTrue(instance.Status.Conditions, constants.Ready)
	}).Should(BeTrue())

	if dbPresent {
		// trillian-db
		Eventually(func(g Gomega) (bool, error) {
			return kubernetes.DeploymentIsRunning(ctx, cli, namespace, map[string]string{
				kubernetes.ComponentLabel: actions.DbComponentName,
			})
		}).Should(BeTrue())
	}

	// log server
	Eventually(func(g Gomega) (bool, error) {
		return kubernetes.DeploymentIsRunning(ctx, cli, namespace, map[string]string{
			kubernetes.ComponentLabel: actions.LogServerComponentName,
		})
	}).Should(BeTrue())

	// log signer
	Eventually(func(g Gomega) (bool, error) {
		return kubernetes.DeploymentIsRunning(ctx, cli, namespace, map[string]string{
			kubernetes.ComponentLabel: actions.LogSignerComponentName,
		})
	}).Should(BeTrue())
}
