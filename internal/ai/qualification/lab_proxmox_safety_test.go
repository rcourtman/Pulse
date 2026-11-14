package qualification

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type pveRunnerFunc func(context.Context, string, ...string) (CommandResult, error)

func (f pveRunnerFunc) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	return f(ctx, name, args...)
}

func pveMutation(script string) bool {
	return strings.HasPrefix(script, "pct start ") || strings.HasPrefix(script, "pct stop ") ||
		strings.HasPrefix(script, "pct exec ") || strings.Contains(script, "kill -9") ||
		strings.Contains(script, "rm -f") || strings.Contains(script, "mkdir ") ||
		strings.Contains(script, "fallocate ") || strings.Contains(script, "ln -T -- ")
}

func assertNoPVEMutations(t *testing.T, calls []string) {
	t.Helper()
	for _, call := range calls {
		if pveMutation(call) {
			t.Fatalf("unowned resource received a mutation: %s", call)
		}
	}
}

func TestProxmoxSafetyPartialPrepareNeverMutatesRefusedOrUnvisitedGuests(t *testing.T) {
	for _, refused := range []int{110, 111, 112} {
		t.Run(strconv.Itoa(refused), func(t *testing.T) {
			node := newFakePVENode()
			for _, guest := range node.guests {
				guest.status = "stopped"
				guest.files[pveGuestFillFile(0)] = true
			}
			node.guests[refused].fixture = false
			node.dump["vzdump-lxc-111-2026_10_01-02_00_03.tar.zst"] = true
			node.dump["vzdump-lxc-111-2026_10_01-02_00_03.log"] = true
			manifest := loadPVEManifest(t, "response.pve-guest-died")
			lab := NewProxmoxLab(node, pveTarget)
			prepared, err := lab.Prepare(context.Background(), manifest, "partial")
			if err == nil || prepared == nil {
				t.Fatalf("expected a retained partial refusal: %v", err)
			}
			result := lab.Cleanup(context.Background(), manifest, prepared)
			if result.InventoryUnchanged || result.Passed {
				t.Fatalf("an uncaptured baseline was reported as verified: %+v", result)
			}
			assertNoPVEMutations(t, node.calls)
			for _, guest := range node.guests {
				if guest.status != "stopped" || !guest.files[pveGuestFillFile(0)] {
					t.Fatal("refused preparation changed a guest or its old bytes")
				}
			}
			if len(node.dump) != 2 {
				t.Fatal("refused preparation changed existing archives/logs")
			}
		})
	}
}

func TestProxmoxSafetyPreparePreservesAndInventoriesCustomStorage(t *testing.T) {
	node := newFakePVENode()
	node.path = "/mnt/custom backup pool"
	for _, file := range []string{"vzdump-qemu-300-2026_01_01-17_28_33.vma.zst", "vzdump-lxc-111-2026_10_01-02_00_03.tar.zst", "vzdump-lxc-111-2026_10_01-02_00_03.log", "notes with\nnewlines\tand tabs"} {
		node.dump[file] = true
	}
	node.guests[111].files[pveGuestFillFile(0)] = true
	manifest := loadPVEManifest(t, "response.pve-storage-old-backups")
	lab := NewProxmoxLab(node, pveTarget)
	prepared, err := lab.Prepare(context.Background(), manifest, "preserve")
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.PreInventory.Volumes) != len(node.dump) {
		t.Fatalf("inventory omitted ordinary/custom backup entries: %+v", prepared.PreInventory)
	}
	var driver Lab = lab
	observed, err := driver.runInventory(context.Background(), prepared)
	if err != nil || !inventoryEqual(observed, prepared.PreInventory) {
		t.Fatalf("runner safety oracle did not observe prepared storage: %+v %v", observed, err)
	}
	if result := lab.Cleanup(context.Background(), manifest, prepared); !result.Passed {
		t.Fatalf("unchanged custom storage did not verify: %+v", result)
	}
	assertNoPVEMutations(t, node.calls)
	if !node.guests[111].files[pveGuestFillFile(0)] || len(node.dump) != 4 {
		t.Fatal("preexisting guest preview/archive/log was removed")
	}
	delete(node.dump, "vzdump-qemu-300-2026_01_01-17_28_33.vma.zst")
	if result := lab.Cleanup(context.Background(), manifest, prepared); result.Passed || result.InventoryUnchanged {
		t.Fatalf("ordinary backup loss was invisible to inventory: %+v", result)
	}
}

func TestProxmoxSafetyCleanupRestoresOnlyAMutatedGuest(t *testing.T) {
	node := newFakePVENode()
	manifest := loadPVEManifest(t, "response.pve-guest-stopped-by-person")
	lab := NewProxmoxLab(node, pveTarget)
	prepared, err := lab.Prepare(context.Background(), manifest, "owned-power")
	if err != nil {
		t.Fatal(err)
	}
	if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err != nil {
		t.Fatal(err)
	}
	// Another actor's change is not a restoration obligation for this run.
	node.guests[110].status = "stopped"
	result := lab.Cleanup(context.Background(), manifest, prepared)
	if node.guests[112].status != "running" || node.guests[110].status != "stopped" {
		t.Fatal("cleanup normalized an untouched guest or failed its own obligation")
	}
	if result.Passed || result.InventoryUnchanged {
		t.Fatalf("an external inventory change was hidden: %+v", result)
	}
}

func TestProxmoxSafetyAdmissionIsRenewedAtApplyAndCleanup(t *testing.T) {
	for _, point := range []string{"apply", "cleanup"} {
		for _, change := range []string{"marker", "hostname", "node"} {
			t.Run(point+"/"+change, func(t *testing.T) {
				node := newFakePVENode()
				manifest := loadPVEManifest(t, "response.pve-guest-stopped-by-person")
				lab := NewProxmoxLab(node, pveTarget)
				prepared, err := lab.Prepare(context.Background(), manifest, "renew")
				if err != nil {
					t.Fatal(err)
				}
				if point == "cleanup" {
					if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err != nil {
						t.Fatal(err)
					}
				}
				switch change {
				case "marker":
					node.guests[112].fixture = false
				case "hostname":
					node.guests[112].hostname = "replacement"
				case "node":
					node.marker = ""
				}
				node.calls = nil
				if point == "apply" {
					if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err == nil {
						t.Fatal("changed admission was accepted")
					}
				} else if result := lab.Cleanup(context.Background(), manifest, prepared); result.Passed {
					t.Fatal("cleanup reported success after admission withdrawal")
				}
				assertNoPVEMutations(t, node.calls)
			})
		}
	}
}

func TestProxmoxSafetyPartialMutationsRemainOwnedForCleanup(t *testing.T) {
	for _, kind := range []string{"power", "guest", "storage"} {
		t.Run(kind, func(t *testing.T) {
			node := newFakePVENode()
			scenario := map[string]string{"power": "response.pve-guest-stopped-by-person", "guest": "response.pve-guest-disk-one-folder", "storage": "response.pve-storage-old-backups"}[kind]
			manifest := loadPVEManifest(t, scenario)
			node.guests[111].files[pveGuestFillFile(0)] = true
			node.dump["existing.log"] = true
			failed := false
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				script := args[len(args)-1]
				result, err := node.Run(ctx, name, args...)
				match := kind == "power" && script == "pct stop 112" || kind == "guest" && strings.Contains(script, "dd if=") || kind == "storage" && strings.Contains(script, "fallocate ")
				if match && !failed {
					failed = true
					return result, errors.New("inert response lost after partial mutation")
				}
				return result, err
			})
			lab := NewProxmoxLab(runner, pveTarget)
			prepared, err := lab.Prepare(context.Background(), manifest, "partial-write")
			if err != nil {
				t.Fatal(err)
			}
			if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err == nil || !failed {
				t.Fatal("partial mutation was not exercised and reported")
			}
			result := lab.Cleanup(context.Background(), manifest, prepared)
			if !result.Passed || !result.SecondCleanupNoop || !result.InventoryUnchanged {
				t.Fatalf("known partial mutation did not clean up: %+v", result)
			}
			if len(node.guests[111].files) != 1 || !node.guests[111].files[pveGuestFillFile(0)] || !node.dump["existing.log"] || len(node.dump) != 1 {
				t.Fatal("cleanup lost preexisting bytes or retained owned files")
			}
		})
	}
}

func TestProxmoxSafetyBackupCollisionAndReplacementAreNeverUnlinked(t *testing.T) {
	for _, point := range []string{"collision", "replacement", "storage-moved", "symlink"} {
		t.Run(point, func(t *testing.T) {
			node := newFakePVENode()
			node.path = "/mnt/custom backups"
			manifest := loadPVEManifest(t, "response.pve-storage-old-backups")
			var foreign string
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				script := args[len(args)-1]
				if point == "collision" && strings.Contains(script, "set -C; printf") && foreign == "" {
					match := fakeReservation.FindStringSubmatch(script)
					foreign = filepath.Base(match[2])
					node.dump[foreign] = true
				}
				return node.Run(ctx, name, args...)
			})
			lab := NewProxmoxLab(runner, pveTarget)
			prepared, err := lab.Prepare(context.Background(), manifest, "backup-custody")
			if err != nil {
				t.Fatal(err)
			}
			err = lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0])
			if (point == "collision") != (err != nil) {
				t.Fatalf("unexpected injection result: %v", err)
			}
			switch point {
			case "replacement":
				for file := range node.markers {
					foreign = file
					delete(node.markers, file)
					break
				}
			case "storage-moved":
				node.path = "/mnt/replacement storage"
			case "symlink":
				for file := range node.markers {
					foreign = file
					node.links[file] = "external symlink"
					break
				}
			}
			result := lab.Cleanup(context.Background(), manifest, prepared)
			if result.Passed || len(prepared.PVEFaultFiles["storage:local"]) == 0 && point != "collision" {
				t.Fatalf("unsafe cleanup was not refused with custody retained: %+v", result)
			}
			if foreign != "" && !node.dump[foreign] {
				t.Fatal("cleanup unlinked a colliding/replaced backup")
			}
		})
	}
}

func TestProxmoxSafetyInventoryFailuresAreNotEmptySuccess(t *testing.T) {
	for _, failure := range []string{"list", "find", "truncated"} {
		t.Run(failure, func(t *testing.T) {
			node := newFakePVENode()
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				script := args[len(args)-1]
				if failure == "list" && strings.HasPrefix(script, "pct list") || failure == "find" && strings.HasPrefix(script, "find ") {
					return CommandResult{}, errors.New("inert inventory read failure")
				}
				if failure == "truncated" && strings.HasPrefix(script, "find ") {
					return CommandResult{Stdout: "unfinished\x00f"}, nil
				}
				return node.Run(ctx, name, args...)
			})
			manifest := loadPVEManifest(t, "response.pve-guest-died")
			lab := NewProxmoxLab(runner, pveTarget)
			prepared, err := lab.Prepare(context.Background(), manifest, "bad-inventory")
			if err == nil || prepared == nil {
				t.Fatal("inventory failure was accepted")
			}
			if result := lab.Cleanup(context.Background(), manifest, prepared); result.Passed || result.InventoryUnchanged {
				t.Fatalf("unavailable inventory became verification: %+v", result)
			}
			assertNoPVEMutations(t, node.calls)
		})
	}
}

func TestProxmoxSafetyUnverifiedDirectoryCreationIsNotAdopted(t *testing.T) {
	for _, response := range []string{"collision", "lost-response"} {
		t.Run(response, func(t *testing.T) {
			node := newFakePVENode()
			manifest := loadPVEManifest(t, "response.pve-guest-disk-one-folder")
			failed := false
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				script := args[len(args)-1]
				if !failed && strings.Contains(script, "mkdir -- ") {
					failed = true
					if response == "lost-response" {
						if _, err := node.Run(ctx, name, args...); err != nil {
							t.Fatal(err)
						}
					} else {
						dir := filepath.Join(pveGuestFillDir, "pulse-qualification-unverified")
						node.guests[111].dirs[dir] = "9:99"
						node.guests[111].files[filepath.Join(dir, "preexisting")] = true
					}
					return CommandResult{}, errors.New("inert unverified creation")
				}
				return node.Run(ctx, name, args...)
			})
			lab := NewProxmoxLab(runner, pveTarget)
			prepared, err := lab.Prepare(context.Background(), manifest, "unverified")
			if err != nil {
				t.Fatal(err)
			}
			if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err == nil || !failed {
				t.Fatal("unverified creation was accepted")
			}
			node.calls = nil
			if result := lab.Cleanup(context.Background(), manifest, prepared); result.Passed || len(result.Errors) == 0 {
				t.Fatal("unknown directory creation became completed cleanup")
			}
			assertNoPVEMutations(t, node.calls)
			if len(node.guests[111].dirs) != 1 {
				t.Fatal("unverified directory was adopted or removed")
			}
			if response == "collision" && len(node.guests[111].files) != 1 {
				t.Fatal("preexisting bytes were removed")
			}
		})
	}
}

func TestProxmoxSafetyLostFileCreationResponseRetainsOnlyOwnBytes(t *testing.T) {
	node := newFakePVENode()
	manifest := loadPVEManifest(t, "response.pve-storage-old-backups")
	failed := false
	runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
		result, err := node.Run(ctx, name, args...)
		if !failed && strings.Contains(args[len(args)-1], "set -C; printf") {
			failed = true
			return result, errors.New("inert lost response after marked creation")
		}
		return result, err
	})
	lab := NewProxmoxLab(runner, pveTarget)
	prepared, err := lab.Prepare(context.Background(), manifest, "lost-file-response")
	if err != nil {
		t.Fatal(err)
	}
	if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err == nil || !failed {
		t.Fatal("lost creation response was not exercised")
	}
	if len(node.dump) != 1 {
		t.Fatal("partial creation did not leave exactly one marked file")
	}
	if result := lab.Cleanup(context.Background(), manifest, prepared); !result.Passed || len(node.dump) != 0 {
		t.Fatalf("the run's marker could not reconcile its partial creation: %+v", result)
	}
}

func TestProxmoxSafetyFailedCleanupRetainsCustody(t *testing.T) {
	for _, kind := range []string{"guest", "storage"} {
		t.Run(kind, func(t *testing.T) {
			node := newFakePVENode()
			scenario := "response.pve-guest-disk-one-folder"
			if kind == "storage" {
				scenario = "response.pve-storage-old-backups"
			}
			manifest := loadPVEManifest(t, scenario)
			refuseCleanup := false
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				if refuseCleanup && strings.Contains(args[len(args)-1], "rm -f") {
					return CommandResult{}, errors.New("inert cleanup failure")
				}
				return node.Run(ctx, name, args...)
			})
			lab := NewProxmoxLab(runner, pveTarget)
			prepared, err := lab.Prepare(context.Background(), manifest, "cleanup-error")
			if err != nil {
				t.Fatal(err)
			}
			if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(prepared.PVEFaultFiles)
			refuseCleanup = true
			result := lab.Cleanup(context.Background(), manifest, prepared)
			after, _ := json.Marshal(prepared.PVEFaultFiles)
			if result.Passed || result.SecondCleanupNoop || len(result.Errors) == 0 || string(before) != string(after) {
				t.Fatalf("failed cleanup lost custody or claimed success: %+v", result)
			}
			if len(node.guests[111].files)+len(node.dump) == 0 {
				t.Fatal("failed cleanup erased unremoved file evidence")
			}
		})
	}
}

func TestProxmoxSafetyManifestAndReportNamesAreNotCustody(t *testing.T) {
	node := newFakePVENode()
	manifest := loadPVEManifest(t, "response.pve-guest-stopped-by-person")
	lab := NewProxmoxLab(node, pveTarget)
	prepared := &PreparedLab{PVENode: pveTarget.Node, ResourceIDs: map[string]string{"gitea": "112"}, PVEFaultFiles: map[string][]string{"guest:112": {pveGuestFillFile(0)}}}
	if err := lab.ApplyFault(context.Background(), manifest, prepared, manifest.Faults[0]); err == nil {
		t.Fatal("manifest names granted fault authority")
	}
	if err := lab.RevertFault(context.Background(), manifest, prepared, manifest.Faults[0]); err == nil {
		t.Fatal("report names granted revert authority")
	}
	if result := lab.Cleanup(context.Background(), manifest, prepared); result.Passed {
		t.Fatal("report names granted cleanup authority")
	}
	assertNoPVEMutations(t, node.calls)
}

// Real local shell semantics for the generated ownership unlink only.
// All files are tiny t.TempDir fixtures; no SSH or Proxmox program is executed.
func TestProxmoxSafetyOwnedUnlinkShellPreservesForeignBytes(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("local shell is unavailable")
	}
	for _, scenario := range []string{"owned", "foreign", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "dump"), 0700); err != nil {
				t.Fatal(err)
			}
			final := filepath.Join(root, "dump", "archive.tar.zst")
			marker := "INFO: Pulse qualification owner local-test\n"
			owned, foreign := []byte(marker+"synthetic bytes"), []byte("preexisting backup bytes")
			data := owned
			if scenario == "foreign" {
				data = foreign
			}
			if scenario == "symlink" {
				if err := os.WriteFile(final+"-target", foreign, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(final+"-target", final); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(final, data, 0600); err != nil {
				t.Fatal(err)
			}
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				if name != "ssh" {
					t.Fatalf("unexpected runner program %q", name)
				}
				script := args[len(args)-1]
				if strings.HasPrefix(script, "pvesh get /storage/") {
					data, _ := json.Marshal(map[string]string{"path": root, "type": "dir"})
					return CommandResult{Stdout: string(data)}, nil
				}
				out, err := exec.CommandContext(ctx, "sh", "-c", script).CombinedOutput()
				return CommandResult{Stdout: string(out)}, err
			})
			lab := NewProxmoxLab(runner, pveTarget)
			state := &pveStorageCustody{path: root, marker: marker, files: []string{final}}
			prepared := &PreparedLab{PVEFaultFiles: map[string][]string{"storage:local": {final}}, pve: &pveRunState{storages: map[string]*pveStorageCustody{"local": state}}}
			err := lab.clearStorageFill(context.Background(), "local", prepared)
			if scenario == "owned" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(final); !os.IsNotExist(err) {
					t.Fatal("owned file survived cleanup")
				}
			} else {
				if err == nil {
					t.Fatal("foreign path was accepted as owned")
				}
				data, err := os.ReadFile(final)
				if err != nil || !reflect.DeepEqual(data, foreign) {
					t.Fatal("refused cleanup changed foreign bytes")
				}
			}
		})
	}
}

func TestProxmoxSafetyExclusiveFileReservationPreservesExistingEntries(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("local shell is unavailable")
	}
	for _, scenario := range []string{"absent", "file", "directory", "symlink", "dangling-symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			file, target := filepath.Join(root, "backup with ' quote.tar.zst"), filepath.Join(root, "target")
			old, marker := "preexisting archive bytes", "INFO: Pulse qualification owner test-marker\n"
			switch scenario {
			case "file":
				if err := os.WriteFile(file, []byte(old), 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(file, 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink", "dangling-symlink":
				if scenario == "symlink" {
					if err := os.WriteFile(target, []byte(old), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, file); err != nil {
					t.Fatal(err)
				}
			}
			output, err := exec.Command("sh", "-c", pveReserveFileScript(file, marker)).CombinedOutput()
			if scenario == "absent" {
				data, readErr := os.ReadFile(file)
				if err != nil || readErr != nil || string(data) != marker {
					t.Fatalf("exclusive creation failed: %v %s", err, output)
				}
			} else {
				if err == nil {
					t.Fatal("existing entry was accepted for creation")
				}
				if scenario == "file" || scenario == "symlink" {
					data, readErr := os.ReadFile(file)
					if readErr != nil || string(data) != old {
						t.Fatal("exclusive creation changed old bytes")
					}
				} else if info, err := os.Lstat(file); err != nil || scenario == "directory" && !info.IsDir() || scenario == "dangling-symlink" && info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("exclusive creation changed an existing directory/symlink")
				}
			}
		})
	}
}

// Exercise the actual generated shell against tiny local fixtures, with only
// Proxmox's read-only storage responses modeled. Nothing calls SSH or a live
// appliance, and the logical storage is only 100 KB.
func TestProxmoxSafetyPruningReleasesSyntheticBackupBytes(t *testing.T) {
	for _, program := range []string{"sh", "fallocate", "head", "touch"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skip(program + " is unavailable")
		}
	}
	root := t.TempDir()
	dump := filepath.Join(root, "dump")
	if err := os.Mkdir(dump, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dump, "preexisting backup.log")
	oldBytes := []byte("preexisting log bytes")
	if err := os.WriteFile(old, oldBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	usedBytes := func() int64 {
		entries, err := os.ReadDir(dump)
		if err != nil {
			t.Fatal(err)
		}
		var used int64 = 40000
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			if info.IsDir() {
				t.Fatal("backup fill retained a private data directory")
			}
			used += info.Size()
		}
		return used
	}
	before := usedBytes()
	runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
		if name != "ssh" {
			t.Fatalf("unexpected program %q", name)
		}
		script := args[len(args)-1]
		if strings.HasPrefix(script, "pvesh get /storage/") {
			data, _ := json.Marshal(map[string]string{"path": root, "type": "dir"})
			return CommandResult{Stdout: string(data)}, nil
		}
		if fakeStorageStatus.MatchString(script) {
			data, _ := json.Marshal(map[string]int64{"used": usedBytes(), "total": 100000})
			return CommandResult{Stdout: string(data)}, nil
		}
		if !strings.Contains(script, root) {
			t.Fatalf("local fixture command escaped its root: %s", script)
		}
		out, err := exec.CommandContext(ctx, "sh", "-c", script).CombinedOutput()
		return CommandResult{Stdout: string(out)}, err
	})
	lab := NewProxmoxLab(runner, pveTarget)
	state := &pveStorageCustody{path: root}
	prepared := &PreparedLab{PVEFaultFiles: map[string][]string{}, pve: &pveRunState{storages: map[string]*pveStorageCustody{"local": state}}}
	if err := lab.fillStorageBackups(context.Background(), prepared, "local", "local", 111); err != nil {
		t.Fatal(err)
	}
	if usedBytes() < 91000 || len(state.files) == 0 {
		t.Fatal("actual synthetic files did not produce the modeled pressure")
	}
	for _, file := range state.files {
		data, err := os.ReadFile(file)
		if err != nil || !strings.HasPrefix(string(data), state.marker) {
			t.Fatal("a partial write lost its ownership marker")
		}
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
	}
	if got := usedBytes(); got != before {
		t.Fatalf("pruning left hidden archive bytes: %d vs %d", got, before)
	}
	if err := lab.clearStorageFill(context.Background(), "local", prepared); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(old); err != nil || string(data) != string(oldBytes) {
		t.Fatal("the old log changed")
	}
}
