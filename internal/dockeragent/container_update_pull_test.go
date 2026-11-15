package dockeragent

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

// pullTestAgent runs an update whose pull returns body and whose pulled tag
// resolves to pulledID. Any stop call is counted: a failed or pointless pull
// must leave the running container alone.
func pullTestAgent(body string, pulledID string, stops *int) *Agent {
	return &Agent{
		docker: &fakeDockerClient{
			containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
				return baseInspect(), nil
			},
			imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(body)), nil
			},
			imageInspectWithRawFn: func(context.Context, string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{ID: pulledID}, nil, nil
			},
			containerStopFn: func(context.Context, string, dockerContainerStopOptions) error {
				*stops++
				return errors.New("stop reached")
			},
		},
		logger: zerolog.Nop(),
	}
}

func TestUpdateFailsWhenThePullStreamReportsAnError(t *testing.T) {
	cases := map[string]string{
		"rate limit":     `{"status":"Pulling from library/nginx"}` + "\n" + `{"errorDetail":{"message":"toomanyrequests: You have reached your pull rate limit"},"error":"toomanyrequests"}`,
		"denied":         `{"errorDetail":{"code":401,"message":"pull access denied"}}`,
		"plain error":    `{"error":"manifest unknown"}`,
		"cut off stream": `{"status":"Downloading","progressDetail":{"current":1`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			stops := 0
			result := pullTestAgent(body, "sha256:new", &stops).updateContainerWithProgress(context.Background(), "container1", nil)
			if result.Success || result.ReasonCode != agentexec.ActionRefusalImagePullFailed || !strings.Contains(result.Error, "Failed to pull image") {
				t.Fatalf("result = %+v, want a failed pull", result)
			}
			if stops != 0 || result.BackupCreated || result.RollbackAttempted {
				t.Fatalf("a failed pull touched the container: stops=%d result=%+v", stops, result)
			}
		})
	}
}

func TestUpdateRefusesWhenThePulledImageIsAlreadyRunning(t *testing.T) {
	stops := 0
	result := pullTestAgent(`{"status":"Image is up to date for nginx:latest"}`, baseInspect().Image, &stops).updateContainerWithProgress(context.Background(), "container1", nil)
	if result.Success || result.ReasonCode != agentexec.ActionRefusalImageUnchanged || !strings.Contains(result.Error, "no newer image") {
		t.Fatalf("result = %+v, want an unchanged-image refusal", result)
	}
	if stops != 0 {
		t.Fatalf("an unchanged image stopped the container %d times", stops)
	}
}

func TestUpdateContinuesPastAPullThatChangedTheImage(t *testing.T) {
	stops := 0
	result := pullTestAgent(`{"status":"Downloaded newer image for nginx:latest"}`, "sha256:new", &stops).updateContainerWithProgress(context.Background(), "container1", nil)
	if stops != 1 || result.ReasonCode != agentexec.ActionRefusalContainerStopFailed {
		t.Fatalf("stops=%d result=%+v, want the update to reach the stop step and name it", stops, result)
	}
}

func TestUpdateNamesEachStepThatStopsBeforeAChange(t *testing.T) {
	t.Run("inspect", func(t *testing.T) {
		agent := &Agent{docker: &fakeDockerClient{
			containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
				return containertypes.InspectResponse{}, errors.New("daemon gone")
			},
		}, logger: zerolog.Nop()}
		if got := agent.updateContainerWithProgress(context.Background(), "container1", nil).ReasonCode; got != agentexec.ActionRefusalTargetInspectionUnavailable {
			t.Fatalf("reason = %q", got)
		}
	})
	t.Run("stopped container rename", func(t *testing.T) {
		agent := &Agent{docker: &fakeDockerClient{
			containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
				inspect := baseInspect()
				inspect.State = &containertypes.State{Running: false}
				return inspect, nil
			},
			imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("{}")), nil
			},
			containerRenameFn: func(context.Context, string, string) error { return errors.New("rename failed") },
		}, logger: zerolog.Nop()}
		result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
		if result.ReasonCode != agentexec.ActionRefusalContainerRenameFailed || result.RollbackAttempted {
			t.Fatalf("result = %+v, want a coded rename failure with nothing to roll back", result)
		}
	})
}

func TestUpdateReportsTheRestartAfterARenameFailureAsARollback(t *testing.T) {
	for name, startErr := range map[string]error{"restored": nil, "not restored": errors.New("start failed")} {
		t.Run(name, func(t *testing.T) {
			agent := &Agent{docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return baseInspect(), nil
				},
				imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("{}")), nil
				},
				containerStopFn:   func(context.Context, string, dockerContainerStopOptions) error { return nil },
				containerRenameFn: func(context.Context, string, string) error { return errors.New("rename failed") },
				containerStartFn:  func(context.Context, string, dockerContainerStartOptions) error { return startErr },
			}, logger: zerolog.Nop()}
			result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
			if !result.RollbackAttempted || result.RolledBack != (startErr == nil) || result.ReasonCode != "" {
				t.Fatalf("result = %+v, want the restart reported as a rollback", result)
			}
		})
	}
}

func TestUpdateTreatsAStopThatLeftTheContainerDownAsARollback(t *testing.T) {
	inspects, starts := 0, 0
	agent := &Agent{docker: &fakeDockerClient{
		containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
			inspects++
			inspect := baseInspect()
			if inspects > 1 {
				// The stop call timed out, but the daemon had stopped it.
				inspect.State = &containertypes.State{Running: false}
			}
			return inspect, nil
		},
		imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("{}")), nil
		},
		containerStopFn: func(context.Context, string, dockerContainerStopOptions) error {
			return errors.New("stop response lost")
		},
		containerStartFn: func(context.Context, string, dockerContainerStartOptions) error {
			starts++
			return nil
		},
	}, logger: zerolog.Nop()}
	result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
	if result.ReasonCode != "" || !result.RollbackAttempted || !result.RolledBack || starts != 1 {
		t.Fatalf("starts=%d result=%+v, want the stopped container restarted and reported as a rollback", starts, result)
	}
}

func TestUpdateRestoresTheNameWhenAFailedRenameHappenedAnyway(t *testing.T) {
	inspects := 0
	var renames []string
	agent := &Agent{docker: &fakeDockerClient{
		containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
			inspects++
			inspect := baseInspect()
			if inspects > 1 {
				inspect.Name = "/app" + backupContainerMarker + "20261115_120000"
			}
			return inspect, nil
		},
		imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("{}")), nil
		},
		containerStopFn: func(context.Context, string, dockerContainerStopOptions) error { return nil },
		containerRenameFn: func(_ context.Context, _ string, name string) error {
			renames = append(renames, name)
			if len(renames) == 1 {
				return errors.New("rename response lost")
			}
			return nil
		},
		containerStartFn: func(context.Context, string, dockerContainerStartOptions) error { return nil },
	}, logger: zerolog.Nop()}
	result := agent.updateContainerWithProgress(context.Background(), "container1", nil)
	if len(renames) != 2 || renames[1] != "app" {
		t.Fatalf("renames = %v, want the original name restored", renames)
	}
	if result.ReasonCode != "" || !result.RollbackAttempted || !result.RolledBack {
		t.Fatalf("result = %+v, want a reported, successful rollback", result)
	}
}
