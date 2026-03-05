package ai

import (
	"errors"
	"strings"
	"testing"
)

type failingMetadataProvider struct {
	guestErr  error
	dockerErr error
	hostErr   error
}

func (m *failingMetadataProvider) SetGuestURL(guestID, customURL string) error {
	return m.guestErr
}

func (m *failingMetadataProvider) SetDockerURL(resourceID, customURL string) error {
	return m.dockerErr
}

func (m *failingMetadataProvider) SetHostURL(hostID, customURL string) error {
	return m.hostErr
}

func TestService_SetMetadataProvider(t *testing.T) {
	svc := &Service{}
	mp := &mockMetadataProvider{}

	svc.SetMetadataProvider(mp)

	if svc.metadataProvider != mp {
		t.Fatal("expected metadata provider to be set")
	}
}

func TestService_SetResourceURL_InvalidScheme(t *testing.T) {
	svc := &Service{metadataProvider: &mockMetadataProvider{}}

	err := svc.SetResourceURL("vm", "id-1", "ftp://example.com")
	if err == nil || !strings.Contains(err.Error(), "URL must use http:// or https:// scheme") {
		t.Fatalf("expected scheme error, got %v", err)
	}
}

func TestService_SetResourceURL_MissingHost(t *testing.T) {
	svc := &Service{metadataProvider: &mockMetadataProvider{}}

	err := svc.SetResourceURL("vm", "id-1", "http://")
	if err == nil || !strings.Contains(err.Error(), "URL must include a host") {
		t.Fatalf("expected host error, got %v", err)
	}
}

func TestService_SetResourceURL_ProviderErrors(t *testing.T) {
	svc := &Service{metadataProvider: &failingMetadataProvider{
		guestErr:  errors.New("guest error"),
		dockerErr: errors.New("docker error"),
		hostErr:   errors.New("host error"),
	}}

	err := svc.SetResourceURL("vm", "id-1", "https://example.com")
	if err == nil || !strings.Contains(err.Error(), "failed to set guest URL") {
		t.Fatalf("expected wrapped guest error, got %v", err)
	}

	err = svc.SetResourceURL("app-container", "id-2", "https://example.com")
	if err == nil || !strings.Contains(err.Error(), "failed to set Docker URL") {
		t.Fatalf("expected wrapped docker error, got %v", err)
	}

	err = svc.SetResourceURL("agent", "id-3", "https://example.com")
	if err == nil || !strings.Contains(err.Error(), "failed to set host URL") {
		t.Fatalf("expected wrapped host error, got %v", err)
	}
}

func TestService_SetResourceURL_RejectsLegacyResourceTypeAliases(t *testing.T) {
	svc := &Service{metadataProvider: &mockMetadataProvider{}}

	for _, legacyType := range []string{"guest", "docker", "container", "lxc", "qemu", "docker_container", "docker_service"} {
		err := svc.SetResourceURL(legacyType, "guest-1", "https://example.com")
		if err == nil || !strings.Contains(err.Error(), "unknown resource type") {
			t.Fatalf("expected unknown resource type error for %q, got %v", legacyType, err)
		}
	}
}
