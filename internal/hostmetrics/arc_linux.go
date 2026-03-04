//go:build linux

package hostmetrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const arcstatsPath = "/proc/spl/kstat/zfs/arcstats"

// readARCSize reads the current ZFS ARC size from /proc/spl/kstat/zfs/arcstats.
// On Linux, ZFS ARC is NOT included in MemAvailable (openzfs/zfs#10255),
// so gopsutil's Used = Total - Available overcounts. This function returns the
// ARC size so the caller can subtract it from used memory.
//
// Returns 0, nil if the file doesn't exist (no ZFS installed).
func readARCSize() (uint64, error) {
	f, err := os.Open(arcstatsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("open arcstats: %w", err)
	}
	defer f.Close()

	// Format:
	//   <header lines (2)>
	//   name                            type data
	//   ...
	//   size                            4    16873652224
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // skip header lines
		}

		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "size" {
			val, err := strconv.ParseUint(fields[2], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parse arcstats size %q: %w", fields[2], err)
			}
			return val, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read arcstats: %w", err)
	}

	// File exists but no "size" field found — unusual but not fatal
	return 0, nil
}
