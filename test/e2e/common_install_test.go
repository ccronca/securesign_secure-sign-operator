//go:build integration

package e2e

import (
	"context"
	"net/http"
	"time"

	"github.com/securesign/operator/internal/controller/common/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers"
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/test/e2e/support"
	"github.com/securesign/operator/test/e2e/support/tas"
	clients "github.com/securesign/operator/test/e2e/support/tas/cli"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Securesign install with certificate generation", Ordered, func() {
	cli, _ := CreateClient()
	ctx := context.TODO()

	var targetImageName string
	var namespace *v1.Namespace
	var securesign *v1alpha1.Securesign

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

		securesign = &v1alpha1.Securesign{
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
					Create: utils.Pointer(true),
				}},
				TimestampAuthority: v1alpha1.TimestampAuthoritySpec{
					ExternalAccess: v1alpha1.ExternalAccess{
						Enabled: true,
					},
					Signer: v1alpha1.TimestampAuthoritySigner{
						CertificateChain: v1alpha1.CertificateChain{
							RootCA: v1alpha1.TsaCertificateAuthority{
								OrganizationName:  "MyOrg",
								OrganizationEmail: "my@email.org",
								CommonName:        "tsa.hostname",
							},
							IntermediateCA: []v1alpha1.TsaCertificateAuthority{
								{
									OrganizationName:  "MyOrg",
									OrganizationEmail: "my@email.org",
									CommonName:        "tsa.hostname",
								},
							},
							LeafCA: v1alpha1.TsaCertificateAuthority{
								OrganizationName:  "MyOrg",
								OrganizationEmail: "my@email.org",
								CommonName:        "tsa.hostname",
							},
						},
					},
				},
			},
		}
	})

	BeforeAll(func() {
		targetImageName = support.PrepareImage(ctx)
	})

	Describe("Install with autogenerated certificates", func() {
		BeforeAll(func() {
			Expect(cli.Create(ctx, securesign)).To(Succeed())
		})

		It("Fulcio is running", func() {
			tas.VerifyFulcio(ctx, cli, namespace.Name, securesign.Name)
		})

		It("operator should generate fulcio secret", func() {
			Eventually(func(g Gomega) *v1.Secret {
				fulcio := tas.GetFulcio(ctx, cli, namespace.Name, securesign.Name)()
				scr := &v1.Secret{}
				g.Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: fulcio.Status.Certificate.PrivateKeyRef.Name}, scr)).To(Succeed())
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
				return tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)().Status.Signer.KeyRef
			}).Should(Not(BeNil()))
			Eventually(func(g Gomega) *v1.Secret {
				rekor := tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)()
				scr := &v1.Secret{}
				g.Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: rekor.Status.Signer.KeyRef.Name}, scr)).To(Succeed())
				return scr
			}).Should(
				WithTransform(func(secret *v1.Secret) map[string][]byte { return secret.Data },
					And(
						&matchers.HaveKeyMatcher{Key: "private"},
					)))
		})

		It("operator should generate TSA secret", func() {
			Eventually(func() *v1.Secret {
				tsa := tas.GetTSA(ctx, cli, namespace.Name, securesign.Name)()
				scr := &v1.Secret{}
				Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: tsa.Status.Signer.File.PrivateKeyRef.Name}, scr)).To(Succeed())
				return scr
			}).Should(
				WithTransform(func(secret *v1.Secret) map[string][]byte { return secret.Data },
					And(
						&matchers.HaveKeyMatcher{Key: "rootPrivateKey"},
						&matchers.HaveKeyMatcher{Key: "rootPrivateKeyPassword"},
						&matchers.HaveKeyMatcher{Key: "interPrivateKey-0"},
						&matchers.HaveKeyMatcher{Key: "interPrivateKeyPassword-0"},
						&matchers.HaveKeyMatcher{Key: "leafPrivateKey"},
						&matchers.HaveKeyMatcher{Key: "leafPrivateKeyPassword"},
						&matchers.HaveKeyMatcher{Key: "certificateChain"},
					)))
		})

		It("fulcio is running with mounted certs", func() {
			server := tas.GetFulcioServerPod(ctx, cli, namespace.Name)()
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
					}, Equal(tas.GetFulcio(ctx, cli, namespace.Name, securesign.Name)().Status.Certificate.CARef.Name)),
				))
		})

		It("rekor is running with mounted certs", func() {
			tas.VerifyRekor(ctx, cli, namespace.Name, securesign.Name)
			server := tas.GetRekorServerPod(ctx, cli, namespace.Name)()
			Expect(server).NotTo(BeNil())
			Expect(server.Spec.Volumes).To(
				ContainElement(
					WithTransform(func(volume v1.Volume) string {
						if volume.VolumeSource.Secret != nil {
							return volume.VolumeSource.Secret.SecretName
						}
						return ""
					}, Equal(tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)().Status.Signer.KeyRef.Name))),
			)

		})

		It("tsa is running with mounted certs", func() {
			tas.VerifyTSA(ctx, cli, namespace.Name, securesign.Name)
			server := tas.GetTSAServerPod(ctx, cli, namespace.Name)()
			Expect(server).NotTo(BeNil())
			Expect(server.Spec.Volumes).To(
				ContainElement(
					WithTransform(func(volume v1.Volume) string {
						if volume.VolumeSource.Secret != nil {
							return volume.VolumeSource.Secret.SecretName
						}
						return ""
					}, Equal(tas.GetTSA(ctx, cli, namespace.Name, securesign.Name)().Status.Signer.File.PrivateKeyRef.Name))),
			)

		})

		It("All other components are running", func() {
			tas.VerifySecuresign(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyTrillian(ctx, cli, namespace.Name, securesign.Name, true)
			tas.VerifyCTLog(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyTuf(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyRekorSearchUI(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyTSA(ctx, cli, namespace.Name, securesign.Name)
		})

		It("Verify Rekor Search UI is accessible", func() {
			rekor := tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)()
			Expect(rekor).ToNot(BeNil())
			Expect(rekor.Status.RekorSearchUIUrl).NotTo(BeEmpty())

			httpClient := http.Client{
				Timeout: time.Second * 10,
			}
			Eventually(func() bool {
				resp, err := httpClient.Get(rekor.Status.RekorSearchUIUrl)
				if err != nil {
					return false
				}
				defer func() { _ = resp.Body.Close() }()
				return resp.StatusCode == http.StatusOK
			}, "30s", "1s").Should(BeTrue(), "Rekor UI should be accessible and return a status code of 200")
		})

		It("Use cosign cli", func() {
			fulcio := tas.GetFulcio(ctx, cli, namespace.Name, securesign.Name)()
			Expect(fulcio).ToNot(BeNil())

			rekor := tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)()
			Expect(rekor).ToNot(BeNil())

			tuf := tas.GetTuf(ctx, cli, namespace.Name, securesign.Name)()
			Expect(tuf).ToNot(BeNil())

			tsa := tas.GetTSA(ctx, cli, namespace.Name, securesign.Name)()
			Expect(tsa).ToNot(BeNil())
			err := tas.GetTSACertificateChain(ctx, cli, tsa.Namespace, tsa.Name, tsa.Status.Url)
			Expect(err).To(BeNil())

			oidcToken, err := support.OidcToken(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(oidcToken).ToNot(BeEmpty())

			// sleep for a while to be sure everything has settled down
			time.Sleep(time.Duration(10) * time.Second)

			Expect(clients.Execute("cosign", "initialize", "--mirror="+tuf.Status.Url, "--root="+tuf.Status.Url+"/root.json")).To(Succeed())

			Expect(clients.Execute(
				"cosign", "sign", "-y",
				"--fulcio-url="+fulcio.Status.Url,
				"--rekor-url="+rekor.Status.Url,
				"--timestamp-server-url="+tsa.Status.Url+"/api/v1/timestamp",
				"--oidc-issuer="+support.OidcIssuerUrl(),
				"--oidc-client-id="+support.OidcClientID(),
				"--identity-token="+oidcToken,
				targetImageName,
			)).To(Succeed())

			Expect(clients.Execute(
				"cosign", "verify",
				"--rekor-url="+rekor.Status.Url,
				"--timestamp-certificate-chain=ts_chain.pem",
				"--certificate-identity-regexp", ".*@redhat",
				"--certificate-oidc-issuer-regexp", ".*keycloak.*",
				targetImageName,
			)).To(Succeed())
		})
	})
})
