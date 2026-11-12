//go:build !unix

package agentstate

import "os"

func openStateFile(path string) (*os.File, error) {
	return os.Open(path)
}
