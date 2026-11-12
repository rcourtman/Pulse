package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigBuildsReportOnlyObserverTargets(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	if err := os.WriteFile(tokenPath, []byte("observer-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "observers.json")
	config, err := json.Marshal(map[string]any{
		"version": 1,
		"observers": []map[string]any{{
			"name":      "dev",
			"url":       "http://127.0.0.1:7656",
			"tokenFile": tokenPath,
		}},
	})
	if err != nil {
		t.Fatalf("marshal observer config: %v", err)
	}
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig([]string{
		"--url", "http://127.0.0.1:7655",
		"--token", "primary-token",
		"--observers-file", configPath,
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.Observers) != 1 || cfg.Observers[0].Token != "observer-token" {
		t.Fatalf("observers = %+v", cfg.Observers)
	}
	dockerTargets := dockerReportTargets(cfg)
	if len(dockerTargets) != 2 || !dockerTargets[0].Authoritative || dockerTargets[1].Authoritative {
		t.Fatalf("docker targets = %+v", dockerTargets)
	}
	kubeTargets := kubernetesReportTargets(cfg)
	if len(kubeTargets) != 2 || !kubeTargets[0].Authoritative || kubeTargets[1].Authoritative {
		t.Fatalf("kubernetes targets = %+v", kubeTargets)
	}
}

func TestObserverRefusalPreservesPrimarySavedAuthority(t *testing.T) {
	for _, refusal := range []string{"oversized-token", "invalid-config", "missing-token"} {
		t.Run(refusal, func(t *testing.T) {
			dir := t.TempDir()
			tokenPath, configPath := filepath.Join(dir, "observer.token"), filepath.Join(dir, "observers.json")
			config, err := json.Marshal(map[string]any{
				"version":   1,
				"observers": []map[string]any{{"name": "isolated", "url": "https://observer.example.test", "tokenFile": tokenPath, "provisionProxmox": false}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if refusal == "invalid-config" {
				config = append(config, '{', '}')
			}
			if err := os.WriteFile(configPath, config, 0600); err != nil {
				t.Fatal(err)
			}
			if refusal == "oversized-token" {
				if err := os.WriteFile(tokenPath, []byte(strings.Repeat("x", 64*1024+1)), 0600); err != nil {
					t.Fatal(err)
				}
			} else if refusal != "missing-token" {
				if err := os.WriteFile(tokenPath, []byte("synthetic-observer-token"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := filepath.Join(dir, "state")
			if err := os.Mkdir(state, 0700); err != nil {
				t.Fatal(err)
			}
			originals := map[string]string{"token": "synthetic-primary-token\n", "runtime.token": "synthetic-runtime-token\n", "agent-id": "synthetic-saved-id\n"}
			infos := map[string]os.FileInfo{}
			for name, content := range originals {
				path := filepath.Join(state, name)
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				infos[name], err = os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			logPath := filepath.Join(dir, "must-not-create.log")
			err = run(context.Background(), []string{"--url", "https://primary.example.test", "--state-dir", state, "--observers-file", configPath, "--log-file", logPath, "--enroll"}, func(string) string { return "" })
			if err == nil || !strings.Contains(err.Error(), "load observer destinations") {
				t.Fatalf("startup did not stop at the observer read: %v", err)
			}
			if _, err := os.Stat(logPath); !os.IsNotExist(err) {
				t.Fatal("observer refusal reached logging/authenticated startup")
			}
			entries, err := os.ReadDir(state)
			if err != nil || len(entries) != len(originals) {
				t.Fatal("observer refusal created new primary authority")
			}
			for name, expected := range originals {
				path := filepath.Join(state, name)
				got, readErr := os.ReadFile(path)
				info, statErr := os.Stat(path)
				if readErr != nil || statErr != nil || string(got) != expected || !os.SameFile(infos[name], info) || info.Mode() != infos[name].Mode() || !info.ModTime().Equal(infos[name].ModTime()) {
					t.Fatal("observer refusal changed saved primary state")
				}
			}
			// Recovery needs only restoration of the observer file, never
			// deletion/re-enrollment or a different primary credential.
			if err := os.WriteFile(tokenPath, []byte("restored-synthetic-observer-token"), 0600); err != nil {
				t.Fatal(err)
			}
			config = []byte(strings.TrimSuffix(string(config), "{}"))
			if err := os.WriteFile(configPath, config, 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig([]string{"--url", "https://primary.example.test", "--state-dir", state, "--observers-file", configPath, "--enroll"}, func(string) string { return "" })
			if err != nil || cfg.APIToken != "synthetic-runtime-token" || len(cfg.Observers) != 1 || cfg.Observers[0].Token != "restored-synthetic-observer-token" || cfg.Observers[0].ProvisionProxmox {
				t.Fatalf("restored observer did not preserve original primary access and choices: %v", err)
			}
		})
	}
}
