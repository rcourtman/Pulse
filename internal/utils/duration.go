package utils

import (
	"strconv"
	"time"
)

// FormatDuration returns a short human-readable elapsed-time string.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return formatUnit(int(d.Minutes()), "minute")
	}
	if d < 24*time.Hour {
		return formatUnit(int(d.Hours()), "hour")
	}
	return formatUnit(int(d.Hours()/24), "day")
}

func formatUnit(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}
