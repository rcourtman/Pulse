package truenas

import (
	"testing"
	"time"
)

// TrueNAS SCALE catalog apps ship one-shot init containers from ixSystems' own
// base images (permissions, postgres_upgrade, pgvecto_upgrade). They run to
// completion and stay exited for the life of the app. TrueNAS reports them as
// EXITED, which its own state machine reserves for a normal exit code, and
// still reports the app as RUNNING. Pulse used to raise a CRITICAL per exited
// container on exactly those healthy apps, and because the containers never
// restart the alerts never cleared. Refs #1677.
func TestRunningTrueNASAppWithCompletedOneShotContainersRaisesNoIncident(t *testing.T) {
	observedAt := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)

	immich := App{
		ID:    "immich",
		Name:  "immich",
		State: "RUNNING",
		Containers: []AppContainer{
			{ID: "immich-server-1", ServiceName: "server", Image: "ghcr.io/immich-app/immich-server:v3.0.3", State: "running"},
			{ID: "immich-pgvecto-1", ServiceName: "pgvecto", Image: "ghcr.io/immich-app/postgres:18", State: "running"},
			{ID: "immich-ml-1", ServiceName: "machine-learning", Image: "ghcr.io/immich-app/immich-ml:v3.0.3-cuda", State: "running"},
			{ID: "immich-redis-1", ServiceName: "redis", Image: "valkey/valkey:9.1.1", State: "running"},
			{ID: "immich-pgvecto-upgrade-1", ServiceName: "pgvecto_upgrade", Image: "ixsystems/postgres-upgrade:1.2.13", State: "exited"},
			{ID: "immich-permissions-1", ServiceName: "permissions", Image: "ixsystems/container-utils:1.0.2", State: "exited"},
		},
	}
	if incidents := incidentsFromAppState(immich, observedAt); len(incidents) != 0 {
		t.Fatalf("running app with completed one-shots must raise nothing, got %+v", incidents)
	}

	// The init container sorting first must not decide the app's rendered state.
	backrest := App{
		ID:    "backrest",
		Name:  "backrest",
		State: "RUNNING",
		Containers: []AppContainer{
			{ID: "backrest-permissions-1", ServiceName: "permissions", Image: "ixsystems/container-utils:1.0.2", State: "exited"},
			{ID: "backrest-1", ServiceName: "backrest", Image: "ghcr.io/garethgeorge/backrest:v1.14.1", State: "running"},
		},
	}
	if incidents := incidentsFromAppState(backrest, observedAt); len(incidents) != 0 {
		t.Fatalf("init container ordering must not raise an incident, got %+v", incidents)
	}
	if got := appContainerState(backrest); got != "running" {
		t.Fatalf("appContainerState = %q, want \"running\"", got)
	}
}

// A container that exits with an abnormal code is reported by TrueNAS as
// CRASHED, not EXITED, and TrueNAS rolls that up into an app-level CRASHED.
// Pulse must keep naming the failing service so the app-level incident is
// actionable.
func TestCrashedTrueNASAppContainerStillRaisesIncident(t *testing.T) {
	observedAt := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)

	app := App{
		ID:    "mealie",
		Name:  "mealie",
		State: "CRASHED",
		Containers: []AppContainer{
			{ID: "mealie-permissions-1", ServiceName: "permissions", State: "exited"},
			{ID: "mealie-1", ServiceName: "mealie", State: "crashed"},
		},
	}
	incidents := incidentsFromAppState(app, observedAt)
	if len(incidents) != 2 {
		t.Fatalf("expected app-level and container-level incidents, got %+v", incidents)
	}
	var sawApp, sawContainer bool
	for _, incident := range incidents {
		switch incident.Code {
		case "truenas_app_crashed":
			sawApp = true
		case "truenas_app_container_failed":
			sawContainer = true
			if incident.NativeID != "app:mealie:container:mealie-1" {
				t.Fatalf("container incident must key on the crashed service, got %q", incident.NativeID)
			}
			if incident.Summary != "Container mealie in TrueNAS app mealie is crashed" {
				t.Fatalf("container incident summary = %q", incident.Summary)
			}
		}
	}
	if !sawApp || !sawContainer {
		t.Fatalf("expected both incident codes, got %+v", incidents)
	}
	if got := appContainerState(app); got != "crashed" {
		t.Fatalf("appContainerState = %q, want \"crashed\"", got)
	}
}

// A stopped app has every container exited, and that must still read as exited
// rather than falling through to the app state.
func TestStoppedTrueNASAppContainerStateStaysExited(t *testing.T) {
	app := App{
		ID:         "adguard-home",
		Name:       "AdGuard Home",
		State:      "STOPPED",
		Containers: []AppContainer{{ID: "adguard-home-1", ServiceName: "adguard-home", State: "exited"}},
	}
	if got := appContainerState(app); got != "exited" {
		t.Fatalf("appContainerState = %q, want \"exited\"", got)
	}
	if got := appContainerState(App{ID: "bare", Name: "bare", State: "DEPLOYING"}); got != "deploying" {
		t.Fatalf("appContainerState with no containers = %q, want \"deploying\"", got)
	}
}
