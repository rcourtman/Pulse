package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AvailabilityCronSchedule represents a validated 5-field UTC cron schedule
// (minute, hour, day-of-month, month, day-of-week).
type AvailabilityCronSchedule struct {
	expr      string
	minutes   uint64 // bits 0..59
	hours     uint32 // bits 0..23
	dom       uint32 // bits 1..31
	months    uint16 // bits 1..12
	dow       uint8  // bits 0..6 (Sunday=0)
	domStar   bool
	dowStar   bool
}

// ParseAvailabilityCron parses a standard 5-field UTC cron expression.
// Supported syntax per field: *, */step, N, A-B, A-B/step, and comma lists.
// Day of week accepts 0..7 where both 0 and 7 represent Sunday.
func ParseAvailabilityCron(expr string) (*AvailabilityCronSchedule, error) {
	trimmed := strings.TrimSpace(expr)
	fields := strings.Fields(trimmed)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields (minute hour day-of-month month day-of-week)")
	}

	minutes, _, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return nil, fmt.Errorf("invalid cron minute field: %w", err)
	}
	hours, _, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return nil, fmt.Errorf("invalid cron hour field: %w", err)
	}
	dom, domStar, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return nil, fmt.Errorf("invalid cron day-of-month field: %w", err)
	}
	months, _, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return nil, fmt.Errorf("invalid cron month field: %w", err)
	}
	dow, dowStar, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return nil, fmt.Errorf("invalid cron day-of-week field: %w", err)
	}

	schedule := &AvailabilityCronSchedule{
		expr:    strings.Join(fields, " "),
		minutes: minutes,
		hours:   uint32(hours),
		dom:     uint32(dom),
		months:  uint16(months),
		dow:     uint8(dow),
		domStar: domStar,
		dowStar: dowStar,
	}

	// Ensure the schedule can fire at least once in a 4-year leap cycle
	// (rejects impossible dates like Feb 31).
	probeBase := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, ok := schedule.Next(probeBase.Add(-time.Minute)); !ok {
		return nil, fmt.Errorf("cron expression never matches any valid calendar date")
	}
	return schedule, nil
}

func (s *AvailabilityCronSchedule) String() string {
	if s == nil {
		return ""
	}
	return s.expr
}

func (s *AvailabilityCronSchedule) matchesDay(t time.Time) bool {
	domMatch := (s.dom & (1 << uint(t.Day()))) != 0
	dowMatch := (s.dow & (1 << uint(t.Weekday()))) != 0
	if !s.domStar && !s.dowStar {
		return domMatch || dowMatch
	}
	return domMatch && dowMatch
}

// Matches reports whether t (evaluated in UTC at minute granularity) matches s.
func (s *AvailabilityCronSchedule) Matches(t time.Time) bool {
	if s == nil {
		return false
	}
	utc := t.UTC()
	if (s.months & (1 << uint(utc.Month()))) == 0 {
		return false
	}
	if !s.matchesDay(utc) {
		return false
	}
	if (s.hours & (1 << uint(utc.Hour()))) == 0 {
		return false
	}
	return (s.minutes & (1 << uint(utc.Minute()))) != 0
}

// Next returns the earliest scheduled UTC time strictly after after.
func (s *AvailabilityCronSchedule) Next(after time.Time) (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	candidate := after.UTC().Truncate(time.Minute).Add(time.Minute)
	limit := candidate.AddDate(5, 0, 0)

	for candidate.Before(limit) {
		if (s.months & (1 << uint(candidate.Month()))) == 0 {
			candidate = time.Date(candidate.Year(), candidate.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if !s.matchesDay(candidate) {
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day()+1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if (s.hours & (1 << uint(candidate.Hour()))) == 0 {
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), candidate.Hour()+1, 0, 0, 0, time.UTC)
			continue
		}
		if (s.minutes & (1 << uint(candidate.Minute()))) == 0 {
			candidate = candidate.Add(time.Minute)
			continue
		}
		return candidate, true
	}
	return time.Time{}, false
}

// Prev returns the latest scheduled UTC time at or before atOrBefore.
func (s *AvailabilityCronSchedule) Prev(atOrBefore time.Time) (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	candidate := atOrBefore.UTC().Truncate(time.Minute)
	limit := candidate.AddDate(-5, 0, 0)

	for !candidate.Before(limit) {
		if (s.months & (1 << uint(candidate.Month()))) == 0 {
			// Jump to the last minute of the previous month.
			candidate = time.Date(candidate.Year(), candidate.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Minute)
			continue
		}
		if !s.matchesDay(candidate) {
			// Jump to 23:59 of the previous day.
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, time.UTC).Add(-time.Minute)
			continue
		}
		if (s.hours & (1 << uint(candidate.Hour()))) == 0 {
			// Jump to :59 of the previous hour.
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), candidate.Hour(), 0, 0, 0, time.UTC).Add(-time.Minute)
			continue
		}
		if (s.minutes & (1 << uint(candidate.Minute()))) == 0 {
			candidate = candidate.Add(-time.Minute)
			continue
		}
		return candidate, true
	}
	return time.Time{}, false
}

func parseCronField(field string, minVal, maxVal int, wrapSevenToZero bool) (uint64, bool, error) {
	if field == "" {
		return 0, false, fmt.Errorf("empty field")
	}
	isWildcard := field == "*"
	var bits uint64

	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return 0, false, fmt.Errorf("empty list entry in %q", field)
		}
		rangePart := part
		step := 1
		if slash := strings.IndexByte(part, '/'); slash >= 0 {
			if strings.Contains(part[slash+1:], "/") {
				return 0, false, fmt.Errorf("invalid step syntax %q", part)
			}
			rangePart = part[:slash]
			parsedStep, err := strconv.Atoi(part[slash+1:])
			if err != nil || parsedStep <= 0 {
				return 0, false, fmt.Errorf("step must be a positive integer in %q", part)
			}
			step = parsedStep
		}

		var start, end int
		switch {
		case rangePart == "*":
			start = minVal
			end = maxVal
			if wrapSevenToZero && end == 7 {
				end = 6
			}
		case strings.Contains(rangePart, "-"):
			bounds := strings.Split(rangePart, "-")
			if len(bounds) != 2 {
				return 0, false, fmt.Errorf("invalid range %q", rangePart)
			}
			var err error
			start, err = strconv.Atoi(bounds[0])
			if err != nil {
				return 0, false, fmt.Errorf("invalid range start %q", bounds[0])
			}
			end, err = strconv.Atoi(bounds[1])
			if err != nil {
				return 0, false, fmt.Errorf("invalid range end %q", bounds[1])
			}
			if start < minVal || start > maxVal || end < minVal || end > maxVal || start > end {
				return 0, false, fmt.Errorf("range %d-%d out of bounds %d..%d", start, end, minVal, maxVal)
			}
		default:
			val, err := strconv.Atoi(rangePart)
			if err != nil {
				return 0, false, fmt.Errorf("invalid number %q", rangePart)
			}
			if val < minVal || val > maxVal {
				return 0, false, fmt.Errorf("value %d out of bounds %d..%d", val, minVal, maxVal)
			}
			start = val
			if strings.Contains(part, "/") {
				end = maxVal
				if wrapSevenToZero && end == 7 {
					end = 6
				}
			} else {
				end = val
			}
		}

		for v := start; v <= end; v += step {
			bit := v
			if wrapSevenToZero && bit == 7 {
				bit = 0
			}
			bits |= 1 << uint(bit)
		}
	}

	if bits == 0 {
		return 0, false, fmt.Errorf("field %q matched no values", field)
	}
	return bits, isWildcard, nil
}
