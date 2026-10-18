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
		"### Method 1: API-Only Connection (Recommended)",
		"proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs",
		"proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'",
		"Stop if the download fails",
		"Create a separate Pulse token in **API Access**",
		"enter an administrator root shell",
		"successful HTTP **200** download and inspection",
		"Do not bypass certificate checks",
	})
	shellExamples := pbsShellExamples(pbsDoc)
	if shellExamples == "" {
		t.Fatal("PBS guide must retain executable shell examples")
	}
	// Bind the guard to the private, non-clobbering recipes, not their old
	// literal-path spelling. installtests also executes these copied commands
	// against independent permission, transport and failure controls.
	assertContainsAll(t, pbsRel, shellExamples, []string{
		`set -eu`,
		`umask 077`,
		`config_dir="$HOME/.config/pulse"`,
		`credential_file="$config_dir/pbs-agent-token"`,
		`[ -L "$HOME/.config" ]`,
		`[ -L "$config_dir" ]`,
		`[ -L "$credential_file" ]`,
		`[ ! -f "$credential_file" ]`,
		`chmod 700 "$config_dir"`,
		`chmod 600 "$credential_file"`,
		`vi "$credential_file"`,
		`installer_file="$config_dir/pbs-agent-install.sh"`,
		`[ -e "$installer_file" ] || [ -L "$installer_file" ]`,
		`download_file=$(mktemp "$config_dir/pbs-agent-download.XXXXXX")`,
		`curl --disable --fail --silent --show-error --proto '=https'`,
		`--connect-timeout 5 --max-time 60 --output "$download_file"`,
		`--write-out '%{http_code}' https://pulse.example.com/install.sh`,
		`[ "$curl_exit" -eq 0 ] || exit "$curl_exit"`,
		`[ "$status" = 200 ]`,
		`mv -n "$download_file" "$installer_file"`,
		`[ ! -e "$download_file" ]`,
		`bash "$HOME/.config/pulse/pbs-agent-install.sh"`,
		`--token-file "$HOME/.config/pulse/pbs-agent-token"`,
		`--enable-proxmox --proxmox-type pbs --enable-docker=false`,
	})
	assertContainsNone(t, pbsRel, shellExamples, []string{
		`curl -sSL "http://<pulse-ip>:7655/api/setup-script?type=pbs&host=https://<pbs-ip>:8007&pulse_url=http://<pulse-ip>:7655" | bash`,
		`PULSE_SETUP_TOKEN=`,
		`sudo env PULSE_SETUP_TOKEN=`,
		`--insecure`,
		`--location`,
		`--trace`,
		`--verbose`,
		` -k`,
		`curl -k`,
		`--token "`,
		`| bash`,
	})
}
