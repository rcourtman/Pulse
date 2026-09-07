// Package filesystem defines source-authored filesystem observations shared by
// agent reports and canonical resource projections.
package filesystem

import "time"

// Observation describes the filesystem visible at one mountpoint in the
// observed resource's namespace. Shared filesystems are not resource quotas.
// A failed observation has no Usage. Missing evidence never implies zero usage.
type Observation struct {
	Mountpoint string    `json:"mountpoint"`
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observedAt"`
	Type       string    `json:"type,omitempty"`
	Usage      *Usage    `json:"usage,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// Usage preserves the kernel counters, including observed zero. AvailableBytes
// excludes blocks reserved from ordinary users. FreeBytes includes them.
// Inodes is absent when the filesystem did not report a finite inode inventory.
type Usage struct {
	CapacityBytes  uint64      `json:"capacityBytes"`
	FreeBytes      uint64      `json:"freeBytes"`
	AvailableBytes uint64      `json:"availableBytes"`
	Inodes         *InodeUsage `json:"inodes,omitempty"`
}

type InodeUsage struct {
	Capacity uint64 `json:"capacity"`
	Free     uint64 `json:"free"`
}

// Clone prevents a projected or retained observation from sharing mutable
// measurement storage with its source report.
func Clone(in []Observation) []Observation {
	if in == nil {
		return nil
	}
	out := make([]Observation, len(in))
	copy(out, in)
	for i := range out {
		if in[i].Usage != nil {
			usage := *in[i].Usage
			if usage.Inodes != nil {
				inodes := *usage.Inodes
				usage.Inodes = &inodes
			}
			out[i].Usage = &usage
		}
	}
	return out
}
