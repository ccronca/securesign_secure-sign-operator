package constants

var (
	TrillianLogSignerImage = "registry.redhat.io/rhtas/trillian-logsigner-rhel9@sha256:920f2fd735525dd612546a874e24d301761ca83c79ddb6898ee7d31470ffc467"
	TrillianServerImage    = "registry.redhat.io/rhtas/trillian-logserver-rhel9@sha256:4478e867e59b5c2d7a4e2630f76fad7899205de611a6f4648d9ca7389392780d"
	TrillianDbImage        = "registry.redhat.io/rhtas/trillian-database-rhel9@sha256:221b4cb0f86d73606520c708499f0e6686838054fb0a759ba323c3f3ac8b7fed"

	// TODO: remove and check the DB pod status
	TrillianNetcatImage = "registry.redhat.io/openshift4/ose-tools-rhel8@sha256:486b4d2dd0d10c5ef0212714c94334e04fe8a3d36cf619881986201a50f123c7"

	FulcioServerImage  = "quay.io/securesign/fulcio-server@sha256:67495de82e2fcd2ab4ad0e53442884c392da1aa3f5dd56d9488a1ed5df97f513"
	RekorRedisImage    = "registry.redhat.io/rhtas/trillian-redis-rhel9@sha256:5f0630c7aa29eeee28668f7ad451f129c9fb2feb86ec21b6b1b0b5cc42b44f4a"
	RekorServerImage   = "registry.redhat.io/rhtas/rekor-server-rhel9@sha256:d4ea970447f3b4c18c309d2f0090a5d02260dd5257a0d41f87fefc4f014a9526"
	RekorSearchUiImage = "registry.redhat.io/rhtas/rekor-search-ui-rhel9@sha256:5eabf561c0549d81862e521ddc1f0ab91a3f2c9d99dcd83ab5a2cf648a95dd19"
	BackfillRedisImage = "registry.redhat.io/rhtas/rekor-backfill-redis-rhel9@sha256:5c7460ab3cd13b2ecf2b979f5061cb384174d6714b7630879e53d063e4cb69d2"

	TufImage = "registry.redhat.io/rhtas/tuf-server-rhel9@sha256:8c229e2c7f9d6cc0ebf4f23dd944373d497be2ed31960f0383b1bb43f16de0db"

	CTLogImage = "quay.io/securesign/certificate-transparency-go@sha256:a0c7d71fc8f4cb7530169a6b54dc3a67215c4058a45f84b87bb04fc62e6e8141"

	ClientServerImage    = "registry.access.redhat.com/ubi9/httpd-24@sha256:7874b82335a80269dcf99e5983c2330876f5fe8bdc33dc6aa4374958a2ffaaee"
	ClientServerImage_cg = "registry.redhat.io/rhtas/client-server-cg-rhel9@sha256:046029a9a2028efa9dcbf8eff9b41fe5ac4e9ad64caf0241f5680a5cb36bf36b"
	ClientServerImage_re = "registry.redhat.io/rhtas/client-server-re-rhel9@sha256:7254f4c94182d21159162ea850e1ed14332fa5dee987103d54e7e4096a6fea31"
	SegmentBackupImage   = "registry.redhat.io/rhtas/segment-reporting-rhel9@sha256:54be793ea9e2af996e5e5c6f9156ee02d5d915adb53b4ab028cb3030f64b1496"
)
