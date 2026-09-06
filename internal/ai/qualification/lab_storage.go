package qualification

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	scratchStoragePath  = "/var/lib/service-cache"
	scratchStorageFile  = scratchStoragePath + "/.qualification-fill"
	scratchStorageBytes = int64(8 * 1024 * 1024)
	scratchStorageMount = scratchStoragePath + ":rw,noexec,nosuid,nodev,size=8388608"
)

// scratchAvailable measures the filesystem independently of Pulse's inventory,
// findings and the service healthcheck. Bind every probe and mutation to the
// prepared container ID and the exact run-owned, fixed-size tmpfs.
func (l *DockerLab) scratchAvailable(ctx context.Context, lab *PreparedLab, state DockerState) (int64, error) {
	if state.ID == "" || state.ID != lab.ResourceIDs[state.Alias] ||
		state.Labels[labRunLabel] != labRunToken(lab.RunID) ||
		state.Labels[labAliasLabel] != state.Alias {
		return 0, fmt.Errorf("scratch storage target %q does not match the prepared run", state.Alias)
	}
	result, err := l.docker(ctx, "exec", state.ID, "stat", "-f", "-c", "%T %S %b %a", scratchStoragePath)
	if err != nil {
		return 0, fmt.Errorf("measure scratch storage: %w", err)
	}
	return parseScratchAvailable(result.Stdout)
}

func parseScratchAvailable(output string) (int64, error) {
	fields := strings.Fields(output)
	if len(fields) != 4 || fields[0] != "tmpfs" {
		return 0, fmt.Errorf("scratch storage must be a fixed-size tmpfs")
	}
	values := [3]int64{}
	for i := range values {
		value, err := strconv.ParseInt(fields[i+1], 10, 64)
		if err != nil || value < 0 || value > scratchStorageBytes {
			return 0, fmt.Errorf("invalid scratch filesystem measurement")
		}
		values[i] = value
	}
	blockSize, blocks, available := values[0], values[1], values[2]
	if blockSize == 0 || blocks == 0 || blockSize*blocks != scratchStorageBytes || available > blocks {
		return 0, fmt.Errorf("scratch storage must be exactly %d bytes", scratchStorageBytes)
	}
	return blockSize * available, nil
}

func (l *DockerLab) setScratchStorage(ctx context.Context, lab *PreparedLab, alias string, fill bool) error {
	state, err := l.inspect(ctx, lab, alias)
	if err != nil {
		return err
	}
	if _, err := l.scratchAvailable(ctx, lab, state); err != nil {
		return err
	}
	if !fill {
		_, err := l.docker(ctx, "exec", state.ID, "rm", "-f", scratchStorageFile)
		return err
	}
	// noclobber refuses a pre-existing file or symlink. The fixed write exceeds
	// the verified tmpfs by one block and must fail specifically with ENOSPC.
	// No manifest-supplied path, size or command participates in this operation.
	result, err := l.docker(ctx, "exec", state.ID, "/bin/sh", "-c",
		"set -C; LC_ALL=C dd if=/dev/zero bs=1048576 count=9 > "+scratchStorageFile)
	if err == nil || result.ExitCode != 1 || !strings.Contains(result.Stderr, "No space left on device") {
		return fmt.Errorf("scratch fill did not produce the expected bounded ENOSPC failure (exit %d)", result.ExitCode)
	}
	available, err := l.scratchAvailable(ctx, lab, state)
	if err != nil {
		return err
	}
	if available != 0 {
		return fmt.Errorf("scratch fill left %d bytes available", available)
	}
	return nil
}
