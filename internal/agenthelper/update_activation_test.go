package agenthelper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testUpdateActivator(t *testing.T, verifier func([]byte, string) error) (UpdateProvider, string, string, string) {
	t.Helper()
	root := t.TempDir()
	quarantine := filepath.Join(root, "quarantine")
	staging := filepath.Join(root, "staging")
	targetDir := filepath.Join(root, "bin")
	stateDir := filepath.Join(root, "state")
	for _, path := range []string{quarantine, staging, targetDir, stateDir} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(targetDir, "pulse-agent")
	if err := os.WriteFile(target, testELF("old-signed-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	provider, err := NewUpdateActivator(UpdateActivatorConfig{
		QuarantineDir: quarantine, StagingDir: staging, TargetPath: target, StatePath: filepath.Join(stateDir, "activation.json"),
		VerifySignature: verifier,
		InspectVersion: func(_ context.Context, path string) (string, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			if strings.Contains(string(data), "old-signed-binary") {
				return "1.0.0", nil
			}
			return "1.1.0", nil
		},
		ValidateCommitter:       func(context.Context, string) error { return nil },
		ValidateOwner:           func(*os.File) error { return nil },
		ValidateQuarantineOwner: func(*os.File) error { return nil },
		Now:                     func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) },
		ScheduleRollback:        func(time.Duration, func()) func() { return func() {} },
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider, quarantine, target, filepath.Join(stateDir, "activation.json")
}

func testELF(body string) []byte {
	return append([]byte{0x7f, 'E', 'L', 'F'}, []byte(body)...)
}

func stageUpdate(t *testing.T, staging, identity string, binary []byte) string {
	t.Helper()
	dir := filepath.Join(staging, identity)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pulse-agent"), binary, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pulse-agent.sig"), []byte("valid-signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	return sha256Hex(binary)
}

func promoteUpdate(t *testing.T, provider UpdateProvider, artifactID, digest string) {
	t.Helper()
	result, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: artifactID, SHA256: digest, Version: "1.1.0"})
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if result.Action != "staged" || result.ArtifactID != artifactID || result.SHA256 != digest {
		t.Fatalf("stage result = %#v", result)
	}
}

func TestUpdateActivationAndIdentityBoundRollbackAreDurable(t *testing.T) {
	provider, staging, target, state := testUpdateActivator(t, func(data []byte, signature string) error {
		if string(data) != string(testELF("new-signed-binary")) || signature != "valid-signature" {
			return errors.New("signature mismatch")
		}
		return nil
	})
	newDigest := stageUpdate(t, staging, "release-1", testELF("new-signed-binary"))
	promoteUpdate(t, provider, "release-1", newDigest)
	oldDigest := sha256Hex(testELF("old-signed-binary"))
	activated, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: newDigest, Version: "1.1.0"})
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if activated.ActiveSHA256 != newDigest || activated.RollbackSHA256 != oldDigest || activated.ActivationID == "" {
		t.Fatalf("activation result = %#v", activated)
	}
	if activated.Action != "pending" || activated.RollbackDeadline.IsZero() {
		t.Fatalf("activation is not durably pending: %#v", activated)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("new-signed-binary")) {
		t.Fatalf("installed binary = %q", installed)
	}
	if info, err := os.Stat(state); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("durable state mode=%v err=%v", info, err)
	}
	retried, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: newDigest, Version: "1.1.0"})
	if err != nil || retried != activated {
		t.Fatalf("idempotent activation retry = %#v err=%v, want %#v", retried, err, activated)
	}
	rolledBack, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: activated.ActivationID, CurrentSHA256: newDigest, RollbackSHA256: oldDigest,
	})
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.ActiveSHA256 != oldDigest || rolledBack.RollbackSHA256 != newDigest {
		t.Fatalf("rollback result = %#v", rolledBack)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("rolled-back binary = %q", installed)
	}
	if _, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: rolledBack.ActivationID, CurrentSHA256: rolledBack.ActiveSHA256, RollbackSHA256: rolledBack.RollbackSHA256,
	}); err == nil {
		t.Fatal("completed rollback replay reactivated the rejected binary")
	}
}

func TestUpdateCommitClosesRollbackWindowAndCleansStaging(t *testing.T) {
	provider, quarantine, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	digest := stageUpdate(t, quarantine, "release-commit", testELF("committed"))
	promoteUpdate(t, provider, "release-commit", digest)
	activation, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-commit", SHA256: digest, Version: "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := provider.Commit(context.Background(), UpdateCommitRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if committed.Action != "committed" || committed.RollbackDeadline != (time.Time{}) {
		t.Fatalf("commit result = %#v", committed)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("committed")) {
		t.Fatalf("committed binary = %q", installed)
	}
	staged := filepath.Join(filepath.Dir(quarantine), "staging", "release-commit")
	if _, err := os.Stat(staged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("committed staging still exists: %v", err)
	}
	if retried, err := provider.Commit(context.Background(), UpdateCommitRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256,
	}); err != nil || retried.Action != "committed" {
		t.Fatalf("idempotent commit = %#v, %v", retried, err)
	}
	if _, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256, RollbackSHA256: activation.RollbackSHA256,
	}); err == nil {
		t.Fatal("committed update remained rollback-eligible")
	}
}

func TestUpdateCommitRequiresActivatingProcessAfterExecTransition(t *testing.T) {
	provider, quarantine, _, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	digest := stageUpdate(t, quarantine, "process-bound", testELF("new-signed-binary"))
	promoteUpdate(t, provider, "process-bound", digest)
	activating := context.WithValue(context.Background(), peerContextKey{}, Peer{UID: 1000, PID: 4321})
	activation, err := provider.Activate(activating, UpdateActivateRequest{ArtifactID: "process-bound", SHA256: digest, Version: "1.1.0"})
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	otherProcess := context.WithValue(context.Background(), peerContextKey{}, Peer{UID: 1000, PID: 4322})
	if _, err := provider.Commit(otherProcess, UpdateCommitRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256,
	}); err == nil || !strings.Contains(err.Error(), "activated collector process") {
		t.Fatalf("different process committed pending activation: %v", err)
	}
	if _, err := provider.Commit(activating, UpdateCommitRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256,
	}); err != nil {
		t.Fatalf("activating process could not commit after validation: %v", err)
	}
}

func TestUpdateActivatorRestartRollsBackUncommittedActivation(t *testing.T) {
	provider, quarantine, target, state := testUpdateActivator(t, func([]byte, string) error { return nil })
	digest := stageUpdate(t, quarantine, "release-restart", testELF("uncommitted"))
	promoteUpdate(t, provider, "release-restart", digest)
	activation, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-restart", SHA256: digest, Version: "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(quarantine)
	recovered, err := NewUpdateActivator(UpdateActivatorConfig{
		QuarantineDir: quarantine,
		StagingDir:    filepath.Join(root, "staging"),
		TargetPath:    target,
		StatePath:     state,
		VerifySignature: func([]byte, string) error {
			return nil
		},
		InspectVersion: func(_ context.Context, path string) (string, error) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			if strings.Contains(string(data), "old-signed-binary") {
				return "1.0.0", nil
			}
			return "1.1.0", nil
		},
		ValidateCommitter:       func(context.Context, string) error { return nil },
		ValidateOwner:           func(*os.File) error { return nil },
		ValidateQuarantineOwner: func(*os.File) error { return nil },
		Now:                     func() time.Time { return activation.RollbackDeadline.Add(-time.Second) },
		ScheduleRollback:        func(time.Duration, func()) func() { return func() {} },
	})
	if err != nil {
		t.Fatalf("restart recovery: %v", err)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("restart recovery installed binary = %q", installed)
	}
	result, err := recovered.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256, RollbackSHA256: activation.RollbackSHA256,
	})
	if err != nil || result.Action != "rolled_back" {
		t.Fatalf("durable restart result = %#v, %v", result, err)
	}
}

func TestUpdateStagingRejectsWrongArtifactVersionAndDowngrade(t *testing.T) {
	provider, quarantine, _, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	digest := stageUpdate(t, quarantine, "wrong-role", testELF("signed-other-component"))
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{
		ArtifactID: "wrong-role", SHA256: digest, Version: "9.9.9",
	}); err == nil || !strings.Contains(err.Error(), "requested pulse-agent version") {
		t.Fatalf("wrong signed artifact role/version accepted: %v", err)
	}

	digest = stageUpdate(t, quarantine, "downgrade", testELF("old-signed-binary"))
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{
		ArtifactID: "downgrade", SHA256: digest, Version: "1.0.0",
	}); err == nil || !strings.Contains(err.Error(), "does not advance") {
		t.Fatalf("signed downgrade accepted: %v", err)
	}
}

func TestUpdateRollbackWatchdogRestoresUncommittedActivation(t *testing.T) {
	provider, quarantine, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	activator := provider.(*updateActivator)
	var watchdog func()
	activator.scheduleRollback = func(_ time.Duration, callback func()) func() {
		watchdog = callback
		return func() {}
	}
	digest := stageUpdate(t, quarantine, "release-watchdog", testELF("uncommitted"))
	promoteUpdate(t, provider, "release-watchdog", digest)
	activation, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-watchdog", SHA256: digest, Version: "1.1.0"})
	if err != nil || watchdog == nil {
		t.Fatalf("Activate = %#v, %v; watchdog=%v", activation, err, watchdog != nil)
	}
	watchdog()
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("watchdog recovery installed binary = %q", installed)
	}
	result, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256, RollbackSHA256: activation.RollbackSHA256,
	})
	if err != nil || result.Action != "rolled_back" {
		t.Fatalf("watchdog durable result = %#v, %v", result, err)
	}
}

func TestUpdateRollbackWatchdogIgnoresSupersededCallback(t *testing.T) {
	provider, quarantine, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	activator := provider.(*updateActivator)
	type scheduledAttempt struct {
		callback func()
	}
	var attempts []scheduledAttempt
	var cancelled []bool
	var failures []UpdateRecoveryFailure
	activator.scheduleRollback = func(_ time.Duration, callback func()) func() {
		index := len(attempts)
		attempts = append(attempts, scheduledAttempt{callback: callback})
		cancelled = append(cancelled, false)
		return func() { cancelled[index] = true }
	}
	activator.reportRecoveryFailure = func(failure UpdateRecoveryFailure) {
		failures = append(failures, failure)
	}

	artifactID := "release-watchdog-reschedule"
	replacement := testELF("uncommitted-reschedule")
	digest := stageUpdate(t, quarantine, artifactID, replacement)
	promoteUpdate(t, provider, artifactID, digest)
	request := UpdateActivateRequest{ArtifactID: artifactID, SHA256: digest, Version: "1.1.0"}
	if _, err := provider.Activate(context.Background(), request); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if _, err := provider.Activate(context.Background(), request); err != nil {
		t.Fatalf("idempotent Activate: %v", err)
	}
	if len(attempts) != 2 || len(cancelled) != 2 || !cancelled[0] {
		t.Fatalf("rescheduled attempts=%d cancelled=%v", len(attempts), cancelled)
	}

	attempts[0].callback()
	if len(attempts) != 2 || len(failures) != 0 || activator.cancelRollback == nil {
		t.Fatalf("superseded callback disturbed current timer: attempts=%d failures=%#v current=%v", len(attempts), failures, activator.cancelRollback != nil)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(replacement) {
		t.Fatalf("superseded callback changed installed binary: %q", installed)
	}
	state, err := activator.readState()
	if err != nil || state.Action != "pending" {
		t.Fatalf("superseded callback state = %#v, %v", state, err)
	}

	attempts[1].callback()
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("current callback did not restore LKG: %q", installed)
	}
}

func TestUpdateRollbackWatchdogIgnoresCallbackAfterTerminalState(t *testing.T) {
	provider, quarantine, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	activator := provider.(*updateActivator)
	var callbacks []func()
	var failures []UpdateRecoveryFailure
	activator.scheduleRollback = func(_ time.Duration, callback func()) func() {
		callbacks = append(callbacks, callback)
		return func() {}
	}
	activator.reportRecoveryFailure = func(failure UpdateRecoveryFailure) {
		failures = append(failures, failure)
	}

	artifactID := "release-watchdog-terminal"
	replacement := testELF("uncommitted-terminal")
	digest := stageUpdate(t, quarantine, artifactID, replacement)
	promoteUpdate(t, provider, artifactID, digest)
	activation, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: artifactID, SHA256: digest, Version: "1.1.0"})
	if err != nil || len(callbacks) != 1 {
		t.Fatalf("Activate = %#v, %v; callbacks=%d", activation, err, len(callbacks))
	}
	if _, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256, RollbackSHA256: activation.RollbackSHA256,
	}); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	callbacks[0]()
	if len(callbacks) != 1 || len(failures) != 0 || activator.cancelRollback != nil {
		t.Fatalf("terminal stale callback changed timer state: callbacks=%d failures=%#v current=%v", len(callbacks), failures, activator.cancelRollback != nil)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("terminal stale callback changed installed binary: %q", installed)
	}
	state, err := activator.readState()
	if err != nil || state.Action != "rolled_back" {
		t.Fatalf("terminal stale callback state = %#v, %v", state, err)
	}
}

func TestUpdateRollbackWatchdogArmsRetryBeforeReportingFailure(t *testing.T) {
	provider, quarantine, _, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	activator := provider.(*updateActivator)
	type scheduledAttempt struct {
		delay    time.Duration
		callback func()
	}
	var attempts []scheduledAttempt
	activator.scheduleRollback = func(delay time.Duration, callback func()) func() {
		attempts = append(attempts, scheduledAttempt{delay: delay, callback: callback})
		return func() {}
	}
	reporterDone := make(chan bool, 1)
	activator.reportRecoveryFailure = func(failure UpdateRecoveryFailure) {
		// Re-entering the update mutex would deadlock if diagnostics were emitted
		// before the retry was armed and the recovery callback released the lock.
		activator.mu.Lock()
		retryArmed := len(attempts) == 2 && attempts[1].delay == failure.RetryIn && activator.cancelRollback != nil
		activator.mu.Unlock()
		reporterDone <- retryArmed
	}

	artifactID := "release-watchdog-report"
	digest := stageUpdate(t, quarantine, artifactID, testELF("uncommitted-report"))
	promoteUpdate(t, provider, artifactID, digest)
	if _, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: artifactID, SHA256: digest, Version: "1.1.0"}); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	rollbackPath := activator.rollbackPath
	unavailableRollback := rollbackPath + ".unavailable"
	if err := os.Rename(rollbackPath, unavailableRollback); err != nil {
		t.Fatal(err)
	}

	callbackDone := make(chan struct{})
	go func() {
		attempts[0].callback()
		close(callbackDone)
	}()
	select {
	case retryArmed := <-reporterDone:
		if !retryArmed {
			t.Fatal("recovery failure was reported before its retry became authoritative")
		}
	case <-time.After(time.Second):
		t.Fatal("recovery reporter could not re-enter the released update mutex")
	}
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("recovery callback did not return after reporting failure")
	}
}

func TestUpdateRollbackWatchdogRetriesPreRestoreFailureWithBoundedBackoff(t *testing.T) {
	provider, quarantine, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	activator := provider.(*updateActivator)
	type scheduledAttempt struct {
		delay    time.Duration
		callback func()
	}
	var attempts []scheduledAttempt
	var failures []UpdateRecoveryFailure
	activator.scheduleRollback = func(delay time.Duration, callback func()) func() {
		attempts = append(attempts, scheduledAttempt{delay: delay, callback: callback})
		return func() {}
	}
	activator.reportRecoveryFailure = func(failure UpdateRecoveryFailure) {
		failures = append(failures, failure)
	}

	artifactID := "release-watchdog-retry"
	replacement := testELF("uncommitted-retry")
	digest := stageUpdate(t, quarantine, artifactID, replacement)
	promoteUpdate(t, provider, artifactID, digest)
	activation, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: artifactID, SHA256: digest, Version: "1.1.0"})
	if err != nil || len(attempts) != 1 {
		t.Fatalf("Activate = %#v, %v; attempts=%d", activation, err, len(attempts))
	}

	rollbackPath := target + ".last-known-good"
	unavailableRollback := rollbackPath + ".unavailable"
	if err := os.Rename(rollbackPath, unavailableRollback); err != nil {
		t.Fatal(err)
	}
	for attemptIndex, wantDelay := range []time.Duration{
		initialRollbackRetryDelay,
		2 * initialRollbackRetryDelay,
		4 * initialRollbackRetryDelay,
		8 * initialRollbackRetryDelay,
		maxRollbackRetryDelay,
		maxRollbackRetryDelay,
	} {
		attempts[attemptIndex].callback()
		if len(attempts) != attemptIndex+2 {
			t.Fatalf("attempt %d scheduled %d callbacks, want %d", attemptIndex, len(attempts), attemptIndex+2)
		}
		if attempts[attemptIndex+1].delay != wantDelay {
			t.Fatalf("retry %d delay = %s, want %s", attemptIndex+1, attempts[attemptIndex+1].delay, wantDelay)
		}
		if len(failures) != attemptIndex+1 || failures[attemptIndex].ActivationID != activation.ActivationID || failures[attemptIndex].RetryIn != wantDelay {
			t.Fatalf("retry %d recovery event = %#v", attemptIndex+1, failures)
		}
		if installed, _ := os.ReadFile(target); string(installed) != string(replacement) {
			t.Fatalf("retry %d changed replacement before LKG recovery: %q", attemptIndex+1, installed)
		}
		state, stateErr := activator.readState()
		if stateErr != nil || state.Action != "pending" || state.ActivationID != activation.ActivationID {
			t.Fatalf("retry %d state = %#v, %v", attemptIndex+1, state, stateErr)
		}
	}

	if err := os.Rename(unavailableRollback, rollbackPath); err != nil {
		t.Fatal(err)
	}
	attempts[len(attempts)-1].callback()
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("successful retry installed binary = %q", installed)
	}
	state, err := activator.readState()
	if err != nil || state.Action != "rolled_back" || state.ActivationID != activation.ActivationID {
		t.Fatalf("successful retry state = %#v, %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(quarantine), "staging", artifactID)); !os.IsNotExist(err) {
		t.Fatalf("successful retry retained staged artifact: %v", err)
	}
}

func TestUpdateRollbackWatchdogRetriesPostRestoreCleanupFailure(t *testing.T) {
	provider, quarantine, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	activator := provider.(*updateActivator)
	type scheduledAttempt struct {
		delay    time.Duration
		callback func()
	}
	var attempts []scheduledAttempt
	var failures []UpdateRecoveryFailure
	activator.scheduleRollback = func(delay time.Duration, callback func()) func() {
		attempts = append(attempts, scheduledAttempt{delay: delay, callback: callback})
		return func() {}
	}
	activator.reportRecoveryFailure = func(failure UpdateRecoveryFailure) {
		failures = append(failures, failure)
	}

	artifactID := "release-watchdog-cleanup"
	replacement := testELF("uncommitted-cleanup")
	digest := stageUpdate(t, quarantine, artifactID, replacement)
	promoteUpdate(t, provider, artifactID, digest)
	activation, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: artifactID, SHA256: digest, Version: "1.1.0"})
	if err != nil || len(attempts) != 1 {
		t.Fatalf("Activate = %#v, %v; attempts=%d", activation, err, len(attempts))
	}

	stagedDir := filepath.Join(filepath.Dir(quarantine), "staging", artifactID)
	stagedSignature := filepath.Join(stagedDir, "pulse-agent.sig")
	if err := os.Remove(stagedSignature); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stagedSignature, 0o700); err != nil {
		t.Fatal(err)
	}
	attempts[0].callback()
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("first recovery did not restore LKG before cleanup failure: %q", installed)
	}
	state, err := activator.readState()
	if err != nil || state.Action != "pending" {
		t.Fatalf("cleanup failure state = %#v, %v", state, err)
	}
	if len(attempts) != 2 || attempts[1].delay != initialRollbackRetryDelay || len(failures) != 1 {
		t.Fatalf("cleanup failure retry attempts=%#v failures=%#v", attempts, failures)
	}

	if err := os.RemoveAll(stagedDir); err != nil {
		t.Fatal(err)
	}
	attempts[1].callback()
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("cleanup retry reactivated rejected binary: %q", installed)
	}
	state, err = activator.readState()
	if err != nil || state.Action != "rolled_back" || state.ActivationID != activation.ActivationID {
		t.Fatalf("cleanup retry state = %#v, %v", state, err)
	}
}

func TestUpdateActivationRejectsInvalidSignatureAndExecutable(t *testing.T) {
	provider, staging, _, _ := testUpdateActivator(t, func([]byte, string) error { return errors.New("untrusted") })
	digest := stageUpdate(t, staging, "bad-signature", testELF("binary"))
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: "bad-signature", SHA256: digest}); err == nil {
		t.Fatal("invalid artifact signature accepted")
	}

	provider, staging, _, _ = testUpdateActivator(t, func([]byte, string) error { return nil })
	digest = stageUpdate(t, staging, "not-elf", []byte("signed but not executable"))
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: "not-elf", SHA256: digest}); err == nil {
		t.Fatal("non-ELF artifact accepted")
	}
}

func TestUpdateActivationRejectsTraversalSymlinksAndDigestMismatch(t *testing.T) {
	provider, staging, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	validDigest := stageUpdate(t, staging, "release-1", testELF("new"))
	for _, request := range []UpdateActivateRequest{
		{ArtifactID: "../release-1", SHA256: validDigest},
		{ArtifactID: "release/1", SHA256: validDigest},
		{ArtifactID: "release-1", SHA256: strings.Repeat("0", 64)},
	} {
		if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: request.ArtifactID, SHA256: request.SHA256}); err == nil {
			t.Fatalf("unsafe activation accepted: %#v", request)
		}
	}
	outside := filepath.Join(filepath.Dir(staging), "outside")
	if err := os.WriteFile(outside, []byte("attacker"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkDir := filepath.Join(staging, "symlink-release")
	if err := os.Mkdir(symlinkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(symlinkDir, "pulse-agent")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(symlinkDir, "pulse-agent.sig"), []byte("valid-signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: "symlink-release", SHA256: sha256Hex([]byte("attacker"))}); err == nil {
		t.Fatal("symlink artifact accepted")
	}
	promoteUpdate(t, provider, "release-1", validDigest)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: validDigest, Version: "1.1.0"}); err == nil {
		t.Fatal("symlink install target accepted")
	}
}

func TestUpdateActivationUsesOpenedBytesAcrossStagingSwap(t *testing.T) {
	var stagingBinary, target string
	provider, staging, resolvedTarget, _ := testUpdateActivator(t, func(data []byte, _ string) error {
		if err := os.Remove(stagingBinary); err != nil {
			return err
		}
		if err := os.Symlink(target, stagingBinary); err != nil {
			return err
		}
		if string(data) != string(testELF("verified-bytes")) {
			return errors.New("unexpected verified data")
		}
		return nil
	})
	target = resolvedTarget
	digest := stageUpdate(t, staging, "release-swap", testELF("verified-bytes"))
	stagingBinary = filepath.Join(staging, "release-swap", "pulse-agent")
	promoteUpdate(t, provider, "release-swap", digest)
	if _, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-swap", SHA256: digest, Version: "1.1.0"}); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("verified-bytes")) {
		t.Fatalf("symlink swap changed activated bytes: %q", installed)
	}
}

func TestUpdateRollbackRejectsWrongIdentityAndChangedBinary(t *testing.T) {
	provider, staging, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	digest := stageUpdate(t, staging, "release-1", testELF("new"))
	promoteUpdate(t, provider, "release-1", digest)
	result, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: digest, Version: "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	bad := UpdateRollbackRequest{ActivationID: "release-2:" + result.ActivationID[len(result.ActivationID)-16:], CurrentSHA256: result.ActiveSHA256, RollbackSHA256: result.RollbackSHA256}
	if _, err := provider.Rollback(context.Background(), bad); err == nil {
		t.Fatal("wrong rollback activation identity accepted")
	}
	if err := os.WriteFile(target, []byte("changed-after-activation"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Rollback(context.Background(), UpdateRollbackRequest{ActivationID: result.ActivationID, CurrentSHA256: result.ActiveSHA256, RollbackSHA256: result.RollbackSHA256}); err == nil {
		t.Fatal("rollback accepted changed active binary")
	}
}
