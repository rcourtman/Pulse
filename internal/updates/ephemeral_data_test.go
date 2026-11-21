package updates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempMountInfo(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write mountinfo: %v", err)
	}
	return p
}

func TestIsDockerDataDirEphemeral(t *testing.T) {
	ephemeralMountInfo := writeTempMountInfo(t, strings.Join([]string{
		"120 98 0:45 / / rw,relatime - overlay overlay rw",
		"121 120 0:46 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw",
		"122 120 8:1 /etc/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/sda1 rw",
	}, "\n")+"\n")

	mountedDataInfo := writeTempMountInfo(t, strings.Join([]string{
		"120 98 0:45 / / rw,relatime - overlay overlay rw",
		"123 120 8:1 /var/lib/docker/volumes/pulse-data/_data /data rw,relatime - ext4 /dev/sda1 rw",
	}, "\n")+"\n")

	mountedParentInfo := writeTempMountInfo(t, strings.Join([]string{
		"120 98 0:45 / / rw,relatime - overlay overlay rw",
		"124 120 8:1 /srv/pulse-storage /var/lib/pulse rw,relatime - ext4 /dev/sda1 rw",
	}, "\n")+"\n")

	escapedSpaceMountInfo := writeTempMountInfo(t, strings.Join([]string{
		"120 98 0:45 / / rw,relatime - overlay overlay rw",
		`125 120 8:1 /srv/data /pulse\040data rw,relatime - ext4 /dev/sda1 rw`,
	}, "\n")+"\n")

	cases := []struct {
		name          string
		isDocker      bool
		dataDir       string
		suppressEnv   string
		mountinfoPath string
		want          bool
	}{
		{
			name:          "docker with unmounted /data is ephemeral",
			isDocker:      true,
			dataDir:       "/data",
			mountinfoPath: ephemeralMountInfo,
			want:          true,
		},
		{
			name:          "docker with trailing slash unmounted /data/ is ephemeral",
			isDocker:      true,
			dataDir:       "/data/",
			mountinfoPath: ephemeralMountInfo,
			want:          true,
		},
		{
			name:          "docker with mounted /data volume is persistent",
			isDocker:      true,
			dataDir:       "/data",
			mountinfoPath: mountedDataInfo,
			want:          false,
		},
		{
			name:          "docker with subdirectory inside mounted parent volume is persistent",
			isDocker:      true,
			dataDir:       "/var/lib/pulse/state",
			mountinfoPath: mountedParentInfo,
			want:          false,
		},
		{
			name:          "docker with escaped space mount point is persistent",
			isDocker:      true,
			dataDir:       "/pulse data",
			mountinfoPath: escapedSpaceMountInfo,
			want:          false,
		},
		{
			name:          "non-docker deployment never warns",
			isDocker:      false,
			dataDir:       "/etc/pulse",
			mountinfoPath: ephemeralMountInfo,
			want:          false,
		},
		{
			name:          "suppression env var silences warning",
			isDocker:      true,
			dataDir:       "/data",
			suppressEnv:   "true",
			mountinfoPath: ephemeralMountInfo,
			want:          false,
		},
		{
			name:          "suppression env var numeric 1 silences warning",
			isDocker:      true,
			dataDir:       "/data",
			suppressEnv:   "1",
			mountinfoPath: ephemeralMountInfo,
			want:          false,
		},
		{
			name:          "missing mountinfo fails safe without false positive",
			isDocker:      true,
			dataDir:       "/data",
			mountinfoPath: filepath.Join(t.TempDir(), "missing"),
			want:          false,
		},
		{
			name:          "root path never warns",
			isDocker:      true,
			dataDir:       "/",
			mountinfoPath: ephemeralMountInfo,
			want:          false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isDockerDataDirEphemeral(tc.isDocker, tc.dataDir, tc.suppressEnv, tc.mountinfoPath)
			if got != tc.want {
				t.Fatalf("isDockerDataDirEphemeral() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFormatDockerEphemeralDataWarningAndCombine(t *testing.T) {
	warn := formatDockerEphemeralDataWarning("/data")
	if !strings.Contains(warn, "Container storage is not persistent") || !strings.Contains(warn, "-v pulse-data:/data") {
		t.Fatalf("unexpected warning text: %q", warn)
	}
	if strings.Contains(warn, ";") {
		t.Fatalf("warning copy must not contain semicolons: %q", warn)
	}

	combined := combineUpdateWarnings("This is a pre-release build.", warn)
	if !strings.HasPrefix(combined, "This is a pre-release build. Container storage is not persistent:") {
		t.Fatalf("unexpected combined warning: %q", combined)
	}
	if got := combineUpdateWarnings("", warn); got != warn {
		t.Fatalf("combineUpdateWarnings with empty prefix = %q, want %q", got, warn)
	}
}
