package notifications

import (
	"context"
	"errors"
	"slices"
	"testing"
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

	nm.appriseExec = func(ctx context.Context, args []string) ([]byte, error) {
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

	nm.appriseExec = func(ctx context.Context, args []string) ([]byte, error) {
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

	nm.appriseExec = func(ctx context.Context, args []string) ([]byte, error) {
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

func TestSendAppriseViaCLIPreservesTelegramTopicTarget(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	const topicTarget = "tgram://bot-token/-1001234567890:42"
	var receivedArgs []string
	nm.appriseExec = func(ctx context.Context, args []string) ([]byte, error) {
		receivedArgs = slices.Clone(args)
		return nil, nil
	}

	err := nm.sendAppriseViaCLI(AppriseConfig{
		CLIPath:        "apprise",
		TimeoutSeconds: 1,
		Targets:        []string{topicTarget},
	}, "title", "body")
	if err != nil {
		t.Fatalf("expected Telegram topic target delivery to succeed, got %v", err)
	}
	if len(receivedArgs) == 0 || receivedArgs[len(receivedArgs)-1] != topicTarget {
		t.Fatalf("expected Telegram topic target to be preserved as one CLI argument, got %#v", receivedArgs)
	}
}
