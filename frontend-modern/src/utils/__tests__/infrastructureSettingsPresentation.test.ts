import { describe, expect, it } from 'vitest';
import {
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
} from '@/utils/infrastructureSettingsPresentation';

describe('infrastructureSettingsPresentation', () => {
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
    expect(getDiscoveryScanStartErrorMessage()).toBe('Failed to start discovery scan');
    expect(getDiscoverySettingUpdateErrorMessage()).toBe('Failed to update discovery setting');
    expect(getDiscoverySubnetUpdateErrorMessage()).toBe('Failed to update discovery subnet');
  });

  it('returns canonical infrastructure node management error copy', () => {
    expect(getNodeTemperatureMonitoringUpdateErrorMessage()).toBe(
      'Failed to update temperature monitoring setting',
    );
    expect(getNodeTemperatureMonitoringUpdateErrorMessage('forbidden')).toBe('forbidden');
    expect(getNodeDeleteErrorMessage()).toBe('Failed to delete node');
    expect(getNodeDeleteErrorMessage('locked')).toBe('locked');
  });
});
