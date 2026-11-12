package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func finishedJobHost(hostID string, container models.DockerContainer) models.DockerHost {
	return models.DockerHost{
		ID:          hostID,
		DisplayName: "Docker Host",
		Hostname:    "docker.local",
		Containers:  []models.DockerContainer{container},
	}
}

// ranFor sets the start and finish times of a container that ran for d and
// ended an hour ago.
func ranFor(container models.DockerContainer, d time.Duration) models.DockerContainer {
	finished := time.Now().Add(-time.Hour)
	started := finished.Add(-d)
	container.StartedAt = &started
	container.FinishedAt = &finished
	return container
}

func dockerStateAlertID(host models.DockerHost) string {
	return fmt.Sprintf("docker-container-state-%s", DockerContainerResourceID(host.ID, host.Containers[0].ID, ""))
}

// A migration or init container that exits 0 under a policy that leaves it
// down has finished. It is not a stopped service and raises no state alert.
func TestDockerContainerStateLeavesFinishedJobsAlone(t *testing.T) {
	for _, policy := range []string{"no", "on-failure", " On-Failure "} {
		t.Run(policy, func(t *testing.T) {
			m := newTestManager(t)
			host := finishedJobHost("host-job", ranFor(models.DockerContainer{
				ID:            "job-1",
				Name:          "db-migrate",
				State:         "exited",
				Status:        "Exited (0) 2 minutes ago",
				ExitCode:      0,
				RestartPolicy: policy,
			}, 40*time.Second))

			m.CheckDockerHost(host)
			m.CheckDockerHost(host)

			if testHasActiveAlert(t, m, dockerStateAlertID(host)) {
				t.Fatalf("a container that exited 0 under restart policy %q raised a state alert", policy)
			}
		})
	}
}

// Every other exited container still alerts: a failure, an OOM kill, a
// container Docker would have restarted, and one whose policy is unknown.
func TestDockerContainerStateStillAlertsOnStoppedServices(t *testing.T) {
	oomKilled := true
	cases := []struct {
		name      string
		container models.DockerContainer
	}{
		{"failed under no", models.DockerContainer{ExitCode: 1, RestartPolicy: "no", Status: "Exited (1) 2 minutes ago"}},
		{"failed under on-failure", models.DockerContainer{ExitCode: 137, RestartPolicy: "on-failure", Status: "Exited (137) 2 minutes ago"}},
		{"oom killed", models.DockerContainer{ExitCode: 0, RestartPolicy: "no", OOMKilled: &oomKilled, Status: "Exited (0) 2 minutes ago"}},
		{"stopped under always", models.DockerContainer{ExitCode: 0, RestartPolicy: "always", Status: "Exited (0) 2 minutes ago"}},
		{"stopped under unless-stopped", models.DockerContainer{ExitCode: 0, RestartPolicy: "unless-stopped", Status: "Exited (0) 2 minutes ago"}},
		{"policy not reported", models.DockerContainer{ExitCode: 0, Status: "Exited (0) 2 minutes ago"}},
		{"policy not recognised", models.DockerContainer{ExitCode: 0, RestartPolicy: "sometimes", Status: "Exited (0) 2 minutes ago"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			container := ranFor(tc.container, 40*time.Second)
			container.ID = "svc-1"
			container.Name = "db"
			container.State = "exited"
			host := finishedJobHost("host-svc", container)

			m.CheckDockerHost(host)
			m.CheckDockerHost(host)

			if !testHasActiveAlert(t, m, dockerStateAlertID(host)) {
				t.Fatalf("expected a state alert for an exited container (%s)", tc.name)
			}
		})
	}
}

// Only "exited" can be a finished job. A container that was created and
// never started, or is dead, still alerts whatever its exit code and policy.
func TestDockerContainerStateFinishedRuleCoversExitedOnly(t *testing.T) {
	for _, state := range []string{"created", "dead", "paused"} {
		t.Run(state, func(t *testing.T) {
			m := newTestManager(t)
			host := finishedJobHost("host-other", ranFor(models.DockerContainer{
				ID:            "other-1",
				Name:          "worker",
				State:         state,
				Status:        state,
				ExitCode:      0,
				RestartPolicy: "no",
			}, 40*time.Second))

			m.CheckDockerHost(host)
			m.CheckDockerHost(host)

			if !testHasActiveAlert(t, m, dockerStateAlertID(host)) {
				t.Fatalf("expected a state alert for a %s container", state)
			}
		})
	}
}

// An alert raised while the policy was unknown clears once the agent
// reports that the container is a finished job, which is what happens when
// an agent is updated to one that sends the policy.
func TestDockerContainerStateClearsWhenFinishedJobIsRecognised(t *testing.T) {
	m := newTestManager(t)
	container := ranFor(models.DockerContainer{
		ID:       "job-2",
		Name:     "certs-init",
		State:    "exited",
		Status:   "Exited (0) 5 minutes ago",
		ExitCode: 0,
	}, 3*time.Second)
	host := finishedJobHost("host-upgrade", container)

	m.CheckDockerHost(host)
	m.CheckDockerHost(host)
	if !testHasActiveAlert(t, m, dockerStateAlertID(host)) {
		t.Fatal("expected a state alert while the restart policy is unknown")
	}

	host.Containers[0].RestartPolicy = "no"
	m.CheckDockerHost(host)

	if testHasActiveAlert(t, m, dockerStateAlertID(host)) {
		t.Fatal("the state alert stayed active after the container was reported as a finished job")
	}
}

// A severity chosen on the container's own row is an explicit request to
// hear whenever it is not running, finished or not.
func TestDockerContainerStateOwnSeverityStillAlertsOnFinishedJobs(t *testing.T) {
	m := newTestManager(t)
	cfg := m.GetConfig()
	cfg.Overrides = map[string]ThresholdConfig{
		dockerContainerOverrideKey("host-own", "nightly-backup"): {PoweredOffSeverity: AlertLevelCritical},
	}
	m.UpdateConfig(cfg)

	host := finishedJobHost("host-own", ranFor(models.DockerContainer{
		ID:            "job-3",
		Name:          "nightly-backup",
		State:         "exited",
		Status:        "Exited (0) 1 hour ago",
		ExitCode:      0,
		RestartPolicy: "no",
	}, 5*time.Minute))

	m.CheckDockerHost(host)
	m.CheckDockerHost(host)

	alert, exists := testLookupActiveAlert(t, m, dockerStateAlertID(host))
	if !exists {
		t.Fatal("a container with its own severity should alert even when it finished cleanly")
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("alert level = %s, want critical from the row's own severity", alert.Level)
	}
}

// A CPU-only override carries no severity of its own, so it does not turn
// the alert back on for a finished job.
func TestDockerContainerStateMetricOverrideLeavesFinishedJobsAlone(t *testing.T) {
	m := newTestManager(t)
	cfg := m.GetConfig()
	cfg.Overrides = map[string]ThresholdConfig{
		dockerContainerOverrideKey("host-cpu", "db-migrate"): {CPU: &HysteresisThreshold{Trigger: 95, Clear: 90}},
	}
	m.UpdateConfig(cfg)

	host := finishedJobHost("host-cpu", ranFor(models.DockerContainer{
		ID:            "job-4",
		Name:          "db-migrate",
		State:         "exited",
		Status:        "Exited (0) 1 hour ago",
		ExitCode:      0,
		RestartPolicy: "on-failure",
	}, 5*time.Minute))

	m.CheckDockerHost(host)
	m.CheckDockerHost(host)

	if testHasActiveAlert(t, m, dockerStateAlertID(host)) {
		t.Fatal("a CPU-only override raised a state alert for a finished job")
	}
}

// A service with no restart policy exits 0 when Docker shuts down for a
// host reboot and then stays down. It ran for days, not minutes, and that
// is what tells it from a job: it still alerts. So does a container whose
// run time is unknown.
func TestDockerContainerStateStillAlertsOnLongRunningServiceStoppedCleanly(t *testing.T) {
	base := models.DockerContainer{
		ID:            "svc-2",
		Name:          "jellyfin",
		State:         "exited",
		Status:        "Exited (0) 3 hours ago",
		ExitCode:      0,
		RestartPolicy: "no",
	}
	finished := time.Now().Add(-3 * time.Hour)
	backwards := finished.Add(time.Minute)
	noFinish := base
	noFinish.StartedAt = &finished
	noStart := base
	noStart.FinishedAt = &finished
	reversed := base
	reversed.StartedAt = &backwards
	reversed.FinishedAt = &finished

	cases := []struct {
		name      string
		container models.DockerContainer
	}{
		{"ran for nine days", ranFor(base, 9*24*time.Hour)},
		{"ran for exactly the limit", ranFor(base, dockerFinishedJobMaxRun)},
		{"no times reported", base},
		{"finish time missing", noFinish},
		{"start time missing", noStart},
		{"finished before it started", reversed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			host := finishedJobHost("host-reboot", tc.container)

			m.CheckDockerHost(host)
			m.CheckDockerHost(host)

			if !testHasActiveAlert(t, m, dockerStateAlertID(host)) {
				t.Fatalf("expected a state alert for a cleanly stopped container (%s)", tc.name)
			}
		})
	}

	t.Run("just under the limit is a job", func(t *testing.T) {
		m := newTestManager(t)
		host := finishedJobHost("host-long-job", ranFor(base, dockerFinishedJobMaxRun-time.Second))

		m.CheckDockerHost(host)
		m.CheckDockerHost(host)

		if testHasActiveAlert(t, m, dockerStateAlertID(host)) {
			t.Fatal("a container that ran for under the limit and exited 0 raised a state alert")
		}
	})
}
