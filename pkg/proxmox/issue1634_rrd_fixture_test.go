package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// Issue #1634: LXC memory rendered as 0% because GuestRRDPoint declared
// memused/memavailable columns that real PVE guest rrddata responses never
// contain (they exist only in node RRD). Every test mocked the fictional
// columns, so CI validated the assumption instead of the API. The tests in
// this file decode recorded responses (see testdata/rrd/README.md) through
// the real client and fail whenever a parsing struct references a column
// absent from every recorded response.

func rrdFixturePath(name string) string {
	return filepath.Join("testdata", "rrd", name)
}

// rrdFixtureColumns returns the union of column names across all rows of a
// recorded fixture.
func rrdFixtureColumns(t *testing.T, name string) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(rrdFixturePath(name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	var envelope struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	if len(envelope.Data) == 0 {
		t.Fatalf("fixture %s has no rows", name)
	}

	columns := make(map[string]bool)
	for _, row := range envelope.Data {
		for key := range row {
			columns[key] = true
		}
	}
	return columns
}

// jsonFieldTags returns the JSON key for every exported field of a struct
// type, with tag options such as ",omitempty" stripped.
func jsonFieldTags(t *testing.T, typ reflect.Type) []string {
	t.Helper()

	var tags []string
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s.%s has no json tag; RRD point fields must map to API columns", typ.Name(), typ.Field(i).Name)
		}
		if comma := indexByte(tag, ','); comma >= 0 {
			tag = tag[:comma]
		}
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// newRRDFixtureClient serves each fixture at its real API path so the
// recorded bytes travel through the actual client request/decode path.
func newRRDFixtureClient(t *testing.T) *Client {
	t.Helper()

	fixtureByPath := map[string]string{
		"/api2/json/nodes/pve9/lxc/108/rrddata":  "pve9_lxc_rrddata.json",
		"/api2/json/nodes/pve9/qemu/100/rrddata": "pve9_qemu_rrddata.json",
		"/api2/json/nodes/pve9/rrddata":          "pve9_node_rrddata.json",
		"/api2/json/nodes/pve8/lxc/101/rrddata":  "pve8_guest_rrddata.json",
		"/api2/json/nodes/pve8/qemu/100/rrddata": "pve8_guest_rrddata.json",
		"/api2/json/nodes/pve8/rrddata":          "pve8_node_rrddata.json",
	}

	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fixture, ok := fixtureByPath[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		raw, err := os.ReadFile(rrdFixturePath(fixture))
		if err != nil {
			t.Errorf("read fixture %s: %v", fixture, err)
			http.Error(w, "missing fixture", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(raw); err != nil {
			t.Errorf("write fixture %s: %v", fixture, err)
		}
	})
}

func TestGuestRRDPointColumnsExistInRecordedResponses(t *testing.T) {
	recorded := make(map[string]bool)
	for _, fixture := range []string{"pve9_lxc_rrddata.json", "pve9_qemu_rrddata.json", "pve8_guest_rrddata.json"} {
		for column := range rrdFixtureColumns(t, fixture) {
			recorded[column] = true
		}
	}

	for _, tag := range jsonFieldTags(t, reflect.TypeOf(GuestRRDPoint{})) {
		if !recorded[tag] {
			t.Errorf("GuestRRDPoint field %q is absent from every recorded guest rrddata response; do not parse columns the PVE API does not send (#1634)", tag)
		}
	}
}

func TestNodeRRDPointColumnsExistInRecordedResponses(t *testing.T) {
	recorded := make(map[string]bool)
	for _, fixture := range []string{"pve9_node_rrddata.json", "pve8_node_rrddata.json"} {
		for column := range rrdFixtureColumns(t, fixture) {
			recorded[column] = true
		}
	}

	for _, tag := range jsonFieldTags(t, reflect.TypeOf(NodeRRDPoint{})) {
		if !recorded[tag] {
			t.Errorf("NodeRRDPoint field %q is absent from every recorded node rrddata response; do not parse columns the PVE API does not send (#1634)", tag)
		}
	}
}

func TestGuestRRDDecodeAgainstRecordedResponses(t *testing.T) {
	client := newRRDFixtureClient(t)
	ctx := context.Background()

	fetches := []struct {
		name  string
		fetch func() ([]GuestRRDPoint, error)
	}{
		{"pve9 lxc", func() ([]GuestRRDPoint, error) { return client.GetLXCRRDData(ctx, "pve9", 108, "hour", "AVERAGE", nil) }},
		{"pve9 qemu", func() ([]GuestRRDPoint, error) { return client.GetVMRRDData(ctx, "pve9", 100, "hour", "AVERAGE", nil) }},
		{"pve8 lxc", func() ([]GuestRRDPoint, error) { return client.GetLXCRRDData(ctx, "pve8", 101, "hour", "AVERAGE", nil) }},
		{"pve8 qemu", func() ([]GuestRRDPoint, error) { return client.GetVMRRDData(ctx, "pve8", 100, "hour", "AVERAGE", nil) }},
	}

	for _, tc := range fetches {
		t.Run(tc.name, func(t *testing.T) {
			points, err := tc.fetch()
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if len(points) == 0 {
				t.Fatal("no points decoded from recorded response")
			}

			var sawMaxMem bool
			for _, p := range points {
				if p.Time == 0 {
					t.Fatalf("point decoded without time: %+v", p)
				}
				if p.MaxMem != nil && *p.MaxMem > 0 {
					sawMaxMem = true
				}
			}
			if !sawMaxMem {
				t.Fatal("no point carried maxmem; fixture should include populated rows")
			}
		})
	}
}

func TestNodeRRDDecodeAgainstRecordedResponses(t *testing.T) {
	client := newRRDFixtureClient(t)
	ctx := context.Background()

	t.Run("pve9", func(t *testing.T) {
		points, err := client.GetNodeRRDData(ctx, "pve9", "hour", "AVERAGE", nil)
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(points) == 0 {
			t.Fatal("no points decoded from recorded response")
		}
		var sawTotal, sawUsed, sawAvail, sawNetIn, sawNetOut bool
		for _, p := range points {
			sawTotal = sawTotal || (p.MemTotal != nil && *p.MemTotal > 0)
			sawUsed = sawUsed || (p.MemUsed != nil && *p.MemUsed > 0)
			sawAvail = sawAvail || (p.MemAvailable != nil && *p.MemAvailable > 0)
			sawNetIn = sawNetIn || p.NetIn != nil
			sawNetOut = sawNetOut || p.NetOut != nil
		}
		if !sawTotal || !sawUsed || !sawAvail || !sawNetIn || !sawNetOut {
			t.Fatalf("PVE 9 node RRD should populate every NodeRRDPoint field (total=%v used=%v avail=%v netin=%v netout=%v)",
				sawTotal, sawUsed, sawAvail, sawNetIn, sawNetOut)
		}
	})

	t.Run("pve8", func(t *testing.T) {
		points, err := client.GetNodeRRDData(ctx, "pve8", "hour", "AVERAGE", nil)
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if len(points) == 0 {
			t.Fatal("no points decoded from recorded response")
		}
		var sawTotal, sawUsed bool
		for _, p := range points {
			sawTotal = sawTotal || (p.MemTotal != nil && *p.MemTotal > 0)
			sawUsed = sawUsed || (p.MemUsed != nil && *p.MemUsed > 0)
			// memavailable was added in PVE 9; PVE 8 node RRD never
			// carries it, which is why the field must stay optional.
			if p.MemAvailable != nil {
				t.Fatalf("MemAvailable decoded from a recorded PVE 8 node response (time=%d)", p.Time)
			}
		}
		if !sawTotal || !sawUsed {
			t.Fatalf("PVE 8 node RRD should populate memtotal/memused (total=%v used=%v)", sawTotal, sawUsed)
		}
	})
}
