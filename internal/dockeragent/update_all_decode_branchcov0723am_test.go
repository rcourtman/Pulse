package dockeragent

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

func TestBranchcov0723AmDecodeUpdateAllPayload(t *testing.T) {
	t.Run("nil map yields missing-containerIds error and zero-value struct", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(nil)
		if err == nil {
			t.Fatal("expected error for nil payload")
		}
		if !strings.Contains(err.Error(), "missing containerIds in payload") {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ContainerIDs != nil {
			t.Fatalf("expected zero-value payload, got %+v", got)
		}
	})

	t.Run("empty map yields missing-containerIds error", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(map[string]any{})
		if err == nil {
			t.Fatal("expected error for empty payload")
		}
		if !strings.Contains(err.Error(), "missing containerIds in payload") {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ContainerIDs != nil {
			t.Fatalf("expected zero-value payload, got %+v", got)
		}
	})

	t.Run("absent field versus empty slice not distinguished at error level", func(t *testing.T) {
		// The ERROR is identical for an absent containerIds field and an
		// explicitly empty containerIds slice: the source applies no
		// defaulting and reports "missing containerIds" for both. The
		// returned structs do differ (nil vs a non-nil empty slice), which
		// simply reflects what encoding/json left on the field.
		absent, errAbsent := decodeUpdateAllPayload(map[string]any{"other": "x"})
		if errAbsent == nil {
			t.Fatal("expected error when containerIds is absent")
		}
		if !strings.Contains(errAbsent.Error(), "missing containerIds in payload") {
			t.Fatalf("unexpected error for absent field: %v", errAbsent)
		}
		if absent.ContainerIDs != nil {
			t.Fatalf("absent field should leave ContainerIDs nil, got %+v", absent.ContainerIDs)
		}

		emptySlice, errEmpty := decodeUpdateAllPayload(map[string]any{"containerIds": []any{}})
		if errEmpty == nil {
			t.Fatal("expected error for empty containerIds slice")
		}
		if errEmpty.Error() != errAbsent.Error() {
			t.Fatalf("expected absent and empty-slice to yield identical errors, got %q vs %q", errEmpty.Error(), errAbsent.Error())
		}
		if emptySlice.ContainerIDs == nil || len(emptySlice.ContainerIDs) != 0 {
			t.Fatalf("empty-slice field should yield non-nil empty ContainerIDs, got %+v", emptySlice.ContainerIDs)
		}
	})

	t.Run("entries normalising to empty yield no-valid-IDs error and raw struct", func(t *testing.T) {
		// Every entry is empty or whitespace-only after trimming, so the
		// normalised slice ends up empty: the function must report a distinct
		// error from the missing-field case. On this error path the returned
		// struct still carries the RAW decoded slice (the `= normalized`
		// assignment only runs on success).
		raw := []any{"", "  ", "\t"}
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": raw})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "containerIds contained no valid container IDs") {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got.ContainerIDs, []string{"", "  ", "\t"}) {
			t.Fatalf("expected raw un-normalised ContainerIDs on error, got %+v", got.ContainerIDs)
		}
	})

	t.Run("marshal error is wrapped", func(t *testing.T) {
		marshalErr := errors.New("boom")
		swap(t, &jsonMarshalFn, func(any) ([]byte, error) {
			return nil, marshalErr
		})

		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": []any{"a"}})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "marshal update_all command payload") {
			t.Fatalf("expected marshal wrap, got: %v", err)
		}
		if !errors.Is(err, marshalErr) {
			t.Fatalf("expected wrapped marshal error, got: %v", err)
		}
		if got.ContainerIDs != nil {
			t.Fatalf("expected zero-value payload, got %+v", got)
		}
	})

	t.Run("wrong field type yields decode error", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": 123})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "decode update_all command payload") {
			t.Fatalf("expected decode wrap, got: %v", err)
		}
		if !strings.Contains(err.Error(), "cannot unmarshal") {
			t.Fatalf("expected json unmarshal error, got: %v", err)
		}
		if got.ContainerIDs != nil {
			t.Fatalf("expected zero-value payload, got %+v", got)
		}
	})

	t.Run("valid single id", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": []any{"a"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got.ContainerIDs, []string{"a"}) {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})

	t.Run("multiple ids preserve input order", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": []any{"b", "a"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got.ContainerIDs, []string{"b", "a"}) {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": []any{"  a  "}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got.ContainerIDs, []string{"a"}) {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})

	t.Run("exact duplicates are deduped", func(t *testing.T) {
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": []any{"a", "a"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got.ContainerIDs, []string{"a"}) {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})

	t.Run("duplicates after trim are deduped", func(t *testing.T) {
		// Verifies the source orders trim-before-dedupe: " a " collapses to
		// "a" and is then deduped against the literal "a".
		got, err := decodeUpdateAllPayload(map[string]any{"containerIds": []any{" a ", "a"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got.ContainerIDs, []string{"a"}) {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})
}

func TestBranchcov0723AmDockerFiltersToClientFilters(t *testing.T) {
	t.Run("zero value returns nil", func(t *testing.T) {
		// A zero-value (nil) dockerFilters maps to a nil client.Filters, not
		// to an initialised empty map.
		var f dockerFilters
		if got := f.toClientFilters(); got != nil {
			t.Fatalf("expected nil Filters for zero-value dockerFilters, got %+v", got)
		}
	})

	t.Run("single filter single value", func(t *testing.T) {
		f := newDockerFilters()
		f.Add("status", "running")
		got := f.toClientFilters()
		want := client.Filters{"status": {"running": true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("single key multiple values", func(t *testing.T) {
		f := newDockerFilters()
		f.Add("label", "a")
		f.Add("label", "b")
		got := f.toClientFilters()
		want := client.Filters{"label": {"a": true, "b": true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("multiple keys", func(t *testing.T) {
		f := newDockerFilters()
		f.Add("status", "running")
		f.Add("name", "web")
		got := f.toClientFilters()
		want := client.Filters{
			"name":   {"web": true},
			"status": {"running": true},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("preserves false value", func(t *testing.T) {
		// Add() only ever writes true; constructing a false value directly
		// confirms toClientFilters copies each bool faithfully rather than
		// coercing it.
		f := dockerFilters{"disabled": {"v": false}}
		got := f.toClientFilters()
		want := client.Filters{"disabled": {"v": false}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("result is a deep copy", func(t *testing.T) {
		f := newDockerFilters()
		f.Add("status", "running")
		got := f.toClientFilters()

		got["status"]["running"] = false
		if got["status"]["running"] != false {
			t.Fatal("expected mutation of result to take effect on the result")
		}
		if f["status"]["running"] != true {
			t.Fatalf("expected original dockerFilters unchanged after mutating result, got %v", f["status"]["running"])
		}
	})
}
