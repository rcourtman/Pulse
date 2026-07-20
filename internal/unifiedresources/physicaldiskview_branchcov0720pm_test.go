package unifiedresources

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

// TestPhysicalDiskViewBranchcov0720pm covers the five PhysicalDiskView
// accessors that were previously 0% covered:
//   - Controller() string
//   - Target() string
//   - StorageGroup() string
//   - IO() *PhysicalDiskIOMeta
//   - Collection() *diskinventory.CollectionStatus
//
// Each accessor exercises three arms where they exist:
//
//	(a) nil-receiver and nil-backing (v.r == nil, then v.r != nil but
//	    v.r.PhysicalDisk == nil) — must be nil/""-safe, no panic;
//	(b) populated backing but the nested field empty/nil (whitespace-only
//	    strings trim to ""; IO()/Collection() inner pointers nil → nil);
//	(c) fully populated — assert the exact returned value, and for the
//	    pointer-returning accessors assert the clone is field-equal and
//	    mutation-independent from the live source.
func TestPhysicalDiskViewBranchcov0720pm(t *testing.T) {
	t.Run("Controller", func(t *testing.T) {
		// (a) nil receiver.
		var zero PhysicalDiskView
		if got := zero.Controller(); got != "" {
			t.Fatalf("nil receiver: expected %q, got %q", "", got)
		}
		// (a) populated Resource, nil PhysicalDisk backing.
		r := &Resource{ID: "pd-ctrl-1", Type: ResourceTypePhysicalDisk}
		if got := NewPhysicalDiskView(r).Controller(); got != "" {
			t.Fatalf("nil PhysicalDisk: expected %q, got %q", "", got)
		}
		// (b) PhysicalDisk present, Controller whitespace-only → trims to "".
		r.PhysicalDisk = &PhysicalDiskMeta{Controller: "   "}
		if got := NewPhysicalDiskView(r).Controller(); got != "" {
			t.Fatalf("whitespace Controller: expected trimmed %q, got %q", "", got)
		}
		// (c) Fully populated → trimmed exact value.
		r.PhysicalDisk.Controller = " mega-raid-0 "
		if got, want := NewPhysicalDiskView(r).Controller(), "mega-raid-0"; got != want {
			t.Fatalf("expected Controller %q, got %q", want, got)
		}
	})

	t.Run("Target", func(t *testing.T) {
		// (a) nil receiver.
		var zero PhysicalDiskView
		if got := zero.Target(); got != "" {
			t.Fatalf("nil receiver: expected %q, got %q", "", got)
		}
		// (a) populated Resource, nil PhysicalDisk backing.
		r := &Resource{ID: "pd-tgt-1", Type: ResourceTypePhysicalDisk}
		if got := NewPhysicalDiskView(r).Target(); got != "" {
			t.Fatalf("nil PhysicalDisk: expected %q, got %q", "", got)
		}
		// (b) PhysicalDisk present, Target whitespace-only → trims to "".
		r.PhysicalDisk = &PhysicalDiskMeta{Target: "  "}
		if got := NewPhysicalDiskView(r).Target(); got != "" {
			t.Fatalf("whitespace Target: expected trimmed %q, got %q", "", got)
		}
		// (c) Fully populated → trimmed exact value.
		r.PhysicalDisk.Target = " 0:0:0:0 "
		if got, want := NewPhysicalDiskView(r).Target(), "0:0:0:0"; got != want {
			t.Fatalf("expected Target %q, got %q", want, got)
		}
	})

	t.Run("StorageGroup", func(t *testing.T) {
		// (a) nil receiver.
		var zero PhysicalDiskView
		if got := zero.StorageGroup(); got != "" {
			t.Fatalf("nil receiver: expected %q, got %q", "", got)
		}
		// (a) populated Resource, nil PhysicalDisk backing.
		r := &Resource{ID: "pd-sg-1", Type: ResourceTypePhysicalDisk}
		if got := NewPhysicalDiskView(r).StorageGroup(); got != "" {
			t.Fatalf("nil PhysicalDisk: expected %q, got %q", "", got)
		}
		// (b) PhysicalDisk present, StorageGroup whitespace-only → trims to "".
		r.PhysicalDisk = &PhysicalDiskMeta{StorageGroup: "\t "}
		if got := NewPhysicalDiskView(r).StorageGroup(); got != "" {
			t.Fatalf("whitespace StorageGroup: expected trimmed %q, got %q", "", got)
		}
		// (c) Fully populated → trimmed exact value.
		r.PhysicalDisk.StorageGroup = " sg-fast "
		if got, want := NewPhysicalDiskView(r).StorageGroup(), "sg-fast"; got != want {
			t.Fatalf("expected StorageGroup %q, got %q", want, got)
		}
	})

	t.Run("IO", func(t *testing.T) {
		// (a) nil receiver.
		var zero PhysicalDiskView
		if got := zero.IO(); got != nil {
			t.Fatalf("nil receiver: expected nil, got %+v", got)
		}
		// (a) populated Resource, nil PhysicalDisk backing.
		r := &Resource{ID: "pd-io-1", Type: ResourceTypePhysicalDisk}
		if got := NewPhysicalDiskView(r).IO(); got != nil {
			t.Fatalf("nil PhysicalDisk: expected nil, got %+v", got)
		}
		// (b) PhysicalDisk present, IO pointer nil → accessor returns nil.
		r.PhysicalDisk = &PhysicalDiskMeta{} // IO is nil
		if got := NewPhysicalDiskView(r).IO(); got != nil {
			t.Fatalf("nil IO: expected nil, got %+v", got)
		}
		// (c) Fully populated → field-equal clone on a fresh allocation.
		want := &PhysicalDiskIOMeta{
			Device:      "sda",
			ReadBytes:   100,
			WriteBytes:  200,
			ReadOps:     1,
			WriteOps:    2,
			ReadTimeMs:  3,
			WriteTimeMs: 4,
			IOTimeMs:    5,
		}
		r.PhysicalDisk.IO = want
		got := NewPhysicalDiskView(r).IO()
		if got == nil {
			t.Fatal("populated: expected non-nil")
		}
		if !reflect.DeepEqual(*got, *want) {
			t.Fatalf("expected field-equal clone, got %+v want %+v", *got, *want)
		}
		// Returned pointer must be a fresh allocation, not the live source.
		if got == r.PhysicalDisk.IO {
			t.Fatal("expected IO() to return a fresh pointer, not the source pointer")
		}
		// Mutating the clone must not leak back to the source.
		got.ReadBytes = 9999
		got.Device = "mutated"
		if r.PhysicalDisk.IO.ReadBytes != 100 || r.PhysicalDisk.IO.Device != "sda" {
			t.Fatalf("mutation leaked to source: got %+v", *r.PhysicalDisk.IO)
		}
	})

	t.Run("Collection", func(t *testing.T) {
		// (a) nil receiver.
		var zero PhysicalDiskView
		if got := zero.Collection(); got != nil {
			t.Fatalf("nil receiver: expected nil, got %+v", got)
		}
		// (a) populated Resource, nil PhysicalDisk backing.
		r := &Resource{ID: "pd-coll-1", Type: ResourceTypePhysicalDisk}
		if got := NewPhysicalDiskView(r).Collection(); got != nil {
			t.Fatalf("nil PhysicalDisk: expected nil, got %+v", got)
		}
		// (b) PhysicalDisk present, Collection pointer nil → accessor returns nil
		//     (diskinventory.CloneStatus(nil) returns nil).
		r.PhysicalDisk = &PhysicalDiskMeta{} // Collection is nil
		if got := NewPhysicalDiskView(r).Collection(); got != nil {
			t.Fatalf("nil Collection: expected nil, got %+v", got)
		}
		// (c) Fully populated → field-equal clone on a fresh allocation.
		want := &diskinventory.CollectionStatus{
			Serial:      diskinventory.Available("smartctl"),
			Temperature: diskinventory.Unavailable("smartctl", "no sensor"),
			IO:          diskinventory.FieldStatus{State: diskinventory.FieldAvailable, Source: "sysfs"},
			Controller:  diskinventory.Unsupported("nvme", "n/a"),
			Pool:        diskinventory.Missing("zfs", "not imported"),
		}
		r.PhysicalDisk.Collection = want
		got := NewPhysicalDiskView(r).Collection()
		if got == nil {
			t.Fatal("populated: expected non-nil")
		}
		if !reflect.DeepEqual(*got, *want) {
			t.Fatalf("expected field-equal clone, got %+v want %+v", *got, *want)
		}
		// Returned pointer must be a fresh allocation, not the live source.
		if got == r.PhysicalDisk.Collection {
			t.Fatal("expected Collection() to return a fresh pointer, not the source pointer")
		}
		// Mutating the clone (scalar via value field on the cloned struct) must
		// not leak back to the source. CollectionStatus is a flat struct of
		// FieldStatus values, so mutating a nested FieldStatus proves
		// independence.
		got.Serial = diskinventory.FieldStatus{State: diskinventory.FieldMissing, Source: "mutated"}
		got.IO.Source = "leaked"
		if r.PhysicalDisk.Collection.Serial.State != diskinventory.FieldAvailable ||
			r.PhysicalDisk.Collection.Serial.Source != "smartctl" {
			t.Fatalf("Serial mutation leaked to source: %+v", r.PhysicalDisk.Collection.Serial)
		}
		if r.PhysicalDisk.Collection.IO.Source != "sysfs" {
			t.Fatalf("IO.Source mutation leaked to source: %q", r.PhysicalDisk.Collection.IO.Source)
		}
	})
}
