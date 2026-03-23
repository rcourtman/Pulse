import { describe, expect, it } from 'vitest';
import {
  getDiscoveryInitialEmptyState,
  getDiscoveryLoadingState,
  getNetworkDiscoveryModePresentation,
  getDiscoveryNotesEmptyState,
  getNetworkDiscoveryPriorityNotice,
  getNetworkDiscoverySectionPresentation,
  getNetworkDiscoverySubnetPresentation,
  getDiscoveryAnalysisProviderBadgeClass,
  getDiscoveryCategoryBadgeClass,
  getDiscoverySuggestedURLFallback,
  getDiscoveryURLSuggestionSourceLabel,
} from '@/utils/discoveryPresentation';

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
  });

  it('returns canonical analysis provider badge classes', () => {
    expect(getDiscoveryAnalysisProviderBadgeClass(true)).toContain('bg-green-100');
    expect(getDiscoveryAnalysisProviderBadgeClass(false)).toContain('bg-blue-100');
  });

  it('returns a canonical discovery category badge class', () => {
    expect(getDiscoveryCategoryBadgeClass()).toContain('bg-blue-100');
    expect(getDiscoveryCategoryBadgeClass()).toContain('text-blue-700');
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

  it('returns canonical network discovery settings copy', () => {
    expect(getNetworkDiscoveryPriorityNotice()).toEqual({
      title: 'Configuration precedence',
      items: [
        'Environment variables still override these settings.',
        'Changes made here are saved to system.json immediately.',
        'These settings remain in effect until an environment override replaces them.',
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
});
