package agentidentity

import (
	"bytes"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity/identitytest"
)

func TestTargetUsesCompilerMetadata(t *testing.T) {
	for _, tc := range []struct{ os, arch, arm, want string }{
		{"linux", "arm", "6", "linux-armv6"}, {"linux", "arm", "6,softfloat", "linux-armv6"},
		{"linux", "arm", "7,hardfloat", "linux-armv7"}, {"linux", "arm", "", ""}, {"linux", "arm", "5", ""}, {"linux", "arm", "6,unknown", ""},
		{"darwin", "arm", "7", ""}, {"solaris", "amd64", "", ""}, {"windows", "386", "", "windows-386"},
	} {
		got, err := Target(&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "GOOS", Value: tc.os}, {Key: "GOARCH", Value: tc.arch}, {Key: "GOARM", Value: tc.arm}}})
		if got != tc.want || (err != nil) != (tc.want == "") {
			t.Fatalf("%+v: %q %v", tc, got, err)
		}
	}
}

func TestRealStrippedExecutableIdentity(t *testing.T) {
	for _, target := range []string{"linux-amd64", "linux-armv6", "linux-armv7", "windows-amd64", "darwin-arm64"} {
		t.Run(target, func(t *testing.T) {
			file := identitytest.Build(t, identitytest.Options{Version: "v6.2.0-rc.8", Target: target})
			if err := Validate(file, "6.2.0-rc.8+server.build", target); err != nil {
				t.Fatal(err)
			}
			for _, version := range []string{"v6.2.0", "v6.2.1", "v6.2.0-rc.9"} {
				if err := Validate(file, version, target); err == nil {
					t.Fatalf("accepted %s as %s", target, version)
				}
			}
			if err := Validate(file, "v6.2.0-rc.8", "windows-386"); err == nil {
				t.Fatal("accepted wrong target")
			}
		})
	}
	for _, tc := range []struct {
		o      identitytest.Options
		reason string
	}{
		{identitytest.Options{Version: "v6.2.10"}, "stale build"},
		{identitytest.Options{Version: "v6.2.0", Extra: "v6.2.1"}, "stale build"},
		{identitytest.Options{Version: "v6.2.1", Package: "example.com/dependency/cmd/pulse-agent"}, "unexpected executable package"},
		{identitytest.Options{Version: "v6.2.1", Stamp: "unstamped"}, "missing bounded executable release identity"},
		{identitytest.Options{Version: "v6.2.1", Extra: "[[pulse-agent-release:v1:v6.2.0]]"}, "conflicting executable release identities"},
	} {
		file := identitytest.Build(t, tc.o)
		if err := Validate(file, "v6.2.1", ""); err == nil || !strings.Contains(err.Error(), tc.reason) {
			t.Fatalf("%+v: expected %q, got %v", tc.o, tc.reason, err)
		}
	}
	file := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", NoTrimpath: true})
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(content, []byte("[[pulse-agent-release:v1:v6.2.1]]")) < 2 {
		t.Fatal("non-trimpath fixture did not contain repeated stamp metadata")
	}
	if err := Validate(file, "v6.2.1", ""); err != nil {
		t.Fatalf("legitimate repeated linker metadata: %v", err)
	}
}

func TestReleaseStampBoundariesAndDuplicates(t *testing.T) {
	stamp := "[[pulse-agent-release:v1:v6.2.1]]"
	for _, body := range []string{stamp, stamp + stamp, strings.Repeat("x", 65530) + stamp} {
		got, err := readIdentity(bytes.NewBufferString(body))
		if got != "v6.2.1" || err != nil {
			t.Fatalf("%s %v", got, err)
		}
	}
	for _, body := range []string{"v6.2.1", stamp[:len(stamp)-1], stamp + "[[pulse-agent-release:v1:v6.2.10]]"} {
		if _, err := readIdentity(strings.NewReader(body)); err == nil {
			t.Fatal("accepted missing or conflicting identity")
		}
	}
}

func TestRuntimeReleaseIdentityRequiresActualVariable(t *testing.T) {
	original := ReleaseIdentity
	t.Cleanup(func() { ReleaseIdentity = original })
	ReleaseIdentity = ""
	if _, err := RuntimeReleaseIdentity("v6.2.1"); err == nil {
		t.Fatal("unstamped executable accepted")
	}
	ReleaseIdentity = "[[pulse-agent-release:v1:v6.2.1]]"
	if got, err := RuntimeReleaseIdentity("6.2.1+build"); err != nil || got != ReleaseIdentity {
		t.Fatalf("canonical identity %q: %v", got, err)
	}
	if _, err := RuntimeReleaseIdentity("v6.2.10"); err == nil {
		t.Fatal("mismatched runtime version accepted")
	}
}

func TestMalformedReleaseStampIsIndependentOfAlignment(t *testing.T) {
	valid := "[[pulse-agent-release:v1:v6.2.1]]"
	malformed := []string{
		"[[pulse-agent-release:v1:v" + strings.Repeat("1", 300) + ".2.3]]",
		"[[pulse-agent-release:v1:v6.2.1",
		"[[pulse-agent-release:v2:v6.2.1]]",
	}
	for _, bad := range malformed {
		for _, padding := range []int{0, 65520, 65535, 65536} {
			body := valid + strings.Repeat("x", padding) + bad
			if _, err := readIdentity(strings.NewReader(body)); err == nil {
				t.Fatalf("malformed identity accepted at padding %d", padding)
			}
		}
	}
}

func TestRecordedStampCorroboratesCanonicalSymbol(t *testing.T) {
	stamp := "[[pulse-agent-release:v1:v6.2.1]]"
	symbol := "github.com/rcourtman/pulse-go-rewrite/internal/agentidentity.ReleaseIdentity="
	for _, tc := range []struct {
		flags string
		ok    bool
	}{
		{"-s -w", true}, {"-X " + symbol + stamp, true}, {"-X=" + symbol + stamp, true}, {"-X '" + symbol + stamp + "'", true},
		{"-X main.UnusedReleaseLiteral=" + stamp, false}, {"-X " + symbol + stamp + "suffix", false},
	} {
		err := validateRecordedStamp(&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "-ldflags", Value: tc.flags}}}, "v6.2.1")
		if (err == nil) != tc.ok {
			t.Fatalf("%q: %v", tc.flags, err)
		}
	}
}
