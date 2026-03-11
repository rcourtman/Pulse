import { describe, expect, it } from 'vitest';
import type { NodeConfigWithStatus } from '@/types/nodes';
import {
  getConfiguredNodeCapabilityBadges,
  isConfiguredNodeTemperatureMonitoringEnabled,
} from '@/utils/configuredNodeCapabilityPresentation';

describe('configuredNodeCapabilityPresentation', () => {
  it('builds canonical PVE capability badges', () => {
    const node = {
      type: 'pve',
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: false,
      monitorPhysicalDisks: true,
      temperatureMonitoringEnabled: null,
    } as NodeConfigWithStatus;

    expect(getConfiguredNodeCapabilityBadges(node, true).map((badge) => badge.label)).toEqual([
      'VMs',
      'Containers',
      'Storage',
      'Physical Disks',
      'Temperature',
    ]);
  });

  it('builds canonical PBS and PMG capability badges', () => {
    const pbsNode = {
      type: 'pbs',
      monitorDatastores: true,
      monitorSyncJobs: false,
      monitorVerifyJobs: true,
      monitorPruneJobs: true,
      monitorGarbageJobs: true,
      temperatureMonitoringEnabled: false,
    } as NodeConfigWithStatus;

    const pmgNode = {
      type: 'pmg',
      monitorMailStats: true,
      monitorQueues: true,
      monitorQuarantine: false,
      monitorDomainStats: true,
    } as NodeConfigWithStatus;

    expect(getConfiguredNodeCapabilityBadges(pbsNode, true).map((badge) => badge.label)).toEqual([
      'Datastores',
      'Verify Jobs',
      'Prune Jobs',
      'Garbage Collection',
    ]);
    expect(getConfiguredNodeCapabilityBadges(pmgNode, true).map((badge) => badge.label)).toEqual([
      'Mail stats',
      'Queues',
      'Domain stats',
    ]);
  });

  it('resolves temperature monitoring from node override or global setting', () => {
    expect(
      isConfiguredNodeTemperatureMonitoringEnabled(
        { temperatureMonitoringEnabled: true } as NodeConfigWithStatus,
        false,
      ),
    ).toBe(true);
    expect(
      isConfiguredNodeTemperatureMonitoringEnabled(
        { temperatureMonitoringEnabled: false } as NodeConfigWithStatus,
        true,
      ),
    ).toBe(false);
    expect(
      isConfiguredNodeTemperatureMonitoringEnabled(
        { temperatureMonitoringEnabled: null } as NodeConfigWithStatus,
        true,
      ),
    ).toBe(true);
  });
});
