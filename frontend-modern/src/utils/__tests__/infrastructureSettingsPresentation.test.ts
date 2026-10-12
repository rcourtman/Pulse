import { describe, expect, it } from 'vitest';
import {
  getInfrastructureSettingsLocationLabel,
  getInfrastructureSettingsTarget,
  getInfrastructureSourceStrategyDescription,
  getDiscoveryScanStartErrorMessage,
  getDiscoverySettingUpdateErrorMessage,
  getDiscoverySubnetInvalidFormatMessage,
  getDiscoverySubnetInvalidValuesMessage,
  getDiscoverySubnetRequiredMessage,
  getDiscoverySubnetUpdateErrorMessage,
  getDiscoverySubnetValidEntryRequiredMessage,
  getDiscoverySubnetValuesRequiredMessage,
  getNodeDeleteErrorMessage,
  getNodeTemperatureMonitoringUpdateErrorMessage,
  getNetworkDiscoveryModePresentation,
  getNetworkDiscoveryPriorityNotice,
  getNetworkDiscoverySectionPresentation,
  getNetworkDiscoverySubnetPresentation,
} from '@/utils/infrastructureSettingsPresentation';

describe('infrastructureSettingsPresentation', () => {
  it('returns canonical Settings Infrastructure target copy', () => {
    expect(getInfrastructureSettingsTarget()).toEqual({
      href: '/settings/infrastructure',
      label: 'Settings → Infrastructure',
    });
    expect(getInfrastructureSettingsLocationLabel()).toBe('Settings → Infrastructure');
    expect(getInfrastructureSourceStrategyDescription()).toBe(
      'Start in Settings → Infrastructure by choosing a source strategy. Connect a platform API for inventory and health, install Pulse Agent for host telemetry, or use both when you want full coverage.',
    );
  });

  it('returns canonical discovery validation and error copy', () => {
    expect(getDiscoverySubnetRequiredMessage()).toBe(
      'Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)',
    );
    expect(getDiscoverySubnetValuesRequiredMessage()).toBe(
      'Enter at least one subnet before enabling discovery',
    );
    expect(getDiscoverySubnetInvalidFormatMessage()).toBe(
      'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
    );
    expect(getDiscoverySubnetInvalidValuesMessage()).toBe(
      'Enter valid CIDR subnet values before enabling discovery',
    );
    expect(getDiscoverySubnetValidEntryRequiredMessage()).toBe(
      'Enter at least one valid subnet in CIDR format',
    );
    expect(getDiscoveryScanStartErrorMessage()).toBe('Unable to start the discovery scan.');
    expect(getDiscoverySettingUpdateErrorMessage()).toBe('Unable to update the discovery setting.');
    expect(getDiscoverySubnetUpdateErrorMessage()).toBe('Unable to update the discovery subnet.');
  });

  it('returns canonical infrastructure node management error copy', () => {
    expect(getNodeTemperatureMonitoringUpdateErrorMessage()).toBe(
      'Unable to update temperature monitoring.',
    );
    expect(getNodeTemperatureMonitoringUpdateErrorMessage('forbidden')).toBe('forbidden');
    expect(getNodeDeleteErrorMessage()).toBe('Unable to delete the node.');
    expect(getNodeDeleteErrorMessage('locked')).toBe('locked');
  });

  it('returns canonical network discovery settings copy', () => {
    expect(getNetworkDiscoveryPriorityNotice()).toEqual({
      title: 'Network scan safety',
      items: [
        'Environment variables still override these settings.',
        'Changes made here are saved to system.json immediately.',
        'Automatic mode can scan every detected interface, including bridge or shared networks. Use custom subnets when scope matters.',
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
