package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadConfigRejectsUnknownRuntimeState(t *testing.T) {
	for _, shape := range []string{"directory", "non-directory-ancestor", "unreadable", "link-loop"} {
		for _, fallback := range []string{"argument", "file", "environment"} {
			t.Run(shape+"/"+fallback, func(t *testing.T) {
				root := t.TempDir()
				dir := filepath.Join(root, "synthetic-private-state")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				tokenPath := filepath.Join(dir, "runtime.token")
				switch shape {
				case "directory":
					if err := os.Mkdir(tokenPath, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(tokenPath, "keep"), []byte("synthetic-private-marker"), 0600); err != nil {
						t.Fatal(err)
					}
				case "non-directory-ancestor":
					dir = filepath.Join(root, "synthetic-private-not-directory")
					if err := os.WriteFile(dir, []byte("synthetic-private-marker"), 0600); err != nil {
						t.Fatal(err)
					}
					tokenPath = filepath.Join(dir, "runtime.token")
				case "unreadable":
					if runtime.GOOS == "windows" {
						t.Skip("Unix mode-based read obstruction")
					}
					if err := os.WriteFile(tokenPath, []byte("synthetic-original-runtime"), 0000); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(tokenPath, 0600) })
				case "link-loop":
					if err := os.Symlink("runtime.token", tokenPath); err != nil {
						if runtime.GOOS == "windows" {
							t.Skip("Windows symlink privilege unavailable")
						}
						t.Fatal(err)
					}
				}
				if _, err := os.ReadFile(tokenPath); err == nil {
					t.Skip("filesystem did not enforce the intended read obstruction")
				}
				args := []string{"--url", "http://127.0.0.1:7655", "--state-dir", dir, "--enroll"}
				env := func(string) string { return "" }
				switch fallback {
				case "argument":
					args = append(args, "--token", "synthetic-bootstrap-argument")
				case "file":
					file := filepath.Join(root, "bootstrap.token")
					if err := os.WriteFile(file, []byte("synthetic-bootstrap-file"), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--token-file", file)
				case "environment":
					env = func(key string) string {
						if key == "PULSE_TOKEN" {
							return "synthetic-bootstrap-environment"
						}
						return ""
					}
				}
				cfg, err := loadConfig(args, env)
				if err == nil || !strings.Contains(err.Error(), "cannot read saved runtime token") || !strings.Contains(err.Error(), "readable by the agent service") {
					t.Errorf("unknown runtime state needs a precise read error: %v", err)
				}
				if cfg.APIToken != "" || cfg.AgentID != "" {
					t.Error("unknown runtime state returned fallback authority/identity")
				}
				for _, private := range []string{root, "synthetic-private", "synthetic-bootstrap", "synthetic-original-runtime"} {
					if strings.Contains(fmt.Sprint(err), private) {
						t.Error("runtime read error disclosed a credential or private path")
					}
				}
				if shape == "directory" {
					data, readErr := os.ReadFile(filepath.Join(tokenPath, "keep"))
					if readErr != nil || !bytes.Equal(data, []byte("synthetic-private-marker")) {
						t.Error("configuration read modified obstructed runtime state")
					}
				}
			})
		}
	}
}

func TestLoadConfigPreservesKnownRuntimeStates(t *testing.T) {
	for _, enroll := range []bool{false, true} {
		for _, state := range []string{"missing", "empty", "whitespace", "saved", "directory"} {
			t.Run(fmt.Sprintf("enroll-%t/%s", enroll, state), func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "runtime.token")
				values := map[string]string{"empty": "", "whitespace": " \t\r\n", "saved": " \tsynthetic-saved-runtime\r\n"}
				if data, ok := values[state]; ok {
					if err := os.WriteFile(path, []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if state == "directory" {
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"--url", "http://127.0.0.1:7655", "--state-dir", dir, "--token", "synthetic-bootstrap"}
				if enroll {
					args = append(args, "--enroll")
				}
				cfg, err := loadConfig(args, func(string) string { return "" })
				if state == "directory" && enroll {
					if err == nil {
						t.Error("unreadable enrollment state was accepted")
					}
					return
				}
				want := "synthetic-bootstrap"
				if enroll && state == "saved" {
					want = "synthetic-saved-runtime"
				}
				if err != nil || cfg.APIToken != want || cfg.Enroll != enroll {
					t.Errorf("known state changed credential selection: %v", err)
				}
			})
		}
	}
}
