//go:build integration

package e2e

import (
	"context"
	"net/http"
	"time"

	"k8s.io/utils/ptr"

	"github.com/securesign/operator/test/e2e/support/tas"

	"github.com/securesign/operator/internal/controller/common/utils"
	"github.com/securesign/operator/test/e2e/support/tas/fulcio"
	"github.com/securesign/operator/test/e2e/support/tas/rekor"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers"
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/test/e2e/support"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Securesign install with certificate generation", Ordered, func() {
	cli, _ := CreateClient()
	ctx := context.TODO()

	var targetImageName string
	var namespace *v1.Namespace
	var s *v1alpha1.Securesign

	AfterEach(func() {
		if CurrentSpecReport().Failed() && support.IsCIEnvironment() {
			support.DumpNamespace(ctx, cli, namespace.Name)
		}
	})

	BeforeAll(func() {
		namespace = support.CreateTestNamespace(ctx, cli)
		DeferCleanup(func() {
			_ = cli.Delete(ctx, namespace)
		})

		s = &v1alpha1.Securesign{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.Name,
				Name:      "test",
				Annotations: map[string]string{
					"rhtas.redhat.com/metrics": "false",
				},
			},
			Spec: v1alpha1.SecuresignSpec{
				Rekor: v1alpha1.RekorSpec{
					ExternalAccess: v1alpha1.ExternalAccess{
						Enabled: true,
					},
					RekorSearchUI: v1alpha1.RekorSearchUI{
						Enabled: utils.Pointer(true),
					},
				},
				Fulcio: v1alpha1.FulcioSpec{
					ExternalAccess: v1alpha1.ExternalAccess{
						Enabled: true,
					},
					Config: v1alpha1.FulcioConfig{
						OIDCIssuers: []v1alpha1.OIDCIssuer{
							{
								ClientID:  support.OidcClientID(),
								IssuerURL: support.OidcIssuerUrl(),
								Issuer:    support.OidcIssuerUrl(),
								Type:      "email",
							},
						}},
					Certificate: v1alpha1.FulcioCert{
						OrganizationName:  "MyOrg",
						OrganizationEmail: "my@email.org",
						CommonName:        "fulcio",
					},
				},
				Ctlog: v1alpha1.CTlogSpec{},
				Tuf: v1alpha1.TufSpec{
					ExternalAccess: v1alpha1.ExternalAccess{
						Enabled: true,
					},
				},
				Trillian: v1alpha1.TrillianSpec{Db: v1alpha1.TrillianDB{
					Create: ptr.To(true),
				}},
			},
		}
	})

	BeforeAll(func() {
		targetImageName = support.PrepareImage(ctx)
	})

	Describe("Install with autogenerated certificates", func() {
		BeforeAll(func() {
			Expect(cli.Create(ctx, s)).To(Succeed())
		})

		It("All other components are running", func() {
			tas.VerifyAllComponents(ctx, cli, s, true)
		})

		It("operator should generate Fulcio's secret", func() {
			Eventually(func(g Gomega) *v1.Secret {
				f := fulcio.Get(ctx, cli, namespace.Name, s.Name)()
				g.Expect(f).ToNot(BeNil())
				g.Expect(f.Status.Certificate).ToNot(BeNil())
				g.Expect(f.Status.Certificate.PrivateKeyRef).ToNot(BeNil())
				scr := &v1.Secret{}
				g.Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: f.Status.Certificate.PrivateKeyRef.Name}, scr)).To(Succeed())
				return scr
			}).Should(
				WithTransform(func(secret *v1.Secret) map[string][]byte { return secret.Data },
					And(
						&matchers.HaveKeyMatcher{Key: "cert"},
						&matchers.HaveKeyMatcher{Key: "private"},
						&matchers.HaveKeyMatcher{Key: "public"},
						&matchers.HaveKeyMatcher{Key: "password"},
					)))
		})

		It("operator should generate rekor secret", func() {
			Eventually(func() *v1alpha1.SecretKeySelector {
				return rekor.Get(ctx, cli, namespace.Name, s.Name)().Status.Signer.KeyRef
			}).Should(Not(BeNil()))
			Eventually(func(g Gomega) *v1.Secret {
				r := rekor.Get(ctx, cli, namespace.Name, s.Name)()
				g.Expect(r).ToNot(BeNil())
				scr := &v1.Secret{}
				g.Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: r.Status.Signer.KeyRef.Name}, scr)).To(Succeed())
				return scr
			}).Should(
				WithTransform(func(secret *v1.Secret) map[string][]byte { return secret.Data },
					And(
						&matchers.HaveKeyMatcher{Key: "private"},
					)))
		})

		It("Fulcio is running with mounted certs", func() {
			server := fulcio.GetServerPod(ctx, cli, namespace.Name)()
			Expect(server).NotTo(BeNil())

			sp := []v1.SecretProjection{}
			for _, volume := range server.Spec.Volumes {
				if volume.Name == "fulcio-cert" {
					for _, source := range volume.VolumeSource.Projected.Sources {
						sp = append(sp, *source.Secret)
					}
				}
			}

			Expect(sp).To(
				ContainElement(
					WithTransform(func(sp v1.SecretProjection) string {
						return sp.Name
					}, Equal(fulcio.Get(ctx, cli, namespace.Name, s.Name)().Status.Certificate.CARef.Name)),
				))
		})

		It("rekor is running with mounted certs", func() {
			rekor.Verify(ctx, cli, namespace.Name, s.Name)
			server := rekor.GetServerPod(ctx, cli, namespace.Name)()
			Expect(server).NotTo(BeNil())
			Expect(server.Spec.Volumes).To(
				ContainElement(
					WithTransform(func(volume v1.Volume) string {
						if volume.VolumeSource.Secret != nil {
							return volume.VolumeSource.Secret.SecretName
						}
						return ""
					}, Equal(rekor.Get(ctx, cli, namespace.Name, s.Name)().Status.Signer.KeyRef.Name))),
			)
		})

		It("Verify Rekor Search UI is accessible", func() {
			r := rekor.Get(ctx, cli, namespace.Name, s.Name)()
			Expect(r).ToNot(BeNil())
			Expect(r.Status.RekorSearchUIUrl).NotTo(BeEmpty())

			httpClient := http.Client{
				Timeout: time.Second * 10,
			}
			Eventually(func() bool {
				resp, err := httpClient.Get(r.Status.RekorSearchUIUrl)
				if err != nil {
					return false
				}
				defer func() { _ = resp.Body.Close() }()
				return resp.StatusCode == http.StatusOK
			}, "30s", "1s").Should(BeTrue(), "Rekor UI should be accessible and return a status code of 200")
		})

		It("Use cosign cli", func() {
			tas.VerifyByCosign(ctx, cli, s, targetImageName)
		})
	})
})
