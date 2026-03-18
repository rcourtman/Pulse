package repoctl

import "testing"

func TestSetupBootstrapDocsStayOnCanonicalArtifactContract(t *testing.T) {
	apiRel := "docs/API.md"
	apiDoc := readRepoFile(t, apiRel)
	assertContainsAll(t, apiRel, apiDoc, []string{
		"`type`, `host`, `url`, `downloadURL`, `scriptFileName`, `command`, `commandWithEnv`,",
		"`commandWithoutEnv`, `setupToken`, `tokenHint`, and `expires`.",
		"canonical root-or-sudo `curl -fsSL` bootstrap commands",
	})
	assertContainsNone(t, apiRel, apiDoc, []string{
		"`type`, `host`, `scriptFileName`, `setupToken`, and `expires`.",
	})

	pbsRel := "docs/PBS.md"
	pbsDoc := readRepoFile(t, pbsRel)
	assertContainsAll(t, pbsRel, pbsDoc, []string{
		`curl -fsSL "http://<pulse-ip>:7655/api/setup-script?type=pbs&host=https://<pbs-ip>:8007&pulse_url=http://<pulse-ip>:7655" | { if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN="<setup-token>" bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN="<setup-token>" bash; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`,
		"Pulse generates that full command for you from **Settings → Nodes**",
	})
	assertContainsNone(t, pbsRel, pbsDoc, []string{
		`curl -sSL "http://<pulse-ip>:7655/api/setup-script?type=pbs&host=https://<pbs-ip>:8007&pulse_url=http://<pulse-ip>:7655" | bash`,
	})
}
