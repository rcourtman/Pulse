package main

import (
	"strings"
	"testing"
)

func FuzzValidateCommand(f *testing.F) {
	seeds := []string{
		"sensors -j",
		"ipmitool sdr",
		"sensors",
		"ipmitool lan print",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		fields := strings.Fields(input)
		if len(fields) == 0 {
			return
		}
		cmd := fields[0]
		args := []string{}
		if len(fields) > 1 {
			args = fields[1:]
		}
		validateCommand(cmd, args) // ensure no panics
	})
}
