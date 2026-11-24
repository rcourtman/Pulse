package configapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// importRefusal is an archive the request's actor may not apply. message is
// written for that actor; cause is the check that failed, when there is one.
type importRefusal struct {
	message string
	cause   error
}

func (e *importRefusal) Error() string { return e.message }

func (e *importRefusal) Unwrap() error { return e.cause }

// checkTenantImport holds a decoded archive to the limits the settings routes
// put on a tenant organization's administrator (see
// securityutil.TenantOutboundPolicy). An archive is sealed only with a
// passphrase its submitter chose, so the addresses and Apprise targets in it
// are that caller's input as much as a settings request body is. A request
// with instance authority carries no policy and passes.
func checkTenantImport(ctx context.Context, archive *config.ExportData, saved *config.ConfigPersistence) error {
	if _, tenant := securityutil.TenantOutboundPolicyFromContext(ctx); !tenant {
		return nil
	}

	// Check the address each client is built with, not the archive's spelling
	// of it: the Create*Config helpers apply the normalization that loading
	// the imported nodes persists.
	for i := range archive.Nodes.PVEInstances {
		pve := &archive.Nodes.PVEInstances[i]
		owner := fmt.Sprintf("Proxmox VE connection %q", pve.Name)
		if err := checkTenantImportDestination(ctx, owner, config.CreateProxmoxConfig(pve).Host); err != nil {
			return err
		}
		for _, member := range pve.ClusterEndpoints {
			memberOwner := fmt.Sprintf("%s, cluster member %q", owner, member.NodeName)
			// A member's Host is the node name Proxmox reports, kept for TLS
			// verification. Pulse often cannot resolve it and connects to the
			// member's IP instead, so a name without a DNS record is not
			// refused: there is no address to reach. A name that is, or
			// resolves to, an excluded address still is.
			host := config.CreateProxmoxConfigWithHost(pve, member.Host, true).Host
			if err := checkTenantImportDestination(ctx, memberOwner, host); err != nil && !nameNotFound(err) {
				return err
			}
			for _, address := range []string{member.IP, member.IPOverride} {
				address = strings.TrimSpace(address)
				if address == "" {
					continue
				}
				// Member addresses are stored without a scheme and the URL is
				// built around them at poll time. Anything but the stored
				// host[:port] form could name one host to this check and
				// another to that URL.
				if stored, err := normalizeClusterEndpointIPOverride(address); err != nil || stored != address {
					return tenantImportDestinationRefusal(ctx, memberOwner,
						fmt.Errorf("%w: %q is not a host or IP address", securityutil.ErrTenantOutboundDestination, address))
				}
				if err := checkTenantImportDestination(ctx, memberOwner, address); err != nil {
					return err
				}
			}
		}
	}
	for i := range archive.Nodes.PBSInstances {
		pbs := &archive.Nodes.PBSInstances[i]
		if err := checkTenantImportDestination(ctx, fmt.Sprintf("Proxmox Backup Server connection %q", pbs.Name), config.CreatePBSConfig(pbs).Host); err != nil {
			return err
		}
	}
	for i := range archive.Nodes.PMGInstances {
		pmg := &archive.Nodes.PMGInstances[i]
		if err := checkTenantImportDestination(ctx, fmt.Sprintf("Proxmox Mail Gateway connection %q", pmg.Name), config.CreatePMGConfig(pmg).Host); err != nil {
			return err
		}
	}
	if err := checkTenantImportDestination(ctx, "SMTP server", archive.Email.SMTPHost); err != nil {
		return err
	}

	// Apprise command-line targets run as this server and outside every
	// outbound check, so only an instance administrator introduces them (the
	// notification routes' refuseAppriseCLI). Targets already saved add no
	// authority, so an organization's own backup still restores.
	apprise := notifications.NormalizeAppriseConfig(archive.Apprise)
	if apprise.Mode == notifications.AppriseModeCLI && len(apprise.Targets) > 0 {
		var current notifications.AppriseConfig
		if loaded, err := saved.LoadAppriseConfig(); err == nil && loaded != nil {
			current = notifications.NormalizeAppriseConfig(*loaded)
		}
		if current.Mode != notifications.AppriseModeCLI || !slices.Equal(apprise.Targets, current.Targets) {
			return &importRefusal{message: "Backup not imported. Its Apprise command-line targets need an instance administrator. Use an Apprise API server instead, or ask an instance administrator to import this backup."}
		}
	}
	return nil
}

// checkTenantImportDestination refuses an address the request's tenant
// outbound policy excludes. An empty address is not a destination.
func checkTenantImportDestination(ctx context.Context, owner, destination string) error {
	if strings.TrimSpace(destination) == "" {
		return nil
	}
	if err := securityutil.CheckTenantOutboundDestination(ctx, destination); err != nil {
		return tenantImportDestinationRefusal(ctx, owner, err)
	}
	return nil
}

func tenantImportDestinationRefusal(ctx context.Context, owner string, err error) error {
	return &importRefusal{
		message: "Backup not imported. " + securityutil.TenantOutboundRefusalMessage(ctx, fmt.Errorf("%s: %w", owner, err)),
		cause:   err,
	}
}

// nameNotFound reports a refusal whose only cause is that the name has no DNS
// record. A lookup that failed for any other reason stays a refusal.
func nameNotFound(err error) bool {
	var lookup *net.DNSError
	return errors.As(err, &lookup) && lookup.IsNotFound
}
