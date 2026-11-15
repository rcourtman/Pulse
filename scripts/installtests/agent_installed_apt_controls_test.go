//go:build linux

package installtests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"gopkg.in/yaml.v3"
)

func TestNativeAPTScenario(t *testing.T) {
	for _, value := range []string{"", "recovery", "collector-only-apt"} {
		got, err := nativeProfileScenario(value)
		if err != nil || (value == "" && got != "recovery") || (value != "" && got != value) {
			t.Fatalf("scenario %q: %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"skip", "collector-only-apt ", "../recovery", "all"} {
		if _, err := nativeProfileScenario(value); err == nil {
			t.Fatalf("accepted unknown scenario %q", value)
		}
	}
}

func TestNativeAPTReferenceInventory(t *testing.T) {
	const two = "Inst pulse-native-apt-one [1.0] (2.0 native [amd64])\nInst pulse-native-apt-two [1.0] (2.0 native [amd64])\n"
	for _, tc := range []struct {
		name, output string
		count        int
	}{
		{"empty", "Reading package lists...\n0 upgraded, 0 newly installed\n", 0},
		{"positive", two, 2},
		{"new-dependency", "Inst new-package (1.0 native [amd64])\n", 1},
		{"complete-beyond-display-bound", strings.Repeat(two, 101), 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nativeAPTParse(tc.output)
			if err != nil || got.Count != tc.count || len(got.Packages) != min(tc.count, 200) || !strings.HasPrefix(got.Hash, "sha256:") {
				t.Fatalf("reference: %+v, %v", got, err)
			}
			if tc.name == "new-dependency" && (got.Packages[0].InstalledVersion != "" || got.Packages[0].AvailableVersion != "1.0") {
				t.Fatal("new dependency version was not accounted")
			}
		})
	}
	a, _ := nativeAPTParse(two)
	b, _ := nativeAPTParse("Inst pulse-native-apt-two [1.0] (2.0 native [amd64])\nInst pulse-native-apt-one [1.0] (2.0 native [amd64])\n")
	if a.Hash != b.Hash || reflect.DeepEqual(a.Packages, b.Packages) {
		t.Fatal("inventory identity must sort all entries without reordering the display")
	}
	c, _ := nativeAPTParse(strings.Repeat(two, 101))
	d, _ := nativeAPTParse(strings.Repeat(two, 100))
	if c.Hash == d.Hash {
		t.Fatal("inventory hash was truncated at the display bound")
	}
	for _, bad := range []string{"Inst broken\n", "Inst broken [1.0 (2.0 repo)\n", "Inst broken [1.0] (\n", "Inst broken [1.0]\n"} {
		if _, err := nativeAPTParse(bad); err == nil {
			t.Fatalf("accepted incomplete reference %q", bad)
		}
	}
}

func TestNativeAPTReferenceSimulation(t *testing.T) {
	if _, err := os.Stat("/usr/bin/apt-get"); err != nil {
		t.Skip("real APT is required for this fixture control; native hosted acceptance remains mandatory")
	}
	root := t.TempDir()
	nativeAPTFixture(t, root)
	before := nativeProfileTree(t, root)
	got := nativeAPTReference(t, nil, root, "")
	if got.Count != 2 || len(got.Packages) != 2 {
		t.Fatalf("real APT fixture inventory: %+v", got)
	}
	for i, name := range []string{"pulse-native-apt-one", "pulse-native-apt-two"} {
		if got.Packages[i] != (agentshost.PackageUpdate{Name: name, InstalledVersion: "1.0", AvailableVersion: "2.0"}) {
			t.Fatalf("real APT package %d: %+v", i, got.Packages[i])
		}
	}
	if !reflect.DeepEqual(before, nativeProfileTree(t, root)) {
		t.Fatal("reference simulation modified its private index or status")
	}
}

func TestNativeAPTSnapshotPreservesLinksWithoutTraversal(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "auxfiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "regular"), []byte("complete index bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	externalFile := filepath.Join(outside, "apt-mirrors.txt")
	if err := os.WriteFile(externalFile, []byte("not part of the index tree"), 0o600); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"auxfiles/_etc_apt_apt-mirrors.txt": externalFile,
		"relative":                          "regular",
		"dangling":                          "missing",
		"directory":                         outside,
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	before, err := nativeAPTSnapshotTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 3+len(links) || before["regular"].Hash != secureRuntimeHash([]byte("complete index bytes")) {
		t.Fatal("snapshot omitted entries/bytes or traversed a directory link")
	}
	for name, identity := range before {
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if !identity.Present || identity.Mode != info.Mode() || identity.UID != stat.Uid || identity.GID != stat.Gid {
			t.Fatalf("snapshot lost entry type, mode or ownership: %s", name)
		}
	}
	for name, target := range links {
		if before[name].Mode&os.ModeSymlink == 0 || before[name].Hash != secureRuntimeHash([]byte(target)) {
			t.Fatalf("snapshot did not bind the literal link bytes: %s", name)
		}
	}
	if before["."].Mode&os.ModeDir == 0 || before["auxfiles"].Mode&os.ModeDir == 0 {
		t.Fatal("snapshot omitted directory identity")
	}
	// Neither an outside referent's bytes nor children of a directory link
	// belong to this tree. Changing them must not expand or change its receipt.
	if err := os.WriteFile(externalFile, []byte("changed outside the tree"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := nativeAPTSnapshotTree(root)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("snapshot followed a link outside its tree: %v", err)
	}
}

func TestNativeAPTSnapshotDetectsEntryChanges(t *testing.T) {
	for _, name := range []string{
		"file-bytes", "file-mode", "directory-mode", "root-mode", "link-target",
		"link-removed", "link-to-file", "file-to-link", "link-to-directory",
		"file-added", "empty-directory-added", "empty-directory-removed",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			file, link, dir := filepath.Join(root, "file"), filepath.Join(root, "link"), filepath.Join(root, "dir")
			if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("file", link); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			before, err := nativeAPTSnapshotTree(root)
			if err != nil {
				t.Fatal(err)
			}
			after, err := nativeAPTSnapshotTree(root)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("unchanged tree differed: %v", err)
			}
			switch name {
			case "file-bytes":
				err = os.WriteFile(file, []byte("changed"), 0o600)
			case "file-mode":
				err = os.Chmod(file, 0o640)
			case "directory-mode":
				err = os.Chmod(dir, 0o750)
			case "root-mode":
				err = os.Chmod(root, 0o750)
			case "file-added":
				err = os.WriteFile(filepath.Join(root, "added"), []byte("file"), 0o600)
			case "empty-directory-added":
				err = os.Mkdir(filepath.Join(root, "added"), 0o700)
			case "empty-directory-removed":
				err = os.Remove(dir)
			default:
				target := link
				if name == "file-to-link" {
					target = file
				}
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				switch name {
				case "link-target":
					err = os.Symlink("./file", link) // Same referent, different literal link bytes.
				case "link-to-file":
					err = os.WriteFile(link, []byte("file"), 0o600) // Same hash, different type.
				case "file-to-link":
					err = os.Symlink("file", file) // A dangling self-link is still observed, not followed.
				case "link-to-directory":
					err = os.Mkdir(link, 0o700)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err = nativeAPTSnapshotTree(root)
			if err != nil || reflect.DeepEqual(before, after) {
				t.Fatalf("changed tree was not fully observed: %v", err)
			}
		})
	}
}

func TestNativeAPTSnapshotRejectsIncompleteTrees(t *testing.T) {
	for _, name := range []string{"missing-root", "file-root", "symlink-root", "fifo-child"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "lists")
			switch name {
			case "file-root":
				if err := os.WriteFile(root, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink-root":
				if err := os.Symlink(t.TempDir(), root); err != nil {
					t.Fatal(err)
				}
			case "fifo-child":
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "a-regular"), []byte("observed first"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(filepath.Join(root, "z-fifo"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got, err := nativeAPTSnapshotTree(root); err == nil || got != nil {
				t.Fatalf("unsupported or incomplete tree became state-preservation evidence: %v", err)
			}
		})
	}
}

func TestNativeAPTReportControls(t *testing.T) {
	after := time.Now().Add(-time.Minute).UTC()
	want, err := nativeAPTParse("Inst one [1.0] (2.0 native [amd64])\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, safe := range []bool{false, true} {
		for _, tc := range []struct {
			name   string
			change func(*secureRuntimeLabReport)
		}{
			{"complete", func(*secureRuntimeLabReport) {}},
			{"absent", func(r *secureRuntimeLabReport) { r.PackageUpdates = nil }},
			{"unsupported", func(r *secureRuntimeLabReport) { r.PackageUpdates.Supported = false }},
			{"manager", func(r *secureRuntimeLabReport) { r.PackageUpdates.Manager = "unknown" }},
			{"error-zero", func(r *secureRuntimeLabReport) {
				r.PackageUpdates.Error = "package update inspection failed"
				r.PackageUpdates.PendingCount = 0
			}},
			{"partial-count", func(r *secureRuntimeLabReport) { r.PackageUpdates.PendingCount = 0 }},
			{"partial-list", func(r *secureRuntimeLabReport) { r.PackageUpdates.Packages = nil }},
			{"wrong-inventory", func(r *secureRuntimeLabReport) { r.PackageUpdates.InventoryHash = "sha256:other" }},
			{"stale-cache", func(r *secureRuntimeLabReport) { r.PackageUpdates.CheckedAt = after }},
			{"future-cache", func(r *secureRuntimeLabReport) { r.PackageUpdates.CheckedAt = r.ReceivedAt.Add(time.Second) }},
			{"old-report", func(r *secureRuntimeLabReport) { r.ReceivedAt = after }},
			{"wrong-source-version", func(r *secureRuntimeLabReport) { r.AgentVersion = "old" }},
			{"wrong-agent", func(r *secureRuntimeLabReport) { r.AgentID = "other" }},
			{"wrong-host", func(r *secureRuntimeLabReport) { r.Hostname = "other" }},
			{"command-authority", func(r *secureRuntimeLabReport) { r.Authority = "command-capable" }},
			{"commands", func(r *secureRuntimeLabReport) { r.CommandsEnabled = true }},
			{"wrong-uid-profile", func(r *secureRuntimeLabReport) { r.RunningAsRoot = safe }},
		} {
			t.Run(tc.name+"-safe-"+map[bool]string{true: "true", false: "false"}[safe], func(t *testing.T) {
				r := secureRuntimeLabReport{ReceivedAt: after.Add(2 * time.Second), AgentID: secureRuntimeLabAgentID, Hostname: secureRuntimeLabHostname,
					AgentVersion: "current", RunningAsRoot: !safe, ServiceUser: "pulse-agent", TypedHelper: safe, Authority: "monitoring-only",
					PackageUpdates: &agentshost.PackageUpdateStatus{Supported: true, Manager: "apt", PendingCount: want.Count, InventoryHash: want.Hash,
						Packages: append([]agentshost.PackageUpdate(nil), want.Packages...), CheckedAt: after.Add(time.Second)}}
				tc.change(&r)
				if nativeAPTReportMatches(r, want, "current", after, safe) != (tc.name == "complete") {
					t.Fatalf("incorrect report acceptance: %+v", r)
				}
			})
		}
	}
}

func TestNativeAPTPackageReportIsAuthenticated(t *testing.T) {
	fixture := newSecureRuntimeLabFixture(nil, "", nil, nil, "fixture")
	t.Cleanup(fixture.actionServer.Shutdown)
	const payload = `{"agent":{"id":"` + secureRuntimeLabAgentID + `","hostname":"` + secureRuntimeLabHostname + `","version":"fixture"},"host":{"packageUpdates":{"supported":true,"manager":"apt","pendingCount":2,"inventoryHash":"sha256:fixture"}}}`
	for _, authorized := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", strings.NewReader(payload))
		if authorized {
			r.Header.Set("X-API-Token", secureRuntimeLabToken)
		}
		response := httptest.NewRecorder()
		fixture.ServeHTTP(response, r)
		reports, _, _, _ := fixture.snapshot()
		if !authorized {
			if response.Code != http.StatusUnauthorized || len(reports) != 0 {
				t.Fatal("unauthenticated package report became accepted evidence")
			}
		} else if response.Code != http.StatusOK || len(reports) != 1 || reports[0].PackageUpdates == nil || reports[0].PackageUpdates.PendingCount != 2 || reports[0].PackageUpdates.InventoryHash != "sha256:fixture" {
			t.Fatal("authenticated package snapshot was lost from the real report consumer")
		}
	}
}

func TestNativeAPTWorkflowIsolation(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Name     string
			Strategy struct {
				FailFast *bool `yaml:"fail-fast"`
				Matrix   struct{ Scenario []string }
			}
			Steps []struct {
				Name string
				Env  map[string]string
				With map[string]string
				Run  string
			}
		}
	}
	if err := yaml.Unmarshal(secureRuntimeReadFile(t, repoFile(".github", "workflows", "unified-agent-native.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["installed-profile-systemd"]
	if job.Name != "${{ matrix.scenario == 'recovery' && 'Signed installer and saved-CA profile recovery' || 'Collector-only APT reporting and profile recovery' }}" {
		t.Fatal("preserve the original recovery check context and a distinct collector-only result")
	}
	if job.Strategy.FailFast == nil || *job.Strategy.FailFast || !reflect.DeepEqual(job.Strategy.Matrix.Scenario, []string{"recovery", "collector-only-apt"}) {
		t.Fatal("both fixed scenarios require independent fresh VMs and terminal results")
	}
	execute, retain := false, false
	for _, step := range job.Steps {
		switch step.Name {
		case "Exercise signed TLS installed profile forward and reverse":
			execute = step.Env["NATIVE_PROFILE_SCENARIO"] == "${{ matrix.scenario }}" && strings.Contains(step.Run, `PULSE_NATIVE_PROFILE_SCENARIO="$NATIVE_PROFILE_SCENARIO"`)
		case "Retain native installed profile observations":
			retain = strings.Contains(step.With["name"], "${{ matrix.scenario }}")
		}
	}
	if !execute || !retain {
		t.Fatal("scenario selection or independent artifact identity was lost")
	}
	// Schema remains JSON-readable by ordinary artifact consumers.
	data, err := json.Marshal(nativeProfileReceipt{Schema: 1, Scenario: "collector-only-apt"})
	if err != nil || !strings.Contains(string(data), `"scenario":"collector-only-apt"`) {
		t.Fatal("native receipt lost its scenario identity")
	}
}
