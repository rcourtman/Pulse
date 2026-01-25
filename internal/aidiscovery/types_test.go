package aidiscovery

import "testing"

func TestResourceIDHelpers(t *testing.T) {
	id := MakeResourceID(ResourceTypeDocker, "host1", "app")
	if id != "docker:host1:app" {
		t.Fatalf("unexpected id: %s", id)
	}

	rt, host, res, err := ParseResourceID(id)
	if err != nil {
		t.Fatalf("ParseResourceID error: %v", err)
	}
	if rt != ResourceTypeDocker || host != "host1" || res != "app" {
		t.Fatalf("unexpected parse result: %s %s %s", rt, host, res)
	}

	if _, _, _, err := ParseResourceID("invalid"); err == nil {
		t.Fatalf("expected parse error for invalid id")
	}
}
