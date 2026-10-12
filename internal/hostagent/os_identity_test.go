package hostagent

import (
	"context"
	"os"
	"testing"
)

func TestResolveHostOSIdentityUsesLinuxDistributionMetadata(t *testing.T) {
	tests := []struct {
		name       string
		platform   string
		version    string
		release    string
		releaseErr error
		vendor     string
		wantName   string
	}{
		{name: "Debian with raspi-config", platform: "raspbian", version: "13.7", release: "ID=debian\nVERSION_ID=13\n", wantName: "debian"},
		{name: "actual Raspbian", platform: "raspbian", version: "13.7", release: "ID=raspbian\n", wantName: "raspbian"},
		{name: "Ubuntu is not its Debian family", platform: "debian", version: "24.04", release: "ID=ubuntu\nID_LIKE=debian\n", wantName: "ubuntu"},
		{name: "double quoted ID", platform: "linux", release: "ID=\"debian\"\n", wantName: "debian"},
		{name: "single quoted ID", platform: "linux", release: "ID='ubuntu'\n", wantName: "ubuntu"},
		{name: "whitespace and CRLF", platform: "linux", release: " # comment\r\n ID = \"debian\" \r\n", wantName: "debian"},
		{name: "distribution punctuation", platform: "linux", release: "ID=opensuse-tumbleweed\n", wantName: "opensuse-tumbleweed"},
		{name: "last ID wins", platform: "linux", release: "ID=debian\nID=ubuntu\n", wantName: "ubuntu"},
		{name: "vendor file when etc is absent", platform: "raspbian", releaseErr: os.ErrNotExist, vendor: "ID=debian\n", wantName: "debian"},
		{name: "etc takes precedence over vendor", platform: "linux", release: "ID=ubuntu\n", vendor: "ID=debian\n", wantName: "ubuntu"},
		{name: "unavailable files", platform: "raspbian", releaseErr: os.ErrNotExist, wantName: "raspbian"},
		{name: "unreadable etc", platform: "raspbian", releaseErr: os.ErrPermission, vendor: "ID=debian\n", wantName: "raspbian"},
		{name: "empty etc does not merge vendor", platform: "raspbian", vendor: "ID=debian\n", wantName: "raspbian"},
		{name: "NAME is not a distro ID", platform: "raspbian", release: "NAME=Debian\nID_LIKE=debian\n", wantName: "raspbian"},
		{name: "empty ID", platform: "raspbian", release: "ID=\"\"\n", wantName: "raspbian"},
		{name: "unclosed quote", platform: "raspbian", release: "ID=\"debian\n", wantName: "raspbian"},
		{name: "mismatched quotes", platform: "raspbian", release: "ID=\"debian'\n", wantName: "raspbian"},
		{name: "extra quote", platform: "raspbian", release: "ID=\"debian\"\"\n", wantName: "raspbian"},
		{name: "multiple words", platform: "raspbian", release: "ID=Debian GNU/Linux\n", wantName: "raspbian"},
		{name: "expansion is not evaluated", platform: "raspbian", release: "ID=\"$(printf debian)\"\n", wantName: "raspbian"},
		{name: "NUL is invalid", platform: "raspbian", release: "ID=debian\x00\n", wantName: "raspbian"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &mockCollector{
				readFileFn: func(path string) ([]byte, error) {
					switch path {
					case "/etc/os-release":
						return []byte(tt.release), tt.releaseErr
					case "/usr/lib/os-release":
						if tt.vendor != "" {
							return []byte(tt.vendor), nil
						}
					}
					return nil, os.ErrNotExist
				},
				commandCombinedOutputFn: func(_ context.Context, _ string, _ ...string) (string, error) {
					t.Fatal("distribution detection must not run commands")
					return "", os.ErrNotExist
				},
			}
			name, version := resolveHostOSIdentity(collector, " "+tt.platform+" ", " "+tt.version+" ")
			if name != tt.wantName || version != tt.version {
				t.Fatalf("identity = %q %q, want %q %q", name, version, tt.wantName, tt.version)
			}
		})
	}
}

func TestResolveHostOSIdentityLeavesNonLinuxUnchanged(t *testing.T) {
	for _, goos := range []string{"windows", "freebsd", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			collector := &mockCollector{
				goos: goos,
				readFileFn: func(string) ([]byte, error) {
					t.Fatal("non-Linux identity must not read Linux release files")
					return nil, os.ErrNotExist
				},
			}
			name, version := resolveHostOSIdentity(collector, " "+goos+" ", " 1.2 ")
			if name != goos || version != "1.2" {
				t.Fatalf("identity = %q %q", name, version)
			}
		})
	}
	if name, version := resolveHostOSIdentity(nil, " linux ", " 1.2 "); name != "linux" || version != "1.2" {
		t.Fatalf("nil collector identity = %q %q", name, version)
	}
}

func TestResolveHostOSIdentityKeepsAppliancePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		pve         bool
		wantName    string
		wantVersion string
	}{
		{name: "Synology", files: map[string]string{"/etc.defaults/VERSION": "productversion=7.2.2\nbuildnumber=72806\n"}, wantName: "Synology DSM", wantVersion: "7.2.2-72806"},
		{name: "QNAP", files: map[string]string{"/etc/config/uLinux.conf": "Version=5.2.0\n"}, wantName: "QNAP QTS", wantVersion: "5.2.0"},
		{name: "Unraid", files: map[string]string{"/etc/unraid-version": "version=7.2.2\n"}, wantName: "Unraid", wantVersion: "7.2.2"},
		{name: "Proxmox", pve: true, wantName: proxmoxPVEOSName, wantVersion: "9.1.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &mockCollector{
				readFileFn: func(path string) ([]byte, error) {
					if path == "/etc/os-release" {
						return []byte("ID=debian\nVERSION_ID=13\n"), nil
					}
					if data, ok := tt.files[path]; ok {
						return []byte(data), nil
					}
					return nil, os.ErrNotExist
				},
				statFn: func(path string) (os.FileInfo, error) {
					if tt.pve && path == "/etc/pve" {
						return nil, nil
					}
					return nil, os.ErrNotExist
				},
				lookPathFn: func(name string) (string, error) {
					if tt.pve && name == "pveversion" {
						return "/synthetic/pveversion", nil
					}
					return "", os.ErrNotExist
				},
				commandCombinedOutputFn: func(_ context.Context, name string, _ ...string) (string, error) {
					if !tt.pve || name != "/synthetic/pveversion" {
						t.Fatalf("unexpected command: %s", name)
					}
					return "pve-manager/9.1.9/synthetic", nil
				},
			}
			name, version := resolveHostOSIdentity(collector, "raspbian", "13.7")
			if name != tt.wantName || version != tt.wantVersion {
				t.Fatalf("identity = %q %q, want %q %q", name, version, tt.wantName, tt.wantVersion)
			}
		})
	}
}
