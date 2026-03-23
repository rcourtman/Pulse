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
