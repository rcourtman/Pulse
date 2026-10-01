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
		"Create a separate Pulse token in **API Access**",
		"enter an administrator root shell",
		`chmod 700 "$HOME/.config/pulse"`,
		`chmod 600 "$HOME/.config/pulse/pbs-agent-token"`,
		`vi "$HOME/.config/pulse/pbs-agent-token"`,
		`curl --fail --silent --show-error`,
		`--output "$HOME/.config/pulse/pbs-agent-install.sh"`,
		`https://pulse.example.com/install.sh`,
		"successful download and inspection",
		`bash "$HOME/.config/pulse/pbs-agent-install.sh"`,
		`--token-file "$HOME/.config/pulse/pbs-agent-token"`,
		`--enable-proxmox --proxmox-type pbs --enable-docker=false`,
		"Do not bypass certificate checks",
	})
	assertContainsNone(t, pbsRel, pbsDoc, []string{
		`curl -sSL "http://<pulse-ip>:7655/api/setup-script?type=pbs&host=https://<pbs-ip>:8007&pulse_url=http://<pulse-ip>:7655" | bash`,
		`PULSE_SETUP_TOKEN=`,
		`sudo env PULSE_SETUP_TOKEN=`,
		`--insecure`,
	})
}
