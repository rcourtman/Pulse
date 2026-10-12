export const INFRASTRUCTURE_SETTINGS_PATH = '/settings/infrastructure';
export const INFRASTRUCTURE_SETTINGS_LABEL = 'Settings → Infrastructure';

export function getInfrastructureSettingsTarget() {
  return {
    href: INFRASTRUCTURE_SETTINGS_PATH,
    label: INFRASTRUCTURE_SETTINGS_LABEL,
  } as const;
}

export function getInfrastructureSettingsLocationLabel(): string {
  return INFRASTRUCTURE_SETTINGS_LABEL;
}

export function getInfrastructureSourceStrategyDescription(): string {
  return `Start in ${INFRASTRUCTURE_SETTINGS_LABEL} by choosing a source strategy. Connect a platform API for inventory and health, install Pulse Agent for host telemetry, or use both when you want full coverage.`;
}

export function getDiscoverySubnetRequiredMessage(): string {
  return 'Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)';
}

export function getDiscoverySubnetValuesRequiredMessage(): string {
  return 'Enter at least one subnet before enabling discovery';
}

export function getDiscoverySubnetInvalidFormatMessage(): string {
  return 'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)';
}

export function getDiscoverySubnetInvalidValuesMessage(): string {
  return 'Enter valid CIDR subnet values before enabling discovery';
}

export function getDiscoverySubnetValidEntryRequiredMessage(): string {
  return 'Enter at least one valid subnet in CIDR format';
}

export function getDiscoveryScanStartErrorMessage(): string {
  return 'Unable to start the discovery scan.';
}

export function getDiscoverySettingUpdateErrorMessage(): string {
  return 'Unable to update the discovery setting.';
}

export function getDiscoverySubnetUpdateErrorMessage(): string {
  return 'Unable to update the discovery subnet.';
}

export function getNodeTemperatureMonitoringUpdateErrorMessage(message?: string): string {
  return message || 'Unable to update temperature monitoring.';
}

export function getNodeDeleteErrorMessage(message?: string): string {
  return message || 'Unable to delete the node.';
}

export function getNetworkDiscoveryPriorityNotice() {
  return {
    title: 'Network scan safety',
    items: [
      'Environment variables still override these settings.',
      'Changes made here are saved to system.json immediately.',
      'Automatic mode can scan every detected interface, including bridge or shared networks. Use custom subnets when scope matters.',
    ],
  } as const;
}

export function getNetworkDiscoverySectionPresentation(discoveryEnabled: boolean) {
  return {
    headerTitle: 'Network discovery',
    headerDescription: 'Control how Pulse scans your network for Proxmox services.',
    toggleTitle: 'Automatic scanning',
    toggleDescription:
      'Enable discovery to surface Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway endpoints automatically.',
    toggleStateLabel: discoveryEnabled ? 'Enabled' : 'Disabled',
    scanScopeLabel: 'Scan scope',
    commonNetworksLabel: 'Common networks',
    environmentOverrideMessage:
      'Discovery settings are locked by environment variables. Update the service configuration and restart Pulse to change them here.',
  } as const;
}

export function getNetworkDiscoveryModePresentation(mode: 'auto' | 'custom') {
  if (mode === 'auto') {
    return {
      label: 'Automatic scan (full network scope)',
      description:
        'Scan every network interface on this host, including container bridges, local subnets, and gateways. Use custom subnets on large or shared networks to reduce scan time.',
    } as const;
  }

  return {
    label: 'Custom subnets (targeted)',
    description:
      'Limit discovery to one or more CIDR ranges for faster, more targeted scans on large networks.',
  } as const;
}

export function getNetworkDiscoverySubnetPresentation(mode: 'auto' | 'custom') {
  return {
    label: 'Discovery subnets',
    helpTooltip:
      'Use CIDR notation, for example 192.168.1.0/24 or 10.0.0.0/24. Smaller ranges finish more quickly.',
    placeholder:
      mode === 'auto' ? 'automatic (scan every detected network)' : '192.168.1.0/24, 10.0.0.0/24',
    guidance:
      mode === 'auto'
        ? 'Automatic mode scans all host network interfaces, which can include shared or corporate networks. Switch to custom subnets for a faster, more targeted scan.'
        : 'Example: 192.168.1.0/24, 10.0.0.0/24. Smaller ranges finish faster and reduce timeout risk.',
  } as const;
}
