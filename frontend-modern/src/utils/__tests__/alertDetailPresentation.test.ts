import { describe, expect, it } from 'vitest';
import {
  formatPlatformAlertCode,
  formatPlatformAlertDetailDateTime,
  formatPlatformAlertEntityType,
  formatPlatformAlertResourceType,
  formatPlatformAlertStartedAt,
} from '@/utils/alertDetailPresentation';

describe('alertDetailPresentation', () => {
  it('formats provider alert codes with canonical prefix stripping and title casing', () => {
    expect(formatPlatformAlertCode('docker_restart_loop', 'docker')).toBe('Restart Loop');
    expect(formatPlatformAlertCode('k8s_pod_crash_loop', 'kubernetes')).toBe('Pod Crash Loop');
    expect(formatPlatformAlertCode('truenas_pool_degraded', 'truenas')).toBe('Pool Degraded');
    expect(formatPlatformAlertCode('vmware_datastore_low_space', 'vmware')).toBe(
      'Datastore Low Space',
    );
    expect(formatPlatformAlertCode('')).toBe('-');
  });

  it('maps platform alert resource types through provider-specific labels', () => {
    expect(formatPlatformAlertResourceType('app-container', 'docker')).toBe('Container');
    expect(formatPlatformAlertResourceType('app-container', 'truenas')).toBe('App');
    expect(formatPlatformAlertResourceType('agent', 'kubernetes')).toBe('Node');
    expect(formatPlatformAlertResourceType('storage', 'vmware')).toBe('Datastore');
  });

  it('formats vSphere alert entity types consistently', () => {
    expect(formatPlatformAlertEntityType('vm')).toBe('VM');
    expect(formatPlatformAlertEntityType('datastore')).toBe('Datastore');
    expect(formatPlatformAlertEntityType('distributed_switch')).toBe('Distributed_switch');
    expect(formatPlatformAlertEntityType('')).toBe('-');
  });

  it('formats table and detail timestamps with the alert table fallbacks', () => {
    expect(formatPlatformAlertStartedAt(undefined)).toBe('-');
    expect(formatPlatformAlertStartedAt('1999-12-31T23:59:59Z')).toBe('-');
    expect(formatPlatformAlertDetailDateTime(undefined)).toBe('-');
    expect(formatPlatformAlertDetailDateTime('not-a-date')).toBe('not-a-date');
    expect(formatPlatformAlertDetailDateTime('1999-12-31T23:59:59Z')).toBe('-');
  });
});
