//go:build linux

package installtests

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAgentInstalledProfileAcknowledgedState(t *testing.T) {
	for _, pendingReads := range []int{0, 1, 2} {
		t.Run(string(rune('0'+pendingReads))+"-pending-reads", func(t *testing.T) {
			root := t.TempDir()
			id, token, optional := filepath.Join(root, "agent-id"), filepath.Join(root, "token"), filepath.Join(root, "absent")
			const privateMarker = "private-fixture-token-not-for-receipts"
			for path, content := range map[string]string{id: secureRuntimeLabAgentID, token: privateMarker} {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			paths := []string{id, token, optional, root}
			before := nativeProfileObserveProtected(paths...)
			reads := 0
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got, err := nativeProfileAwaitAcknowledgedState(ctx, before, id, func() map[string]nativeProfileProtectedIdentity {
				value := secureRuntimeLabAgentID
				if reads < pendingReads {
					value += "\n"
				}
				reads++
				if err := os.WriteFile(id, []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
				return nativeProfileObserveProtected(paths...)
			})
			if err != nil || !got.Matched || got.Observations != pendingReads+1 || reads != pendingReads+1 ||
				!reflect.DeepEqual(got.Final.After, before) || len(got.Final.ChangedPaths) != 0 {
				t.Fatalf("did not wait for byte-exact persistence: %+v %v", got, err)
			}
			if pendingReads > 0 && (!reflect.DeepEqual(got.Initial.ChangedPaths, []string{id}) ||
				got.Initial.After[id].Hash != secureRuntimeHash([]byte(secureRuntimeLabAgentID+"\n"))) {
				t.Fatal("successful receipt erased the initial pending bytes")
			}
			data, err := json.Marshal(got)
			if err != nil || strings.Contains(string(data), privateMarker) || strings.Contains(string(data), secureRuntimeLabAgentID) {
				t.Fatal("persistence receipt exposed file contents")
			}
		})
	}
}

func TestAgentInstalledProfileAcknowledgedStateRejectsOtherChanges(t *testing.T) {
	const id, token, optional, root = "agent-id", "token", "optional", "state-root"
	baseline := map[string]nativeProfileProtectedIdentity{
		id:       {Present: true, Type: "regular", Mode: 0o600, Hash: secureRuntimeHash([]byte(secureRuntimeLabAgentID))},
		token:    {Present: true, Type: "regular", Mode: 0o600, Hash: secureRuntimeHash([]byte("private-token"))},
		optional: {Type: "absent"},
		root:     {Present: true, Type: "directory", Mode: os.ModeDir | 0o700},
	}
	for _, name := range []string{"different-id", "two-newlines", "id-permissions", "id-owner", "id-group", "id-type", "id-absent", "id-read-error",
		"token-bytes", "token-permissions", "token-owner", "token-group", "token-error", "lifecycle-added", "root-mode", "root-owner", "extra-path", "missing-path"} {
		t.Run(name, func(t *testing.T) {
			after := make(map[string]nativeProfileProtectedIdentity, len(baseline))
			for path, identity := range baseline {
				after[path] = identity
			}
			value := after[id]
			value.Hash = secureRuntimeHash([]byte(secureRuntimeLabAgentID + "\n"))
			after[id] = value
			path := id
			if strings.HasPrefix(name, "token-") {
				path = token
			} else if strings.HasPrefix(name, "root-") {
				path = root
			} else if name == "lifecycle-added" {
				path = optional
			}
			value = after[path]
			switch name {
			case "different-id", "token-bytes":
				value.Hash = secureRuntimeHash([]byte("different"))
			case "two-newlines":
				value.Hash = secureRuntimeHash([]byte(secureRuntimeLabAgentID + "\n\n"))
			case "id-permissions", "token-permissions", "root-mode":
				value.Mode ^= 0o040
			case "id-owner", "token-owner", "root-owner":
				value.UID++
			case "id-group", "token-group":
				value.GID++
			case "id-type":
				value.Type = "symlink"
			case "id-absent":
				value = nativeProfileProtectedIdentity{Type: "absent"}
			case "id-read-error", "token-error":
				value.Error = "read-failed"
			case "lifecycle-added":
				value = baseline[token]
			}
			after[path] = value
			if name == "extra-path" {
				after["unexpected"] = baseline[token]
			} else if name == "missing-path" {
				delete(after, optional)
			}
			reads := 0
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := nativeProfileAwaitAcknowledgedState(ctx, baseline, id, func() map[string]nativeProfileProtectedIdentity {
				reads++
				return after
			})
			if err == nil || got.Matched || reads != 1 || got.Observations != 1 || !reflect.DeepEqual(got.Final.After, after) {
				t.Fatalf("unknown drift was waited out or hidden: %+v %v reads=%d", got, err, reads)
			}
		})
	}
}

func TestAgentInstalledProfileAcknowledgedStateDeadline(t *testing.T) {
	const id = "agent-id"
	before := map[string]nativeProfileProtectedIdentity{id: {Present: true, Type: "regular", Mode: 0o600,
		Hash: secureRuntimeHash([]byte(secureRuntimeLabAgentID))}}
	pending := before[id]
	pending.Hash = secureRuntimeHash([]byte(secureRuntimeLabAgentID + "\n"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := nativeProfileAwaitAcknowledgedState(ctx, before, id, func() map[string]nativeProfileProtectedIdentity {
		cancel()
		return map[string]nativeProfileProtectedIdentity{id: pending}
	})
	if !errors.Is(err, context.Canceled) || got.Matched || got.Observations != 1 ||
		!reflect.DeepEqual(got.Initial.After, got.Final.After) || !reflect.DeepEqual(got.Final.ChangedPaths, []string{id}) {
		t.Fatalf("stuck startup form gained a pass or lost its observation: %+v %v", got, err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	got, err = nativeProfileAwaitAcknowledgedState(ctx, before, id, func() map[string]nativeProfileProtectedIdentity {
		t.Fatal("expired deadline must not start another observation")
		return before
	})
	if !errors.Is(err, context.DeadlineExceeded) || got.Matched || got.Observations != 0 {
		t.Fatalf("expired deadline gained a pass or an observation: %+v %v", got, err)
	}
	// A deadline reached during a read must not turn a late exact match green.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	got, err = nativeProfileAwaitAcknowledgedState(ctx, before, id, func() map[string]nativeProfileProtectedIdentity {
		cancel()
		return before
	})
	if !errors.Is(err, context.Canceled) || got.Matched || got.Observations != 1 {
		t.Fatalf("late observation cleared cancellation: %+v %v", got, err)
	}
	// Baselines are acquired after the acknowledgement, never captured midway
	// through startup and normalized into a new expected result.
	before[id] = pending
	got, err = nativeProfileAwaitAcknowledgedState(context.Background(), before, id, func() map[string]nativeProfileProtectedIdentity {
		t.Fatal("unacknowledged baseline must fail before observation")
		return nil
	})
	if err == nil || got.Matched || got.Observations != 0 {
		t.Fatal("accepted an unacknowledged baseline")
	}
}
