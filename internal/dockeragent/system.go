package dockeragent

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
)

func readProcUptime() (float64, error) {
	f, err := openProcUptime()
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return 0, err
		}
		return 0, errors.New("empty /proc/uptime")
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) == 0 {
		return 0, errors.New("invalid /proc/uptime contents")
	}

	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}

	return value, nil
}
