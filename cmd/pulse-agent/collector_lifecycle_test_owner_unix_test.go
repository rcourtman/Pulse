//go:build !windows

package main

import (
	"os"
	"strconv"
)

func collectorLifecycleTestOwnerUID() string { return strconv.Itoa(os.Geteuid()) }
