// Package identitytest builds small real Go executable fixtures for identity
// contract tests. It never substitutes a byte-string stand-in for build metadata.
package identitytest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
)

var cache = struct {
	sync.Mutex
	binaries map[string][]byte
}{binaries: map[string][]byte{}}

type Options struct {
	Version, Stamp, Target, Package, Extra, ReportPath, VersionOutput, IdentityOutput string
	NoTrimpath                                                                        bool
}

func Build(t testing.TB, o Options) string {
	t.Helper()
	if o.Target == "" {
		o.Target = runtime.GOOS + "-" + runtime.GOARCH
		if runtime.GOARCH == "arm" {
			info, ok := debug.ReadBuildInfo()
			if !ok {
				t.Fatal("missing fixture compiler metadata")
			}
			level := ""
			for _, setting := range info.Settings {
				if setting.Key == "GOARM" {
					level = strings.SplitN(setting.Value, ",", 2)[0]
				}
			}
			if level != "6" && level != "7" {
				t.Fatalf("unsupported fixture GOARM %s", level)
			}
			o.Target = runtime.GOOS + "-armv" + level
		}
	}
	if o.Package == "" {
		o.Package = "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent"
	}
	if o.Stamp == "" {
		o.Stamp = "[[pulse-agent-release:v1:" + o.Version + "]]"
	}
	key := fmt.Sprintf("%#v", o)
	cache.Lock()
	defer cache.Unlock()
	content, ok := cache.binaries[key]
	if !ok {
		root := t.TempDir()
		source := fmt.Sprintf(`package main
import("fmt";"os")
var Version string
var Stamp string
var Extra string
func main(){
 if len(os.Args)>1 && os.Args[1]=="--release-identity" {value:=[]byte{%s};for i:=range value{value[i]^=0x55};if output:=string(value);output!=""{fmt.Print(output)}else{fmt.Println(Stamp)};return}
 if len(os.Args)>1 && os.Args[1]=="--version" {if output:=%q;output!=""{fmt.Print(output)}else{fmt.Println(Version)};return}
 fmt.Print(Stamp,Extra,%q)
}
`, byteList(o.IdentityOutput), o.VersionOutput, o.ReportPath)
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+o.Package+"\n\ngo 1.26.0\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(root, "fixture")
		flags := "-s -w -X main.Version=" + o.Version + " -X main.Stamp=" + o.Stamp + " -X main.Extra=" + o.Extra
		if o.NoTrimpath {
			flags += " -X github.com/rcourtman/pulse-go-rewrite/internal/agentidentity.ReleaseIdentity=" + o.Stamp
		}
		args := []string{"build", "-p=1", "-buildvcs=false", "-ldflags=" + flags, "-o", binary}
		if !o.NoTrimpath {
			args = append(args, "-trimpath")
		}
		args = append(args, ".")
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		parts := strings.Split(o.Target, "-")
		arch := parts[1]
		arm := ""
		if strings.HasPrefix(arch, "armv") {
			arm = strings.TrimPrefix(arch, "armv")
			arch = "arm"
		}
		cmd.Env = append(os.Environ(), "GOOS="+parts[0], "GOARCH="+arch, "GOARM="+arm, "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build real identity fixture: %v\n%s", err, out)
		}
		var err error
		content, err = os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		cache.binaries[key] = content
	}
	binary := filepath.Join(t.TempDir(), "pulse-agent")
	if strings.HasPrefix(o.Target, "windows-") {
		binary += ".exe"
	}
	if err := os.WriteFile(binary, content, 0700); err != nil {
		t.Fatal(err)
	}
	return binary
}

func byteList(value string) string {
	parts := make([]string, len(value))
	for i := range []byte(value) {
		parts[i] = fmt.Sprint(value[i] ^ 0x55)
	}
	return strings.Join(parts, ",")
}
