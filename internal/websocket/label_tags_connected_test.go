package websocket_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// These are source-native inventories, not pre-sorted ResourceFrontend fixtures.
// Every label-derived Docker adapter and Kubernetes kind passes through a real
// registry generation, monitor projection, wire snapshot and client delta queue.
func labelTagInventory(now time.Time, labels map[string]string) models.StateSnapshot {
	cluster := models.KubernetesCluster{ID: "label-cluster", Name: "label-cluster", Status: "online", LastSeen: now}
	// The Kubernetes inventory has many kinds with the same UID/name/labels
	// envelope. Include each one declared by models; the explicit count below
	// prevents an empty or partial fixture from silently satisfying the test.
	value := reflect.ValueOf(&cluster).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() != reflect.Slice || field.Type().Elem().Kind() != reflect.Struct {
			continue
		}
		entry := reflect.New(field.Type().Elem()).Elem()
		labelField := entry.FieldByName("Labels")
		if !labelField.IsValid() {
			continue
		}
		name := "label-" + strings.ToLower(value.Type().Field(i).Name)
		for key, text := range map[string]string{"UID": "uid-" + name, "Name": name, "Namespace": "workloads", "Phase": "Running"} {
			if target := entry.FieldByName(key); target.IsValid() && target.Kind() == reflect.String {
				target.SetString(text)
			}
		}
		if value.Type().Field(i).Name == "Namespaces" {
			entry.FieldByName("Name").SetString("workloads")
			entry.FieldByName("Phase").SetString("Active")
		}
		labelField.Set(reflect.ValueOf(labels))
		field.Set(reflect.Append(field, entry))
	}
	return models.StateSnapshot{
		LastUpdate: now,
		DockerHosts: []models.DockerHost{{
			ID: "label-docker", Hostname: "label-docker", Status: "online", LastSeen: now,
			Swarm:    &models.DockerSwarmInfo{ClusterID: "label-swarm", ClusterName: "label-swarm"},
			Images:   []models.DockerImage{{ID: "sha256:label-image", RepoTags: []string{"z:latest", "a:old"}, Labels: labels}},
			Volumes:  []models.DockerVolume{{Name: "label-volume", Labels: labels}},
			Networks: []models.DockerNetwork{{ID: "label-network", Name: "label-network", Labels: labels}},
			Services: []models.DockerService{{ID: "label-service", Name: "label-service", Labels: labels}},
			Nodes:    []models.DockerNode{{ID: "label-node", Hostname: "swarm-node", Labels: labels}},
			Secrets:  []models.DockerSecret{{ID: "label-secret", Name: "label-secret", Labels: labels}},
			Configs:  []models.DockerConfig{{ID: "label-config", Name: "label-config", Labels: labels}},
		}},
		KubernetesClusters: []models.KubernetesCluster{cluster},
	}
}

func labelTagValues() map[string]string {
	return map[string]string{"app": "api", "env": "production", "team": "ops", "tier": "backend", "region": "west", "release": "stable", "managed": "yes", "version": "1"}
}

type labelTagProjection struct {
	adapter *unifiedresources.MonitorAdapter
	store   *unifiedresources.MemoryStore
	monitor *monitoring.Monitor
}

func newLabelTagProjection(snapshot models.StateSnapshot) *labelTagProjection {
	store := unifiedresources.NewMemoryStore()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	adapter.PopulateFromSnapshot(snapshot)
	monitor := &monitoring.Monitor{}
	// Completed ingest is the baseline: a dashboard read must not replace it
	// with this test Monitor's empty live polling state.
	monitor.SetResourceStore(&broadcastReadStore{adapter})
	return &labelTagProjection{adapter: adapter, store: store, monitor: monitor}
}

func (p *labelTagProjection) state() models.StateFrontend {
	return p.monitor.BuildBroadcastFrontendState()
}

func (p *labelTagProjection) changes(t *testing.T) int {
	t.Helper()
	count, err := p.store.CountRecentChanges("", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

type labelTagWireDelta struct {
	Upserts []map[string]any `json:"upserts"`
	Removed []string         `json:"removed"`
	Order   []string         `json:"order"`
}

func labelTagFrameDelta(t *testing.T, frame []byte) labelTagWireDelta {
	t.Helper()
	if len(frame) == 0 {
		return labelTagWireDelta{}
	}
	var message struct {
		Type string                     `json:"type"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(frame, &message); err != nil || message.Type != "rawData" {
		t.Fatalf("invalid state frame: %s / %v", frame, err)
	}
	if _, full := message.Data["resources"]; full {
		t.Fatal("connected client unexpectedly received a full resource array")
	}
	var delta labelTagWireDelta
	if encoded, ok := message.Data["resourceDelta"]; ok {
		if err := json.Unmarshal(encoded, &delta); err != nil {
			t.Fatal(err)
		}
	}
	return delta
}

func labelTagResourceMaps(t *testing.T, state models.StateFrontend) map[string]map[string]any {
	t.Helper()
	encoded, err := json.Marshal(state.Resources)
	if err != nil {
		t.Fatal(err)
	}
	var resources []map[string]any
	if err := json.Unmarshal(encoded, &resources); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]map[string]any, len(resources))
	for _, resource := range resources {
		out[resource["id"].(string)] = resource
	}
	return out
}

// Independent RFC 7396 receiver oracle; do not reuse the production diff builder.
func applyLabelTagMergePatch(previous map[string]any, patch map[string]any) map[string]any {
	out := maps.Clone(previous)
	if out == nil {
		out = make(map[string]any)
	}
	for key, value := range patch {
		if value == nil {
			delete(out, key)
		} else if object, ok := value.(map[string]any); ok {
			old, _ := out[key].(map[string]any)
			out[key] = applyLabelTagMergePatch(old, object)
		} else {
			out[key] = value
		}
	}
	return out
}

func TestLabelTagsConnectedRebuildsAreQuiet(t *testing.T) {
	labels := labelTagValues()
	before := maps.Clone(labels)
	seed := labelTagInventory(time.Now().UTC(), labels)
	projection := newLabelTagProjection(seed)
	tagged := 0
	for _, resource := range projection.state().Resources {
		if len(resource.Tags) == len(labels) {
			tagged++
		}
	}
	if tagged != 33 { // Seven Docker adapters and twenty-six Kubernetes kinds.
		t.Fatalf("fixture covers %d label-derived resources, want 33", tagged)
	}
	broadcast, err := websocket.NewBroadcastProjectionCaptureForTest(func(string) interface{} { return projection.state() }, []string{"default", "default"})
	if err != nil {
		t.Fatal(err)
	}
	changes := projection.changes(t)
	var tagPatches, upserts, wireBytes int
	for i := 0; i < 32; i++ {
		projection.adapter.PopulateFromSnapshot(seed)
		frames, err := broadcast("default")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(frames[0], frames[1]) {
			t.Fatal("equal client baselines received different frames")
		}
		for _, frame := range frames {
			wireBytes += len(frame)
			delta := labelTagFrameDelta(t, frame)
			if len(delta.Removed) != 0 || len(delta.Order) != 0 {
				t.Fatal("equivalent rebuild changed resource identity or order")
			}
			upserts += len(delta.Upserts)
			for _, patch := range delta.Upserts {
				if _, ok := patch["tags"]; ok {
					tagPatches++
				}
			}
		}
	}
	t.Logf("33 label-derived resources, 32 rebuilds, two clients: tag patches=%d, all resource upserts=%d, queued wire bytes=%d", tagPatches, upserts, wireBytes)
	if tagPatches != 0 || upserts != 0 {
		t.Fatalf("unchanged label sets created %d tag patches / %d resource upserts", tagPatches, upserts)
	}
	if projection.changes(t) != changes || !reflect.DeepEqual(labels, before) {
		t.Fatal("equivalent rebuild changed history or mutated source labels")
	}
}

func TestLabelTagsConnectedChangesAndClearsRemainVisible(t *testing.T) {
	labels := labelTagValues()
	now := time.Now().UTC()
	projection := newLabelTagProjection(labelTagInventory(now, labels))
	otherTenant := newLabelTagProjection(labelTagInventory(now, map[string]string{"private": "other-tenant"}))
	broadcast, err := websocket.NewBroadcastProjectionCaptureForTest(func(org string) interface{} {
		if org == "other" {
			return otherTenant.state()
		}
		return projection.state()
	}, []string{"default", "default", "other"})
	if err != nil {
		t.Fatal(err)
	}
	received := labelTagResourceMaps(t, projection.state())
	changes := projection.changes(t)
	for _, name := range []string{"replace", "add", "remove", "clear", "restore"} {
		t.Run(name, func(t *testing.T) {
			labels = maps.Clone(labels)
			switch name {
			case "replace":
				labels["env"] = "staging"
			case "add":
				labels["new"] = "visible"
			case "remove":
				delete(labels, "app")
			case "clear":
				labels = nil
			case "restore":
				labels = labelTagValues()
			}
			projection.adapter.PopulateFromSnapshot(labelTagInventory(now, labels))
			frames, err := broadcast("default")
			if err != nil {
				t.Fatal(err)
			}
			if len(frames[0]) == 0 || !bytes.Equal(frames[0], frames[1]) || len(frames[2]) != 0 {
				t.Fatal("change was lost, differed per client or reached another tenant")
			}
			delta := labelTagFrameDelta(t, frames[0])
			if len(delta.Upserts) != 33 || len(delta.Removed) != 0 || len(delta.Order) != 0 {
				t.Fatalf("metadata edit changed identity/order or missed a resource: %+v", delta)
			}
			for _, patch := range delta.Upserts {
				if _, changed := patch["tags"]; !changed {
					t.Fatalf("real label change omitted tags: %v", patch)
				}
				id := patch["id"].(string)
				received[id] = applyLabelTagMergePatch(received[id], patch)
			}
			want := labelTagResourceMaps(t, projection.state())
			if !reflect.DeepEqual(received, want) {
				t.Fatal("receiver replay differs from the full current projection (including labels/tags clears)")
			}
			for _, resource := range projection.state().Resources {
				if !slices.IsSorted(resource.Tags) {
					t.Fatalf("changed tags are not sorted for %s: %v", resource.ID, resource.Tags)
				}
				if name == "clear" && (len(resource.Tags) != 0 || len(resource.Labels) != 0) {
					t.Fatalf("cleared metadata remained on %s", resource.ID)
				}
			}
			changes += 33
			if got := projection.changes(t); got != changes {
				t.Fatalf("history lost or duplicated real changes: %d, want %d", got, changes)
			}
		})
	}
}

func TestLabelTagsFleetHeartbeatDeltas(t *testing.T) {
	now := time.Now().UTC()
	labels := labelTagValues()
	seed := models.StateSnapshot{LastUpdate: now, KubernetesClusters: []models.KubernetesCluster{{ID: "fleet", Name: "fleet", Status: "online", LastSeen: now}}}
	for i := 0; i < 1000; i++ {
		seed.KubernetesClusters[0].Pods = append(seed.KubernetesClusters[0].Pods, models.KubernetesPod{
			UID: fmt.Sprintf("pod-%04d", i), Name: fmt.Sprintf("pod-%04d", i), Namespace: "workloads", Phase: "Running", Labels: labels,
		})
	}
	projection := newLabelTagProjection(seed)
	broadcast, err := websocket.NewBroadcastProjectionCaptureForTest(func(string) interface{} { return projection.state() }, []string{"default"})
	if err != nil {
		t.Fatal(err)
	}
	var tagPatches, wireBytes int
	for i := 0; i < 8; i++ {
		// Advance observed freshness as a real heartbeat does, not labels.
		seed.LastUpdate = now.Add(time.Duration(i+1) * time.Second)
		seed.KubernetesClusters[0].LastSeen = seed.LastUpdate
		projection.adapter.PopulateFromSnapshot(seed)
		frames, err := broadcast("default")
		if err != nil || len(frames[0]) == 0 {
			t.Fatalf("fresh heartbeat was not queued: %v", err)
		}
		wireBytes += len(frames[0])
		for _, patch := range labelTagFrameDelta(t, frames[0]).Upserts {
			if _, changed := patch["tags"]; changed {
				tagPatches++
			}
			if _, changed := patch["labels"]; changed {
				t.Fatal("heartbeat changed label values")
			}
		}
	}
	t.Logf("1,000 pods, eight fresh heartbeats, one client: false tag patches=%d, queued wire bytes=%d (includes real freshness deltas)", tagPatches, wireBytes)
	if tagPatches != 0 {
		t.Fatalf("unchanged fleet labels created %d tag patches", tagPatches)
	}
}
