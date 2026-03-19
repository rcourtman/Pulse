package unifiedresources

import (
	"strings"
	"testing"
)

func TestParseResourceChangeFilters(t *testing.T) {
	filters, err := ParseResourceChangeFilters(
		[]string{" state_transition,RESTART ", "config_update"},
		[]string{" platform_event ", "pulse_diff, HEURISTIC"},
		[]string{" docker_adapter ", "proxmox_adapter,agent:ops-helper"},
	)
	if err != nil {
		t.Fatalf("ParseResourceChangeFilters() error = %v", err)
	}

	if got, want := len(filters.Kinds), 3; got != want {
		t.Fatalf("kinds length = %d, want %d", got, want)
	}
	if filters.Kinds[0] != ChangeStateTransition || filters.Kinds[1] != ChangeRestart || filters.Kinds[2] != ChangeConfigUpdate {
		t.Fatalf("unexpected parsed kinds: %#v", filters.Kinds)
	}
	if got, want := len(filters.SourceTypes), 3; got != want {
		t.Fatalf("source types length = %d, want %d", got, want)
	}
	if filters.SourceTypes[0] != SourcePlatformEvent || filters.SourceTypes[1] != SourcePulseDiff || filters.SourceTypes[2] != SourceHeuristic {
		t.Fatalf("unexpected parsed source types: %#v", filters.SourceTypes)
	}
	if got, want := len(filters.SourceAdapters), 3; got != want {
		t.Fatalf("source adapters length = %d, want %d", got, want)
	}
	if filters.SourceAdapters[0] != AdapterDocker || filters.SourceAdapters[1] != AdapterProxmox || filters.SourceAdapters[2] != AdapterOpsAgent {
		t.Fatalf("unexpected parsed source adapters: %#v", filters.SourceAdapters)
	}
}

func TestParseResourceChangeFiltersRejectsInvalidValues(t *testing.T) {
	if _, err := ParseResourceChangeFilters([]string{"unknown"}, nil, nil); err == nil || !strings.Contains(err.Error(), "invalid kind value") {
		t.Fatalf("expected invalid kind error, got %v", err)
	}
	if _, err := ParseResourceChangeFilters(nil, []string{"bad-source"}, nil); err == nil || !strings.Contains(err.Error(), "invalid sourceType value") {
		t.Fatalf("expected invalid source type error, got %v", err)
	}
	if _, err := ParseResourceChangeFilters(nil, nil, []string{"bad-adapter"}); err == nil || !strings.Contains(err.Error(), "invalid sourceAdapter value") {
		t.Fatalf("expected invalid source adapter error, got %v", err)
	}
}
