package dockeragent

import (
	"strings"
	"testing"
)

func TestBoundFailureLogLinesKeepsTheNewestWithinEveryBound(t *testing.T) {
	// Control characters, blank lines and a progress line rewritten in place.
	lines, truncated := BoundFailureLogLines([]string{
		"starting\twith\ttabs",
		"   ",
		"downloading 10%\rdownloading 100%",
		"bell\x07 and escape\x1b gone",
		"",
	})
	want := []string{"starting with tabs", "downloading 100%", "bell and escape gone"}
	if truncated || strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %q truncated=%t, want %q", lines, truncated, want)
	}

	// More lines than allowed: the newest stay.
	var many []string
	for i := 0; i < FailureLogMaxLines+12; i++ {
		many = append(many, "line "+string(rune('a'+i%26)))
	}
	lines, truncated = BoundFailureLogLines(many)
	if !truncated || len(lines) != FailureLogMaxLines || lines[len(lines)-1] != many[len(many)-1] || lines[0] != many[12] {
		t.Fatalf("kept %d lines (truncated=%t), first %q last %q", len(lines), truncated, lines[0], lines[len(lines)-1])
	}

	// One line longer than allowed is cut on a character boundary.
	long := strings.Repeat("é", FailureLogMaxLineBytes)
	lines, truncated = BoundFailureLogLines([]string{long})
	if !truncated || len(lines) != 1 || len(lines[0]) > FailureLogMaxLineBytes || !strings.HasSuffix(lines[0], "...") || !strings.HasPrefix(lines[0], "é") {
		t.Fatalf("long line: %d bytes, truncated=%t", len(lines[0]), truncated)
	}
	if strings.ContainsRune(lines[0], '�') {
		t.Fatal("the cut split a character")
	}

	// The whole stays under the total, dropping the oldest lines.
	var heavy []string
	for i := 0; i < 20; i++ {
		heavy = append(heavy, strings.Repeat(string(rune('a'+i)), 400))
	}
	lines, truncated = BoundFailureLogLines(heavy)
	total := 0
	for _, line := range lines {
		total += len(line) + 1
	}
	if !truncated || total > FailureLogMaxBytes || len(lines) == 0 || lines[len(lines)-1] != heavy[len(heavy)-1] {
		t.Fatalf("total = %d bytes in %d lines, truncated=%t", total, len(lines), truncated)
	}

	if lines, truncated := BoundFailureLogLines(nil); len(lines) != 0 || truncated {
		t.Fatalf("no input gave %q, %t", lines, truncated)
	}
}

func TestValidFailureLogReason(t *testing.T) {
	for _, reason := range []string{FailureLogReasonExited, FailureLogReasonOOMKilled, FailureLogReasonRestarting, FailureLogReasonUnhealthy} {
		if !ValidFailureLogReason(reason) {
			t.Errorf("%q should be valid", reason)
		}
	}
	for _, reason := range []string{"", "running", "Exited", "exited; rm -rf"} {
		if ValidFailureLogReason(reason) {
			t.Errorf("%q should not be valid", reason)
		}
	}
}

// Cleaning keeps line breaks and removes what is not text, so that masking
// sees each line as it will be sent.
func TestCleanFailureLogTextKeepsLinesAndDropsControls(t *testing.T) {
	got := CleanFailureLogText("one\r\ntwo\x00 split\nprogress 10%\rprogress 100%\nbad \xff utf8\n")
	want := "one\ntwo split\nprogress 100%\nbad  utf8\n"
	if got != want {
		t.Fatalf("cleaned = %q, want %q", got, want)
	}
}
