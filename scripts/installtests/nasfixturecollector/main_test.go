package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

func TestCollectorRecordsReadinessBeforeBoundedWait(t *testing.T) {
	var output bytes.Buffer
	calls := 0
	if err := run([]string{"30"}, &output, func(duration time.Duration) {
		calls++
		if duration != 30*time.Second || output.String() != ready {
			t.Fatalf("wait %s before readiness %q", duration, output.String())
		}
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("wait called %d times", calls)
	}
}

func TestCollectorRejectsChangedOrMissingLifetime(t *testing.T) {
	for _, args := range [][]string{nil, {"0"}, {"-1"}, {"300"}, {"30", "extra"}} {
		t.Run("invalid", func(t *testing.T) {
			var output bytes.Buffer
			err := run(args, &output, func(time.Duration) { t.Fatal("invalid input started a collector") })
			if err == nil || output.Len() != 0 {
				t.Fatalf("invalid input produced readiness or success: %v, %q", err, output.String())
			}
		})
	}
}

type refusedOutput struct{}

func (refusedOutput) Write([]byte) (int, error) { return 0, errors.New("synthetic write refusal") }

type shortOutput struct{}

func (shortOutput) Write([]byte) (int, error) { return 1, nil }

func TestCollectorCannotWaitAfterUnrecordedReadiness(t *testing.T) {
	for _, output := range []io.Writer{refusedOutput{}, shortOutput{}} {
		if err := run([]string{"30"}, output, func(time.Duration) {
			t.Fatal("failed readiness started a collector")
		}); err == nil {
			t.Fatal("failed readiness accepted")
		}
	}
	if err := run([]string{"30"}, io.Discard, func(time.Duration) {}); err != nil {
		t.Fatal(err)
	}
}
