package models

// SnapshotProvider provides a current state snapshot.
//
// This is the canonical, shared interface for any component that provides
// infrastructure state snapshots. All packages should depend on this single
// definition rather than declaring their own identical interface.
//
// The monitoring.Monitor type satisfies this interface via its ReadSnapshot() method.
type SnapshotProvider interface {
	ReadSnapshot() StateSnapshot
}
