package monitoring

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Where a guest's network devices are plugged in changes when someone edits
// the guest, not from poll to poll. A running VM's configuration is therefore
// read about every five minutes instead of with every status read; a stopped
// VM's is already read each poll for its start-on-boot setting and is reused.
const (
	vmNICConfigTTL          = 5 * time.Minute
	vmNICConfigTTLJitter    = 150 * time.Second
	vmNICConfigRetryBackoff = 2 * time.Minute
	// After a start every running VM is unread at once. Their first reads are
	// spread over this window so they do not double the first poll's requests.
	vmNICConfigStartupSpread = time.Minute
	vmNICConfigRetention     = 30 * time.Minute
	vmNICConfigSweepInterval = time.Minute
	vmNICConfigReadTimeout   = 5 * time.Second
)

var proxmoxNetDeviceKey = regexp.MustCompile(`^net\d+$`)

// vmNICConfigEntry is the last accepted reading of one VM's network devices
// and PCI passthrough accelerators.
type vmNICConfigEntry struct {
	devices      []containerNetworkDetails
	accelerators []string
	readAt       time.Time // last successful read; zero until one succeeds
	attemptedAt  time.Time // last read of either outcome
	failed       bool      // the last attempt failed; devices are the earlier reading
}

// vmNICConfigCache is usable from its zero value.
type vmNICConfigCache struct {
	mu        sync.Mutex
	entries   map[string]vmNICConfigEntry
	startedAt time.Time
	sweptAt   time.Time
}

// vmNICConfigJitter spreads guests deterministically across a window so
// refreshes never line up on one poll.
func vmNICConfigJitter(key string, window time.Duration) time.Duration {
	if window <= 0 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return time.Duration(uint64(hash.Sum32()) % uint64(window))
}

// due reports whether a read should be sent now, and returns the devices to
// use meanwhile.
func (c *vmNICConfigCache) due(key string, now time.Time) ([]containerNetworkDetails, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.startedAt.IsZero() {
		c.startedAt = now
	}
	entry, ok := c.entries[key]
	if !ok {
		return nil, !now.Before(c.startedAt.Add(vmNICConfigJitter(key, vmNICConfigStartupSpread)))
	}
	devices := cloneContainerNetworkDetails(entry.devices)
	if entry.failed {
		return devices, now.Sub(entry.attemptedAt) >= vmNICConfigRetryBackoff
	}
	return devices, now.Sub(entry.readAt) >= vmNICConfigTTL+vmNICConfigJitter(key, vmNICConfigTTLJitter)
}

func (c *vmNICConfigCache) store(key string, devices []containerNetworkDetails, now time.Time) {
	c.storeWithAccelerators(key, devices, nil, now)
}

func (c *vmNICConfigCache) storeWithAccelerators(key string, devices []containerNetworkDetails, accelerators []string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]vmNICConfigEntry)
	}
	c.entries[key] = vmNICConfigEntry{
		devices:      cloneContainerNetworkDetails(devices),
		accelerators: append([]string(nil), accelerators...),
		readAt:       now,
		attemptedAt:  now,
	}
	c.sweepLocked(now)
}

func (c *vmNICConfigCache) accelerators(key string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || len(entry.accelerators) == 0 {
		return nil
	}
	return append([]string(nil), entry.accelerators...)
}

func (c *vmNICConfigCache) fail(key string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]vmNICConfigEntry)
	}
	entry := c.entries[key]
	entry.attemptedAt = now
	entry.failed = true
	c.entries[key] = entry
	c.sweepLocked(now)
}

// sweepLocked forgets guests nothing has asked about for a while: a removed
// or migrated VM stops being polled under its old key.
func (c *vmNICConfigCache) sweepLocked(now time.Time) {
	if now.Sub(c.sweptAt) < vmNICConfigSweepInterval {
		return
	}
	c.sweptAt = now
	for key, entry := range c.entries {
		if now.Sub(entry.attemptedAt) > vmNICConfigRetention {
			delete(c.entries, key)
		}
	}
}

func cloneContainerNetworkDetails(in []containerNetworkDetails) []containerNetworkDetails {
	if len(in) == 0 {
		return nil
	}
	out := make([]containerNetworkDetails, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Addresses = append([]string(nil), in[i].Addresses...)
	}
	return out
}

// rememberVMNICConfig records a configuration this poll had to read anyway.
func (m *Monitor) rememberVMNICConfig(instanceName, node string, vmid int, config map[string]interface{}) []containerNetworkDetails {
	devices := parseVMConfigNetworks(config)
	accelerators := parseProxmoxVMConfigAccelerators(config)
	m.vmNICConfig.storeWithAccelerators(guestMetadataCacheKey(instanceName, node, vmid), devices, accelerators, time.Now())
	return devices
}

func (m *Monitor) cachedVMAccelerators(instanceName, node string, vmid int) []string {
	return m.vmNICConfig.accelerators(guestMetadataCacheKey(instanceName, node, vmid))
}

// runningVMNICConfig returns where a running VM's network devices are plugged
// in, reading its configuration only when the last reading has aged out. A
// closed enrichment budget, a client without configuration reads and a failed
// read all keep the earlier reading.
func (m *Monitor) runningVMNICConfig(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) []containerNetworkDetails {
	key := guestMetadataCacheKey(instanceName, node, vmid)
	now := time.Now()
	devices, due := m.vmNICConfig.due(key, now)
	if !due || ctx.Err() != nil {
		return devices
	}
	vmClient, ok := client.(vmConfigReader)
	if !ok {
		return devices
	}
	readCtx, cancel := context.WithTimeout(ctx, vmNICConfigReadTimeout)
	config, err := vmClient.GetVMConfig(readCtx, node, vmid)
	cancel()
	if err != nil || len(config) == 0 {
		m.vmNICConfig.fail(key, time.Now())
		return devices
	}
	return m.rememberVMNICConfig(instanceName, node, vmid, config)
}

// vmConfigReader is the configuration read only some PVE clients offer; it is
// not part of PVEClientInterface.
type vmConfigReader interface {
	GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
}

// parseVMConfigNetworks reads the netN lines of a QEMU guest's configuration,
// for example `virtio=BC:24:11:2E:5F:01,bridge=vmbr0,tag=30,firewall=1`. A
// device with no bridge (user-mode networking) has no placement and is left
// out. Name is the configuration key, since QEMU does not name the interface
// the guest will see.
func parseVMConfigNetworks(config map[string]interface{}) []containerNetworkDetails {
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, 4)
	for key := range config {
		if proxmoxNetDeviceKey.MatchString(strings.ToLower(strings.TrimSpace(key))) {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return netDeviceIndex(keys[i]) < netDeviceIndex(keys[j])
	})

	devices := make([]containerNetworkDetails, 0, len(keys))
	for _, key := range keys {
		device := containerNetworkDetails{Name: strings.ToLower(strings.TrimSpace(key))}
		for _, part := range strings.Split(fmt.Sprint(config[key]), ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(kv[0]))
			value := strings.TrimSpace(kv[1])
			switch name {
			case "bridge":
				device.Bridge = value
			case "tag":
				device.VLAN = parseProxmoxVLANTag(value)
			case "firewall":
				device.Firewall = parseProxmoxFlag(value)
			case "macaddr":
				device.MAC = normalizeMAC(value)
			default:
				// The first pair is `<model>=<mac>`, whatever the model is called.
				if device.MAC == "" {
					device.MAC = normalizeMAC(value)
				}
			}
		}
		if device.Bridge == "" {
			continue
		}
		devices = append(devices, device)
	}
	if len(devices) == 0 {
		return nil
	}
	return devices
}

func netDeviceIndex(key string) int {
	index, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(key)), "net"))
	if err != nil {
		return 0
	}
	return index
}

// normalizeMAC returns the upper-case colon form, or "" for anything that is
// not a 48-bit hardware address.
func normalizeMAC(value string) string {
	mac, err := net.ParseMAC(strings.TrimSpace(value))
	if err != nil || len(mac) != 6 {
		return ""
	}
	return strings.ToUpper(mac.String())
}

// parseProxmoxVLANTag accepts the string, number and JSON-number forms the
// PVE API uses, and returns 0 (untagged) for anything outside 1-4094.
func parseProxmoxVLANTag(value interface{}) int {
	if value == nil {
		return 0
	}
	tag, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
	if err != nil {
		if number, isFloat := value.(float64); isFloat && number == float64(int(number)) {
			tag = int(number)
		} else {
			return 0
		}
	}
	if tag < 1 || tag > 4094 {
		return 0
	}
	return tag
}

func parseProxmoxFlag(value interface{}) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// applyVMNICConfig stamps each guest-reported interface with where Proxmox
// plugs in the device carrying its hardware address, and adds a device the
// guest reported nothing for (no guest agent, or the VM is stopped) under its
// configuration key. It rewrites every placement, so a device removed from the
// configuration loses the one an earlier poll gave it.
func applyVMNICConfig(interfaces []models.GuestNetworkInterface, devices []containerNetworkDetails) []models.GuestNetworkInterface {
	out := guestReportedInterfaces(interfaces)
	for i := range out {
		out[i].Bridge, out[i].VLAN, out[i].Firewall = "", 0, false
	}
	for _, device := range devices {
		if device.Bridge == "" {
			continue
		}
		matched := false
		if device.MAC != "" {
			for i := range out {
				if strings.EqualFold(strings.TrimSpace(out[i].MAC), device.MAC) {
					out[i].Bridge, out[i].VLAN, out[i].Firewall = device.Bridge, device.VLAN, device.Firewall
					matched = true
				}
			}
		}
		if matched {
			continue
		}
		out = append(out, models.GuestNetworkInterface{
			Name:      device.Name,
			MAC:       device.MAC,
			Addresses: []string{},
			Bridge:    device.Bridge,
			VLAN:      device.VLAN,
			Firewall:  device.Firewall,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isPlacementOnlyInterface reports an entry applyVMNICConfig added for a
// device the guest itself said nothing about.
func isPlacementOnlyInterface(iface models.GuestNetworkInterface) bool {
	return iface.Bridge != "" &&
		len(iface.Addresses) == 0 &&
		iface.RXBytes == 0 &&
		iface.TXBytes == 0 &&
		proxmoxNetDeviceKey.MatchString(iface.Name)
}

// guestReportedInterfaces returns a copy without placement-only entries. What
// is left is what the guest reported, which is the only part that counts as
// evidence that something inside the guest answered.
func guestReportedInterfaces(interfaces []models.GuestNetworkInterface) []models.GuestNetworkInterface {
	if len(interfaces) == 0 {
		return nil
	}
	out := make([]models.GuestNetworkInterface, 0, len(interfaces))
	for _, iface := range cloneGuestNetworkInterfaces(interfaces) {
		if isPlacementOnlyInterface(iface) {
			continue
		}
		out = append(out, iface)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
