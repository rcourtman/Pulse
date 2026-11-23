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

func decodeWithCounterPresence(data []byte, decode func([]byte, IOCounterPresence) error) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return decode(data, counterPresence(raw))
}

func (v *VM) UnmarshalJSON(data []byte) error {
	return decodeWithCounterPresence(data, v.unmarshalWithKnownCounters)
}

func (v *VM) unmarshalWithKnownCounters(data []byte, presence IOCounterPresence) error {
	type alias VM
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.IOCounters = presence
	*v = VM(decoded)
	return nil
}

func (c *Container) UnmarshalJSON(data []byte) error {
	return decodeWithCounterPresence(data, c.unmarshalWithKnownCounters)
}

func (c *Container) unmarshalWithKnownCounters(data []byte, presence IOCounterPresence) error {
	type alias Container
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.IOCounters = presence
	*c = Container(decoded)
	return nil
}

func (r *ClusterResource) UnmarshalJSON(data []byte) error {
	return decodeWithCounterPresence(data, r.unmarshalWithKnownCounters)
}

func (r *ClusterResource) unmarshalWithKnownCounters(data []byte, presence IOCounterPresence) error {
	type alias ClusterResource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.IOCounters = presence
	*r = ClusterResource(decoded)
	return nil
}

func (s *VMStatus) UnmarshalJSON(data []byte) error {
	return decodeWithCounterPresence(data, s.unmarshalWithKnownCounters)
}

func (s *VMStatus) unmarshalWithKnownCounters(data []byte, presence IOCounterPresence) error {
	type alias VMStatus
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.IOCounters = presence
	*s = VMStatus(decoded)
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
