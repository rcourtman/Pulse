package models

import (
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/proxmoxidentity"
)

// AttributedPBSBackup is a PBS snapshot with the guest it belongs to.
type AttributedPBSBackup struct {
	PBSBackup
	// GuestInstance is the PVE connection whose guest of this snapshot's type
	// and VMID owns it. A VMID is unique inside a connection, so together they
	// name one guest. Empty when the evidence does not settle it, including when
	// no such guest exists any more.
	GuestInstance string `json:"guestInstance,omitempty"`
}

// NormalizeCollections keeps the snapshot's own collection contract.
func (b AttributedPBSBackup) NormalizeCollections() AttributedPBSBackup {
	b.PBSBackup = b.PBSBackup.NormalizeCollections()
	return b
}

// PBSGuestCandidate is a live guest that a PBS snapshot of its type and VMID
// could belong to.
type PBSGuestCandidate struct {
	BackupType string // "vm" or "ct"
	VMID       int
	Instance   string
	Node       string
	Name       string
}

type pbsBackupSubject struct {
	backupType string
	vmid       int
}

func subjectOfPBSBackup(backup PBSBackup) (pbsBackupSubject, bool) {
	vmid, err := strconv.Atoi(strings.TrimSpace(backup.VMID))
	if err != nil || vmid <= 0 {
		return pbsBackupSubject{}, false
	}
	backupType := strings.ToLower(strings.TrimSpace(backup.BackupType))
	if backupType != "vm" && backupType != "ct" {
		return pbsBackupSubject{}, false
	}
	return pbsBackupSubject{backupType: backupType, vmid: vmid}, true
}

// AttributePBSBackups decides which PVE connection's guest owns each snapshot.
// It is the one attribution rule: guest backup badges, recovery points and the
// Backups table all read its answer, so one snapshot cannot read as different
// guests on different surfaces. The result lines up with backups, and an empty
// entry means the evidence does not settle that snapshot.
//
// Source inference learns from the snapshots attributable on their own evidence
// (unique typed VMIDs, placement or name matches, exclusive storage views), then
// resolves collision VMIDs that carry no such evidence (#1639). In-flight
// snapshots and rows carried over a failed read are attributed but teach nothing.
func AttributePBSBackups(backups []PBSBackup, guests []PBSGuestCandidate, exclusiveConfirmers map[PBSSnapshotRef]string) []string {
	out := make([]string, len(backups))
	if len(backups) == 0 || len(guests) == 0 {
		return out
	}

	guestsBySubject := make(map[pbsBackupSubject][]PBSGuestCandidate, len(guests))
	for _, guest := range guests {
		subject := pbsBackupSubject{backupType: guest.BackupType, vmid: guest.VMID}
		guestsBySubject[subject] = append(guestsBySubject[subject], guest)
	}

	type attributionInput struct {
		index     int
		guests    []PBSGuestCandidate
		confirmer string
	}
	inputs := make([]attributionInput, 0, len(backups))
	for i, backup := range backups {
		subject, ok := subjectOfPBSBackup(backup)
		if !ok || len(guestsBySubject[subject]) == 0 {
			continue
		}
		inputs = append(inputs, attributionInput{
			index:  i,
			guests: guestsBySubject[subject],
			confirmer: exclusiveConfirmers[PBSSnapshotRef{
				BackupType: subject.backupType,
				VMID:       subject.vmid,
				Time:       backup.BackupTime.Unix(),
			}],
		})
	}

	learner := proxmoxidentity.NewPBSSourceLearner()
	teaches := func(backup PBSBackup) bool { return !backup.InProgress && !backup.CarriedOver }
	for _, input := range inputs {
		backup := backups[input.index]
		if !teaches(backup) {
			continue
		}
		for _, guest := range input.guests {
			learner.RegisterCandidate(backup.Instance, guest.Instance)
		}
	}
	for _, input := range inputs {
		backup := backups[input.index]
		if !teaches(backup) {
			continue
		}
		attributed := attributePBSBackup(backup, input.guests, input.confirmer, nil)
		if attributed == "" {
			// An exclusive view is positive evidence about the source even when
			// that connection no longer has this guest, and it makes the cluster
			// behind it visible to the learner's coverage check.
			attributed = input.confirmer
		}
		if attributed != "" {
			learner.Observe(backup.Instance, backup.Datastore, backup.Owner, attributed)
		}
	}

	for _, input := range inputs {
		out[input.index] = attributePBSBackup(backups[input.index], input.guests, input.confirmer, learner)
	}
	return out
}

func attributePBSBackup(backup PBSBackup, guests []PBSGuestCandidate, confirmer string, learner *proxmoxidentity.PBSSourceLearner) string {
	connections := make(map[string]int, len(guests))
	for _, guest := range guests {
		score := proxmoxidentity.BackupGuestMatchScore(backup.Namespace, backup.Comment, backup.VMID, guest.Name, guest.Instance, guest.Node)
		if existing, seen := connections[guest.Instance]; !seen || score > existing {
			connections[guest.Instance] = score
		}
	}
	// A VMID is unique inside its connection, so one connection settles it.
	if len(connections) == 1 {
		for instance := range connections {
			return instance
		}
	}

	strongest := 0
	strongestConnections := make(map[string]struct{}, len(connections))
	for instance, score := range connections {
		if score > strongest {
			strongest = score
			strongestConnections = make(map[string]struct{}, len(connections))
		}
		if score == strongest && score > 0 {
			strongestConnections[instance] = struct{}{}
		}
	}
	if len(strongestConnections) == 1 {
		for instance := range strongestConnections {
			return instance
		}
	}

	learned, decisive := learner.Resolve(backup.Instance, backup.Datastore, backup.Owner)
	// Two independent evidence paths, neither authoritative over the other. An
	// exclusive storage view is authorship evidence, so it settles labels that
	// fit several guests equally, but only among those guests: it does not
	// outvote a stronger match, and a connection with no such guest takes nothing
	// (#2681). The learned source mapping is inference, so where both speak they
	// must agree.
	if confirmer != "" {
		if decisive && learned != confirmer {
			return ""
		}
		_, candidate := connections[confirmer]
		if strongest > 0 {
			_, candidate = strongestConnections[confirmer]
		}
		if candidate {
			return confirmer
		}
		return ""
	}
	// A namespace such as "pve" can match two clusters with the same node label.
	// Inference does not break that tie: it can see the other connection on
	// another PBS instance without proving which one wrote this snapshot (#2292).
	if strongest > 0 || !decisive {
		return ""
	}
	if _, candidate := connections[learned]; candidate {
		return learned
	}
	return ""
}

// pbsGuestCandidates copies only the guests these snapshots could belong to, so a
// backup read does not grow with the unrelated estate.
func pbsGuestCandidates(backups []PBSBackup, vms []VM, containers []Container) []PBSGuestCandidate {
	subjects := make(map[pbsBackupSubject]struct{}, len(backups))
	for _, backup := range backups {
		if subject, ok := subjectOfPBSBackup(backup); ok {
			subjects[subject] = struct{}{}
		}
	}
	if len(subjects) == 0 {
		return nil
	}
	var guests []PBSGuestCandidate
	for i := range vms {
		if _, wanted := subjects[pbsBackupSubject{backupType: "vm", vmid: vms[i].VMID}]; wanted {
			guests = append(guests, PBSGuestCandidate{BackupType: "vm", VMID: vms[i].VMID, Instance: vms[i].Instance, Node: vms[i].Node, Name: vms[i].Name})
		}
	}
	for i := range containers {
		if _, wanted := subjects[pbsBackupSubject{backupType: "ct", vmid: containers[i].VMID}]; wanted {
			guests = append(guests, PBSGuestCandidate{BackupType: "ct", VMID: containers[i].VMID, Instance: containers[i].Instance, Node: containers[i].Node, Name: containers[i].Name})
		}
	}
	return guests
}

func attributedPBSBackups(backups []PBSBackup, guestInstances []string) []AttributedPBSBackup {
	if backups == nil {
		return nil
	}
	out := make([]AttributedPBSBackup, len(backups))
	for i := range backups {
		out[i] = AttributedPBSBackup{PBSBackup: clonePBSBackup(backups[i]), GuestInstance: guestInstances[i]}
	}
	return out
}

func pbsGuestInstancesByID(backups []PBSBackup, guestInstances []string) map[string]string {
	out := make(map[string]string, len(backups))
	for i := range backups {
		if guestInstances[i] != "" {
			out[backups[i].ID] = guestInstances[i]
		}
	}
	return out
}

// pbsGuestsCollide reports whether any type and VMID belongs to guests on more than
// one connection. Exclusive storage views only ever settle such a collision.
func pbsGuestsCollide(guests []PBSGuestCandidate) bool {
	connections := make(map[pbsBackupSubject]string, len(guests))
	for _, guest := range guests {
		subject := pbsBackupSubject{backupType: guest.BackupType, vmid: guest.VMID}
		if seen, ok := connections[subject]; ok && seen != guest.Instance {
			return true
		}
		connections[subject] = guest.Instance
	}
	return false
}

func (s *State) pbsGuestInstancesLocked() []string {
	guests := pbsGuestCandidates(s.PBSBackups, s.VMs, s.Containers)
	var confirmers map[PBSSnapshotRef]string
	if pbsGuestsCollide(guests) {
		confirmers = s.pbsExclusiveConfirmersLocked()
	}
	return AttributePBSBackups(s.PBSBackups, guests, confirmers)
}

// GetAttributedPBSBackups returns the PBS snapshots with the guest each one
// belongs to, read under one lock so a snapshot and its answer always match.
func (s *State) GetAttributedPBSBackups() []AttributedPBSBackup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	backups := attributedPBSBackups(s.PBSBackups, s.pbsGuestInstancesLocked())
	if backups == nil {
		backups = []AttributedPBSBackup{}
	}
	return backups
}

// PBSBackupGuestInstances returns the attributed PVE connection by PBS backup ID.
// Snapshots the evidence does not settle are absent.
func (s *State) PBSBackupGuestInstances() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return pbsGuestInstancesByID(s.PBSBackups, s.pbsGuestInstancesLocked())
}

// AttributedPBSBackups is the complete-snapshot form of GetAttributedPBSBackups.
// A snapshot carries no PVE storage views, so only the rest of the evidence
// applies; mock mode reads through here.
func (s StateSnapshot) AttributedPBSBackups() []AttributedPBSBackup {
	return attributedPBSBackups(s.PBSBackups, AttributePBSBackups(s.PBSBackups, pbsGuestCandidates(s.PBSBackups, s.VMs, s.Containers), nil))
}

// PBSBackupGuestInstances is the complete-snapshot form of
// State.PBSBackupGuestInstances, with the same limit as AttributedPBSBackups.
func (s StateSnapshot) PBSBackupGuestInstances() map[string]string {
	return pbsGuestInstancesByID(s.PBSBackups, AttributePBSBackups(s.PBSBackups, pbsGuestCandidates(s.PBSBackups, s.VMs, s.Containers), nil))
}
