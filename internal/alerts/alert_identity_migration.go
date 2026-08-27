package alerts

import (
	"sort"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// CurrentAlertIdentitySchemaVersion is the persisted alert configuration
// identity schema understood by this release.
const CurrentAlertIdentitySchemaVersion = 1

// AlertIdentityMigrationDeferred describes an override which the dry-run
// planner deliberately refused to move. The source row remains untouched.
type AlertIdentityMigrationDeferred struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// AlertIdentityMigrationPlan is a non-mutating preview of the versioned
// identity migration. ApplyAlertIdentityMigration applies the exact planned
// override snapshot only if the source schema version is still current.
type AlertIdentityMigrationPlan struct {
	FromVersion         int                              `json:"fromVersion"`
	ToVersion           int                              `json:"toVersion"`
	RemovedOverrideKeys []string                         `json:"removedOverrideKeys,omitempty"`
	AddedOverrideKeys   []string                         `json:"addedOverrideKeys,omitempty"`
	Deferred            []AlertIdentityMigrationDeferred `json:"deferred,omitempty"`
	UnsupportedVersion  bool                             `json:"unsupportedVersion,omitempty"`

	plannedOverrides map[string]ThresholdConfig
}

// Changed reports whether applying the plan would change persisted config.
func (p AlertIdentityMigrationPlan) Changed() bool {
	return !p.UnsupportedVersion && (p.FromVersion != p.ToVersion || len(p.RemovedOverrideKeys) > 0 || len(p.AddedOverrideKeys) > 0)
}

// PlanAlertIdentityMigration builds a dry-run migration from the live unified
// resource snapshot. Unknown and ambiguous identities fail closed and remain
// in the result for a later poll that can prove their ownership.
func PlanAlertIdentityMigration(config AlertConfig, resources []unifiedresources.Resource) AlertIdentityMigrationPlan {
	plan := AlertIdentityMigrationPlan{
		FromVersion: config.IdentitySchemaVersion,
		ToVersion:   CurrentAlertIdentitySchemaVersion,
	}
	if config.IdentitySchemaVersion > CurrentAlertIdentitySchemaVersion {
		plan.ToVersion = config.IdentitySchemaVersion
		plan.UnsupportedVersion = true
		return plan
	}

	original := cloneThresholdOverrides(config.Overrides)
	working := config
	working.Overrides = cloneThresholdOverrides(config.Overrides)

	MigrateCanonicalOverrideKeys(&working, resources)
	MigrateDockerContainerOverrideKeys(&working, resources)
	plan.Deferred = append(plan.Deferred, migrateGuestOverrideKeys(&working, resources)...)
	plan.Deferred = append(plan.Deferred, migrateStorageOverrideKeys(&working, resources)...)
	working.IdentitySchemaVersion = CurrentAlertIdentitySchemaVersion

	for key := range original {
		if _, exists := working.Overrides[key]; !exists {
			plan.RemovedOverrideKeys = append(plan.RemovedOverrideKeys, key)
		}
	}
	for key := range working.Overrides {
		if _, exists := original[key]; !exists {
			plan.AddedOverrideKeys = append(plan.AddedOverrideKeys, key)
		}
	}
	sort.Strings(plan.RemovedOverrideKeys)
	sort.Strings(plan.AddedOverrideKeys)
	sort.Slice(plan.Deferred, func(i, j int) bool {
		if plan.Deferred[i].Key == plan.Deferred[j].Key {
			return plan.Deferred[i].Reason < plan.Deferred[j].Reason
		}
		return plan.Deferred[i].Key < plan.Deferred[j].Key
	})
	plan.plannedOverrides = cloneThresholdOverrides(working.Overrides)
	return plan
}

// ApplyAlertIdentityMigration applies a previously generated plan. A stale
// plan is rejected if another writer changed the schema version in between.
func ApplyAlertIdentityMigration(config *AlertConfig, plan AlertIdentityMigrationPlan) bool {
	if config == nil || !plan.Changed() || plan.UnsupportedVersion || config.IdentitySchemaVersion != plan.FromVersion {
		return false
	}
	config.Overrides = cloneThresholdOverrides(plan.plannedOverrides)
	config.IdentitySchemaVersion = plan.ToVersion
	return true
}

func cloneThresholdOverrides(source map[string]ThresholdConfig) map[string]ThresholdConfig {
	if source == nil {
		return nil
	}
	cloned := make(map[string]ThresholdConfig, len(source))
	for key, override := range source {
		cloned[key] = override
	}
	return cloned
}

type overrideAliasClaims struct {
	targets   map[string]string
	ambiguous map[string]struct{}
}

func newOverrideAliasClaims() *overrideAliasClaims {
	return &overrideAliasClaims{
		targets:   make(map[string]string),
		ambiguous: make(map[string]struct{}),
	}
}

func (c *overrideAliasClaims) add(alias, target string) {
	alias = strings.TrimSpace(alias)
	target = strings.TrimSpace(target)
	if alias == "" || target == "" || alias == target {
		return
	}
	if existing, exists := c.targets[alias]; exists && existing != target {
		delete(c.targets, alias)
		c.ambiguous[alias] = struct{}{}
		return
	}
	if _, conflict := c.ambiguous[alias]; conflict {
		return
	}
	c.targets[alias] = target
}

func migrateGuestOverrideKeys(config *AlertConfig, resources []unifiedresources.Resource) []AlertIdentityMigrationDeferred {
	claims := newOverrideAliasClaims()
	for _, resource := range resources {
		typeKey := unifiedresources.CanonicalResourceType(resource.Type)
		if typeKey != unifiedresources.ResourceTypeVM && typeKey != unifiedresources.ResourceTypeSystemContainer {
			continue
		}
		if resource.Proxmox == nil {
			continue
		}
		ident, ok := guestOverrideIdentityFromParts(resource.Proxmox.Instance, resource.Proxmox.NodeName, resource.Proxmox.VMID)
		if !ok {
			continue
		}
		vmid := strconv.Itoa(ident.vmid)
		target := guestOverridePrimaryKey(nil, strings.Join([]string{ident.instance, ident.node, vmid}, ":"))
		aliases := []string{
			resource.ID,
			strings.Join([]string{ident.instance, ident.node, vmid}, ":"),
			legacyGuestStableOverrideKey(ident.instance, ident.vmid),
		}
		if ident.instance != ident.node {
			aliases = append(aliases, legacyClusterGuestOverrideKey(ident.instance, ident.node, ident.vmid))
		}
		aliases = append(aliases, resource.SupersededCanonicalIDs...)
		for _, alias := range aliases {
			claims.add(alias, target)
		}
	}
	return applyOverrideAliasClaims(config, claims, true)
}

func migrateStorageOverrideKeys(config *AlertConfig, resources []unifiedresources.Resource) []AlertIdentityMigrationDeferred {
	claims := newOverrideAliasClaims()
	for _, resource := range resources {
		if unifiedresources.CanonicalResourceType(resource.Type) != unifiedresources.ResourceTypeStorage {
			continue
		}
		target := ""
		if resource.MetricsTarget != nil && strings.EqualFold(resource.MetricsTarget.ResourceType, "storage") {
			target = strings.TrimSpace(resource.MetricsTarget.ResourceID)
		}
		if target == "" && resource.Proxmox != nil {
			target = strings.TrimSpace(resource.Proxmox.SourceID)
		}
		if target == "" {
			target = strings.TrimSpace(resource.ID)
		}
		aliases := append([]string{resource.ID}, resource.SupersededCanonicalIDs...)
		if resource.Storage != nil {
			aliases = append(aliases, resource.Storage.AliasIDs...)
			if resource.Storage.Shared && resource.Proxmox != nil {
				for _, node := range resource.Storage.Nodes {
					prefix := strings.TrimSpace(node)
					instance := strings.TrimSpace(resource.Proxmox.Instance)
					if instance != "" && prefix != "" && !strings.HasPrefix(strings.ToLower(prefix), strings.ToLower(instance)+"-") {
						prefix = instance + "-" + prefix
					}
					if prefix != "" && strings.TrimSpace(resource.Name) != "" {
						aliases = append(aliases, prefix+"-"+strings.TrimSpace(resource.Name))
					}
				}
			}
			if strings.EqualFold(resource.Storage.Platform, "pbs") {
				if slash := strings.LastIndex(target, "/"); slash > 0 {
					aliases = append(aliases, target[:slash]+"-"+target[slash+1:])
				}
			}
		}
		for _, alias := range aliases {
			claims.add(alias, target)
			if cephAlias, ok := cephPoolStorageSourceAliasID(alias); ok {
				claims.add(cephAlias, target)
			}
		}
	}
	return applyOverrideAliasClaims(config, claims, false)
}

func applyOverrideAliasClaims(config *AlertConfig, claims *overrideAliasClaims, includeGuestDisks bool) []AlertIdentityMigrationDeferred {
	if config == nil || len(config.Overrides) == 0 {
		return nil
	}
	original := cloneThresholdOverrides(config.Overrides)
	moves := make(map[string][]string)
	deferred := make([]AlertIdentityMigrationDeferred, 0)
	for key := range original {
		alias := key
		suffix := ""
		if includeGuestDisks && strings.HasPrefix(key, "guest-disk:") {
			if split := strings.Index(strings.TrimPrefix(key, "guest-disk:"), "/disk:"); split >= 0 {
				baseAndSuffix := strings.TrimPrefix(key, "guest-disk:")
				alias = baseAndSuffix[:split]
				suffix = baseAndSuffix[split:]
			}
		}
		if _, conflict := claims.ambiguous[alias]; conflict {
			deferred = append(deferred, AlertIdentityMigrationDeferred{Key: key, Reason: "ambiguous live identity"})
			continue
		}
		target, claimed := claims.targets[alias]
		if !claimed {
			continue
		}
		if suffix != "" {
			target = "guest-disk:" + target + suffix
		}
		if target != key {
			moves[target] = append(moves[target], key)
		}
	}

	changed := false
	result := cloneThresholdOverrides(original)
	for target, sources := range moves {
		sort.Strings(sources)
		if _, targetExists := original[target]; targetExists {
			for _, source := range sources {
				delete(result, source)
				changed = true
			}
			continue
		}
		if len(sources) != 1 {
			for _, source := range sources {
				deferred = append(deferred, AlertIdentityMigrationDeferred{Key: source, Reason: "multiple legacy rows claim one canonical identity"})
			}
			continue
		}
		source := sources[0]
		result[target] = original[source]
		delete(result, source)
		changed = true
	}
	if changed {
		config.Overrides = result
	}
	return deferred
}
