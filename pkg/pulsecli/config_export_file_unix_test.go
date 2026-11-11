//go:build !windows

package pulsecli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestConfigExportFileRejectsFIFO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.enc")
	require.NoError(t, unix.Mkfifo(path, 0o600))
	// No opening the FIFO, hence no reader/writer rendezvous or timing race.
	err := writeConfigExportFile(path, []byte("synthetic encrypted export"))
	require.ErrorContains(t, err, "not a regular file")
	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeNamedPipe)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
