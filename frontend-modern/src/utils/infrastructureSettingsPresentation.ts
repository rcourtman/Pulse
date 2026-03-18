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
  return 'Failed to start discovery scan';
}

export function getDiscoverySettingUpdateErrorMessage(): string {
  return 'Failed to update discovery setting';
}

export function getDiscoverySubnetUpdateErrorMessage(): string {
  return 'Failed to update discovery subnet';
}

export function getNodeTemperatureMonitoringUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update temperature monitoring setting';
}

export function getNodeDeleteErrorMessage(message?: string): string {
  return message || 'Failed to delete node';
}
