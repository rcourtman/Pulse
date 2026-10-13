// nasfixturecollector is an inert, native-built input for the private NAS
// watchdog/removal fixture. It has no agent, service, network or state access.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const ready = "private NAS fixture collector ready\n"

func run(args []string, output io.Writer, wait func(time.Duration)) error {
	// The generated wrapper passes this one fixed lifetime. Reject anything
	// else rather than allowing an accidentally unbounded fixture process.
	if len(args) != 1 || args[0] != "30" {
		return errors.New("private NAS fixture requires its fixed 30-second lifetime")
	}
	if n, err := io.WriteString(output, ready); err != nil || n != len(ready) {
		return errors.New("private NAS fixture could not record readiness")
	}
	wait(30 * time.Second)
	return nil
}

func main() {
	// Both generated wrappers discard stdout and retain stderr in their own
	// watchdog log. This fixed marker discloses no process arguments or paths.
	if err := run(os.Args[1:], os.Stderr, time.Sleep); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
