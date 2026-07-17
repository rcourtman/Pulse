import { describe, expect, it } from 'vitest';
import {
  FACTORY_AGENT_DEFAULTS,
  FACTORY_BACKUP_DEFAULTS,
  FACTORY_DISK_TEMP_BY_TYPE,
  FACTORY_DOCKER_DEFAULTS,
  FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
  FACTORY_DOCKER_STATE_SEVERITY,
  FACTORY_DOCKER_STATE_SEVERITY as DOCKER_STATE_SEVERITY,
  FACTORY_GUEST_DEFAULTS,
  FACTORY_KUBERNETES_DEFAULTS,
  FACTORY_NODE_DEFAULTS,
  FACTORY_PBS_DEFAULTS,
  FACTORY_SNAPSHOT_DEFAULTS,
  FACTORY_STORAGE_DEFAULT,
  FACTORY_TRUENAS_DEFAULTS,
  FACTORY_TRUENAS_DISK_DEFAULTS,
  FACTORY_VMWARE_DEFAULTS,
} from '@/utils/alertThresholdDefaults';
import type { BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';

describe('alertThresholdDefaults', () => {
  describe('FACTORY_GUEST_DEFAULTS', () => {
    it('exposes the canonical guest metric thresholds', () => {
      expect(FACTORY_GUEST_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
    });

    it('disables throughput metrics with the -1 sentinel', () => {
      for (const key of ['diskRead', 'diskWrite', 'networkIn', 'networkOut'] as const) {
        expect(FACTORY_GUEST_DEFAULTS[key]).toBe(-1);
      }
    });
  });

  describe('FACTORY_NODE_DEFAULTS', () => {
    it('exposes cpu/memory/disk thresholds plus temperature', () => {
      expect(FACTORY_NODE_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        temperature: 80,
      });
    });
  });

  describe('FACTORY_PBS_DEFAULTS', () => {
    it('limits PBS defaults to cpu and memory only', () => {
      expect(FACTORY_PBS_DEFAULTS).toEqual({ cpu: 80, memory: 85 });
      expect(Object.keys(FACTORY_PBS_DEFAULTS).sort()).toEqual(['cpu', 'memory']);
    });
  });

  describe('FACTORY_KUBERNETES_DEFAULTS', () => {
    it('mirrors the guest default shape for kubernetes workloads', () => {
      expect(FACTORY_KUBERNETES_DEFAULTS).toEqual(FACTORY_GUEST_DEFAULTS);
      expect(FACTORY_KUBERNETES_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
    });
  });

  describe('FACTORY_TRUENAS_DEFAULTS', () => {
    it('exposes truenas thresholds including usage and temperature', () => {
      expect(FACTORY_TRUENAS_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 85,
        usage: 85,
        temperature: 80,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
    });

    it('uses a tighter disk threshold than the guest default', () => {
      expect(FACTORY_TRUENAS_DEFAULTS.disk).toBe(85);
      expect(FACTORY_TRUENAS_DEFAULTS.disk).toBeLessThan(FACTORY_GUEST_DEFAULTS.disk);
    });
  });

  describe('FACTORY_TRUENAS_DISK_DEFAULTS', () => {
    it('only carries a disk temperature trigger', () => {
      expect(FACTORY_TRUENAS_DISK_DEFAULTS).toEqual({ temperature: 55 });
      expect(Object.keys(FACTORY_TRUENAS_DISK_DEFAULTS)).toEqual(['temperature']);
    });
  });

  describe('FACTORY_VMWARE_DEFAULTS', () => {
    it('exposes vmware thresholds including usage and disabled throughput', () => {
      expect(FACTORY_VMWARE_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        usage: 85,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
    });
  });

  describe('FACTORY_AGENT_DEFAULTS', () => {
    it('exposes agent host thresholds including disk temperature', () => {
      expect(FACTORY_AGENT_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        diskTemperature: 55,
      });
    });
  });

  describe('FACTORY_DISK_TEMP_BY_TYPE', () => {
    it.each([
      ['nvme', 70],
      ['sas', 65],
      ['sata', 55],
    ])('maps %s disk type to its seeded trigger temperature', (type, expected) => {
      expect(FACTORY_DISK_TEMP_BY_TYPE[type]).toBe(expected);
    });

    it('contains exactly the nvme/sas/sata entries', () => {
      expect(FACTORY_DISK_TEMP_BY_TYPE).toEqual({
        nvme: 70,
        sas: 65,
        sata: 55,
      });
    });

    it('orders temperatures nvme > sas > sata', () => {
      expect(FACTORY_DISK_TEMP_BY_TYPE.nvme).toBeGreaterThan(FACTORY_DISK_TEMP_BY_TYPE.sas);
      expect(FACTORY_DISK_TEMP_BY_TYPE.sas).toBeGreaterThan(FACTORY_DISK_TEMP_BY_TYPE.sata);
    });
  });

  describe('FACTORY_DOCKER_DEFAULTS', () => {
    it('exposes the docker container and service gap thresholds', () => {
      expect(FACTORY_DOCKER_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 85,
        restartCount: 3,
        restartWindow: 300,
        memoryWarnPct: 90,
        memoryCriticalPct: 95,
        serviceWarnGapPercent: 10,
        serviceCriticalGapPercent: 50,
        updateAlertDelayHours: 24,
      });
    });

    it('keeps memory critical above memory warn', () => {
      expect(FACTORY_DOCKER_DEFAULTS.memoryCriticalPct).toBeGreaterThan(
        FACTORY_DOCKER_DEFAULTS.memoryWarnPct,
      );
    });

    it('keeps service critical gap above service warn gap', () => {
      expect(FACTORY_DOCKER_DEFAULTS.serviceCriticalGapPercent).toBeGreaterThan(
        FACTORY_DOCKER_DEFAULTS.serviceWarnGapPercent,
      );
    });
  });

  describe('docker state constants', () => {
    it('leaves connectivity disabled by default', () => {
      expect(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY).toBe(false);
    });

    it('defaults the docker state severity to warning', () => {
      expect(FACTORY_DOCKER_STATE_SEVERITY).toBe('warning');
      // Type-level guard: only 'warning' | 'critical' is allowed.
      const _severity: 'warning' | 'critical' = DOCKER_STATE_SEVERITY;
      expect(_severity).toBe('warning');
    });
  });

  describe('FACTORY_STORAGE_DEFAULT', () => {
    it('exposes the shared storage usage default', () => {
      expect(FACTORY_STORAGE_DEFAULT).toBe(85);
    });
  });

  describe('FACTORY_SNAPSHOT_DEFAULTS', () => {
    it('starts disabled with warning/critical day thresholds', () => {
      expect(FACTORY_SNAPSHOT_DEFAULTS).toEqual({
        enabled: false,
        warningDays: 30,
        criticalDays: 45,
      });
    });

    it('conforms to the SnapshotAlertConfig contract', () => {
      const config: SnapshotAlertConfig = FACTORY_SNAPSHOT_DEFAULTS;
      expect(config.criticalDays).toBeGreaterThan(config.warningDays);
    });
  });

  describe('FACTORY_BACKUP_DEFAULTS', () => {
    it('starts disabled with day/freshness/orphan defaults and an empty ignore list', () => {
      expect(FACTORY_BACKUP_DEFAULTS).toEqual({
        enabled: false,
        warningDays: 7,
        criticalDays: 14,
        freshHours: 24,
        staleHours: 72,
        alertOrphaned: true,
        ignoreVMIDs: [],
      });
    });

    it('conforms to the BackupAlertConfig contract', () => {
      const config: BackupAlertConfig = FACTORY_BACKUP_DEFAULTS;
      expect(config.criticalDays).toBeGreaterThan(config.warningDays);
    });

    it('uses staleHours greater than freshHours', () => {
      expect(FACTORY_BACKUP_DEFAULTS.staleHours).toBeGreaterThan(
        FACTORY_BACKUP_DEFAULTS.freshHours ?? 0,
      );
    });

    it('starts with an empty (mutable) ignore list', () => {
      expect(Array.isArray(FACTORY_BACKUP_DEFAULTS.ignoreVMIDs)).toBe(true);
      expect(FACTORY_BACKUP_DEFAULTS.ignoreVMIDs).toHaveLength(0);
    });
  });
});
