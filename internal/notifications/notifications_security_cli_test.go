package notifications

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSendAppriseViaCLINoTargets(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	err := nm.sendAppriseViaCLI(AppriseConfig{
		CLIPath:        "apprise",
		TimeoutSeconds: 1,
	}, "title", "body")
	if err == nil {
		t.Fatalf("expected error when no targets configured")
	}
}

func TestSendAppriseViaCLIExecError(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		return []byte("boom"), errors.New("exec failed")
	}

	err := nm.sendAppriseViaCLI(AppriseConfig{
		CLIPath:        "apprise",
		TimeoutSeconds: 1,
		Targets:        []string{"discord://token"},
	}, "title", "body")
	if err == nil {
		t.Fatalf("expected exec error")
	}
}

func TestSendAppriseViaCLISuccess(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}

	err := nm.sendAppriseViaCLI(AppriseConfig{
		CLIPath:        "apprise",
		TimeoutSeconds: 1,
		Targets:        []string{"discord://token"},
	}, "title", "body")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestSendAppriseViaCLISuccessNoOutput(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		return nil, nil
	}

	err := nm.sendAppriseViaCLI(AppriseConfig{
		CLIPath:        "apprise",
		TimeoutSeconds: 1,
		Targets:        []string{"discord://token"},
	}, "title", "body")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestDefaultAppriseExecRunsCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := defaultAppriseExec(ctx, "true", nil); err != nil {
		t.Fatalf("expected defaultAppriseExec to run, got %v", err)
	}
}
