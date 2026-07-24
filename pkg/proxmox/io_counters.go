package proxmox

import (
	"bytes"
	"encoding/json"
	"time"
)

// IOCounterPresence distinguishes explicit zero counters from fields omitted
// by a Proxmox endpoint or permission-limited response.
type IOCounterPresence struct {
	Explicit   bool
	DiskRead   bool
	DiskWrite  bool
	NetworkIn  bool
	NetworkOut bool
}

// Effective keeps manually constructed fixtures and older producers
// compatible while decoded API responses retain exact field presence.
func (p IOCounterPresence) Effective() IOCounterPresence {
	if p.Explicit {
		return p
	}
	return IOCounterPresence{
		Explicit:   true,
		DiskRead:   true,
		DiskWrite:  true,
		NetworkIn:  true,
		NetworkOut: true,
	}
}

func counterPresence(raw map[string]json.RawMessage) IOCounterPresence {
	return IOCounterPresence{
		Explicit:   true,
		DiskRead:   jsonFieldPresent(raw, "diskread"),
		DiskWrite:  jsonFieldPresent(raw, "diskwrite"),
		NetworkIn:  jsonFieldPresent(raw, "netin"),
		NetworkOut: jsonFieldPresent(raw, "netout"),
	}
}

func jsonFieldPresent(raw map[string]json.RawMessage, key string) bool {
	value, ok := raw[key]
	return ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func decodeWithCounterPresence(data []byte, target any) (IOCounterPresence, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return IOCounterPresence{}, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return IOCounterPresence{}, err
	}
	return counterPresence(raw), nil
}

func (v *VM) UnmarshalJSON(data []byte) error {
	type alias VM
	var decoded alias
	presence, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*v = VM(decoded)
	v.IOCounters = presence
	return nil
}

func (c *Container) UnmarshalJSON(data []byte) error {
	type alias Container
	var decoded alias
	presence, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*c = Container(decoded)
	c.IOCounters = presence
	return nil
}

func (r *ClusterResource) UnmarshalJSON(data []byte) error {
	type alias ClusterResource
	var decoded alias
	presence, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*r = ClusterResource(decoded)
	r.IOCounters = presence
	return nil
}

func (s *VMStatus) UnmarshalJSON(data []byte) error {
	type alias VMStatus
	var decoded alias
	presence, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*s = VMStatus(decoded)
	s.IOCounters = presence
	return nil
}

func stampVMObservation(values []VM, observedAt time.Time) {
	for i := range values {
		values[i].ObservedAt = observedAt
	}
}

func stampContainerObservation(values []Container, observedAt time.Time) {
	for i := range values {
		values[i].ObservedAt = observedAt
	}
}

func stampClusterResourceObservation(values []ClusterResource, observedAt time.Time) {
	for i := range values {
		values[i].ObservedAt = observedAt
	}
}
