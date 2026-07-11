package platformsupport

import "testing"

func TestNormalizeAgentReportedPlatform(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		want     string
	}{
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"legacy windows caption", "microsoft windows 11 pro", RuntimePlatformWindows},
		{"legacy windows caption mixed case", "Microsoft Windows 11 Pro", RuntimePlatformWindows},
		{"windows server caption", "Microsoft Windows Server 2022 Standard", RuntimePlatformWindows},
		{"exact windows", "windows", RuntimePlatformWindows},
		{"darwin", "darwin", RuntimePlatformMacOS},
		{"macos", "macos", RuntimePlatformMacOS},
		{"mac", "mac", RuntimePlatformMacOS},
		{"mac os x caption", "Mac OS X", RuntimePlatformMacOS},
		{"freebsd", "freebsd", RuntimePlatformFreeBSD},
		{"freebsd with version", "FreeBSD 14.1-RELEASE", RuntimePlatformFreeBSD},
		{"linux distro preserved", "ubuntu", "ubuntu"},
		{"linux distro lowercased", "Debian GNU/Linux", "debian gnu/linux"},
		{"unraid preserved", "unraid", "unraid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeAgentReportedPlatform(tc.platform); got != tc.want {
				t.Fatalf("NormalizeAgentReportedPlatform(%q) = %q, want %q", tc.platform, got, tc.want)
			}
		})
	}
}

func TestAgentCommandPlatform(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		want     string
	}{
		{"legacy windows caption", "microsoft windows 11 pro", RuntimePlatformWindows},
		{"darwin", "darwin", RuntimePlatformMacOS},
		{"freebsd caption", "FreeBSD 14.1-RELEASE", RuntimePlatformFreeBSD},
		{"linux distro defaults to linux", "ubuntu", RuntimePlatformLinux},
		{"empty defaults to linux", "", RuntimePlatformLinux},
		{"unknown defaults to linux", "beos", RuntimePlatformLinux},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AgentCommandPlatform(tc.platform); got != tc.want {
				t.Fatalf("AgentCommandPlatform(%q) = %q, want %q", tc.platform, got, tc.want)
			}
		})
	}
}
