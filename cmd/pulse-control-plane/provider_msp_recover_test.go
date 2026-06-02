package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
)

func TestProviderMSPCommandExposesRecover(t *testing.T) {
	cmd := newProviderMSPCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "recover" {
			return
		}
	}
	t.Fatal("provider-msp recover command is not registered")
}

func TestPrintProviderMSPRecoveryReport(t *testing.T) {
	var buf bytes.Buffer
	stdout := captureStdoutForProviderMSPRecoverTest(t, &buf)
	printProviderMSPRecoveryReport(&cloudcp.ProviderMSPRecoveryReport{
		OK:             true,
		DryRun:         true,
		PlanVersion:    "msp_growth",
		PlanSource:     cloudcp.ProviderMSPPlanSourceLicenseFile,
		LicenseID:      "lic_test",
		LicenseEmail:   "provider@example.com",
		WorkspaceLimit: 15,
		RecoverCount:   1,
		SkippedCount:   1,
		Items: []cloudcp.ProviderMSPRecoveryItem{
			{
				TenantID:          "t-STUCK",
				DisplayName:       "Client A",
				State:             "provisioning",
				Action:            "recover",
				Reason:            "workspace is stuck in provisioning",
				StuckProvisioning: true,
			},
			{
				TenantID:    "t-HEALTHY",
				DisplayName: "Client B",
				State:       "active",
				Action:      "skip",
				Reason:      "workspace is active and healthy",
			},
		},
	})
	stdout()

	output := buf.String()
	for _, want := range []string{
		"provider_msp_recovery_ok=true",
		"dry_run=true",
		"recover_count=1",
		"skipped_count=1",
		"workspace=t-STUCK",
		"stuck_provisioning=true",
		"reason=\"workspace is stuck in provisioning\"",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func captureStdoutForProviderMSPRecoverTest(t *testing.T, buf *bytes.Buffer) func() {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	return func() {
		if err := w.Close(); err != nil {
			t.Fatalf("close stdout writer: %v", err)
		}
		if _, err := io.Copy(buf, r); err != nil {
			t.Fatalf("copy stdout: %v", err)
		}
		if err := r.Close(); err != nil {
			t.Fatalf("close stdout reader: %v", err)
		}
		os.Stdout = old
	}
}
