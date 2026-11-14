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
			if len(node.guests[111].files) != 1 || !node.guests[111].files[pveGuestFillFile(0)] || !node.dump["existing.log"] || len(node.dump) != 1 || len(node.staged) != 0 {
				t.Fatal("cleanup lost preexisting bytes or retained owned files")
			}
		})
	}
}

func TestProxmoxSafetyBackupCollisionAndReplacementAreNeverUnlinked(t *testing.T) {
	for _, point := range []string{"collision", "replacement", "storage-moved", "directory-replaced"} {
		t.Run(point, func(t *testing.T) {
			node := newFakePVENode()
			node.path = "/mnt/custom backups"
			manifest := loadPVEManifest(t, "response.pve-storage-old-backups")
			var foreign string
			runner := pveRunnerFunc(func(ctx context.Context, name string, args ...string) (CommandResult, error) {
				script := args[len(args)-1]
				if point == "collision" && strings.Contains(script, " && ln -T -- ") && foreign == "" {
					quoted := fakeQuoted.FindAllStringSubmatch(script, -1)
					foreign = filepath.Base(quoted[len(quoted)-1][1])
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
				for file := range node.links {
					foreign = file
					delete(node.links, file)
					break
				}
			case "storage-moved":
				node.path = "/mnt/replacement storage"
			case "directory-replaced":
				for dir := range node.dirs {
					node.dirs[dir] = "9:999"
				}
			}
			result := lab.Cleanup(context.Background(), manifest, prepared)
			if result.Passed || len(node.staged) == 0 {
				t.Fatalf("unsafe cleanup was not refused with anchors retained: %+v", result)
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

// Real local shell semantics for the generated ownership guard/unlink only.
// All files are tiny t.TempDir fixtures; no SSH or Proxmox program is executed.
func TestProxmoxSafetyOwnedUnlinkShellPreservesForeignBytes(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("local shell is unavailable")
	}
	if _, err := exec.LookPath("stat"); err != nil {
		t.Skip("stat is unavailable")
	}
	for _, scenario := range []string{"owned", "collision", "replaced-directory", "symlink-directory"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "dump", ".pulse-qualification-test")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			anchor, final := filepath.Join(dir, "archive.tar.zst"), filepath.Join(root, "dump", "archive.tar.zst")
			owned := []byte("owned archive")
			foreign := []byte("preexisting backup bytes")
			if err := os.WriteFile(anchor, owned, 0o600); err != nil {
				t.Fatal(err)
			}
			id, err := exec.Command("stat", "-c", "%d:%i", "--", dir).Output()
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "collision" {
				if err := os.WriteFile(final, foreign, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Link(anchor, final); err != nil {
				t.Fatal(err)
			}
			if scenario == "replaced-directory" || scenario == "symlink-directory" {
				if err := os.Rename(dir, dir+"-original"); err != nil {
					t.Fatal(err)
				}
				if scenario == "symlink-directory" {
					if err := os.Symlink(dir+"-original", dir); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
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
			state := &pveStorageCustody{path: root, dir: dir, dirID: strings.TrimSpace(string(id)), files: []string{anchor}, links: map[string]string{final: anchor}}
			prepared := &PreparedLab{PVEFaultFiles: map[string][]string{"storage:local": {final}}, pve: &pveRunState{storages: map[string]*pveStorageCustody{"local": state}}}
			err = lab.clearStorageFill(context.Background(), "local", prepared)
			if scenario == "owned" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(final); !os.IsNotExist(err) {
					t.Fatal("owned link survived cleanup")
				}
			} else {
				if err == nil {
					t.Fatal("foreign path was accepted as owned")
				}
				data, err := os.ReadFile(final)
				if scenario != "collision" {
					foreign = owned
				}
				if err != nil || !reflect.DeepEqual(data, foreign) {
					t.Fatal("refused cleanup changed preexisting bytes")
				}
			}
		})
	}
}
