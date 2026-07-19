package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigBuildsReportOnlyObserverTargets(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "dev.token")
	if err := os.WriteFile(tokenPath, []byte("observer-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "observers.json")
	config := `{"version":1,"observers":[{"name":"dev","url":"http://127.0.0.1:7656","tokenFile":"` + tokenPath + `"}]}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
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
