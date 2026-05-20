import { describe, expect, it } from 'vitest';
import {
  getDiscoveryApiAccessSettingsTarget,
  getDiscoveryCommandSettingsTarget,
  getDiscoveryIdentifiedSummary,
  getDiscoveryInitialEmptyState,
  getDiscoveryLoadingState,
  getDiscoveryNoConnectedAgentMessage,
  getDiscoveryProvenanceBadgeClass,
  getDiscoveryProvenanceIconClass,
  getDiscoveryProvenanceLabel,
  getDiscoveryProvenanceTitle,
  getNetworkDiscoveryModePresentation,
  getDiscoveryNotesEmptyState,
  getDiscoveryObservedSourceLabel,
  getNetworkDiscoveryPriorityNotice,
  getNetworkDiscoverySectionPresentation,
  getNetworkDiscoverySubnetPresentation,
  getDiscoveryAnalysisProviderBadgeClass,
  getDiscoveryCategoryBadgeClass,
  getDiscoverySuggestedURLActionClass,
  getDiscoverySuggestedURLCardClass,
  getDiscoverySuggestedURLFallback,
  getDiscoverySuggestedURLReason,
  getDiscoveryURLSuggestionSourceLabel,
} from '@/utils/discoveryPresentation';
import type { ResourceDiscovery } from '@/types/discovery';

describe('discoveryPresentation', () => {
  it('returns canonical proxmox profile labels', () => {
    expect(getDiscoveryURLSuggestionSourceLabel('host_management_profile_pve')).toBe(
      'Proxmox node profile',
    );
    expect(getDiscoveryURLSuggestionSourceLabel('host_management_profile_pbs')).toBe(
      'Proxmox Backup profile',
    );
    expect(getDiscoveryURLSuggestionSourceLabel('host_management_profile_pmg')).toBe(
      'Proxmox Mail Gateway profile',
    );
  });

  it('falls back to the generic discovery heuristic label', () => {
    expect(getDiscoveryURLSuggestionSourceLabel('')).toBe('Discovery heuristic');
    expect(getDiscoveryURLSuggestionSourceLabel('unknown-code')).toBe('Discovery heuristic');
    expect(getDiscoverySuggestedURLReason(null)).toEqual({ text: '', title: '' });
    expect(
      getDiscoverySuggestedURLReason({
        suggested_url_source_code: 'web_port_inference',
        suggested_url_source_detail: 'detected port 8080/tcp',
      }),
    ).toEqual({
      text: 'Detected port 8080/tcp',
      title: 'Detected web port: detected port 8080/tcp',
    });
  });

  it('returns canonical analysis provider badge classes', () => {
    expect(getDiscoveryAnalysisProviderBadgeClass(true)).toContain('bg-green-100');
    expect(getDiscoveryAnalysisProviderBadgeClass(false)).toContain('bg-blue-100');
  });

  it('returns a canonical discovery category badge class', () => {
    expect(getDiscoveryCategoryBadgeClass()).toContain('bg-blue-100');
    expect(getDiscoveryCategoryBadgeClass()).toContain('text-blue-700');
    expect(getDiscoverySuggestedURLCardClass()).toContain('bg-blue-50');
    expect(getDiscoverySuggestedURLActionClass()).toContain('text-blue-700');
    expect(getDiscoveryProvenanceLabel()).toBe('Discovery');
    expect(getDiscoveryProvenanceTitle()).toContain('opt-in Discovery');
    expect(getDiscoveryProvenanceBadgeClass()).toContain('border-cyan-200');
    expect(getDiscoveryProvenanceIconClass()).toContain('h-4');
  });

  it('returns canonical discovery empty-state copy', () => {
    expect(getDiscoveryInitialEmptyState(true)).toEqual({
      title: 'Checking existing discovery data...',
      description: 'You can run a discovery scan if this takes too long.',
    });
    expect(getDiscoveryInitialEmptyState(false)).toEqual({
      title: 'No discovery data yet',
      description: 'Run a discovery scan to identify services and configuration details.',
    });
    expect(getDiscoverySuggestedURLFallback('diag')).toEqual({
      title: 'No suggested URL available',
      description: 'diag',
    });
    expect(getDiscoveryLoadingState()).toEqual({
      text: 'Loading discovery data...',
    });
    expect(getDiscoveryNotesEmptyState()).toEqual({
      text: 'No discovery notes yet. Add notes to capture important context.',
    });
  });

  it('returns canonical discovery command guidance targets and errors', () => {
    const commandSettingsTarget = getDiscoveryCommandSettingsTarget();
    const disconnectedMessage = getDiscoveryNoConnectedAgentMessage();

    expect(commandSettingsTarget).toEqual({
      href: '/settings/infrastructure',
      label: 'Settings → Infrastructure',
    });
    expect(commandSettingsTarget.label).not.toContain('Settings → Infrastructure → Proxmox');
    expect(getDiscoveryApiAccessSettingsTarget()).toEqual({
      href: '/settings/security/api',
      label: 'Settings → API Access',
    });
    expect(getDiscoveryNoConnectedAgentMessage(false)).toBe(
      'Commands not enabled. Enable Pulse commands from Settings → Infrastructure for this agent.',
    );
    expect(getDiscoveryNoConnectedAgentMessage(true)).toBe(
      'Agent not connected for command execution. The API token may be missing the "agent:exec" scope. Check Settings → API Access.',
    );
    expect(disconnectedMessage).toBe(
      'No agent available for command execution. Enable Pulse commands from Settings → Infrastructure and make sure the API token has "agent:exec" scope in Settings → API Access.',
    );
    expect(disconnectedMessage).not.toContain('Settings → Infrastructure → Proxmox');
  });

  it('returns canonical network discovery settings copy', () => {
    expect(getNetworkDiscoveryPriorityNotice()).toEqual({
      title: 'Network scan safety',
      items: [
        'Environment variables still override these settings.',
        'Changes made here are saved to system.json immediately.',
        'Automatic mode can scan every detected interface, including bridge or shared networks; use custom subnets when scope matters.',
      ],
    });

    expect(getNetworkDiscoverySectionPresentation(true)).toEqual({
      headerTitle: 'Network discovery',
      headerDescription: 'Control how Pulse scans your network for Proxmox services.',
      toggleTitle: 'Automatic scanning',
      toggleDescription:
        'Enable discovery to surface Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway endpoints automatically.',
      toggleStateLabel: 'Enabled',
      scanScopeLabel: 'Scan scope',
      commonNetworksLabel: 'Common networks',
      environmentOverrideMessage:
        'Discovery settings are locked by environment variables. Update the service configuration and restart Pulse to change them here.',
    });

    expect(getNetworkDiscoveryModePresentation('auto')).toEqual({
      label: 'Automatic scan (full network scope)',
      description:
        'Scan every network interface on this host, including container bridges, local subnets, and gateways. Use custom subnets on large or shared networks to reduce scan time.',
    });
    expect(getNetworkDiscoveryModePresentation('custom')).toEqual({
      label: 'Custom subnets (targeted)',
      description:
        'Limit discovery to one or more CIDR ranges for faster, more targeted scans on large networks.',
    });

    expect(getNetworkDiscoverySubnetPresentation('auto')).toEqual({
      label: 'Discovery subnets',
      helpTooltip:
        'Use CIDR notation, for example 192.168.1.0/24 or 10.0.0.0/24. Smaller ranges finish more quickly.',
      placeholder: 'automatic (scan every detected network)',
      guidance:
        'Automatic mode scans all host network interfaces, which can include shared or corporate networks. Switch to custom subnets for a faster, more targeted scan.',
    });
    expect(getNetworkDiscoverySubnetPresentation('custom')).toEqual({
      label: 'Discovery subnets',
      helpTooltip:
        'Use CIDR notation, for example 192.168.1.0/24 or 10.0.0.0/24. Smaller ranges finish more quickly.',
      placeholder: '192.168.1.0/24, 10.0.0.0/24',
      guidance:
        'Example: 192.168.1.0/24, 10.0.0.0/24. Smaller ranges finish faster and reduce timeout risk.',
    });
  });

  it('compacts a populated discovery record into the identified-service summary', () => {
    const record: ResourceDiscovery = {
      id: 'system-container:delly:141',
      resource_type: 'system-container',
      resource_id: '141',
      target_id: 'delly',
      service_name: 'Homepage Dashboard',
      service_type: 'homepage',
      service_version: '0.9.0',
      category: 'web_server',
      confidence: 0.95,
      cli_access: 'docker exec -it homepage /bin/sh',
      ports: [
        { port: 3000, protocol: 'tcp', process: 'next-server', address: '0.0.0.0' },
      ] as unknown as ResourceDiscovery['ports'],
      facts: [
        {
          key: 'os',
          value: 'Debian 12',
          source: 'os_release',
          category: 'security',
          confidence: 1,
          discovered_at: '',
        },
      ] as unknown as ResourceDiscovery['facts'],
      config_paths: ['/opt/homepage/config'],
      data_paths: ['/opt/homepage/config'],
      log_paths: ['/var/log/syslog', '/var/log/daemon.log'],
      discovered_at: '2026-05-17T09:49:19.049058+01:00',
      updated_at: '2026-05-18T09:49:19.049058+01:00',
      suggested_url: 'http://192.0.2.10:3000',
      suggested_url_source_code: 'web_port_inference',
      suggested_url_source_detail: 'detected 3000/tcp',
    } as ResourceDiscovery;
    expect(getDiscoveryIdentifiedSummary(record)).toEqual({
      serviceName: 'Homepage Dashboard',
      serviceType: 'homepage',
      serviceVersion: '0.9.0',
      category: 'web_server',
      confidence: 0.95,
      confidencePercent: '95%',
      cliAccess: 'docker exec -it homepage /bin/sh',
      portCount: 1,
      configPathCount: 1,
      dataPathCount: 1,
      logPathCount: 2,
      discoveredAt: '2026-05-17T09:49:19.049058+01:00',
      observedAt: '2026-05-18T09:49:19.049058+01:00',
      sourceLabel: getDiscoveryObservedSourceLabel(),
      suggestedUrl: 'http://192.0.2.10:3000',
      suggestedUrlReasonText: 'Detected 3000/tcp',
      suggestedUrlReasonTitle: 'Detected web port: detected 3000/tcp',
      suggestedUrlDiagnostic: undefined,
      hasEndpointCandidate: true,
    });
  });

  it('treats an endpoint-only discovery record as useful out-of-tab context', () => {
    expect(
      getDiscoveryIdentifiedSummary({
        id: 'docker:agent:container',
        resource_type: 'docker',
        resource_id: 'container',
        target_id: 'agent',
        service_name: 'Unknown',
        confidence: 0,
        ports: [],
        facts: [
          {
            category: 'service',
            key: 'status',
            value: 'online',
            source: 'metadata',
            confidence: 1,
            discovered_at: '2026-05-19T00:00:00Z',
          },
          {
            category: 'config',
            key: 'missing_config',
            value: 'nodes/delly/lxc/152.conf not found on host',
            source: 'all_commands',
            confidence: 1,
            discovered_at: '2026-05-19T00:00:00Z',
          },
        ],
        config_paths: [],
        data_paths: [],
        log_paths: [],
        suggested_url: 'http://192.0.2.10:8123',
      } as unknown as ResourceDiscovery),
    ).toMatchObject({
      serviceName: 'Unidentified service',
      suggestedUrl: 'http://192.0.2.10:8123',
      hasEndpointCandidate: true,
    });
  });

  it('collapses empty / low-signal discovery records to null so out-of-tab surfaces hide cleanly', () => {
    expect(getDiscoveryIdentifiedSummary(null)).toBeNull();
    expect(getDiscoveryIdentifiedSummary(undefined)).toBeNull();
    // Empty record: no service name, confidence zero, no ports / facts / paths / cli.
    expect(
      getDiscoveryIdentifiedSummary({
        id: 'docker:agent:container',
        resource_type: 'docker',
        resource_id: 'container',
        target_id: 'agent',
        service_name: 'Unknown',
        confidence: 0,
        ports: [],
        facts: [
          {
            category: 'service',
            key: 'status',
            value: 'online',
            source: 'metadata',
            confidence: 1,
            discovered_at: '2026-05-19T00:00:00Z',
          },
        ],
        config_paths: [],
        data_paths: [],
        log_paths: [],
      } as unknown as ResourceDiscovery),
    ).toBeNull();
    expect(
      getDiscoveryIdentifiedSummary({
        id: 'system-container:pve4:152',
        resource_type: 'system-container',
        resource_id: '152',
        target_id: 'pve4',
        service_name: 'Unknown Container',
        service_type: 'unknown',
        service_version: 'unknown',
        category: 'unknown',
        confidence: 0,
        cli_access: 'pct exec 152 -- /bin/bash',
        ports: [],
        facts: [],
        config_paths: [],
        data_paths: [],
        log_paths: [],
        suggested_url_diagnostic: 'no host or IP candidate available',
      } as unknown as ResourceDiscovery),
    ).toBeNull();
    expect(
      getDiscoveryIdentifiedSummary({
        id: 'system-container:pve4:152',
        resource_type: 'system-container',
        resource_id: '152',
        target_id: 'pve4',
        service_name: '',
        service_type: 'service',
        service_version: '',
        category: 'unknown',
        confidence: 0,
        cli_access: 'pct exec 152 -- /bin/bash',
        ports: [],
        facts: [],
        config_paths: [],
        data_paths: [],
        log_paths: [],
        suggested_url_diagnostic: 'no host or IP candidate available',
      } as unknown as ResourceDiscovery),
    ).toBeNull();
  });
});
