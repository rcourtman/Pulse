import { describe, expect, it } from 'vitest';
import {
  getMetricSeverity,
  getMetricColorClass,
  getMetricColorRgba,
  getMetricColorHex,
  getMetricTextColorClass,
  getDefaultDisplayMetricThresholds,
  getDefaultMetricDisplayThresholds,
  resolveDiskTemperatureDisplayThresholds,
  resolveMetricDisplayThresholds,
  resolveTrueNASDiskTemperatureDisplayThresholds,
  METRIC_THRESHOLDS,
  type MetricType,
} from '@/utils/metricThresholds';
import {
  FACTORY_AGENT_DEFAULTS,
  FACTORY_KUBERNETES_DEFAULTS,
  FACTORY_SNAPSHOT_DEFAULTS,
  FACTORY_TRUENAS_DEFAULTS,
  FACTORY_TRUENAS_DISK_DEFAULTS,
  FACTORY_VMWARE_DEFAULTS,
} from '@/utils/alertThresholdDefaults';
import type { AlertConfig } from '@/types/alerts';

describe('metricThresholds', () => {
  describe('getMetricSeverity', () => {
    describe('cpu', () => {
      const metric: MetricType = 'cpu';

      it('returns normal for value below warning threshold', () => {
        expect(getMetricSeverity(50, metric)).toBe('normal');
        expect(getMetricSeverity(0, metric)).toBe('normal');
        expect(getMetricSeverity(79, metric)).toBe('normal');
      });

      it('returns warning for value at or above warning threshold', () => {
        expect(getMetricSeverity(80, metric)).toBe('warning');
        expect(getMetricSeverity(85, metric)).toBe('warning');
        expect(getMetricSeverity(89, metric)).toBe('warning');
      });

      it('returns critical for value at or above critical threshold', () => {
        expect(getMetricSeverity(90, metric)).toBe('critical');
        expect(getMetricSeverity(95, metric)).toBe('critical');
        expect(getMetricSeverity(100, metric)).toBe('critical');
      });
    });

    describe('memory', () => {
      const metric: MetricType = 'memory';

      it('returns normal for value below warning threshold', () => {
        expect(getMetricSeverity(50, metric)).toBe('normal');
        expect(getMetricSeverity(74, metric)).toBe('normal');
      });

      it('returns warning for value at or above warning threshold', () => {
        expect(getMetricSeverity(75, metric)).toBe('warning');
        expect(getMetricSeverity(80, metric)).toBe('warning');
      });

      it('returns critical for value at or above critical threshold', () => {
        expect(getMetricSeverity(85, metric)).toBe('critical');
        expect(getMetricSeverity(100, metric)).toBe('critical');
      });
    });

    describe('disk', () => {
      const metric: MetricType = 'disk';

      it('returns normal for value below warning threshold', () => {
        expect(getMetricSeverity(50, metric)).toBe('normal');
        expect(getMetricSeverity(79, metric)).toBe('normal');
      });

      it('returns warning for value at or above warning threshold', () => {
        expect(getMetricSeverity(80, metric)).toBe('warning');
        expect(getMetricSeverity(89, metric)).toBe('warning');
      });

      it('returns critical for value at or above critical threshold', () => {
        expect(getMetricSeverity(90, metric)).toBe('critical');
        expect(getMetricSeverity(100, metric)).toBe('critical');
      });
    });

    describe('temperature', () => {
      it('uses configured display thresholds instead of static frontend cutoffs', () => {
        const thresholds = { warning: 80, critical: 85 };

        expect(getMetricSeverity(76, 'temperature', thresholds)).toBe('normal');
        expect(getMetricSeverity(80, 'temperature', thresholds)).toBe('warning');
        expect(getMetricSeverity(85, 'temperature', thresholds)).toBe('critical');
      });
    });
  });

  describe('getMetricColorClass', () => {
    it('returns correct class for normal severity', () => {
      const result = getMetricColorClass(50, 'cpu');
      expect(result).toContain('bg-metric-normal-bg');
    });

    it('returns correct class for warning severity', () => {
      const result = getMetricColorClass(85, 'cpu');
      expect(result).toContain('bg-metric-warning-bg');
    });

    it('returns correct class for critical severity', () => {
      const result = getMetricColorClass(95, 'cpu');
      expect(result).toContain('bg-metric-critical-bg');
    });
  });

  describe('getMetricColorRgba', () => {
    it('returns green for normal severity', () => {
      const result = getMetricColorRgba(50, 'cpu');
      expect(result).toBe('rgba(34, 197, 94, 0.6)');
    });

    it('returns yellow for warning severity', () => {
      const result = getMetricColorRgba(85, 'cpu');
      expect(result).toBe('rgba(234, 179, 8, 0.6)');
    });

    it('returns red for critical severity', () => {
      const result = getMetricColorRgba(95, 'cpu');
      expect(result).toBe('rgba(239, 68, 68, 0.6)');
    });
  });

  describe('getMetricColorHex', () => {
    it('returns green for normal severity', () => {
      const result = getMetricColorHex(50, 'cpu');
      expect(result).toBe('#22c55e');
    });

    it('returns yellow for warning severity', () => {
      const result = getMetricColorHex(85, 'cpu');
      expect(result).toBe('#eab308');
    });

    it('returns red for critical severity', () => {
      const result = getMetricColorHex(95, 'cpu');
      expect(result).toBe('#ef4444');
    });
  });

  describe('getMetricTextColorClass', () => {
    it('returns muted for normal severity', () => {
      const result = getMetricTextColorClass(50, 'cpu');
      expect(result).toContain('text-muted');
    });

    it('returns yellow for warning severity', () => {
      const result = getMetricTextColorClass(85, 'cpu');
      expect(result).toContain('text-yellow-600');
    });

    it('returns red for critical severity', () => {
      const result = getMetricTextColorClass(95, 'cpu');
      expect(result).toContain('text-red-600');
    });
  });

  describe('METRIC_THRESHOLDS', () => {
    it('has correct values for cpu', () => {
      expect(METRIC_THRESHOLDS.cpu).toEqual({ warning: 80, critical: 90 });
    });

    it('has correct values for memory', () => {
      expect(METRIC_THRESHOLDS.memory).toEqual({ warning: 75, critical: 85 });
    });

    it('has correct values for disk', () => {
      expect(METRIC_THRESHOLDS.disk).toEqual({ warning: 80, critical: 90 });
    });
  });

  describe('alert-backed display thresholds', () => {
    it('derives guest display thresholds from alert trigger and clear defaults', () => {
      expect(getDefaultMetricDisplayThresholds('cpu')).toEqual({ warning: 75, critical: 80 });
      expect(getDefaultMetricDisplayThresholds('memory')).toEqual({ warning: 80, critical: 85 });
      expect(getDefaultMetricDisplayThresholds('disk')).toEqual({ warning: 85, critical: 90 });
      expect(getDefaultDisplayMetricThresholds('temperature', 'node')).toEqual({
        warning: 75,
        critical: 80,
      });
      expect(getDefaultDisplayMetricThresholds('diskTemperature')).toEqual({
        warning: 50,
        critical: 55,
      });
      expect(getDefaultMetricDisplayThresholds('generic')).toEqual({
        warning: 75,
        critical: 90,
      });
    });

    it('resolves configured defaults and resource override candidates', () => {
      const config = {
        enabled: true,
        guestDefaults: {
          cpu: { trigger: 82, clear: 77 },
        },
        nodeDefaults: {},
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'guest:cluster-a:100': {
            cpu: { trigger: 95, clear: 90 },
          },
        },
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'missing')).toEqual({
        warning: 77,
        critical: 82,
      });
      expect(
        resolveMetricDisplayThresholds(config, 'guest', 'cpu', [
          'cluster-a:node-a:100',
          'guest:cluster-a:100',
        ]),
      ).toEqual({
        warning: 90,
        critical: 95,
      });
    });

    it('resolves kubernetes, truenas, and vmware scope defaults with factory fallbacks', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        storageDefault: { trigger: 85, clear: 80 },
        kubernetesDefaults: {
          memory: { trigger: 92, clear: 87 },
        },
        overrides: {
          'k8s:prod:node:worker-01': {
            cpu: { trigger: 97, clear: 94 },
          },
        },
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'kubernetes', 'memory')).toEqual({
        warning: 87,
        critical: 92,
      });
      expect(
        resolveMetricDisplayThresholds(config, 'kubernetes', 'cpu', ['k8s:prod:node:worker-01']),
      ).toEqual({
        warning: 94,
        critical: 97,
      });
      // No configured defaults: factory platform defaults apply, not the
      // hardcoded METRIC_THRESHOLDS display constants.
      expect(resolveMetricDisplayThresholds(config, 'truenas', 'disk')).toEqual({
        warning: 80,
        critical: 85,
      });
      expect(resolveMetricDisplayThresholds(config, 'vmware', 'memory')).toEqual({
        warning: 80,
        critical: 85,
      });
    });

    it('does not treat external notification activation as detector state', () => {
      const config = {
        enabled: true,
        activationState: 'pending_review',
        guestDefaults: {
          cpu: { trigger: 88, clear: 82 },
        },
        nodeDefaults: {},
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {},
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu')).toEqual({
        warning: 82,
        critical: 88,
      });

      config.activationState = 'snoozed';
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu')).toEqual({
        warning: 82,
        critical: 88,
      });
    });

    it('uses default display coloring when alert thresholds are disabled', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'guest:cluster-a:100': {
            disk: { trigger: -1, clear: 0 },
          },
        },
      } as AlertConfig;

      expect(
        resolveMetricDisplayThresholds(config, 'guest', 'disk', 'guest:cluster-a:100'),
      ).toBeNull();
      expect(getMetricColorClass(99, 'disk', null)).toContain('bg-metric-critical-bg');
    });

    it('honors disabled docker defaults and storage usage aliases', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        dockerDefaults: {
          cpu: { trigger: 0, clear: 0 },
          memory: { trigger: 0, clear: 0 },
          disk: { trigger: 0, clear: 0 },
        },
        storageDefault: { trigger: 92, clear: 86 },
        overrides: {},
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'docker', 'cpu')).toBeNull();
      expect(resolveMetricDisplayThresholds(config, 'storage', 'usage')).toEqual({
        warning: 86,
        critical: 92,
      });
    });

    it('resolves disk temperature display thresholds per disk type', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        agentDefaults: {
          diskTemperature: { trigger: 55, clear: 50 },
        },
        diskTempByType: {
          nvme: { trigger: 72, clear: 66 },
          sas: { trigger: 65, clear: 60 },
          sata: { trigger: 55, clear: 50 },
        },
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'host-1': {
            diskTemperature: { trigger: 80, clear: 75 },
          },
        },
      } as AlertConfig;

      // Per-type map wins over the global agent default.
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme')).toEqual({
        warning: 66,
        critical: 72,
      });
      expect(resolveDiskTemperatureDisplayThresholds(config, 'NVMe')).toEqual({
        warning: 66,
        critical: 72,
      });
      // Unknown types fall back to the global agent default.
      expect(resolveDiskTemperatureDisplayThresholds(config, 'scsi')).toEqual({
        warning: 50,
        critical: 55,
      });
      expect(resolveDiskTemperatureDisplayThresholds(config, undefined)).toEqual({
        warning: 50,
        critical: 55,
      });
      // An explicit host override beats the per-type map, mirroring the backend.
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme', 'host-1')).toEqual({
        warning: 75,
        critical: 80,
      });
    });

    it('switches every disk type off with the agent disk temperature default', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        agentDefaults: { diskTemperature: { trigger: 0, clear: 0 } },
        diskTempByType: {
          nvme: { trigger: 70, clear: 65 },
          sata: { trigger: 55, clear: 50 },
        },
        storageDefault: { trigger: 85, clear: 80 },
        overrides: { 'host-1': { diskTemperature: { trigger: 80, clear: 75 } } },
      } as AlertConfig;
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme')).toBeNull();
      expect(resolveDiskTemperatureDisplayThresholds(config, 'sata')).toBeNull();
      expect(resolveDiskTemperatureDisplayThresholds(config, '')).toBeNull();
      // An explicit host override still applies, as CheckHost applies it.
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme', 'host-1')).toEqual({
        warning: 75,
        critical: 80,
      });
    });

    it('judges a disk by the alert overrides of the machine that reports it', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
        diskTempByType: { nvme: { trigger: 70, clear: 65 } },
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'host-raised': { diskTemperature: { trigger: 80, clear: 75 } },
          'host-memory-only': { memory: { trigger: 95, clear: 90 } },
          'host-off': { disabled: true },
        },
      } as AlertConfig;
      // The machine's first matching key decides, as CheckHost's chain does.
      expect(
        resolveDiskTemperatureDisplayThresholds(config, 'nvme', [
          'agent:host-raised',
          'host-raised',
        ]),
      ).toEqual({ warning: 75, critical: 80 });
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme', ['host-memory-only'])).toEqual(
        {
          warning: 65,
          critical: 70,
        },
      );
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme', ['host-plain'])).toEqual({
        warning: 65,
        critical: 70,
      });
      // A host whose alerts are switched off raises no disk temperature alert,
      // so none of its disks is judged hot.
      expect(resolveDiskTemperatureDisplayThresholds(config, 'nvme', ['host-off'])).toBeNull();
      // An agent default that switches every host's alerts off does the same.
      const agentsOff = {
        ...config,
        agentDefaults: { ...config.agentDefaults, disabled: true },
      } as AlertConfig;
      expect(
        resolveDiskTemperatureDisplayThresholds(agentsOff, 'nvme', ['host-raised']),
      ).toBeNull();
    });

    it('judges a TrueNAS disk by the tiers its temperature alert resolves', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
        diskTempByType: {
          nvme: { trigger: 70, clear: 65 },
          sata: { trigger: 55, clear: 50 },
        },
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'disk-raised': { temperature: { trigger: 75, clear: 70 } },
          'disk-off-value': { temperature: { trigger: 0, clear: 0 } },
          'disk-minus-one': { temperature: { trigger: -1, clear: 0 } },
          'disk-muted': { disabled: true },
          'disk-note-only': { note: 'spare shelf' },
          'disk-agent-key': { diskTemperature: { trigger: 80, clear: 75 } },
        },
      } as AlertConfig;
      const resolve = (cfg: AlertConfig, diskType: string, id: string) =>
        resolveTrueNASDiskTemperatureDisplayThresholds(cfg, diskType, id);

      // Without a TrueNAS-wide value each disk follows its type's policy.
      expect(resolve(config, 'nvme', 'disk-plain')).toEqual({ warning: 65, critical: 70 });
      expect(resolve(config, 'SATA', 'disk-plain')).toEqual({ warning: 50, critical: 55 });
      // An empty or unknown type, or one with no entry, takes the Disk Temp
      // default, as the backend's per-type lookup does.
      expect(resolve(config, '', 'disk-plain')).toEqual({ warning: 50, critical: 55 });
      expect(resolve(config, 'scsi', 'disk-plain')).toEqual({ warning: 50, critical: 55 });
      expect(resolve(config, 'sas', 'disk-plain')).toEqual({ warning: 50, critical: 55 });

      // The disk's own temperature override beats the type, raising or
      // switching it off; an override that mutes the disk's alerts is off too.
      expect(resolve(config, 'nvme', 'disk-raised')).toEqual({ warning: 70, critical: 75 });
      expect(resolve(config, 'nvme', 'disk-off-value')).toBeNull();
      expect(resolve(config, 'nvme', 'disk-minus-one')).toBeNull();
      expect(resolve(config, 'nvme', 'disk-muted')).toBeNull();
      // An override without a temperature, or with only the agent disk
      // temperature metric, leaves the default tiers.
      expect(resolve(config, 'nvme', 'disk-note-only')).toEqual({ warning: 65, critical: 70 });
      expect(resolve(config, 'nvme', 'disk-agent-key')).toEqual({ warning: 65, critical: 70 });
      expect(resolveTrueNASDiskTemperatureDisplayThresholds(config, 'nvme', null)).toEqual({
        warning: 65,
        critical: 70,
      });

      // A TrueNAS-wide value replaces the type's policy for every disk, and a
      // disk override still beats it. Zero switches it off.
      const trueNASWide = {
        ...config,
        truenasDiskDefaults: { temperature: { trigger: 62, clear: 57 } },
      } as AlertConfig;
      expect(resolve(trueNASWide, 'nvme', 'disk-plain')).toEqual({ warning: 57, critical: 62 });
      expect(resolve(trueNASWide, 'sata', 'disk-plain')).toEqual({ warning: 57, critical: 62 });
      expect(resolve(trueNASWide, 'nvme', 'disk-raised')).toEqual({ warning: 70, critical: 75 });
      const trueNASOff = {
        ...config,
        truenasDiskDefaults: { temperature: { trigger: 0, clear: 0 } },
      } as AlertConfig;
      expect(resolve(trueNASOff, 'nvme', 'disk-plain')).toBeNull();
      expect(resolve(trueNASOff, 'nvme', 'disk-raised')).toEqual({ warning: 70, critical: 75 });
      // An empty TrueNAS Disks entry is unset: the type's policy applies.
      const trueNASUnset = { ...config, truenasDiskDefaults: {} } as AlertConfig;
      expect(resolve(trueNASUnset, 'nvme', 'disk-plain')).toEqual({ warning: 65, critical: 70 });
      // TrueNAS Disks defaults that switch alerts off skip every disk's
      // evaluation, its own override included.
      const trueNASMuted = {
        ...config,
        truenasDiskDefaults: { disabled: true, temperature: { trigger: 62, clear: 57 } },
      } as AlertConfig;
      expect(resolve(trueNASMuted, 'nvme', 'disk-plain')).toBeNull();
      expect(resolve(trueNASMuted, 'nvme', 'disk-raised')).toBeNull();
    });

    it('reaches a TrueNAS disk only through the agent Disk Temp value', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
        diskTempByType: { nvme: { trigger: 70, clear: 65 } },
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {},
      } as AlertConfig;
      // No host agent reports a TrueNAS disk, so the agent default's alert
      // switch leaves it judged.
      const agentsMuted = {
        ...config,
        agentDefaults: { ...config.agentDefaults, disabled: true },
      } as AlertConfig;
      expect(resolveTrueNASDiskTemperatureDisplayThresholds(agentsMuted, 'nvme', 'd')).toEqual({
        warning: 65,
        critical: 70,
      });
      // A switched-off Disk Temp default switches the per-type tier off, but a
      // TrueNAS-wide value still applies.
      const diskTempOff = {
        ...config,
        agentDefaults: { diskTemperature: { trigger: 0, clear: 0 } },
      } as AlertConfig;
      expect(resolveTrueNASDiskTemperatureDisplayThresholds(diskTempOff, 'nvme', 'd')).toBeNull();
      expect(
        resolveTrueNASDiskTemperatureDisplayThresholds(
          { ...diskTempOff, truenasDiskDefaults: { temperature: { trigger: 60, clear: 55 } } },
          'nvme',
          'd',
        ),
      ).toEqual({ warning: 55, critical: 60 });
      // Without config, the factory per-type triggers apply.
      expect(resolveTrueNASDiskTemperatureDisplayThresholds(null, 'nvme', 'd')).toEqual({
        warning: 65,
        critical: 70,
      });
    });

    it('falls back to seeded per-type disk temperature defaults without config', () => {
      expect(resolveDiskTemperatureDisplayThresholds(null, 'nvme')).toEqual({
        warning: 65,
        critical: 70,
      });
      expect(resolveDiskTemperatureDisplayThresholds(null, 'sas')).toEqual({
        warning: 60,
        critical: 65,
      });
      expect(resolveDiskTemperatureDisplayThresholds(null, 'sata')).toEqual({
        warning: 50,
        critical: 55,
      });
      expect(resolveDiskTemperatureDisplayThresholds(null, '')).toEqual({
        warning: 50,
        critical: 55,
      });
    });

    it('resolves node temperature display thresholds from configured alert defaults', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {
          temperature: { trigger: 85, clear: 80 },
        },
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {},
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'node', 'temperature')).toEqual({
        warning: 80,
        critical: 85,
      });
    });

    it('keeps snapshot factory defaults carrying size thresholds disabled at zero', () => {
      // The size pair must exist on the factory config so the thresholds
      // editor and the save payload round-trip them; 0 means size-based
      // snapshot alerts are off until an operator enables them.
      expect(FACTORY_SNAPSHOT_DEFAULTS).toEqual({
        enabled: false,
        warningDays: 30,
        criticalDays: 45,
        warningSizeGiB: 0,
        criticalSizeGiB: 0,
      });
    });

    it('keeps Kubernetes, TrueNAS, and vSphere factory defaults explicit for alert configuration', () => {
      expect(FACTORY_KUBERNETES_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
      expect(FACTORY_TRUENAS_DEFAULTS).toMatchObject({
        cpu: 80,
        memory: 85,
        disk: 85,
        usage: 85,
        temperature: 80,
      });
      expect(FACTORY_TRUENAS_DISK_DEFAULTS).toEqual({ temperature: undefined });
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

    it('keeps SMART display defaults aligned with host alert evaluation', () => {
      expect(FACTORY_AGENT_DEFAULTS).toMatchObject({
        smartHealthFailure: 1,
        smartReallocated: 1,
        smartPending: 1,
        smartUncorrectable: 1,
        smartMediaErrors: 1,
        smartCrcErrorDelta: 1,
        smartLifeWarning: 10,
        smartLifeCritical: 5,
        smartSpareWarning: 20,
        smartSpareCritical: 10,
      });
    });
  });
});
