package notifications_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

const checkpointProcessMarker = "GROUPED_PROCESS "

type checkpointProcessState struct {
	Phase   string                           `json:"phase"`
	Stats   map[string]int                   `json:"stats"`
	Log     []notifications.DeliveryLogEntry `json:"entries"`
	Started time.Time                        `json:"started"`
	Active  int                              `json:"active"`
}

// This registered helper runs real alert/notification managers in a separate
// process with persistent state, guest-local receivers and an actual SIGTERM.
// It is not the installed server executable or a system-service qualification.
func TestGroupedWarningProcessHelper(t *testing.T) {
	dir := os.Getenv("PULSE_GROUPED_TEST_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	t.Setenv("PULSE_DATA_DIR", dir)
	url := os.Getenv("PULSE_GROUPED_TEST_RECEIVER")
	cooldown, err := strconv.Atoi(os.Getenv("PULSE_GROUPED_TEST_COOLDOWN"))
	require.NoError(t, err)
	var am *alerts.Manager
	var nm *notifications.NotificationManager
	fired := make(chan *alerts.Alert, 8)
	open := func() {
		nm = notifications.NewNotificationManagerWithDeferredQueue("", dir)
		require.NotNil(t, nm.GetQueue())
		require.NoError(t, nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
		nm.SetCooldown(cooldown)
		nm.SetGroupingConfig(true, 4, true, true)
		for _, id := range []string{"left", "right"} {
			nm.AddWebhook(notifications.WebhookConfig{ID: id, Name: "Fixture " + id, URL: url + "/" + id, Enabled: true, Service: "generic"})
		}
		am = alerts.NewManagerWithDataDir(dir, alerts.WithDurableAlertStore())
		cfg := am.GetConfig()
		cfg.Enabled = true
		cfg.ActivationState = alerts.ActivationActive
		cfg.FlappingEnabled = false
		cfg.Schedule.Cooldown = cooldown
		cfg.Schedule.MaxAlertsHour = 100
		cfg.TimeThresholds = map[string]int{"agent": 0, "storage": 0}
		cfg.SuppressionWindow = 0
		am.UpdateConfig(cfg)
		am.SetAlertCallback(func(a *alerts.Alert) { nm.SendAlert(a); fired <- a.Clone() })
		nm.StartQueueProcessing()
	}
	stop := func() {
		if am != nil {
			am.Stop()
			am = nil
		}
		if nm != nil {
			nm.Stop()
			nm = nil
		}
	}
	defer stop()
	open()
	started := time.Now().UTC()
	host := models.Host{ID: "process-host", Hostname: "Process host", Sensors: models.HostSensorSummary{Custom: []models.HostCustomSensorMetric{
		{ID: "probe-one", Name: "Probe one", Status: "warning", ObservedAt: started},
		{ID: "probe-two", Name: "Probe two", Status: "warning", ObservedAt: started},
	}}}
	if os.Getenv("PULSE_GROUPED_TEST_SEED") == "1" {
		am.CheckHost(host)
		for range 2 {
			select {
			case a := <-fired:
				require.Equal(t, alerts.AlertLevelWarning, a.Level)
			case <-time.After(5 * time.Second):
				t.Fatal("CheckHost did not dispatch both warnings")
			}
		}
		require.NoError(t, am.SaveActiveAlerts())
	} else {
		active := am.GetActiveAlerts()
		require.Len(t, active, 2)
		for _, a := range active {
			require.NotNil(t, a.LastNotified, "dispatch marker must survive restore")
		}
		am.CheckHost(host)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	commands := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			commands <- scanner.Text()
		}
		close(commands)
	}()
	emit := func(phase string) {
		stats, err := nm.GetQueue().GetQueueStats()
		require.NoError(t, err)
		entries, err := nm.GetQueue().GetDeliveryLog(time.Now().Add(-time.Hour), 100)
		require.NoError(t, err)
		encoded, err := json.Marshal(checkpointProcessState{phase, stats, entries, started, len(am.GetActiveAlerts())})
		require.NoError(t, err)
		fmt.Println(checkpointProcessMarker + string(encoded))
	}
	emit("ready")
	for {
		select {
		case <-signals:
			stop()
			fmt.Println(checkpointProcessMarker + `{"phase":"stopped"}`)
			return
		case command, ok := <-commands:
			if !ok {
				return
			}
			switch command {
			case "reload":
				stop()
				open()
				am.CheckHost(host)
				emit("reloaded")
			case "status":
				emit("status")
			case "resolve":
				// Recovery dispatch uses the same public occurrence-cancellation
				// and receipt-gated entry points as the monitor callback.
				nm.SetGroupingConfig(false, 0, false, false)
				for _, a := range am.GetActiveAlerts() {
					nm.CancelResolvedAlert(&a)
					nm.SendResolvedAlert(&alerts.ResolvedAlert{Alert: &a, ResolvedTime: time.Now().UTC()})
				}
				emit("resolved")
			case "stop":
				stop()
				fmt.Println(checkpointProcessMarker + `{"phase":"stopped"}`)
				return
			default:
				t.Fatalf("unknown fixture command %q", command)
			}
		}
	}
}

func TestGroupedWarningGracefulProcessRestartAndReload(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		cooldown int
	}{{"restart", 0}, {"reload", 5}} {
		t.Run(tc.mode, func(t *testing.T) {
			type delivery struct {
				At      time.Time
				Path    string
				Event   string          `json:"event"`
				Alerts  []*alerts.Alert `json:"alerts"`
				Count   int             `json:"count"`
				Grouped bool            `json:"grouped"`
			}
			received := make(chan delivery, 16)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var d delivery
				if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				d.At, d.Path = time.Now().UTC(), r.URL.Path
				received <- d
				w.WriteHeader(200)
			}))
			defer server.Close()
			dir := t.TempDir()
			type child struct {
				cmd    *exec.Cmd
				input  io.WriteCloser
				states chan checkpointProcessState
				stderr *bytes.Buffer
			}
			start := func(seed bool) *child {
				cmd := exec.Command(os.Args[0], "-test.run=^TestGroupedWarningProcessHelper$", "-test.timeout=45s", "-test.v")
				cmd.Env = append(os.Environ(), "PULSE_GROUPED_TEST_DIR="+dir, "PULSE_GROUPED_TEST_RECEIVER="+server.URL, "PULSE_GROUPED_TEST_COOLDOWN="+strconv.Itoa(tc.cooldown), "PULSE_GROUPED_TEST_SEED="+map[bool]string{true: "1", false: "0"}[seed])
				stdout, err := cmd.StdoutPipe()
				require.NoError(t, err)
				input, err := cmd.StdinPipe()
				require.NoError(t, err)
				c := &child{cmd, input, make(chan checkpointProcessState, 32), &bytes.Buffer{}}
				cmd.Stderr = c.stderr
				require.NoError(t, cmd.Start())
				go func() {
					defer close(c.states)
					scanner := bufio.NewScanner(stdout)
					for scanner.Scan() {
						line := scanner.Text()
						if strings.HasPrefix(line, checkpointProcessMarker) {
							var state checkpointProcessState
							if json.Unmarshal([]byte(strings.TrimPrefix(line, checkpointProcessMarker)), &state) == nil {
								c.states <- state
							}
						}
					}
				}()
				return c
			}
			read := func(c *child, phase string) checkpointProcessState {
				t.Helper()
				select {
				case state, ok := <-c.states:
					require.True(t, ok, "child ended before %s", phase)
					require.Equal(t, phase, state.Phase)
					return state
				case <-time.After(15 * time.Second):
					t.Fatalf("no child phase %s", phase)
					return checkpointProcessState{}
				}
			}
			finish := func(c *child, sig bool) {
				t.Helper()
				if sig {
					require.NoError(t, c.cmd.Process.Signal(syscall.SIGTERM))
				} else {
					_, err := fmt.Fprintln(c.input, "stop")
					require.NoError(t, err)
				}
				read(c, "stopped")
				require.NoError(t, c.input.Close())
				err := c.cmd.Wait()
				if err != nil {
					t.Log(c.stderr.String())
				}
				require.NoError(t, err, "helper must exit normally after graceful stop")
			}
			c := start(true)
			// Cleanup only the child owned by this test, even on an assertion failure.
			t.Cleanup(func() {
				if c != nil && c.cmd.ProcessState == nil {
					_ = c.input.Close()
					_ = c.cmd.Process.Signal(syscall.SIGTERM)
					_ = c.cmd.Wait()
				}
			})
			seed := read(c, "ready")
			require.Equal(t, 2, seed.Active)
			require.Empty(t, seed.Log, "buffer admission is not a provider attempt")
			if tc.mode == "restart" {
				finish(c, true)
				c = start(false)
				read(c, "ready")
			} else {
				_, err := fmt.Fprintln(c.input, "reload")
				require.NoError(t, err)
				read(c, "reloaded")
			}
			_, err := fmt.Fprintln(c.input, "status")
			require.NoError(t, err)
			pending := read(c, "status")
			require.Equal(t, 2, pending.Stats["pending"], "restored dispatch markers must not strand accepted warnings")
			require.Empty(t, pending.Log)
			t.Logf("GROUPED_BROWSER_SNAPSHOT %s", mustCheckpointJSON(t, pending))
			seen := map[string]bool{}
			for range 2 {
				select {
				case d := <-received:
					require.False(t, d.At.Before(seed.Started.Add(4*time.Second)), "shutdown shortened grouping window")
					require.True(t, d.Grouped)
					require.Equal(t, 2, d.Count)
					require.Len(t, d.Alerts, 2)
					require.Equal(t, "alert", d.Event)
					require.False(t, seen[d.Path])
					seen[d.Path] = true
				case <-time.After(10 * time.Second):
					t.Fatal("wanted firing did not reach both receivers after restart/reload")
				}
			}
			// Wait for durable status/audit completion, not merely receiver arrival.
			waitSent := func(want int) checkpointProcessState {
				t.Helper()
				deadline := time.Now().Add(5 * time.Second)
				for {
					_, err := fmt.Fprintln(c.input, "status")
					require.NoError(t, err)
					state := read(c, "status")
					if state.Stats["sent"] == want {
						return state
					}
					require.True(t, time.Now().Before(deadline), "durable completion missing")
					time.Sleep(5 * time.Millisecond)
				}
			}
			sent := waitSent(2)
			t.Logf("GROUPED_BROWSER_SNAPSHOT %s", mustCheckpointJSON(t, sent))
			// Another real process boundary proves each occurrence/destination
			// firing receipt, not the in-memory delivery map, authorises recovery.
			finish(c, true)
			c = start(false)
			read(c, "ready")
			_, err = fmt.Fprintln(c.input, "resolve")
			require.NoError(t, err)
			read(c, "resolved")
			recoveries := map[string]int{}
			for range 4 {
				select {
				case d := <-received:
					require.Equal(t, "resolved", d.Event)
					require.Len(t, d.Alerts, 1)
					require.True(t, seen[d.Path])
					recoveries[d.Path]++
				case <-time.After(10 * time.Second):
					t.Fatal("receipt-gated recovery missing after process restart")
				}
			}
			require.Equal(t, map[string]int{"/left": 2, "/right": 2}, recoveries)
			resolved := waitSent(6)
			t.Logf("GROUPED_BROWSER_SNAPSHOT %s", mustCheckpointJSON(t, resolved))
			finish(c, false)
			c = nil
			select {
			case d := <-received:
				t.Fatalf("duplicate receiver message %+v", d)
			default:
			}
			t.Log("CheckHost warning markers, original grouped deadline, normal SIGTERM exit/reload, two scoped grouped HTTP firings and four receipt-gated recoveries verified without edited queue clocks or provider access")
		})
	}
}

func mustCheckpointJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return string(b)
}
