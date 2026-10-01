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
		"### Method 1: API-Only Connection (Recommended)",
		"proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs",
		"proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'",
		`chmod 600 "$HOME/.config/pulse/pbs-agent-token"`,
		`--output "$HOME/.config/pulse/pbs-agent-install.sh"`,
		"https://pulse.example.com/install.sh",
		`--token-file "$HOME/.config/pulse/pbs-agent-token"`,
		"Stop if the download fails",
		"Do not bypass certificate checks",
	})
	assertContainsNone(t, pbsRel, pbsDoc, []string{
		"PULSE_SETUP_TOKEN=",
		"sudo env PULSE_SETUP_TOKEN=",
		"curl -k",
		"--insecure",
		`--token "`,
		"| bash",
	})
}
