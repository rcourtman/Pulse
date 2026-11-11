package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveTokenImplicitFileReadStates(t *testing.T) {
	for _, stateDir := range []string{"", filepath.Join(string(filepath.Separator), "synthetic-private-instance")} {
		for _, tc := range []struct {
			name      string
			data      []byte
			err       error
			want      string
			wantError bool
		}{
			{name: "saved", data: []byte(" \tsynthetic-credential\r\n"), want: "synthetic-credential"},
			{name: "empty"},
			{name: "whitespace", data: []byte(" \t\r\n")},
			{name: "absent", err: os.ErrNotExist},
			{name: "wrapped absent", err: fmt.Errorf("synthetic-private-error: %w", os.ErrNotExist)},
			{name: "permission", err: &os.PathError{Op: "open", Path: "synthetic-private-path", Err: os.ErrPermission}, wantError: true},
			{name: "invalid", err: os.ErrInvalid, wantError: true},
			{name: "other", data: []byte("synthetic-credential"), err: errors.New("synthetic-private-error"), wantError: true},
		} {
			t.Run(fmt.Sprintf("default-%t/%s", stateDir == "", tc.name), func(t *testing.T) {
				wantPath := filepath.Join(stateDir, "token")
				if stateDir == "" {
					wantPath = defaultTokenFilePath()
				}
				calls := 0
				got, err := resolveTokenInternal("", "", "", stateDir, func(path string) ([]byte, error) {
					calls++
					if path != wantPath {
						t.Error("implicit read escaped the selected instance")
					}
					return tc.data, tc.err
				})
				if calls != 1 || got != tc.want {
					t.Fatal("implicit token read count or credential selection changed")
				}
				if !tc.wantError {
					if err != nil {
						t.Fatalf("known saved-token state failed: %v", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "cannot read saved API token") || !strings.Contains(err.Error(), "readable by the agent service") {
					t.Fatalf("unknown token state did not give actionable read guidance: %v", err)
				}
				for _, private := range []string{wantPath, "synthetic-private", "synthetic-credential"} {
					if strings.Contains(err.Error(), private) {
						t.Fatal("saved-token read error exposed private configuration")
					}
				}
			})
		}
	}
}

func TestResolveTokenDoesNotReadUnselectedImplicitFile(t *testing.T) {
	for _, tc := range []struct {
		name        string
		argument    string
		selected    string
		environment string
		want        string
	}{
		{name: "argument", argument: "synthetic-argument", selected: "unselected", environment: "synthetic-environment", want: "synthetic-argument"},
		{name: "selected file", selected: "synthetic-selected-file", environment: "synthetic-environment", want: "synthetic-file"},
		{name: "environment", environment: "synthetic-environment", want: "synthetic-environment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTokenInternal(tc.argument, tc.selected, tc.environment, "/synthetic-private-instance", func(path string) ([]byte, error) {
				if tc.selected != "" && tc.argument == "" && path == tc.selected {
					return []byte("synthetic-file"), nil
				}
				t.Error("read a credential source the operator did not select")
				return nil, os.ErrPermission
			})
			if err != nil || got != tc.want {
				t.Fatalf("higher-priority credential was affected by an unselected saved file: %v", err)
			}
		})
	}
}

func TestLoadConfigRejectsUnreadableImplicitToken(t *testing.T) {
	for _, shape := range []string{"directory", "unreadable", "link-loop"} {
		t.Run(shape, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "token")
			marker := []byte("synthetic-private-keep-marker")
			markerPath := path
			switch shape {
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				markerPath = filepath.Join(path, "keep")
				if err := os.WriteFile(markerPath, marker, 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if runtime.GOOS == "windows" {
					t.Skip("Unix mode-based read obstruction")
				}
				if err := os.WriteFile(path, marker, 0000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			case "link-loop":
				if err := os.Symlink("token", path); err != nil {
					if runtime.GOOS == "windows" {
						t.Skip("Windows symlink privilege unavailable")
					}
					t.Fatal(err)
				}
			}
			if _, err := os.ReadFile(path); err == nil {
				t.Skip("filesystem did not enforce the intended read obstruction")
			} else if errors.Is(err, os.ErrNotExist) {
				t.Skip("platform classifies this obstruction as a missing file")
			}
			cfg, err := loadConfig([]string{"--url", "https://pulse.example.test", "--state-dir", dir}, func(string) string { return "" })
			if err == nil || !strings.Contains(err.Error(), "cannot read saved API token") {
				t.Fatalf("unreadable saved token did not stop configuration loading: %v", err)
			}
			if cfg.APIToken != "" || cfg.AgentID != "" || cfg.EnableCommands {
				t.Fatal("unknown credential state produced a usable agent configuration")
			}
			if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), string(marker)) {
				t.Fatal("saved-token error exposed private configuration")
			}
			if shape == "unreadable" {
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if shape == "link-loop" {
				if target, err := os.Readlink(path); err != nil || target != "token" {
					t.Fatal("configuration read changed the obstructed file")
				}
			} else if data, err := os.ReadFile(markerPath); err != nil || !bytes.Equal(data, marker) {
				t.Fatal("configuration read changed saved state")
			}
		})
	}
}

func TestLoadConfigKnownImplicitTokenStates(t *testing.T) {
	for _, state := range []string{"missing", "empty", "whitespace", "saved", "argument", "selected file", "environment", "runtime"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			args := []string{"--url", "https://pulse.example.test", "--state-dir", dir, "--agent-id", "synthetic-saved-agent", "--command-authority", "monitoring-only"}
			want, env := "", ""
			switch state {
			case "empty", "whitespace", "saved":
				value := map[string]string{"empty": "", "whitespace": " \t\r\n", "saved": " \tsynthetic-saved-credential\n"}[state]
				if err := os.WriteFile(filepath.Join(dir, "token"), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
				if state == "saved" {
					want = "synthetic-saved-credential"
				}
			case "argument", "selected file", "environment", "runtime":
				// An unreadable implicit file is irrelevant if it was not selected.
				if err := os.Mkdir(filepath.Join(dir, "token"), 0700); err != nil {
					t.Fatal(err)
				}
				want = "synthetic-selected-credential"
				switch state {
				case "argument":
					args = append(args, "--token", want)
				case "selected file":
					selected := filepath.Join(dir, "selected-token")
					if err := os.WriteFile(selected, []byte(want), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--token-file", selected)
				case "environment":
					env = want
				case "runtime":
					if err := os.WriteFile(filepath.Join(dir, "runtime.token"), []byte(want), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--enroll")
				}
			}
			cfg, err := loadConfig(args, func(key string) string {
				if key == "PULSE_TOKEN" {
					return env
				}
				return ""
			})
			if err != nil || cfg.APIToken != want {
				t.Fatalf("known credential state changed selection: %v", err)
			}
			if cfg.AgentID != "synthetic-saved-agent" || cfg.StateDir != dir || cfg.EnableCommands || cfg.InsecureSkipVerify || string(cfg.CommandAuthorityProfile) != "monitoring-only" {
				t.Fatal("saved-token diagnostic changed identity, TLS or command ceilings")
			}
		})
	}
}
