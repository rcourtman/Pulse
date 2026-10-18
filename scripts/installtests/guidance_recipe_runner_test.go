//go:build !windows

package installtests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type guidanceLimits struct{ preparation, request, completion time.Duration }

func guidanceRecipeLimits(requestLimit time.Duration) guidanceLimits {
	// These are fixture watchdogs, not changed request limits. The five-second
	// margin observes curl's own terminal result; preparation and subsequent
	// shell work cannot consume the documented request budget.
	return guidanceLimits{30 * time.Second, requestLimit + 5*time.Second, 5 * time.Second}
}

type guidanceObservation struct {
	output                           string
	exit, curlStarts, curlEnds       int
	preparation, request, completion time.Duration
	phase                            string
	signal                           string
	terminal, outputComplete         bool
}

func (r guidanceObservation) summary() string {
	// Never put recipe output, argv, credentials or response bodies in phase
	// diagnostics. Independent synthetic peers still check HTTP request counts.
	return fmt.Sprintf("phase=%s preparation=%s request=%s completion=%s curl_starts=%d curl_ends=%d bash_exit=%d bash_signal=%s terminal=%t output_complete=%t",
		r.phase, r.preparation, r.request, r.completion, r.curlStarts, r.curlEnds, r.exit, r.signal, r.terminal, r.outputComplete)
}

// Observe copied Bash recipes without rewriting their curl arguments. The shim
// calls the existing PATH fixture (or real curl), preserving its argv capture,
// output, TLS, privacy, request deadlines and exit status. A dedicated pipe
// carries only two fixed phase markers, never public stdout or private data.
func observeGuidanceRecipe(t *testing.T, script, home, bin string, limits guidanceLimits, extraEnv ...string) (result guidanceObservation, resultErr error) {
	t.Helper()
	result.exit, result.phase = -1, "preparation"
	curl := filepath.Join(bin, "curl")
	if _, err := os.Stat(curl); os.IsNotExist(err) {
		curl, err = exec.LookPath("curl")
		if err != nil {
			return result, errors.New("curl is required for copied guidance")
		}
	} else if err != nil {
		return result, err
	}
	shimDir := t.TempDir()
	const shim = `#!/bin/sh
printf 'curl-start\n' >&3
"$PULSE_GUIDANCE_CURL" "$@"
curl_exit=$?
printf 'curl-end %d\n' "$curl_exit" >&3
exit "$curl_exit"
`
	if err := os.WriteFile(filepath.Join(shimDir, "curl"), []byte(shim), 0700); err != nil {
		return result, err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	// Killing only Bash can leave curl holding CombinedOutput pipes open. Kill
	// this fixture's entire process group and bound pipe drainage separately.
	cmd.WaitDelay = 2 * time.Second
	cmd.ExtraFiles = []*os.File{writer}
	cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+shimDir+":"+bin+":"+os.Getenv("PATH"),
		"http_proxy=", "HTTP_PROXY=", "https_proxy=", "HTTPS_PROXY=", "ALL_PROXY=", "all_proxy=", "NO_PROXY=", "no_proxy=")
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Env = append(cmd.Env, "PULSE_GUIDANCE_CURL="+curl)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	started := time.Now()
	if err := cmd.Start(); err != nil {
		return result, err
	}
	_ = writer.Close()
	type event struct {
		line string
		at   time.Time
	}
	events := make(chan event, 4)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 128), 128)
		for scanner.Scan() {
			select {
			case events <- event{scanner.Text(), time.Now()}:
			case <-ctx.Done():
				return
			}
		}
		if scanner.Err() != nil {
			select {
			case events <- event{"phase-pipe-error", time.Now()}:
			case <-ctx.Done():
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(limits.preparation)
	defer timer.Stop()
	phaseStarted := started
	reset := func(d time.Duration) {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(d)
	}
	observe := func(e event) error {
		switch {
		case e.line == "curl-start" && result.phase == "preparation":
			result.curlStarts++
			result.preparation = e.at.Sub(started)
			phaseStarted, result.phase = e.at, "request"
			reset(limits.request)
		case strings.HasPrefix(e.line, "curl-end ") && result.phase == "request":
			if _, err := strconv.Atoi(strings.TrimPrefix(e.line, "curl-end ")); err != nil {
				return errors.New("invalid curl terminal marker")
			}
			result.curlEnds++
			result.request = e.at.Sub(phaseStarted)
			phaseStarted, result.phase = e.at, "completion"
			reset(limits.completion)
		default:
			return errors.New("unexpected or repeated curl phase marker")
		}
		return nil
	}
	for {
		select {
		case e, open := <-events:
			if !open {
				events = nil
			} else if err := observe(e); err != nil {
				resultErr = err
				cancel()
			}
		case <-timer.C:
			resultErr = fmt.Errorf("%s fixture watchdog expired", result.phase)
			cancel()
		case err := <-done:
			// Wait may win the select after a fast request. Collect its already
			// written markers before evaluating the final observation.
			if errors.Is(err, exec.ErrWaitDelay) {
				_ = cmd.Cancel()
			}
			markerTimer := time.NewTimer(2 * time.Second)
			for events != nil {
				select {
				case e, open := <-events:
					if !open {
						events = nil
					} else if eventErr := observe(e); eventErr != nil && resultErr == nil {
						resultErr = eventErr
					}
				case <-markerTimer.C:
					_ = cmd.Cancel()
					_ = reader.Close()
					cancel()
					if resultErr == nil {
						resultErr = errors.New("phase-pipe collection did not complete")
					}
					events = nil
				}
			}
			markerTimer.Stop()
			result.output, result.exit, result.terminal = output.String(), cmd.ProcessState.ExitCode(), true
			if state, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && state.Signaled() {
				result.signal = state.Signal().String()
			}
			result.outputComplete = !errors.Is(err, exec.ErrWaitDelay)
			switch result.phase {
			case "preparation":
				result.preparation = time.Since(started)
			case "request":
				result.request = time.Since(phaseStarted)
			case "completion":
				result.completion = time.Since(phaseStarted)
			}
			var exitError *exec.ExitError
			if err != nil && !errors.As(err, &exitError) && resultErr == nil {
				resultErr = err
			}
			if resultErr == nil && result.curlStarts != result.curlEnds {
				resultErr = errors.New("missing curl terminal observation")
			}
			return result, resultErr
		}
	}
}

func guidanceRunnerFixture(t *testing.T, curlScript string) (home, bin, childFile string) {
	t.Helper()
	home, bin = t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\n"+curlScript), 0700); err != nil {
		t.Fatal(err)
	}
	return home, bin, filepath.Join(home, "child-pid")
}

func guidanceRunnerChildStopped(t *testing.T, childFile string) {
	t.Helper()
	data, err := os.ReadFile(childFile)
	if err != nil {
		t.Fatalf("owned child identity missing: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		t.Fatal("invalid owned child identity")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("owned child remains after fixture cancellation")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestGuidanceRecipeRunnerSeparatesPreparationFromCurl(t *testing.T) {
	home, bin, _ := guidanceRunnerFixture(t, "printf 'synthetic-private-response'; exit 22\n")
	limits := guidanceLimits{2 * time.Second, 200 * time.Millisecond, time.Second}
	result, err := observeGuidanceRecipe(t, "sleep 0.5\ncurl --disable --max-time 15", home, bin, limits)
	if err != nil || result.exit != 22 || result.curlStarts != 1 || result.curlEnds != 1 || result.preparation < 500*time.Millisecond || result.request >= limits.request || !result.terminal || !result.outputComplete {
		t.Fatalf("preparation consumed curl budget or changed terminal result: %v; %s", err, result.summary())
	}
	if result.output != "synthetic-private-response" || strings.Contains(result.summary(), "synthetic-private") {
		t.Fatal("observer changed output or disclosed it in phase diagnostics")
	}
}

func TestGuidanceRecipeRunnerBoundsEachPhaseAndStopsOwnedChildren(t *testing.T) {
	const child = "sleep 60 & child=$!; printf '%s\\n' \"$child\" > \"$CHILD_FILE\"; wait\n"
	for _, phase := range []string{"preparation", "request", "completion"} {
		t.Run(phase, func(t *testing.T) {
			curlScript, recipe := "exit 0\n", child
			if phase == "request" {
				curlScript, recipe = child, "curl --disable"
			} else if phase == "completion" {
				recipe = "curl --disable\n" + child
			}
			home, bin, childFile := guidanceRunnerFixture(t, curlScript)
			limits := guidanceLimits{2 * time.Second, 2 * time.Second, 2 * time.Second}
			switch phase {
			case "preparation":
				limits.preparation = 500 * time.Millisecond
			case "request":
				limits.request = 500 * time.Millisecond
			case "completion":
				limits.completion = 500 * time.Millisecond
			}
			started := time.Now()
			result, err := observeGuidanceRecipe(t, recipe, home, bin, limits, "CHILD_FILE="+childFile)
			if err == nil || !strings.Contains(err.Error(), phase+" fixture watchdog expired") || result.phase != phase || result.exit != -1 || result.signal != "killed" || !result.terminal || !result.outputComplete || time.Since(started) > 4*time.Second {
				t.Fatalf("phase did not reach bounded terminal observation: %v; %s", err, result.summary())
			}
			if phase == "preparation" && result.curlStarts != 0 || phase != "preparation" && result.curlStarts != 1 {
				t.Fatal("request admission attribution is wrong")
			}
			guidanceRunnerChildStopped(t, childFile)
		})
	}
}

func TestGuidanceRecipeRunnerRejectsDuplicateAndPrivateMarkers(t *testing.T) {
	for _, recipe := range []string{"curl --disable\ncurl --disable", "printf 'synthetic-private-marker\\n' >&3", "printf '%0200d\\n' 0 >&3"} {
		home, bin, _ := guidanceRunnerFixture(t, "exit 0\n")
		result, err := observeGuidanceRecipe(t, recipe, home, bin, guidanceLimits{time.Second, time.Second, time.Second})
		if err == nil || strings.Contains(err.Error(), "synthetic-private") || strings.Contains(result.summary(), "synthetic-private") {
			t.Fatalf("invalid phase protocol accepted or disclosed: %v; %s", err, result.summary())
		}
	}
}

func TestGuidanceRecipeRunnerRetainsIncompletePipeResult(t *testing.T) {
	home, bin, childFile := guidanceRunnerFixture(t, "exit 0\n")
	recipe := "sleep 60 & child=$!; printf '%s\\n' \"$child\" > \"$CHILD_FILE\"; printf done\n"
	result, err := observeGuidanceRecipe(t, recipe, home, bin, guidanceLimits{10 * time.Second, time.Second, time.Second}, "CHILD_FILE="+childFile)
	if !errors.Is(err, exec.ErrWaitDelay) || result.outputComplete || !result.terminal || result.exit != 0 {
		t.Fatalf("incomplete output became a pass: %v; %s", err, result.summary())
	}
	guidanceRunnerChildStopped(t, childFile)
}

func TestGuidanceRecipeRunnerWatchdogsPreserveDocumentedLimits(t *testing.T) {
	for _, duration := range []time.Duration{15 * time.Second, 20 * time.Second, 60 * time.Second} {
		limits := guidanceRecipeLimits(duration)
		if limits.request != duration+5*time.Second || limits.preparation != 30*time.Second || limits.completion != 5*time.Second {
			t.Fatal("fixture phases no longer leave the documented curl limit intact")
		}
	}
}
