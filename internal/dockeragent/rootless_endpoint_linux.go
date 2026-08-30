//go:build linux

package dockeragent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	rootlessRuntimeRoot = "/run/user"
	effectiveUID        = os.Geteuid
)

func collectorOwnsRootlessEndpoint(endpoint string) bool {
	const unixPrefix = "unix://"
	if !strings.HasPrefix(endpoint, unixPrefix) {
		return false
	}
	path := strings.TrimPrefix(endpoint, unixPrefix)
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	uid := effectiveUID()
	root := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(uid))
	if path != root && !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(uid)
}
