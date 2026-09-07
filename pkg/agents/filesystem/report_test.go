package filesystem

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestObservationDistinguishesUnavailableFromExhausted(t *testing.T) {
	unavailable, err := json.Marshal(Observation{Mountpoint: "/cache", Error: "namespace unavailable"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unavailable), "capacityBytes") || strings.Contains(string(unavailable), "usage") {
		t.Fatal(string(unavailable))
	}
	full, err := json.Marshal(Observation{Mountpoint: "/cache", Usage: &Usage{CapacityBytes: 8 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(full), `"availableBytes":0`) || !strings.Contains(string(full), `"freeBytes":0`) {
		t.Fatal(string(full))
	}
	if strings.Contains(string(full), `"inodes"`) {
		t.Fatal("unknown inode inventory became observed zero")
	}
}

func TestCloneOwnsMeasurements(t *testing.T) {
	in := []Observation{{Usage: &Usage{CapacityBytes: 123, Inodes: &InodeUsage{Capacity: 456}}}, {Error: "unavailable"}}
	out := Clone(in)
	out[0].Usage.CapacityBytes = 999
	out[0].Usage.Inodes.Capacity = 999
	if in[0].Usage.CapacityBytes != 123 || in[0].Usage.Inodes.Capacity != 456 || out[1].Usage != nil {
		t.Fatal("cloned observations alias source")
	}
	if Clone(nil) != nil {
		t.Fatal("nil evidence became present")
	}
}
