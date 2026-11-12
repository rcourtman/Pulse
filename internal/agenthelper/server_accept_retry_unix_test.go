//go:build unix

package agenthelper

import (
	"net"
	"os"
	"syscall"
	"testing"
)

// These are the net package's real descriptor-exhaustion error shapes, not an
// actual descriptor-limit change or resource-exhaustion experiment.
func TestServerAcceptRetryDescriptorErrors(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EMFILE, syscall.ENFILE} {
		t.Run(errno.Error(), func(t *testing.T) {
			err := &net.OpError{Op: "accept", Net: "unix", Err: os.NewSyscallError("accept", errno)}
			if !err.Temporary() {
				t.Fatal("descriptor-exhaustion fixture is not a temporary listener error")
			}
			testAcceptRetryBackoff(t, err)
		})
	}
}
