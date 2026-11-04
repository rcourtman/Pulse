package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var pbsAttributionTime = time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)

func collidingPBSGuests() []PBSGuestCandidate {
	return []PBSGuestCandidate{
		{BackupType: "vm", VMID: 100, Instance: "home", Node: "pmx1", Name: "docker1"},
		{BackupType: "vm", VMID: 100, Instance: "hetzner", Node: "pmx1", Name: "docker1"},
		// Only one connection has these, so their snapshots teach the sources.
		{BackupType: "vm", VMID: 200, Instance: "home", Node: "pmx2", Name: "web"},
		{BackupType: "vm", VMID: 300, Instance: "hetzner", Node: "pmx3", Name: "mail"},
	}
}

func pbsAttributionBackup(id, vmid string, extra PBSBackup) PBSBackup {
	extra.ID = id
	extra.VMID = vmid
	extra.BackupType = "vm"
	extra.BackupTime = pbsAttributionTime
	if extra.Instance == "" {
		extra.Instance = "pbs-main"
	}
	return extra
}

func teachingPBSBackups() []PBSBackup {
	return []PBSBackup{
		pbsAttributionBackup("home-own", "200", PBSBackup{Datastore: "home-store", Owner: "home@pbs!token"}),
		pbsAttributionBackup("hetzner-own", "300", PBSBackup{Datastore: "hetzner-store", Owner: "hetzner@pbs!token"}),
	}
}

func attributedOf(t *testing.T, backups []PBSBackup, guests []PBSGuestCandidate, confirmers map[PBSSnapshotRef]string, id string) string {
	t.Helper()
	attributed := AttributePBSBackups(backups, guests, confirmers)
	if len(attributed) != len(backups) {
		t.Fatalf("attributions = %d, want one per backup (%d)", len(attributed), len(backups))
	}
	for i, backup := range backups {
		if backup.ID == id {
			return attributed[i]
		}
	}
	t.Fatalf("no backup %q", id)
	return ""
}

func TestAttributePBSBackups(t *testing.T) {
	ref := PBSSnapshotRef{BackupType: "vm", VMID: 100, Time: pbsAttributionTime.Unix()}
	for _, tc := range []struct {
		name       string
		backup     PBSBackup
		guests     []PBSGuestCandidate
		confirmers map[PBSSnapshotRef]string
		want       string
	}{
		{
			name:   "a VMID only one connection has belongs to it",
			backup: pbsAttributionBackup("b", "200", PBSBackup{}),
			want:   "home",
		},
		{
			name:   "no guest of that VMID",
			backup: pbsAttributionBackup("b", "999", PBSBackup{}),
		},
		{
			name:   "a VM snapshot cannot take a container's VMID",
			backup: pbsAttributionBackup("b", "100", PBSBackup{}),
			guests: []PBSGuestCandidate{{BackupType: "ct", VMID: 100, Instance: "home", Node: "pmx1"}},
		},
		{
			name:   "a connection's own stale duplicates are still one connection",
			backup: pbsAttributionBackup("b", "400", PBSBackup{}),
			guests: []PBSGuestCandidate{
				{BackupType: "vm", VMID: 400, Instance: "home", Node: "pmx1"},
				{BackupType: "vm", VMID: 400, Instance: "home", Node: "pmx2"},
			},
			want: "home",
		},
		{
			name:   "the stronger label match wins",
			backup: pbsAttributionBackup("b", "100", PBSBackup{Comment: "web-b"}),
			guests: []PBSGuestCandidate{
				{BackupType: "vm", VMID: 100, Instance: "home", Node: "pmx1", Name: "web-a"},
				{BackupType: "vm", VMID: 100, Instance: "hetzner", Node: "pmx1", Name: "web-b"},
			},
			want: "hetzner",
		},
		{
			name:   "tied labels with no other evidence stay unattributed",
			backup: pbsAttributionBackup("b", "100", PBSBackup{Comment: "docker1"}),
		},
		{
			name:       "an exclusive view settles tied labels",
			backup:     pbsAttributionBackup("b", "100", PBSBackup{Comment: "docker1"}),
			confirmers: map[PBSSnapshotRef]string{ref: "home"},
			want:       "home",
		},
		{
			name:       "an exclusive view settles a snapshot with no labels",
			backup:     pbsAttributionBackup("b", "100", PBSBackup{}),
			confirmers: map[PBSSnapshotRef]string{ref: "hetzner"},
			want:       "hetzner",
		},
		{
			name:       "an exclusive view does not outvote a stronger match",
			backup:     pbsAttributionBackup("b", "100", PBSBackup{Comment: "web-b"}),
			confirmers: map[PBSSnapshotRef]string{ref: "home"},
			guests: []PBSGuestCandidate{
				{BackupType: "vm", VMID: 100, Instance: "home", Node: "pmx1", Name: "web-a"},
				{BackupType: "vm", VMID: 100, Instance: "hetzner", Node: "pmx1", Name: "web-b"},
			},
			want: "hetzner",
		},
		{
			name:       "an exclusive view from a connection with no such guest takes nothing",
			backup:     pbsAttributionBackup("b", "100", PBSBackup{}),
			confirmers: map[PBSSnapshotRef]string{ref: "elsewhere"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guests := tc.guests
			if guests == nil {
				guests = collidingPBSGuests()
			}
			got := attributedOf(t, []PBSBackup{tc.backup}, guests, tc.confirmers, "b")
			if got != tc.want {
				t.Errorf("attributed to %q, want %q", got, tc.want)
			}
		})
	}
}

// The unmatched Backups row tells people that a PBS namespace named after each
// connection keeps colliding guests apart, whatever else they share.
func TestAttributePBSBackupsNamespaceNamedAfterConnectionSettlesCollision(t *testing.T) {
	for _, tc := range []struct{ namespace, want string }{
		{namespace: "home", want: "home"},
		{namespace: "Hetzner", want: "hetzner"},
	} {
		backup := pbsAttributionBackup("b", "100", PBSBackup{Namespace: tc.namespace, Comment: "docker1"})
		if got := attributedOf(t, []PBSBackup{backup}, collidingPBSGuests(), nil, "b"); got != tc.want {
			t.Errorf("namespace %q attributed to %q, want %q", tc.namespace, got, tc.want)
		}
	}
}

// With both clusters visible through their own snapshots, a source only one of
// them uses identifies a collision snapshot that has no labels of its own (#1639).
func TestAttributePBSBackupsLearnsSubmissionSources(t *testing.T) {
	collision := pbsAttributionBackup("collision", "100", PBSBackup{Datastore: "home-store", Owner: "home@pbs!token"})
	unknownSource := pbsAttributionBackup("unknown", "100", PBSBackup{Datastore: "other-store", Owner: "mystery@pbs!token"})
	backups := append(teachingPBSBackups(), collision, unknownSource)

	if got := attributedOf(t, backups, collidingPBSGuests(), nil, "collision"); got != "home" {
		t.Errorf("collision attributed to %q, want home through its source", got)
	}
	if got := attributedOf(t, backups, collidingPBSGuests(), nil, "unknown"); got != "" {
		t.Errorf("unknown source attributed to %q, want none", got)
	}
}

// A cluster that has contributed no attributable snapshot is invisible, so a
// source that looks exclusive to the other cluster proves nothing (#1639).
func TestAttributePBSBackupsInvisibleClusterKeepsCollisionUnattributed(t *testing.T) {
	backups := []PBSBackup{
		pbsAttributionBackup("home-own", "200", PBSBackup{Datastore: "shared", Owner: "shared@pbs!token"}),
		pbsAttributionBackup("collision", "100", PBSBackup{Datastore: "shared", Owner: "shared@pbs!token"}),
	}
	if got := attributedOf(t, backups, collidingPBSGuests(), nil, "collision"); got != "" {
		t.Errorf("collision attributed to %q, want none while hetzner is unobserved", got)
	}
}

// Source inference does not break a tie between labels, even when it is decisive
// (#2292). Recovery points used to, so they could disagree with the badge.
func TestAttributePBSBackupsInferenceDoesNotBreakATie(t *testing.T) {
	backups := append(teachingPBSBackups(),
		pbsAttributionBackup("tied", "100", PBSBackup{Comment: "docker1", Datastore: "home-store", Owner: "home@pbs!token"}))
	if got := attributedOf(t, backups, collidingPBSGuests(), nil, "tied"); got != "" {
		t.Errorf("tied snapshot attributed to %q, want none", got)
	}
}

// Where an exclusive view and the learned source both speak, they must agree,
// whether or not the labels tie. A view normally teaches the learner its own
// source, so they can only disagree over a row that teaches nothing.
func TestAttributePBSBackupsViewAndInferenceMustAgree(t *testing.T) {
	ref := PBSSnapshotRef{BackupType: "vm", VMID: 100, Time: pbsAttributionTime.Add(time.Hour).Unix()}
	for _, comment := range []string{"", "docker1"} {
		backup := pbsAttributionBackup("disputed", "100", PBSBackup{Comment: comment, Datastore: "hetzner-store", Owner: "hetzner@pbs!token", CarriedOver: true})
		backup.BackupTime = pbsAttributionTime.Add(time.Hour)
		backups := append(teachingPBSBackups(), backup)
		if got := attributedOf(t, backups, collidingPBSGuests(), map[PBSSnapshotRef]string{ref: "home"}, "disputed"); got != "" {
			t.Errorf("comment %q: attributed to %q, want none when they disagree", comment, got)
		}
		if got := attributedOf(t, backups, collidingPBSGuests(), map[PBSSnapshotRef]string{ref: "hetzner"}, "disputed"); got != "hetzner" {
			t.Errorf("comment %q: attributed to %q, want hetzner when they agree", comment, got)
		}
	}
}

// A snapshot confirmed by a connection that no longer has the guest still shows
// that connection uses the source, so the source stops identifying anyone else.
func TestAttributePBSBackupsConfirmerWithNoSuchGuestStillTeaches(t *testing.T) {
	gone := pbsAttributionBackup("gone", "100", PBSBackup{Datastore: "home-store", Owner: "home@pbs!token"})
	gone.BackupTime = pbsAttributionTime.Add(time.Hour)
	collision := pbsAttributionBackup("collision", "100", PBSBackup{Datastore: "home-store", Owner: "home@pbs!token"})
	backups := append(teachingPBSBackups(), gone, collision)
	confirmers := map[PBSSnapshotRef]string{{BackupType: "vm", VMID: 100, Time: gone.BackupTime.Unix()}: "elsewhere"}
	if got := attributedOf(t, backups, collidingPBSGuests(), confirmers, "gone"); got != "" {
		t.Errorf("gone attributed to %q, want none", got)
	}
	if got := attributedOf(t, backups, collidingPBSGuests(), confirmers, "collision"); got != "" {
		t.Errorf("collision attributed to %q, want none: its source also serves another connection", got)
	}
}

// A row carried over a failed read, and an in-flight snapshot, are attributed
// like any other but teach nothing about other snapshots.
func TestAttributePBSBackupsOnlyCompletedReadRowsTeach(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*PBSBackup)
	}{
		{name: "carried over a failed read", mutate: func(b *PBSBackup) { b.CarriedOver = true }},
		{name: "in flight", mutate: func(b *PBSBackup) { b.InProgress = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backups := append(teachingPBSBackups(),
				pbsAttributionBackup("collision", "100", PBSBackup{Datastore: "home-store", Owner: "home@pbs!token"}))
			tc.mutate(&backups[0])
			if got := attributedOf(t, backups, collidingPBSGuests(), nil, "home-own"); got != "home" {
				t.Errorf("the row itself attributed to %q, want home", got)
			}
			if got := attributedOf(t, backups, collidingPBSGuests(), nil, "collision"); got != "" {
				t.Errorf("collision attributed to %q, want none: the only teacher of its source did not count", got)
			}
		})
	}
}

func TestAttributePBSBackupsIgnoresOtherSubjects(t *testing.T) {
	backups := []PBSBackup{
		{ID: "host", VMID: "200", BackupType: "host", BackupTime: pbsAttributionTime},
		{ID: "name", VMID: "not-a-vmid", BackupType: "vm", BackupTime: pbsAttributionTime},
		{ID: "zero", VMID: "0", BackupType: "vm", BackupTime: pbsAttributionTime},
	}
	for _, id := range []string{"host", "name", "zero"} {
		if got := attributedOf(t, backups, collidingPBSGuests(), nil, id); got != "" {
			t.Errorf("%s attributed to %q, want none", id, got)
		}
	}
}

// The badge and the API answer come from one rule over one state.
func TestStateAttributionMatchesGuestBackupBadge(t *testing.T) {
	state := issue2681State("", "", pbsAttributionTime)
	state.UpdatePBSGuestConfirmationsForInstance("home", []PBSGuestConfirmation{
		{Storage: "pbs", BackupType: "vm", VMID: 100, Time: pbsAttributionTime.Unix()},
	})
	state.SyncGuestBackupTimes()
	home, hetzner := issue2681LastBackups(state)
	if !home.Equal(pbsAttributionTime) || !hetzner.IsZero() {
		t.Fatalf("badge home=%v hetzner=%v, want only home", home, hetzner)
	}
	byID := state.PBSBackupGuestInstances()
	if byID["home-100"] != "home" || byID["home-101"] != "home" {
		t.Fatalf("attributions = %+v, want both on home", byID)
	}
	for _, backup := range state.GetAttributedPBSBackups() {
		if backup.GuestInstance != byID[backup.ID] {
			t.Errorf("%s reads %q, want %q", backup.ID, backup.GuestInstance, byID[backup.ID])
		}
	}
}

func TestAttributedPBSBackupSerializesGuestBesideTheSnapshot(t *testing.T) {
	encoded, err := json.Marshal(AttributedPBSBackup{
		PBSBackup:     PBSBackup{ID: "b", VMID: "100", CarriedOver: true},
		GuestInstance: "home",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"id":"b"`) || !strings.Contains(text, `"guestInstance":"home"`) {
		t.Errorf("encoded = %s, want the snapshot fields and guestInstance side by side", text)
	}
	if strings.Contains(strings.ToLower(text), "carriedover") {
		t.Errorf("encoded = %s, the carried-over mark is internal", text)
	}
	encoded, err = json.Marshal(AttributedPBSBackup{PBSBackup: PBSBackup{ID: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "guestInstance") {
		t.Errorf("encoded = %s, an unattributed snapshot omits guestInstance", encoded)
	}
}
