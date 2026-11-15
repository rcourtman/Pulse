//go:build linux

package installtests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
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
			Strategy struct {
				FailFast *bool `yaml:"fail-fast"`
				Matrix   map[string][]string
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
	if job.Strategy.FailFast == nil || *job.Strategy.FailFast || !reflect.DeepEqual(job.Strategy.Matrix["scenario"], []string{"recovery", "collector-only-apt"}) {
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
