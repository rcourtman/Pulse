package repoctl

import (
	"strings"
	"testing"
)

func pbsShellExamples(document string) string {
	var examples []string
	blocks := strings.Split(document, "```")
	for i := 1; i < len(blocks); i += 2 {
		language, body, _ := strings.Cut(blocks[i], "\n")
		switch strings.TrimSpace(language) {
		case "bash", "sh", "shell":
			examples = append(examples, body)
		}
	}
	return strings.Join(examples, "\n")
}

func TestPBSShellExamplesKeepWarningsSeparateFromCommands(t *testing.T) {
	document := "Do not use `--insecure` or `-k`.\n```bash\ncurl --fail https://pbs.example.com\n```\n" +
		"```sh\ncurl --insecure https://pbs.example.com\n```\n"
	examples := pbsShellExamples(document)
	if examples != "curl --fail https://pbs.example.com\n\ncurl --insecure https://pbs.example.com\n" {
		t.Fatalf("unexpected shell examples: %q", examples)
	}
}

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
	shellExamples := pbsShellExamples(pbsDoc)
	if shellExamples == "" {
		t.Fatal("PBS guide must retain executable shell examples")
	}
	assertContainsNone(t, pbsRel, shellExamples, []string{
		`curl -sSL "http://<pulse-ip>:7655/api/setup-script?type=pbs&host=https://<pbs-ip>:8007&pulse_url=http://<pulse-ip>:7655" | bash`,
		`PULSE_SETUP_TOKEN=`,
		`sudo env PULSE_SETUP_TOKEN=`,
		`--insecure`,
		` -k`,
	})
}
