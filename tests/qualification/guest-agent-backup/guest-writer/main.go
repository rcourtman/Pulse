// guest-writer observes a disposable guest filesystem without using QGA.
// It never starts a backup, freezes/thaws a filesystem or restarts a service.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

var opaqueID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

type options struct {
	directory, runID, filesystemID string
	duration, interval             time.Duration
}

func (o options) validate() error {
	if !opaqueID.MatchString(o.runID) || !opaqueID.MatchString(o.filesystemID) {
		return errors.New("opaque run and filesystem IDs required")
	}
	if o.duration < 3*time.Second || o.duration > 10*time.Minute || o.interval < time.Second || o.interval > 5*time.Second {
		return errors.New("duration must be 3..600 seconds and interval 1000..5000 milliseconds")
	}
	if !filepath.IsAbs(o.directory) || filepath.Clean(o.directory) != o.directory {
		return errors.New("absolute clean scratch directory required")
	}
	return nil
}

// The owner must map this directory to a filesystem actually covered by the
// native backup. A tmpfs or container overlay is not an equivalent witness.
func prepareWriter(o options) (func(int) (string, error), error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(o.directory)
	if err != nil || resolved != o.directory {
		return nil, errors.New("scratch directory unavailable or symlinked")
	}
	info, err := os.Stat(o.directory)
	if err != nil || !info.IsDir() {
		return nil, errors.New("scratch directory unavailable")
	}
	dir := filepath.Join(o.directory, "pulse-backup-witness-"+o.runID+"-"+o.filesystemID)
	// Never reuse a run or overwrite any pre-existing file. Retain synthetic
	// files on both success and failure; fixture teardown belongs to its owner.
	if err := os.Mkdir(dir, 0700); err != nil {
		return nil, errors.New("new witness directory could not be created")
	}
	return func(seq int) (string, error) {
		payload := []byte(fmt.Sprintf("Pulse disposable backup witness\n%s\n%s\n%d\n", o.runID, o.filesystemID, seq))
		payload = append(payload, bytes.Repeat([]byte{'x'}, 512-len(payload))...)
		path := filepath.Join(dir, fmt.Sprintf("witness-%06d", seq))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", errors.New("witness write failed")
		}
		_, writeErr := f.Write(payload)
		syncErr := f.Sync()
		closeErr := f.Close()
		if errors.Join(writeErr, syncErr, closeErr) != nil {
			return "", errors.New("witness write failed")
		}
		d, err := os.Open(dir)
		if err != nil {
			return "", errors.New("witness directory sync failed")
		}
		syncErr, closeErr = d.Sync(), d.Close()
		if errors.Join(syncErr, closeErr) != nil {
			return "", errors.New("witness directory sync failed")
		}
		readback, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(readback, payload) {
			return "", errors.New("witness readback failed")
		}
		digest := sha256.Sum256(readback)
		return hex.EncodeToString(digest[:]), nil
	}, nil
}

type event struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	RunID         string `json:"run_id"`
	FilesystemID  string `json:"filesystem_id"`
	UnixMS        int64  `json:"unix_ms"`
	ElapsedMS     int64  `json:"elapsed_ms"`
	Sequence      int    `json:"sequence,omitempty"`
	StartedUnixMS int64  `json:"started_unix_ms,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Result        string `json:"result,omitempty"`
}

type writeResult struct {
	sequence int
	started  time.Time
	digest   string
	err      error
}

func observe(ctx context.Context, o options, output io.Writer, write func(int) (string, error)) error {
	start := time.Now()
	deadline := start.Add(o.duration)
	encoder := json.NewEncoder(output)
	emit := func(e event) error {
		now := time.Now()
		e.SchemaVersion, e.RunID, e.FilesystemID = 1, o.runID, o.filesystemID
		e.UnixMS, e.ElapsedMS = now.UnixMilli(), now.Sub(start).Milliseconds()
		return encoder.Encode(e)
	}
	if err := emit(event{Kind: "start"}); err != nil {
		return err
	}
	timer := time.NewTimer(o.duration)
	defer timer.Stop()
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	results := make(chan writeResult, 1)
	sequence, pending := 0, false
	begin := func() {
		sequence++
		pending = true
		seq, began := sequence, time.Now()
		go func() {
			digest, err := write(seq)
			results <- writeResult{seq, began, digest, err}
		}()
	}
	stop := func(result string) error { return emit(event{Kind: "stop", Result: result}) }
	if ctx.Err() == nil {
		begin()
	}
	for {
		select {
		case <-ctx.Done():
			_ = stop("interrupted")
			return errors.New("witness interrupted")
		case <-timer.C:
			if pending {
				_ = stop("write-pending")
				return errors.New("witness deadline with write pending")
			}
			return stop("complete")
		case <-ticker.C:
			if err := emit(event{Kind: "heartbeat"}); err != nil {
				return err
			}
			// A blocked filesystem write must not queue more work or prevent
			// independent guest liveness observations on stdout.
			if !pending && time.Now().Before(deadline) {
				begin()
			}
		case result := <-results:
			pending = false
			if result.err != nil {
				_ = stop("write-failed")
				return errors.New("witness write failed")
			}
			if err := emit(event{Kind: "write", Sequence: result.sequence, StartedUnixMS: result.started.UnixMilli(), SHA256: result.digest}); err != nil {
				return err
			}
		}
	}
}

func main() {
	var o options
	var durationSeconds, intervalMS int
	flag.StringVar(&o.directory, "directory", "", "owner-provisioned scratch directory on the covered filesystem")
	flag.StringVar(&o.runID, "run-id", "", "opaque run ID; no names or paths")
	flag.StringVar(&o.filesystemID, "filesystem-id", "", "opaque filesystem ID from the private mount crosswalk")
	flag.IntVar(&durationSeconds, "duration-seconds", 600, "preselected observation duration, 3..600")
	flag.IntVar(&intervalMS, "interval-ms", 1000, "synthetic write/heartbeat interval, 1000..5000")
	flag.Parse()
	if flag.NArg() != 0 || durationSeconds < 3 || durationSeconds > 600 || intervalMS < 1000 || intervalMS > 5000 {
		fmt.Fprintln(os.Stderr, "invalid witness arguments")
		os.Exit(2)
	}
	o.duration, o.interval = time.Duration(durationSeconds)*time.Second, time.Duration(intervalMS)*time.Millisecond
	write, err := prepareWriter(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := observe(ctx, o, os.Stdout, write); err != nil {
		// Fixed errors only: never emit the private filesystem path or a
		// provider/service environment. An output error is still a failure.
		fmt.Fprintln(os.Stderr, "witness observation failed")
		os.Exit(1)
	}
}
