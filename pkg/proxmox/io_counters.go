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

// DiskUsagePresence keeps a measured empty filesystem distinct from an omitted
// or null usage/capacity field. Legacy in-process producers remain compatible.
type DiskUsagePresence struct {
	Explicit bool
	Used     bool
	Total    bool
}

func (p DiskUsagePresence) Known() bool {
	return !p.Explicit || (p.Used && p.Total)
}

func decodeWithCounterPresence(data []byte, target any) (IOCounterPresence, DiskUsagePresence, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return IOCounterPresence{}, DiskUsagePresence{}, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return IOCounterPresence{}, DiskUsagePresence{}, err
	}
	return counterPresence(raw), DiskUsagePresence{Explicit: true, Used: jsonFieldPresent(raw, "disk"), Total: jsonFieldPresent(raw, "maxdisk")}, nil
}

// CPU uses -1 for an omitted/null observation. encoding/json leaves that
// initial value unchanged for null; a measured zero still overwrites it.
// Keep the same marker through node inventories, cluster listings and detail.
func (v *VM) UnmarshalJSON(data []byte) error {
	type alias VM
	decoded := alias{CPU: -1}
	presence, _, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*v = VM(decoded)
	v.IOCounters = presence
	return nil
}

func (c *Container) UnmarshalJSON(data []byte) error {
	type alias Container
	decoded := alias{CPU: -1}
	presence, diskPresence, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*c = Container(decoded)
	c.IOCounters = presence
	c.DiskUsage = diskPresence
	return nil
}

func (d *ContainerDiskUsage) UnmarshalJSON(data []byte) error {
	type alias ContainerDiskUsage
	var decoded alias
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*d = ContainerDiskUsage(decoded)
	d.DiskUsage = DiskUsagePresence{Explicit: true, Used: jsonFieldPresent(raw, "used"), Total: jsonFieldPresent(raw, "total")}
	return nil
}

func (r *ClusterResource) UnmarshalJSON(data []byte) error {
	type alias ClusterResource
	decoded := alias{CPU: -1}
	presence, diskPresence, err := decodeWithCounterPresence(data, &decoded)
	if err != nil {
		return err
	}
	*r = ClusterResource(decoded)
	r.IOCounters = presence
	r.DiskUsage = diskPresence
	return nil
}

func (s *VMStatus) UnmarshalJSON(data []byte) error {
	type alias VMStatus
	decoded := alias{CPU: -1}
	presence, _, err := decodeWithCounterPresence(data, &decoded)
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
