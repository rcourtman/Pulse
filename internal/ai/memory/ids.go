package memory

import (
	"strconv"
	"sync/atomic"
	"time"
)

// nextSequencedID mints prefix + local wall-clock second + "-" + the next value
// of seq. Each sequence is package-level and shared by every store in the
// process, including one store per tenant monitor, so it must stay atomic.
// The suffix never wraps, so one sequence never repeats an ID within a process
// however many IDs share a second.
func nextSequencedID(prefix string, seq *atomic.Int64) string {
	return prefix + time.Now().Format("20060102150405") + "-" + strconv.FormatInt(seq.Add(1), 10)
}
