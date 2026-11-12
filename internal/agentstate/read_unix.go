//go:build unix

package agentstate

import (
	"os"
	"syscall"
)

func openStateFile(path string) (*os.File, error) {
	// A path admitted as regular may become a FIFO before open. Nonblocking
	// open lets the descriptor check reject it without waiting for a writer.
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
