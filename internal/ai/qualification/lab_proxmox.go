package qualification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The proxmox lab driver runs Patrol's Proxmox faults on one dedicated,
// disposable Proxmox node. It never creates or destroys guests: the node
// holds a few fixture containers built once for the lab, and the driver only
// stops, kills, fills and restores them. Every command goes over SSH to that
// node, and nothing runs until the node has proved it is the lab.
const (
	pveProbeGuestStatus          = "pve.guest_status"
	pveProbeGuestDiskUsedPercent = "pve.guest_disk_used_percent"
	pveProbeStorageUsedPercent   = "pve.storage_used_percent"

	// pveInjectorStopGuest stops a container the way a person does in the
	// Proxmox UI, so the node logs a stop task.
	pveInjectorStopGuest = "pve_stop_guest"
	// pveInjectorKillGuestInit kills the container's init from outside. The
	// container dies with no stop task, as a crashed guest does.
	pveInjectorKillGuestInit = "pve_kill_guest_init"
	// pveInjectorFillGuestFolder grows one cache folder inside a container
	// until its root disk is nearly full.
	pveInjectorFillGuestFolder = "pve_fill_guest_folder"
	// pveInjectorFillStorageBackups piles up a month of old backup archives of
	// one guest on a node storage until it is nearly full.
	pveInjectorFillStorageBackups = "pve_fill_storage_backups"

	// pveLabMarkerPath must hold exactly pveLabMarkerPrefix plus the node name.
	pveLabMarkerPath   = "/etc/pulse-qualification-lab"
	pveLabMarkerPrefix = "pulse-qualification-lab node="
	// pveFixtureMarker must appear in a guest's description before the driver
	// touches it. The rig sets it when it builds the fixture guests.
	pveFixtureMarker = "pulse-qualification-fixture"

	// The fill layouts are fixed here, so a manifest cannot point writes
	// anywhere else. The guest folder is a web app's preview cache.
	pveGuestFillDir       = "/var/lib/nextcloud/data/appdata_ocpq7x/preview"
	pveGuestFillTarget    = 92.0
	pveGuestFillFileMiB   = 16
	pveGuestFillMaxFiles  = 90
	pveStorageFillTarget  = 91.0
	pveStorageFillMaxDays = 30
)

var (
	pveHostnameLine = regexp.MustCompile(`(?m)^hostname:\s*(\S+)\s*$`)
	pveDescription  = regexp.MustCompile(`(?m)^description:\s*(.*)$`)
	pveStatusLine   = regexp.MustCompile(`(?m)^status:\s*(\S+)\s*$`)
	pveNodeName     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

func pveProbeResourceKind(probe string) string {
	switch probe {
	case pveProbeGuestStatus, pveProbeGuestDiskUsedPercent:
		return resourceKindPVEGuest
	case pveProbeStorageUsedPercent:
		return resourceKindPVEStorage
	}
	return ""
}

// ProxmoxTarget names the dedicated lab node: how to reach it as root over
// SSH, and the Proxmox node name it must report.
type ProxmoxTarget struct {
	SSHHost string
	Node    string
	// IdentityFile is an optional dedicated lab key. With it, the node's host
	// key is kept in a known-hosts file beside the key, so a rebuilt lab node
	// never touches the operator's own known hosts.
	IdentityFile string
}

func (t ProxmoxTarget) Validate() error {
	if strings.TrimSpace(t.SSHHost) == "" || strings.ContainsAny(t.SSHHost, " \t'\"`;$") {
		return errors.New("a Proxmox lab needs an SSH host for its node")
	}
	if !pveNodeName.MatchString(t.Node) {
		return errors.New("a Proxmox lab needs the node name it must report")
	}
	if t.IdentityFile != "" && !strings.HasPrefix(t.IdentityFile, "/") {
		return errors.New("the Proxmox lab key must be an absolute path")
	}
	return nil
}

type ProxmoxLab struct {
	runner    CommandRunner
	target    ProxmoxTarget
	confirmed bool
}

func NewProxmoxLab(runner CommandRunner, target ProxmoxTarget) *ProxmoxLab {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return &ProxmoxLab{runner: runner, target: target}
}

func (l *ProxmoxLab) targetLabel() string { return "proxmox:" + l.target.Node }

// node runs one shell command line on the lab node. SSH hands the joined
// arguments to the node's shell, so callers quote every value they insert.
func (l *ProxmoxLab) node(ctx context.Context, script string) (CommandResult, error) {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=15"}
	if key := l.target.IdentityFile; key != "" {
		args = append(args, "-i", key, "-o", "IdentitiesOnly=yes", "-o", "UserKnownHostsFile="+key+".known_hosts", "-o", "StrictHostKeyChecking=accept-new")
	}
	return l.runner.Run(ctx, "ssh", append(args, l.target.SSHHost, script)...)
}

// confirmLabNode refuses to go on unless the node carries the lab marker for
// exactly this node name, reports that host name, and is a single-node
// installation of that name. A production node carries no marker.
func (l *ProxmoxLab) confirmLabNode(ctx context.Context) error {
	if l.confirmed {
		return nil
	}
	if err := l.target.Validate(); err != nil {
		return err
	}
	result, err := l.node(ctx, "cat "+pveLabMarkerPath+"; echo ---; hostname; echo ---; pvesh get /cluster/resources --type node --output-format json")
	if err != nil {
		return fmt.Errorf("read the lab marker on %s: %w", l.target.SSHHost, err)
	}
	parts := strings.SplitN(result.Stdout, "---\n", 3)
	if len(parts) != 3 {
		return fmt.Errorf("%s did not answer the lab check", l.target.SSHHost)
	}
	if strings.TrimSpace(parts[0]) != pveLabMarkerPrefix+l.target.Node {
		return fmt.Errorf("%s is not marked as the Proxmox lab node %s; refusing to touch it", l.target.SSHHost, l.target.Node)
	}
	if strings.TrimSpace(parts[1]) != l.target.Node {
		return fmt.Errorf("%s reports host name %q, want %q", l.target.SSHHost, strings.TrimSpace(parts[1]), l.target.Node)
	}
	var nodes []struct {
		Node string `json:"node"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(parts[2])), &nodes); err != nil {
		return fmt.Errorf("read the node list on %s: %w", l.target.SSHHost, err)
	}
	if len(nodes) != 1 || nodes[0].Node != l.target.Node {
		return fmt.Errorf("%s is not a single-node installation of %s; refusing to touch it", l.target.SSHHost, l.target.Node)
	}
	l.confirmed = true
	return nil
}

func pveGuestSpec(manifest Manifest, alias string) (ResourceSpec, error) {
	spec, ok := manifest.resource(alias)
	if !ok || spec.Kind != resourceKindPVEGuest || spec.VMID < 100 {
		return ResourceSpec{}, fmt.Errorf("resource %q is not a Proxmox guest", alias)
	}
	return spec, nil
}

func pveStorageSpec(manifest Manifest, alias string) (ResourceSpec, error) {
	spec, ok := manifest.resource(alias)
	if !ok || spec.Kind != resourceKindPVEStorage || !safeID.MatchString(spec.Storage) {
		return ResourceSpec{}, fmt.Errorf("resource %q is not a Proxmox storage", alias)
	}
	return spec, nil
}

// guestIdentity reads a fixture guest's host name and refuses a guest whose
// description does not carry the fixture marker.
func (l *ProxmoxLab) guestIdentity(ctx context.Context, vmid int) (string, error) {
	result, err := l.node(ctx, "pct config "+strconv.Itoa(vmid))
	if err != nil {
		return "", fmt.Errorf("read container %d on the lab node: %w", vmid, err)
	}
	description := pveDescription.FindStringSubmatch(result.Stdout)
	if description == nil || !strings.Contains(description[1], pveFixtureMarker) {
		return "", fmt.Errorf("container %d on the lab node is not a qualification fixture; refusing to touch it", vmid)
	}
	hostname := pveHostnameLine.FindStringSubmatch(result.Stdout)
	if hostname == nil {
		return "", fmt.Errorf("container %d on the lab node has no host name", vmid)
	}
	return hostname[1], nil
}

func (l *ProxmoxLab) guestStatus(ctx context.Context, vmid int) (string, error) {
	result, err := l.node(ctx, "pct status "+strconv.Itoa(vmid))
	if err != nil {
		return "", err
	}
	status := pveStatusLine.FindStringSubmatch(result.Stdout)
	if status == nil {
		return "", fmt.Errorf("container %d reported no status", vmid)
	}
	return status[1], nil
}

// pveUsage reads used and total bytes from a Proxmox API path, the figures
// Proxmox shows and Pulse alerts on. They are not df's Use%: df counts
// reserved blocks as used, so a container df shows at 92% is about 87% to
// Proxmox and never crosses Pulse's 90% threshold (14 Nov 2026).
func (l *ProxmoxLab) pveUsage(ctx context.Context, apiPath, usedKey, totalKey string) (used, total int64, err error) {
	result, err := l.node(ctx, "pvesh get "+shellQuote(apiPath)+" --output-format json")
	if err != nil {
		return 0, 0, err
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(result.Stdout)))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil {
		return 0, 0, fmt.Errorf("read %s: %w", apiPath, err)
	}
	number := func(key string) (int64, error) {
		value, ok := fields[key].(json.Number)
		if !ok {
			return 0, fmt.Errorf("no %s", key)
		}
		return value.Int64()
	}
	used, usedErr := number(usedKey)
	total, totalErr := number(totalKey)
	if usedErr != nil || totalErr != nil || total <= 0 {
		return 0, 0, fmt.Errorf("%s reported no %s and %s", apiPath, usedKey, totalKey)
	}
	return used, total, nil
}

func usedPercent(used, total int64) float64 {
	return float64(used) * 100 / float64(total)
}

// guestDiskUsedPercent reads the container's root disk as Proxmox reports it.
func (l *ProxmoxLab) guestDiskUsedPercent(ctx context.Context, vmid int) (float64, error) {
	used, total, err := l.pveUsage(ctx, "/nodes/"+l.target.Node+"/lxc/"+strconv.Itoa(vmid)+"/status/current", "disk", "maxdisk")
	if err != nil {
		return 0, err
	}
	return usedPercent(used, total), nil
}

// storagePath reads where a directory storage keeps its files.
func (l *ProxmoxLab) storagePath(ctx context.Context, storage string) (string, error) {
	result, err := l.node(ctx, "pvesh get /storage/"+shellQuote(storage)+" --output-format json")
	if err != nil {
		return "", err
	}
	var config struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &config); err != nil {
		return "", fmt.Errorf("read storage %s: %w", storage, err)
	}
	if config.Type != "dir" || !strings.HasPrefix(config.Path, "/") || strings.Contains(config.Path, "..") {
		return "", fmt.Errorf("storage %s is not a directory storage the lab can fill", storage)
	}
	return config.Path, nil
}

func (l *ProxmoxLab) storageUsage(ctx context.Context, storage string) (used, total int64, err error) {
	return l.pveUsage(ctx, "/nodes/"+l.target.Node+"/storage/"+storage+"/status", "used", "total")
}

// storageUsedPercent reads a storage's use as Proxmox reports it.
func (l *ProxmoxLab) storageUsedPercent(ctx context.Context, storage string) (float64, error) {
	used, total, err := l.storageUsage(ctx, storage)
	if err != nil {
		return 0, err
	}
	return usedPercent(used, total), nil
}

func (l *ProxmoxLab) Prepare(ctx context.Context, manifest Manifest, runID string) (*PreparedLab, error) {
	if !safeID.MatchString(runID) {
		return nil, errors.New("unsafe lab run id")
	}
	if err := l.confirmLabNode(ctx); err != nil {
		return nil, err
	}
	lab := &PreparedLab{
		RunID:          runID,
		ScenarioID:     manifest.ID,
		ResourceNames:  make(map[string]string, len(manifest.Resources)),
		ResourceIDs:    make(map[string]string, len(manifest.Resources)),
		BaselineStates: make(map[string]DockerState),
		PVEFaultFiles:  make(map[string][]string),
		PVENode:        l.target.Node,
	}
	for _, resource := range manifest.Resources {
		switch resource.Kind {
		case resourceKindPVEGuest:
			hostname, err := l.guestIdentity(ctx, resource.VMID)
			if err != nil {
				return lab, err
			}
			lab.ResourceIDs[resource.Alias] = strconv.Itoa(resource.VMID)
			lab.ResourceNames[resource.Alias] = hostname
			// Leftovers of an interrupted run are driver-owned, so clear them
			// before the baseline: a guest is running, its fill folder empty.
			if err := l.clearGuestFill(ctx, resource.VMID, nil); err != nil {
				return lab, err
			}
		case resourceKindPVEStorage:
			if _, err := l.storagePath(ctx, resource.Storage); err != nil {
				return lab, err
			}
			lab.ResourceIDs[resource.Alias] = resource.Storage
			lab.ResourceNames[resource.Alias] = resource.Storage
			if err := l.clearStorageFill(ctx, resource.Storage, nil); err != nil {
				return lab, err
			}
		default:
			return lab, fmt.Errorf("resource %q of kind %s cannot run on the proxmox driver", resource.Alias, resource.Kind)
		}
	}
	pre, err := l.inventory(ctx)
	if err != nil {
		return lab, fmt.Errorf("capture pre-lab inventory: %w", err)
	}
	lab.PreInventory = pre
	if _, err := l.Observe(ctx, manifest, lab, manifest.Baseline); err != nil {
		return lab, fmt.Errorf("baseline did not converge: %w", err)
	}
	lab.ExpectedInventory = pre
	return lab, nil
}

func (l *ProxmoxLab) ApplyFault(ctx context.Context, manifest Manifest, lab *PreparedLab, fault FaultSpec) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()
	if contains(lab.AppliedFaults, fault.ID) {
		return fmt.Errorf("fault %q is already applied", fault.ID)
	}
	if err := l.confirmLabNode(ctx); err != nil {
		return err
	}
	alias := fault.Injector.Resource
	switch fault.Injector.Kind {
	case pveInjectorStopGuest:
		spec, err := pveGuestSpec(manifest, alias)
		if err != nil {
			return err
		}
		if _, err := l.node(ctx, "pct stop "+strconv.Itoa(spec.VMID)); err != nil {
			return fmt.Errorf("stop container %d: %w", spec.VMID, err)
		}
	case pveInjectorKillGuestInit:
		spec, err := pveGuestSpec(manifest, alias)
		if err != nil {
			return err
		}
		id := strconv.Itoa(spec.VMID)
		// lxc-info names the container's init; killing it ends the container
		// without a Proxmox stop task, which is what a crash looks like.
		if _, err := l.node(ctx, "pid=$(lxc-info -n "+id+" -pH) && [ -n \"$pid\" ] && kill -9 \"$pid\""); err != nil {
			return fmt.Errorf("kill the init of container %d: %w", spec.VMID, err)
		}
	case pveInjectorFillGuestFolder:
		spec, err := pveGuestSpec(manifest, alias)
		if err != nil {
			return err
		}
		if err := l.fillGuestFolder(ctx, lab, alias, spec.VMID); err != nil {
			return err
		}
	case pveInjectorFillStorageBackups:
		spec, err := pveStorageSpec(manifest, alias)
		if err != nil {
			return err
		}
		guest, err := pveGuestSpec(manifest, fault.Injector.Value)
		if err != nil {
			return fmt.Errorf("fault %q: %w", fault.ID, err)
		}
		if err := l.fillStorageBackups(ctx, lab, alias, spec.Storage, guest.VMID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("the proxmox driver does not run injector %q", fault.Injector.Kind)
	}
	lab.AppliedFaults = append(lab.AppliedFaults, fault.ID)
	return nil
}

func (l *ProxmoxLab) RevertFault(ctx context.Context, manifest Manifest, lab *PreparedLab, fault FaultSpec) error {
	if err := l.confirmLabNode(ctx); err != nil {
		return err
	}
	alias := fault.Injector.Resource
	switch fault.Injector.Kind {
	case pveInjectorStopGuest, pveInjectorKillGuestInit:
		spec, err := pveGuestSpec(manifest, alias)
		if err != nil {
			return err
		}
		return l.startGuest(ctx, spec.VMID)
	case pveInjectorFillGuestFolder:
		spec, err := pveGuestSpec(manifest, alias)
		if err != nil {
			return err
		}
		return l.clearGuestFill(ctx, spec.VMID, lab)
	case pveInjectorFillStorageBackups:
		spec, err := pveStorageSpec(manifest, alias)
		if err != nil {
			return err
		}
		return l.clearStorageFill(ctx, spec.Storage, lab)
	}
	return fmt.Errorf("the proxmox driver does not revert injector %q", fault.Injector.Kind)
}

func (l *ProxmoxLab) startGuest(ctx context.Context, vmid int) error {
	status, err := l.guestStatus(ctx, vmid)
	if err != nil {
		return err
	}
	if status == "running" {
		return nil
	}
	if _, err := l.node(ctx, "pct start "+strconv.Itoa(vmid)); err != nil {
		return fmt.Errorf("start container %d: %w", vmid, err)
	}
	return nil
}

func pveGuestFillFile(index int) string {
	return path.Join(pveGuestFillDir, fmt.Sprintf("%02x-thumbs-4096-4096-max.png", index))
}

// fillGuestFolder writes one file at a time, recording each before writing,
// until the container's root disk reaches the target.
func (l *ProxmoxLab) fillGuestFolder(ctx context.Context, lab *PreparedLab, alias string, vmid int) error {
	id := strconv.Itoa(vmid)
	if _, err := l.node(ctx, "pct exec "+id+" -- mkdir -p "+pveGuestFillDir); err != nil {
		return fmt.Errorf("create the fill folder in container %d: %w", vmid, err)
	}
	key := "guest:" + id
	for index := 0; index < pveGuestFillMaxFiles; index++ {
		used, err := l.guestDiskUsedPercent(ctx, vmid)
		if err != nil {
			return err
		}
		if used >= pveGuestFillTarget {
			return nil
		}
		file := pveGuestFillFile(index)
		lab.PVEFaultFiles[key] = append(lab.PVEFaultFiles[key], file)
		// Small synced steps: a container this size kills a single large write.
		if _, err := l.node(ctx, "pct exec "+id+" -- sh -c "+shellQuote("dd if=/dev/urandom of="+file+" bs=1M count="+strconv.Itoa(pveGuestFillFileMiB)+" conv=fsync status=none")); err != nil {
			used, _ := l.guestDiskUsedPercent(ctx, vmid)
			if used >= pveGuestFillTarget {
				return nil
			}
			return fmt.Errorf("fill container %d: %w", vmid, err)
		}
	}
	used, err := l.guestDiskUsedPercent(ctx, vmid)
	if err == nil && used < pveGuestFillTarget {
		return fmt.Errorf("container %d reached only %.0f%% after %d files", vmid, used, pveGuestFillMaxFiles)
	}
	return err
}

// clearGuestFill removes the driver's fill files from a container. With a lab
// it removes what that run recorded; without one (Prepare) it removes any file
// matching the driver's own naming in the driver's own folder.
func (l *ProxmoxLab) clearGuestFill(ctx context.Context, vmid int, lab *PreparedLab) error {
	id := strconv.Itoa(vmid)
	if err := l.startGuest(ctx, vmid); err != nil {
		return err
	}
	if lab != nil {
		key := "guest:" + id
		files := lab.PVEFaultFiles[key]
		if len(files) == 0 {
			return nil
		}
		quoted := make([]string, len(files))
		for i, file := range files {
			quoted[i] = file
		}
		if _, err := l.node(ctx, "pct exec "+id+" -- rm -f "+strings.Join(quoted, " ")); err != nil {
			return fmt.Errorf("clear the fill in container %d: %w", vmid, err)
		}
		delete(lab.PVEFaultFiles, key)
		return nil
	}
	_, err := l.node(ctx, "pct exec "+id+" -- sh -c "+shellQuote("rm -f "+pveGuestFillDir+"/*-thumbs-4096-4096-max.png"))
	if err != nil {
		return fmt.Errorf("clear leftover fill in container %d: %w", vmid, err)
	}
	return nil
}

func pveBackupArchiveName(vmid int, day time.Time, index int) string {
	return fmt.Sprintf("vzdump-lxc-%d-%s-02_00_%02d.tar.zst", vmid, day.Format("2006_01_02"), index%60)
}

// fillStorageBackups writes a month of nightly archives of one guest into the
// storage's dump folder, oldest first, until the storage reaches the target.
// Each archive gets the log a real backup leaves beside it.
func (l *ProxmoxLab) fillStorageBackups(ctx context.Context, lab *PreparedLab, alias, storage string, vmid int) error {
	dir, err := l.storagePath(ctx, storage)
	if err != nil {
		return err
	}
	dump := path.Join(dir, "dump")
	if _, err := l.node(ctx, "mkdir -p "+shellQuote(dump)); err != nil {
		return err
	}
	used, total, err := l.storageUsage(ctx, storage)
	if err != nil {
		return err
	}
	need := int64(float64(total)*pveStorageFillTarget/100) - used
	if need <= 0 {
		return fmt.Errorf("storage %s is already past %.0f%%", storage, pveStorageFillTarget)
	}
	size := need/int64(pveStorageFillMaxDays-2) + 1
	key := "storage:" + storage
	now := time.Now().UTC()
	for day := pveStorageFillMaxDays; day >= 1; day-- {
		percent, err := l.storageUsedPercent(ctx, storage)
		if err != nil {
			return err
		}
		if percent >= pveStorageFillTarget {
			return nil
		}
		stamp := now.AddDate(0, 0, -day)
		archive := path.Join(dump, pveBackupArchiveName(vmid, stamp, day))
		logFile := strings.TrimSuffix(archive, ".tar.zst") + ".log"
		lab.PVEFaultFiles[key] = append(lab.PVEFaultFiles[key], archive, logFile)
		touch := stamp.Format("200601020000")
		script := "fallocate -l " + strconv.FormatInt(size, 10) + " " + shellQuote(archive) +
			" && printf 'INFO: Starting Backup of VM " + strconv.Itoa(vmid) + " (lxc)\\nINFO: Finished Backup of VM " + strconv.Itoa(vmid) + "\\nINFO: Backup job finished successfully\\n' > " + shellQuote(logFile) +
			" && touch -t " + touch + " " + shellQuote(archive) + " " + shellQuote(logFile)
		if _, err := l.node(ctx, script); err != nil {
			return fmt.Errorf("write an old backup archive to %s: %w", storage, err)
		}
	}
	percent, err := l.storageUsedPercent(ctx, storage)
	if err == nil && percent < pveStorageFillTarget {
		return fmt.Errorf("storage %s reached only %.0f%%", storage, percent)
	}
	return err
}

// clearStorageFill removes the archives a run wrote, or, from Prepare, every
// archive in the dump folder that carries the driver's fixed 02:00 naming
// for a fixture guest of this lab.
func (l *ProxmoxLab) clearStorageFill(ctx context.Context, storage string, lab *PreparedLab) error {
	dir, err := l.storagePath(ctx, storage)
	if err != nil {
		return err
	}
	dump := path.Join(dir, "dump")
	if lab != nil {
		key := "storage:" + storage
		files := lab.PVEFaultFiles[key]
		if len(files) == 0 {
			return nil
		}
		quoted := make([]string, len(files))
		for i, file := range files {
			quoted[i] = shellQuote(file)
		}
		if _, err := l.node(ctx, "rm -f "+strings.Join(quoted, " ")); err != nil {
			return fmt.Errorf("clear the archives on %s: %w", storage, err)
		}
		delete(lab.PVEFaultFiles, key)
		return nil
	}
	_, err = l.node(ctx, "rm -f "+shellQuote(dump)+"/vzdump-lxc-*-02_00_*.tar.zst "+shellQuote(dump)+"/vzdump-lxc-*-02_00_*.log")
	return err
}

func (l *ProxmoxLab) Observe(ctx context.Context, manifest Manifest, lab *PreparedLab, predicates []Predicate) ([]PredicateObservation, error) {
	observations := make([]PredicateObservation, 0, len(predicates))
	for _, predicate := range predicates {
		observation, err := l.waitPredicate(ctx, manifest, lab, predicate)
		observations = append(observations, observation)
		if err != nil {
			return observations, err
		}
	}
	return observations, nil
}

func (l *ProxmoxLab) waitPredicate(ctx context.Context, manifest Manifest, lab *PreparedLab, predicate Predicate) (PredicateObservation, error) {
	timeout := 30 * time.Second
	if predicate.Timeout != "" {
		parsed, err := positiveDuration(predicate.Timeout)
		if err != nil {
			return PredicateObservation{Predicate: predicate, Error: err.Error()}, err
		}
		timeout = parsed
	}
	deadline := time.Now().Add(timeout)
	var last PredicateObservation
	for {
		last = PredicateObservation{Predicate: predicate, CheckedAt: time.Now().UTC()}
		observed, err := l.probe(ctx, manifest, lab, predicate)
		last.Observed = observed
		if err != nil {
			last.Error = err.Error()
		} else {
			passed, compareErr := comparePredicate(observed, predicate.Operator, predicate.Value)
			if compareErr != nil {
				last.Error = compareErr.Error()
			} else if passed {
				last.Passed = true
				return last, nil
			}
		}
		if time.Now().After(deadline) {
			return last, fmt.Errorf("oracle predicate %s %s %s did not converge: observed=%v error=%s", predicate.Probe, predicate.Operator, string(predicate.Value), observed, last.Error)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (l *ProxmoxLab) probe(ctx context.Context, manifest Manifest, lab *PreparedLab, predicate Predicate) (any, error) {
	switch predicate.Probe {
	case "inventory.same_as_pre":
		inventory, err := l.inventory(ctx)
		return inventoryEqual(inventory, lab.PreInventory), err
	case pveProbeGuestStatus:
		spec, err := pveGuestSpec(manifest, predicate.Target)
		if err != nil {
			return nil, err
		}
		return l.guestStatus(ctx, spec.VMID)
	case pveProbeGuestDiskUsedPercent:
		spec, err := pveGuestSpec(manifest, predicate.Target)
		if err != nil {
			return nil, err
		}
		return l.guestDiskUsedPercent(ctx, spec.VMID)
	case pveProbeStorageUsedPercent:
		spec, err := pveStorageSpec(manifest, predicate.Target)
		if err != nil {
			return nil, err
		}
		return l.storageUsedPercent(ctx, spec.Storage)
	}
	return nil, fmt.Errorf("the proxmox driver has no probe %q", predicate.Probe)
}

// inventory lists the node's containers and the driver-named archives on its
// storages. A run that ends where it started leaves both as they were.
func (l *ProxmoxLab) inventory(ctx context.Context) (DockerInventory, error) {
	result, err := l.node(ctx, "pct list | awk 'NR>1{print $1\":\"$2}'; echo ---; ls /var/lib/vz/dump 2>/dev/null | grep -E '^vzdump-lxc-[0-9]+-[0-9_]+-02_00_[0-9]+\\.(tar\\.zst|log)$' || true")
	if err != nil {
		return DockerInventory{}, err
	}
	parts := strings.SplitN(result.Stdout, "---", 2)
	guests := strings.Fields(parts[0])
	sort.Strings(guests)
	var archives []string
	if len(parts) == 2 {
		archives = strings.Fields(parts[1])
		sort.Strings(archives)
	}
	return DockerInventory{Containers: guests, Volumes: archives}, nil
}

// Cleanup reverts what a run left applied: running guests, empty fill
// folders, no archives. A second pass must find nothing to do.
func (l *ProxmoxLab) Cleanup(ctx context.Context, manifest Manifest, lab *PreparedLab) CleanupResult {
	var errs []string
	cleanup := func() int {
		changed := 0
		for _, resource := range manifest.Resources {
			switch resource.Kind {
			case resourceKindPVEGuest:
				key := "guest:" + strconv.Itoa(resource.VMID)
				if len(lab.PVEFaultFiles[key]) > 0 {
					changed++
				}
				status, err := l.guestStatus(ctx, resource.VMID)
				if err == nil && status != "running" {
					changed++
				}
				if err := l.clearGuestFill(ctx, resource.VMID, lab); err != nil {
					errs = append(errs, err.Error())
				}
			case resourceKindPVEStorage:
				if len(lab.PVEFaultFiles["storage:"+resource.Storage]) > 0 {
					changed++
				}
				if err := l.clearStorageFill(ctx, resource.Storage, lab); err != nil {
					errs = append(errs, err.Error())
				}
			}
		}
		return changed
	}
	if err := l.confirmLabNode(ctx); err != nil {
		return CleanupResult{Errors: []string{err.Error()}}
	}
	cleanup()
	secondNoop := cleanup() == 0
	post, err := l.inventory(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	}
	unchanged := inventoryEqual(post, lab.PreInventory)
	passed := len(errs) == 0
	if manifest.Teardown.RequireSecondNoop && !secondNoop {
		passed = false
		errs = append(errs, "second cleanup was not a no-op")
	}
	if manifest.Teardown.RequireInventorySame && !unchanged {
		passed = false
		errs = append(errs, "post-lab Proxmox inventory differs from pre-lab inventory")
	}
	return CleanupResult{PostInventory: post, SecondCleanupNoop: secondNoop, InventoryUnchanged: unchanged, Passed: passed, Errors: errs}
}
