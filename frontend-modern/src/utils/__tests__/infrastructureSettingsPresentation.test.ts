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
});
