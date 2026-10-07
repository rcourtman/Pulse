package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exercise the generated helper, not a second copy of its channel parser. The
// transport is local-only, but the installer and sidecar use real SSH signatures.
func TestManualUpdateChannelAdmission(t *testing.T) {
	for _, tool := range []string{"bash", "jq", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("manual update proof requires %s: %v", tool, err)
		}
	}
	source, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}

	assets := t.TempDir()
	key := filepath.Join(assets, "signing-key")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("create fixture key: %v\n%s", err, out)
	}
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	installer := filepath.Join(assets, "install.sh")
	if err := os.WriteFile(installer, []byte("#!/bin/bash\nset -eu\nprintf '%s\\n' EXECUTED \"$@\" > \"$MANUAL_UPDATE_CALLS\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ssh-keygen", "-q", "-Y", "sign", "-f", key, "-n", "pulse-install", installer).CombinedOutput(); err != nil {
		t.Fatalf("sign fixture installer: %v\n%s", err, out)
	}

	cases := []struct {
		name, config, marker string
		args, want           []string
		blocked              bool
		missingConfig        bool
		missingJQ            bool
		directory            bool
		brokenLink           bool
		badSignature         bool
	}{
		{name: "stable", config: `{"updateChannel":"stable"}`},
		{name: "rc", config: `{"updateChannel":"rc"}`, want: []string{"--rc"}},
		{name: "normalized_rc", config: `{"updateChannel":" RC "}`, want: []string{"--rc"}},
		{name: "normalized_stable", config: `{"updateChannel":" STABLE "}`},
		{name: "missing_setting", config: `{}`},
		{name: "null_setting", config: `{"updateChannel":null}`},
		{name: "empty_setting", config: `{"updateChannel":" "}`},
		{name: "missing_file", missingConfig: true},
		{name: "nested_only", config: `{"other":{"updateChannel":"rc"}}`},
		{name: "stable_with_nested_rc", config: `{"updateChannel":"stable","other":{"updateChannel":"rc"}}`},
		{name: "rc_with_nested_stable", config: `{"updateChannel":"rc","other":{"updateChannel":"stable"}}`, want: []string{"--rc"}},
		{name: "escaped_setting_name", config: `{"update\u0043hannel":"rc"}`, want: []string{"--rc"}},
		{name: "quoted_only", config: `{"note":"\"updateChannel\":\"rc\""}`},
		{name: "truncated", config: `{"updateChannel":"rc"`, blocked: true},
		{name: "valid_object_then_garbage", config: `{"updateChannel":"rc"} trailing`, blocked: true},
		{name: "two_objects", config: `{"updateChannel":"stable"} {"updateChannel":"rc"}`, blocked: true},
		{name: "empty_file", blocked: true},
		{name: "array", config: `[{"updateChannel":"rc"}]`, blocked: true},
		{name: "string", config: `"rc"`, blocked: true},
		{name: "boolean", config: `{"updateChannel":false}`, blocked: true},
		{name: "number", config: `{"updateChannel":1}`, blocked: true},
		{name: "channel_array", config: `{"updateChannel":["rc"]}`, blocked: true},
		{name: "unknown_channel", config: `{"updateChannel":"beta"}`, blocked: true},
		{name: "missing_jq", config: `{"updateChannel":"rc"}`, missingJQ: true, blocked: true},
		{name: "directory", directory: true, blocked: true},
		{name: "broken_link", brokenLink: true, blocked: true},
		{name: "explicit_stable_over_rc", config: `{"updateChannel":"rc"}`, args: []string{"--stable"}, want: []string{"--stable"}},
		{name: "explicit_rc_over_stable", config: `{"updateChannel":"stable"}`, args: []string{"--rc"}, want: []string{"--rc"}},
		{name: "explicit_prerelease_over_invalid", config: `{"updateChannel":false}`, args: []string{"--prerelease"}, want: []string{"--prerelease"}},
		{name: "explicit_version_over_invalid", config: `{`, args: []string{"--version", "v6.5.0"}, want: []string{"--version", "v6.5.0"}},
		{name: "explicit_source_over_invalid", config: `{`, args: []string{"--source", "main"}, want: []string{"--source", "main"}},
		{name: "explicit_archive_over_invalid", config: `{`, args: []string{"--archive", "/tmp/signed-release.tar.gz"}, want: []string{"--archive", "/tmp/signed-release.tar.gz"}},
		{name: "ordinary_options_keep_rc", config: `{"updateChannel":"rc"}`, args: []string{"--no-auto-update"}, want: []string{"--rc", "--no-auto-update"}},
		{name: "source_marker_over_invalid", config: `{`, marker: "maintenance\n", want: []string{"--source", "maintenance"}},
		{name: "explicit_channel_over_source_marker", config: `{`, marker: "main\n", args: []string{"--stable"}, want: []string{"--stable"}},
		{name: "signature_still_required", config: `{"updateChannel":"rc"}`, badSignature: true, blocked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			configDir := filepath.Join(dir, "config")
			if err := os.Mkdir(configDir, 0700); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(configDir, "system.json")
			switch {
			case tc.directory:
				err = os.Mkdir(configPath, 0700)
			case tc.brokenLink:
				err = os.Symlink(filepath.Join(dir, "absent"), configPath)
			case !tc.missingConfig:
				err = os.WriteFile(configPath, []byte(tc.config), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.marker != "" {
				if err := os.WriteFile(filepath.Join(dir, "BUILD_FROM_SOURCE"), []byte(tc.marker), 0600); err != nil {
					t.Fatal(err)
				}
			}
			helper := filepath.Join(dir, "update")
			generate := exec.Command(bash, "-c", `source "$INSTALLER_SOURCE"
PINNED_RELEASE_SSH_PUBLIC_KEY="$FIXTURE_PUBLIC_KEY"
setup_update_command
`)
			generate.Env = append(os.Environ(), "INSTALLER_SOURCE="+source, "FIXTURE_PUBLIC_KEY="+strings.TrimSpace(string(publicKey)),
				"PULSE_INSTALL_DIR="+dir, "PULSE_CONFIG_DIR="+configDir, "PULSE_UPDATE_HELPER_PATH="+helper,
				"PULSE_PROFILE_PATH="+filepath.Join(dir, "profile"), "PULSE_BASHRC_PATH="+filepath.Join(dir, "bashrc"))
			if out, err := generate.CombinedOutput(); err != nil {
				t.Fatalf("generate helper: %v\n%s", err, out)
			}
			if out, err := exec.Command(bash, "-n", helper).CombinedOutput(); err != nil {
				t.Fatalf("generated Bash syntax: %v\n%s", err, out)
			}
			curl := `#!/bin/bash
set -eu
url=""
dest=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        -o) dest="$2"; shift 2 ;;
        https://github.com/rcourtman/Pulse/releases/latest/download/install.sh*) url="$1"; shift ;;
        -fsSL) shift ;;
        *) exit 98 ;;
    esac
done
[[ -n "$url" && -n "$dest" ]] || exit 97
printf '%s\n' "$url" >> "$MANUAL_UPDATE_TRANSPORT"
case "$url" in
    *.sshsig) if [[ "$BAD_SIGNATURE" == true ]]; then printf invalid > "$dest"; else cp "$FIXTURE_ASSETS/install.sh.sig" "$dest"; fi ;;
    */install.sh) cp "$FIXTURE_ASSETS/install.sh" "$dest" ;;
    *) exit 96 ;;
esac
`
			if err := os.WriteFile(filepath.Join(dir, "curl"), []byte(curl), 0700); err != nil {
				t.Fatal(err)
			}
			path := dir + string(os.PathListSeparator) + os.Getenv("PATH")
			if tc.missingJQ {
				path = dir // parser fails before any non-builtin operation
			}
			calls := filepath.Join(dir, "calls")
			transport := filepath.Join(dir, "transport")
			cmd := exec.Command(bash, append([]string{helper}, tc.args...)...)
			cmd.Env = append(os.Environ(), "PATH="+path, "FIXTURE_ASSETS="+assets, "MANUAL_UPDATE_CALLS="+calls, "MANUAL_UPDATE_TRANSPORT="+transport,
				"BAD_SIGNATURE="+map[bool]string{true: "true", false: "false"}[tc.badSignature])
			out, runErr := cmd.CombinedOutput()
			if tc.blocked {
				if runErr == nil {
					t.Fatalf("invalid preference/signature admitted: %s", out)
				}
				if _, err := os.Stat(calls); !os.IsNotExist(err) {
					t.Fatalf("rejected helper executed installer: %v\n%s", err, out)
				}
				if !tc.badSignature {
					if _, err := os.Stat(transport); !os.IsNotExist(err) {
						t.Fatalf("invalid preference reached download: %v\n%s", err, out)
					}
					if !strings.Contains(string(out), "saved update channel") || !strings.Contains(string(out), "--stable or --rc") {
						t.Fatalf("missing safe recovery guidance: %s", out)
					}
				}
			} else {
				if runErr != nil {
					t.Fatalf("valid helper failed: %v\n%s", runErr, out)
				}
				got, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				want := append([]string{"EXECUTED"}, tc.want...)
				if !reflect.DeepEqual(strings.Split(strings.TrimSuffix(string(got), "\n"), "\n"), want) {
					t.Fatalf("installer args %q, want %q", got, want)
				}
			}
			if !tc.missingConfig && !tc.directory && !tc.brokenLink {
				got, err := os.ReadFile(configPath)
				if err != nil || string(got) != tc.config {
					t.Fatalf("helper changed configuration: %v, %q", err, got)
				}
			}
		})
	}
}
