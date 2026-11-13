package dockeragent

import (
	"strings"
	"unicode/utf8"
)

// BoundFailureLogLines applies the failure-log bounds to already masked
// lines: control characters removed, each line and the whole capped, blank
// lines dropped and the newest lines kept. The agent applies it before
// sending and the server applies it again on receipt, so neither depends on
// the other having done so.
func BoundFailureLogLines(in []string) (lines []string, truncated bool) {
	for _, line := range in {
		line = strings.TrimRight(cleanFailureLogLine(strings.ToValidUTF8(line, "")), " ")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > FailureLogMaxLineBytes {
			line = truncateUTF8(line, FailureLogMaxLineBytes-3) + "..."
			truncated = true
		}
		lines = append(lines, line)
	}
	if len(lines) > FailureLogMaxLines {
		lines = lines[len(lines)-FailureLogMaxLines:]
		truncated = true
	}
	total := 0
	for i := len(lines) - 1; i >= 0; i-- {
		total += len(lines[i]) + 1
		if total > FailureLogMaxBytes {
			lines = lines[i+1:]
			truncated = true
			break
		}
	}
	return lines, truncated
}

// CleanFailureLogText removes what is not text from raw log output and keeps
// its line breaks: invalid UTF-8, control characters, and anything a carriage
// return overwrote. It has to run before masking, never after: a secret split
// by a control character matches no masking rule until the character is gone,
// and removing it afterwards would rebuild the secret unmasked.
func CleanFailureLogText(raw string) string {
	raw = strings.ToValidUTF8(raw, "")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = cleanFailureLogLine(line)
	}
	return strings.Join(lines, "\n")
}

// cleanFailureLogLine drops control characters. A tab becomes a space, a line
// break inside a line becomes a space, and a carriage return ends the visible
// text before it, as a terminal would show a progress line.
func cleanFailureLogLine(line string) string {
	line = strings.TrimRight(line, "\r\n")
	if i := strings.LastIndexByte(line, '\r'); i >= 0 {
		line = line[i+1:]
	}
	var b strings.Builder
	b.Grow(len(line))
	for _, r := range line {
		switch {
		case r == '\t' || r == '\n':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
