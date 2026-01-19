package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestMetadataProvider_SetURLs(t *testing.T) {
	dir := t.TempDir()
	guestStore := config.NewGuestMetadataStore(dir, nil)
	dockerStore := config.NewDockerMetadataStore(dir, nil)
	hostStore := config.NewHostMetadataStore(dir, nil)

	provider := NewMetadataProvider(guestStore, dockerStore, hostStore)

	if err := provider.SetGuestURL("guest1", "https://guest"); err != nil {
		t.Fatalf("SetGuestURL error: %v", err)
	}
	if got := guestStore.Get("guest1"); got == nil || got.CustomURL != "https://guest" {
		t.Fatalf("unexpected guest metadata: %+v", got)
	}

	if err := provider.SetDockerURL("ctr1", "https://docker"); err != nil {
		t.Fatalf("SetDockerURL error: %v", err)
	}
	if got := dockerStore.Get("ctr1"); got == nil || got.CustomURL != "https://docker" {
		t.Fatalf("unexpected docker metadata: %+v", got)
	}

	if err := provider.SetHostURL("host1", "https://host"); err != nil {
		t.Fatalf("SetHostURL error: %v", err)
	}
	if got := hostStore.Get("host1"); got == nil || got.CustomURL != "https://host" {
		t.Fatalf("unexpected host metadata: %+v", got)
	}
}

func TestMetadataProvider_SetURLMissingStore(t *testing.T) {
	provider := NewMetadataProvider(nil, nil, nil)

	if err := provider.SetGuestURL("guest1", "https://guest"); err == nil {
		t.Fatal("expected guest store error")
	}
	if err := provider.SetDockerURL("ctr1", "https://docker"); err == nil {
		t.Fatal("expected docker store error")
	}
	if err := provider.SetHostURL("host1", "https://host"); err == nil {
		t.Fatal("expected host store error")
	}
}
