package unifiedresources

import (
	"maps"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestLabelTagsStableAcrossMapIterations(t *testing.T) {
	labels := map[string]string{
		"app": "api", "env": "production", "team": "ops", "tier": "backend",
		"region": "west", "release": "stable", "managed": "yes", "version": "1",
	}
	before := maps.Clone(labels)
	want := []string{"app:api", "env:production", "managed:yes", "region:west", "release:stable", "team:ops", "tier:backend", "version:1"}
	for i := 0; i < 256; i++ {
		if got := labelsToTags(labels); !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d: tags = %v, want stable %v", i, got, want)
		}
	}
	if !reflect.DeepEqual(labels, before) {
		t.Fatal("tag conversion mutated source labels")
	}
}

func TestLabelTagsRetainExistingValueSemantics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{"nil", nil, nil},
		{"empty", map[string]string{}, nil},
		{"blank-key", map[string]string{" ": "ignored"}, []string{}},
		{"bare-keys", map[string]string{" z ": "\t", "a": ""}, []string{"a", "z"}},
		{"deduplicated-trimmed-keys", map[string]string{" a": "v", "a ": "v"}, []string{"a:v"}},
		{"case-colon-and-value-spaces", map[string]string{"Z": "https://example.invalid/a:b", "a": " Mixed Case "}, []string{"Z:https://example.invalid/a:b", "a: Mixed Case"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := labelsToTags(tc.labels); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("tags = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestLabelTagsDoNotSortOrderedIdentityInputs(t *testing.T) {
	values := []string{" z ", "a", "z", "b", ""}
	before := append([]string(nil), values...)
	if got := uniqueStrings(values); !reflect.DeepEqual(got, []string{"z", "a", "b"}) {
		t.Fatalf("first-occurrence order changed: %v", got)
	}
	if !reflect.DeepEqual(values, before) {
		t.Fatal("deduplication changed the input list")
	}

	image, identity := resourceFromDockerImage(models.DockerImage{
		ID: "sha256:example", RepoTags: []string{"z:last", "a:first"},
		Labels: map[string]string{"z": "last", "a": "first"},
	}, models.DockerHost{Hostname: "runtime-host"})
	if image.Name != "z:last" || !reflect.DeepEqual(image.Docker.RepoTags, []string{"z:last", "a:first"}) ||
		!reflect.DeepEqual(identity.Hostnames, []string{"z:last", "runtime-host:z:last"}) {
		t.Fatalf("label sorting reordered image references or identity: %+v / %+v", image, identity)
	}
}
