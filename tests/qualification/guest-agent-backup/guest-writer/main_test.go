package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testOptions(t *testing.T) options {
	t.Helper()
	return options{directory: t.TempDir(), runID: "run-01", filesystemID: "data", duration: 3 * time.Second, interval: time.Second}
}

func events(t *testing.T, output *bytes.Buffer) []event {
	t.Helper()
	var result []event
	decoder := json.NewDecoder(output)
	for {
		var e event
		if err := decoder.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		result = append(result, e)
	}
	return result
}

func TestWitnessWritesSyncsAndReadsOnlyNewSyntheticFiles(t *testing.T) {
	o := testOptions(t)
	untouched := filepath.Join(o.directory, "existing")
	if err := os.WriteFile(untouched, []byte("do not overwrite"), 0600); err != nil {
		t.Fatal(err)
	}
	write, err := prepareWriter(o)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(o.directory, "pulse-backup-witness-run-01-data")
	for seq := 1; seq <= 2; seq++ {
		digest, err := write(seq)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, fmt.Sprintf("witness-%06d", seq))
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(payload)
		if len(payload) != 512 || digest != hex.EncodeToString(hash[:]) || !bytes.Contains(payload, []byte("\nrun-01\ndata\n")) {
			t.Fatal("witness did not read back the new synthetic payload")
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("witness file permissions")
		}
	}
	if _, err := write(1); err == nil {
		t.Fatal("repeated sequence overwrote existing evidence")
	}
	if _, err := prepareWriter(o); err == nil {
		t.Fatal("repeated run reused existing evidence")
	}
	data, err := os.ReadFile(untouched)
	if err != nil || string(data) != "do not overwrite" {
		t.Fatal("unrelated file changed")
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("witness directory permissions")
	}
}

func TestWitnessRejectsInvalidInputsBeforeCreatingFiles(t *testing.T) {
	changes := map[string]func(*options){
		"relative":             func(o *options) { o.directory = "relative" },
		"run traversal":        func(o *options) { o.runID = "../elsewhere" },
		"filesystem traversal": func(o *options) { o.filesystemID = "../elsewhere" },
		"oversize ID":          func(o *options) { o.runID = strings.Repeat("a", 33) },
		"duration small":       func(o *options) { o.duration = time.Second },
		"duration large":       func(o *options) { o.duration = 11 * time.Minute },
		"interval small":       func(o *options) { o.interval = time.Millisecond },
		"interval large":       func(o *options) { o.interval = 6 * time.Second },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			o := testOptions(t)
			original := o.directory
			change(&o)
			if _, err := prepareWriter(o); err == nil {
				t.Fatal("unsafe or unbounded input accepted")
			}
			entries, err := os.ReadDir(original)
			if err != nil || len(entries) != 0 {
				t.Fatal("invalid input created files")
			}
		})
	}
	for _, parent := range []bool{false, true} {
		t.Run("symlink", func(t *testing.T) {
			o := testOptions(t)
			real := t.TempDir()
			link := filepath.Join(o.directory, "link")
			if err := os.Symlink(real, link); err != nil {
				t.Fatal(err)
			}
			o.directory = link
			if parent {
				if err := os.Mkdir(filepath.Join(real, "child"), 0700); err != nil {
					t.Fatal(err)
				}
				o.directory = filepath.Join(link, "child")
			}
			if _, err := prepareWriter(o); err == nil {
				t.Fatal("symlinked scratch path accepted")
			}
		})
	}
}

func TestObserverCompletesWithOrderedWriteAndLivenessEvidence(t *testing.T) {
	o := testOptions(t)
	// Short timers exercise the event loop, not the CLI's native bounds.
	o.duration, o.interval = 250*time.Millisecond, 30*time.Millisecond
	var output bytes.Buffer
	if err := observe(context.Background(), o, &output, func(int) (string, error) { return strings.Repeat("a", 64), nil }); err != nil {
		t.Fatal(err)
	}
	seen := events(t, &output)
	if len(seen) < 4 || seen[0].Kind != "start" || seen[len(seen)-1].Result != "complete" {
		t.Fatalf("incomplete lifecycle: %#v", seen)
	}
	sequence := 0
	for i, event := range seen {
		if event.SchemaVersion != 1 || event.RunID != o.runID || event.FilesystemID != o.filesystemID || (i > 0 && event.ElapsedMS < seen[i-1].ElapsedMS) {
			t.Fatal("event identity or order")
		}
		if event.Kind == "write" {
			sequence++
			if event.Sequence != sequence || event.StartedUnixMS > event.UnixMS || event.SHA256 == "" {
				t.Fatal("write evidence incomplete")
			}
		}
	}
	if sequence == 0 {
		t.Fatal("no writes observed")
	}
}

func TestObserverBlockedWriteDoesNotQueueMoreAndCannotPass(t *testing.T) {
	o := testOptions(t)
	o.duration, o.interval = 250*time.Millisecond, 30*time.Millisecond
	var output bytes.Buffer
	var calls atomic.Int32
	release := make(chan struct{})
	defer close(release)
	err := observe(context.Background(), o, &output, func(int) (string, error) {
		calls.Add(1)
		<-release
		return "", nil
	})
	if err == nil || calls.Load() != 1 {
		t.Fatal("blocked write passed or queued more work")
	}
	seen := events(t, &output)
	if len(seen) < 3 || seen[len(seen)-1].Result != "write-pending" {
		t.Fatal("blocked write erased liveness or deadline failure")
	}
	for _, event := range seen {
		if event.Kind == "write" {
			t.Fatal("uncompleted write reported successful")
		}
	}
}

func TestObserverFailureAndInterruptionRemainAdverse(t *testing.T) {
	o := testOptions(t)
	var output bytes.Buffer
	if err := observe(context.Background(), o, &output, func(int) (string, error) { return "", errors.New("private path") }); err == nil {
		t.Fatal("failed write passed")
	}
	if strings.Contains(output.String(), "private path") || events(t, &output)[1].Result != "write-failed" {
		t.Fatal("failure missing or private error disclosed")
	}
	output.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if err := observe(ctx, o, &output, func(int) (string, error) { called = true; return "", nil }); err == nil || called {
		t.Fatal("cancelled run started writes or passed")
	}
	if events(t, &output)[1].Result != "interrupted" {
		t.Fatal("interruption not retained")
	}
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, errors.New("private output path") }

func TestObserverUncollectedOutputFailsBeforeStartingWrites(t *testing.T) {
	called := false
	err := observe(context.Background(), testOptions(t), failedOutput{}, func(int) (string, error) { called = true; return "", nil })
	if err == nil || called {
		t.Fatal("missing output passed or began writes")
	}
}
