package cloudcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

type ProofTenantAuditItem struct {
	TenantID  string
	AccountID string
	Email     string
	State     registry.TenantState
	CreatedAt time.Time
	Age       time.Duration
}

type CloudAuditReport struct {
	OK                       bool
	Failures                 []string
	Storage                  *StorageGuardrailReport
	TenantCounts             map[registry.TenantState]int
	TenantTotal              int
	RegistryUnhealthyActive  int
	DockerManagedTotal       int
	DockerManagedRunning     int
	DockerManagedUnhealthy   int
	DockerUnavailable        string
	StaleProofTenants        []ProofTenantAuditItem
	ManagedRuntimeContainers []cpDocker.RuntimeContainerSummary
}

func AuditCloud(ctx context.Context, cfg *CPConfig) (*CloudAuditReport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	report := &CloudAuditReport{
		OK:           true,
		TenantCounts: make(map[registry.TenantState]int),
	}

	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}
	defer reg.Close()

	tenants, err := reg.List()
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	report.TenantTotal = len(tenants)
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		report.TenantCounts[tenant.State]++
		if tenant.State == registry.TenantStateActive && !tenant.HealthCheckOK {
			report.RegistryUnhealthyActive++
		}
	}
	if report.RegistryUnhealthyActive > 0 {
		report.addFailure(fmt.Sprintf("%d active tenants are unhealthy in the registry", report.RegistryUnhealthyActive))
	}
	report.StaleProofTenants = findStaleProofTenants(tenants, cfg.ProofTenantMatchers, cfg.ProofTenantMaxAge, time.Now().UTC())
	if len(report.StaleProofTenants) > 0 {
		report.addFailure(fmt.Sprintf("%d proof/canary tenants are older than %s", len(report.StaleProofTenants), cfg.ProofTenantMaxAge))
	}

	dockerMgr, err := cpDocker.NewManager(cpDocker.ManagerConfig{
		Image:                    cfg.PulseImage,
		Network:                  cfg.DockerNetwork,
		BaseDomain:               baseDomainFromURL(cfg.BaseURL),
		TrialActivationPublicKey: cfg.TrialActivationPublicKey,
		TrustedProxyCIDRs:        cfg.TrustedProxyCIDRs,
		MemoryLimit:              cfg.TenantMemoryLimit,
		CPUShares:                cfg.TenantCPUShares,
		TenantLogMaxSize:         cfg.TenantLogMaxSize,
		TenantLogMaxFile:         cfg.TenantLogMaxFile,
	})
	if err != nil {
		report.DockerUnavailable = err.Error()
		report.addFailure("docker manager unavailable: " + err.Error())
	} else {
		defer dockerMgr.Close()
		containers, listErr := dockerMgr.ListManagedRuntimeContainers(ctx)
		if listErr != nil {
			report.DockerUnavailable = listErr.Error()
			report.addFailure("managed tenant container audit failed: " + listErr.Error())
		} else {
			report.ManagedRuntimeContainers = containers
			report.DockerManagedTotal = len(containers)
			for _, container := range containers {
				if container.State == "running" {
					report.DockerManagedRunning++
				}
				if runtimeContainerUnhealthy(container) {
					report.DockerManagedUnhealthy++
				}
			}
			if report.DockerManagedUnhealthy > 0 {
				report.addFailure(fmt.Sprintf("%d managed tenant containers are not healthy", report.DockerManagedUnhealthy))
			}
		}
	}

	storage, err := CheckStorageGuardrails(ctx, cfg, dockerMgr)
	if err != nil {
		return nil, err
	}
	report.Storage = storage
	if !storage.OK {
		report.addFailure("storage guardrails failed: " + strings.Join(storage.Failures, "; "))
	}
	return report, nil
}

func (r *CloudAuditReport) addFailure(failure string) {
	if r == nil {
		return
	}
	failure = strings.TrimSpace(failure)
	if failure == "" {
		return
	}
	r.OK = false
	r.Failures = append(r.Failures, failure)
}

func runtimeContainerUnhealthy(container cpDocker.RuntimeContainerSummary) bool {
	if container.State != "running" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(container.HealthStatus)) {
	case "", "none", "healthy":
		return false
	default:
		return true
	}
}

func findStaleProofTenants(tenants []*registry.Tenant, matchers []string, maxAge time.Duration, now time.Time) []ProofTenantAuditItem {
	if maxAge <= 0 {
		return nil
	}
	cutoff := now.Add(-maxAge)
	items := make([]ProofTenantAuditItem, 0)
	for _, tenant := range tenants {
		if tenant == nil || tenant.CreatedAt.IsZero() || tenant.CreatedAt.After(cutoff) {
			continue
		}
		if !matchesProofTenant(tenant, matchers) {
			continue
		}
		createdAt := tenant.CreatedAt.UTC()
		items = append(items, ProofTenantAuditItem{
			TenantID:  strings.TrimSpace(tenant.ID),
			AccountID: strings.TrimSpace(tenant.AccountID),
			Email:     strings.TrimSpace(tenant.Email),
			State:     tenant.State,
			CreatedAt: createdAt,
			Age:       now.Sub(createdAt),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].TenantID < items[j].TenantID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func matchesProofTenant(tenant *registry.Tenant, matchers []string) bool {
	if tenant == nil {
		return false
	}
	parts := []string{
		tenant.ID,
		tenant.AccountID,
		tenant.Email,
		tenant.DisplayName,
		tenant.StripeCustomerID,
		tenant.StripeSubscriptionID,
		tenant.StripePriceID,
		tenant.PlanVersion,
	}
	haystack := strings.ToLower(strings.Join(parts, " "))
	for _, matcher := range matchers {
		matcher = strings.ToLower(strings.TrimSpace(matcher))
		if matcher == "" {
			continue
		}
		if strings.Contains(haystack, matcher) {
			return true
		}
	}
	return false
}
