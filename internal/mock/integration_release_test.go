//go:build release

package mock

import (
	"errors"
	"testing"
)

func TestSetEnabled_ReleaseRequiresAuthorization(t *testing.T) {
	resetMockIntegrationState(t)
	SetReleaseFixturesAuthorized(false)
	t.Cleanup(func() { SetReleaseFixturesAuthorized(false) })

	err := SetEnabled(true)
	if !errors.Is(err, ErrReleaseFixturesUnauthorized) {
		t.Fatalf("SetEnabled(true) error = %v, want %v", err, ErrReleaseFixturesUnauthorized)
	}
	if IsMockEnabled() {
		t.Fatal("expected mock mode to remain disabled without release authorization")
	}
}

func TestSetEnabled_ReleaseAuthorizedRuntimeCanEnable(t *testing.T) {
	resetMockIntegrationState(t)
	SetReleaseFixturesAuthorized(true)
	t.Cleanup(func() { SetReleaseFixturesAuthorized(false) })

	if err := SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	if !IsMockEnabled() {
		t.Fatal("expected mock mode to enable once release authorization is granted")
	}

	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	if IsMockEnabled() {
		t.Fatal("expected mock mode to disable cleanly")
	}
}
