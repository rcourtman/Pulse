import { describe, expect, it } from 'vitest';
import type { AlertConfig, RawOverrideConfig } from '@/types/alerts';
import {
  getDefaultDisplayMetricThresholds,
  getDefaultMetricDisplayThresholds,
  getMetricSeverity,
  resolveMetricDisplayThresholds,
} from '@/utils/metricThresholds';

// `resolveThreshold`, `getScopeThresholds`, `getOverrideValue`, `normalizeMargin`,
// `getBaseThresholdValue`, `getFallbackCritical`, and `getFallbackSeverityThresholds`
// are module-private (non-exported) helpers, so they are exercised indirectly
// through the exported entry points below, asserting on their observable outputs
// (mirroring the sibling availabilityProbePresentation branchcov2 convention).

// Minimal valid AlertConfig base; callers override individual fields. Every scope
// that resolves off the base falls through to the seeded factory fallbacks.
const makeConfig = (overrides: Partial<AlertConfig> = {}): AlertConfig => ({
  enabled: true,
  guestDefaults: {},
  nodeDefaults: {},
  storageDefault: { trigger: 92, clear: 86 },
  overrides: {},
  ...overrides,
});

// Builds an override entry that may hold deliberately wrong-typed values (plain
// numbers where the public type expects a HysteresisThreshold, or a hysteresis
// object missing `clear`). The cast keeps the file type-checking under the real
// tsconfig while still driving the defensive `typeof value === 'number'` and
// `clear === null` branches at runtime.
const malformedOverride = (value: Record<string, unknown>): RawOverrideConfig =>
  value as unknown as RawOverrideConfig;

describe('metricThresholds — branch coverage (branchcov2)', () => {
  describe('normalizeMargin (via resolveMetricDisplayThresholds)', () => {
    // A guest cpu override of `{ trigger: 90 }` (no clear) hits resolveThreshold's
    // hysteresis clear-null arm, which yields `warning = max(0, critical - margin)`.
    // That makes normalizeMargin(config.hysteresisMargin) observable as `90 - margin`.

    it('defaults to the 5-point margin when hysteresisMargin is absent', () => {
      const config = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 90 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 85,
        critical: 90,
      });
    });

    it('honors a positive configured hysteresisMargin', () => {
      const config = makeConfig({
        hysteresisMargin: 10,
        overrides: { r1: malformedOverride({ cpu: { trigger: 90 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 80,
        critical: 90,
      });
    });

    it('clamps a negative hysteresisMargin to 0', () => {
      const config = makeConfig({
        hysteresisMargin: -3,
        overrides: { r1: malformedOverride({ cpu: { trigger: 90 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 90,
        critical: 90,
      });
    });

    it('falls back to the default margin for a NaN hysteresisMargin', () => {
      const config = makeConfig({
        hysteresisMargin: NaN,
        overrides: { r1: malformedOverride({ cpu: { trigger: 90 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 85,
        critical: 90,
      });
    });

    it('falls back to the default margin for a non-finite (Infinity) hysteresisMargin', () => {
      const config = makeConfig({
        hysteresisMargin: Infinity,
        overrides: { r1: malformedOverride({ cpu: { trigger: 90 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 85,
        critical: 90,
      });
    });
  });

  describe('getScopeThresholds (via resolveMetricDisplayThresholds) — every switch arm', () => {
    it('reads guestDefaults for the guest scope', () => {
      const config = makeConfig({ guestDefaults: { cpu: { trigger: 82, clear: 77 } } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu')).toEqual({
        warning: 77,
        critical: 82,
      });
    });

    it('reads nodeDefaults for the node scope', () => {
      const config = makeConfig({
        nodeDefaults: { temperature: { trigger: 85, clear: 80 } },
      });
      expect(resolveMetricDisplayThresholds(config, 'node', 'temperature')).toEqual({
        warning: 80,
        critical: 85,
      });
    });

    it('reads pbsDefaults for the pbs scope', () => {
      const config = makeConfig({ pbsDefaults: { cpu: { trigger: 70, clear: 65 } } });
      expect(resolveMetricDisplayThresholds(config, 'pbs', 'cpu')).toEqual({
        warning: 65,
        critical: 70,
      });
    });

    it('reads agentDefaults for the agent scope', () => {
      const config = makeConfig({
        agentDefaults: { diskTemperature: { trigger: 60, clear: 55 } },
      });
      expect(resolveMetricDisplayThresholds(config, 'agent', 'diskTemperature')).toEqual({
        warning: 55,
        critical: 60,
      });
    });

    it('reads dockerDefaults for the docker scope', () => {
      const config = makeConfig({ dockerDefaults: { cpu: { trigger: 88, clear: 82 } } });
      expect(resolveMetricDisplayThresholds(config, 'docker', 'cpu')).toEqual({
        warning: 82,
        critical: 88,
      });
    });

    it('reads storageDefault for the storage scope', () => {
      // storageDefault is the whole-object HysteresisThreshold; usage/disk read it.
      expect(resolveMetricDisplayThresholds(makeConfig(), 'storage', 'usage')).toEqual({
        warning: 86,
        critical: 92,
      });
    });

    it('returns undefined for every scope when config is null (optional chaining)', () => {
      // config?.<field> short-circuits to undefined; fallbacks still resolve.
      expect(resolveMetricDisplayThresholds(null, 'guest', 'cpu')).toEqual({
        warning: 75,
        critical: 80,
      });
      expect(resolveMetricDisplayThresholds(null, 'agent', 'diskTemperature')).toEqual({
        warning: 50,
        critical: 55,
      });
    });
  });

  describe('getOverrideValue (via resolveMetricDisplayThresholds)', () => {
    it('returns undefined when no override matches the resource id', () => {
      // No matching override → falls through to base/fallback (guest cpu → 80).
      const config = makeConfig({
        overrides: { other: malformedOverride({ cpu: 88 }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'missing')).toEqual({
        warning: 75,
        critical: 80,
      });
    });

    it('returns a plain numeric override value verbatim', () => {
      const config = makeConfig({ overrides: { r1: malformedOverride({ cpu: 88 }) } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 83,
        critical: 88,
      });
    });

    it('returns a hysteresis override value', () => {
      const config = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 90, clear: 80 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 80,
        critical: 90,
      });
    });

    it('aliases override.usage to disk when disk is absent (numeric)', () => {
      const config = makeConfig({ overrides: { r1: malformedOverride({ usage: 88 }) } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'disk', 'r1')).toEqual({
        warning: 83,
        critical: 88,
      });
    });

    it('aliases override.usage to disk when disk is absent (hysteresis)', () => {
      const config = makeConfig({
        overrides: { r1: malformedOverride({ usage: { trigger: 90, clear: 84 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'disk', 'r1')).toEqual({
        warning: 84,
        critical: 90,
      });
    });

    it('returns undefined for disk when neither disk nor usage is present on the override', () => {
      // Override holds only cpu; getOverrideValue(disk) is undefined → baseValue used.
      const config = makeConfig({
        guestDefaults: { disk: { trigger: 90, clear: 85 } },
        overrides: { r1: malformedOverride({ cpu: 50 }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'disk', 'r1')).toEqual({
        warning: 85,
        critical: 90,
      });
    });

    it('does not consult the usage alias for non-disk metrics', () => {
      // Override has only usage; asking for cpu → undefined → fallback guest cpu 80.
      const config = makeConfig({ overrides: { r1: malformedOverride({ usage: 88 }) } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 75,
        critical: 80,
      });
    });
  });

  describe('getBaseThresholdValue (via resolveMetricDisplayThresholds)', () => {
    it('returns undefined when scope thresholds are entirely absent', () => {
      // pbsDefaults omitted → getScopeThresholds returns undefined → base undefined
      // → fallback SCOPE_DEFAULTS.pbs.cpu = 80.
      expect(resolveMetricDisplayThresholds(makeConfig(), 'pbs', 'cpu')).toEqual({
        warning: 75,
        critical: 80,
      });
    });

    it('returns the whole-object hysteresis for storage + usage/disk', () => {
      expect(resolveMetricDisplayThresholds(makeConfig(), 'storage', 'usage')).toEqual({
        warning: 86,
        critical: 92,
      });
      expect(resolveMetricDisplayThresholds(makeConfig(), 'storage', 'disk')).toEqual({
        warning: 86,
        critical: 92,
      });
    });

    it('returns undefined for storage + a non-disk/usage metric (whole-object guard)', () => {
      // storageDefault is a HysteresisThreshold but metric is cpu → base undefined
      // → fallback undefined → null.
      expect(resolveMetricDisplayThresholds(makeConfig(), 'storage', 'cpu')).toBeNull();
    });

    it('returns a numeric field on a record threshold', () => {
      const config = makeConfig({
        guestDefaults: { cpu: 88 } as unknown as AlertConfig['guestDefaults'],
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu')).toEqual({
        warning: 83,
        critical: 88,
      });
    });

    it('returns a hysteresis field on a record threshold', () => {
      const config = makeConfig({ guestDefaults: { cpu: { trigger: 82, clear: 77 } } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu')).toEqual({
        warning: 77,
        critical: 82,
      });
    });

    it('returns undefined when the metric field is absent from the record', () => {
      // guestDefaults present but empty → base undefined → fallback guest cpu 80.
      expect(
        resolveMetricDisplayThresholds(makeConfig({ guestDefaults: {} }), 'guest', 'cpu'),
      ).toEqual({ warning: 75, critical: 80 });
    });
  });

  describe('getFallbackCritical (via getDefaultDisplayMetricThresholds)', () => {
    it('returns FACTORY_STORAGE_DEFAULT for storage + disk/usage', () => {
      expect(getDefaultDisplayMetricThresholds('usage', 'storage')).toEqual({
        warning: 80,
        critical: 85,
      });
      expect(getDefaultDisplayMetricThresholds('disk', 'storage')).toEqual({
        warning: 80,
        critical: 85,
      });
    });

    it('returns undefined for storage + a non-disk/usage metric', () => {
      expect(getDefaultDisplayMetricThresholds('cpu', 'storage')).toBeNull();
      expect(getDefaultDisplayMetricThresholds('temperature', 'storage')).toBeNull();
    });

    it('maps each non-storage scope to its seeded factory default', () => {
      expect(getDefaultDisplayMetricThresholds('cpu', 'guest')).toEqual({
        warning: 75,
        critical: 80,
      });
      expect(getDefaultDisplayMetricThresholds('temperature', 'node')).toEqual({
        warning: 75,
        critical: 80,
      });
      expect(getDefaultDisplayMetricThresholds('cpu', 'pbs')).toEqual({
        warning: 75,
        critical: 80,
      });
      expect(getDefaultDisplayMetricThresholds('diskTemperature', 'agent')).toEqual({
        warning: 50,
        critical: 55,
      });
      expect(getDefaultDisplayMetricThresholds('cpu', 'docker')).toEqual({
        warning: 75,
        critical: 80,
      });
    });

    it('returns null when the scope has no default for the metric', () => {
      // FACTORY_PBS_DEFAULTS has no disk/temperature entry → fallback undefined.
      expect(getDefaultDisplayMetricThresholds('disk', 'pbs')).toBeNull();
      expect(getDefaultDisplayMetricThresholds('temperature', 'pbs')).toBeNull();
    });
  });

  describe('resolveThreshold (via resolveMetricDisplayThresholds / getDefaultDisplayMetricThresholds)', () => {
    it('returns null when a hysteresis trigger is non-finite (NaN)', () => {
      const config = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: NaN, clear: 70 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toBeNull();
    });

    it('returns null when a hysteresis trigger is <= 0', () => {
      const triggerZero = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 0, clear: 0 } }) },
      });
      expect(resolveMetricDisplayThresholds(triggerZero, 'guest', 'cpu', 'r1')).toBeNull();
      const triggerNeg = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: -5, clear: 0 } }) },
      });
      expect(resolveMetricDisplayThresholds(triggerNeg, 'guest', 'cpu', 'r1')).toBeNull();
    });

    it('derives warning from (critical - margin) when clear is absent or non-finite', () => {
      const noClear = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 90 } }) },
      });
      expect(resolveMetricDisplayThresholds(noClear, 'guest', 'cpu', 'r1')).toEqual({
        warning: 85,
        critical: 90,
      });
      const badClear = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 90, clear: NaN } }) },
      });
      expect(resolveMetricDisplayThresholds(badClear, 'guest', 'cpu', 'r1')).toEqual({
        warning: 85,
        critical: 90,
      });
    });

    it('clamps a negative (critical - margin) warning to 0', () => {
      // trigger 3, no clear, margin 5 → max(0, 3 - 5) = 0.
      const config = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 3 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 0,
        critical: 3,
      });
    });

    it('uses min(clear, critical) when clear is finite and below critical', () => {
      const config = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 90, clear: 80 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 80,
        critical: 90,
      });
    });

    it('caps clear at critical when clear exceeds critical', () => {
      // min(95, 80) = 80 === critical.
      const config = makeConfig({
        overrides: { r1: malformedOverride({ cpu: { trigger: 80, clear: 95 } }) },
      });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 80,
        critical: 80,
      });
    });

    it('returns null for a numeric value <= 0', () => {
      const zero = makeConfig({ overrides: { r1: malformedOverride({ cpu: 0 }) } });
      expect(resolveMetricDisplayThresholds(zero, 'guest', 'cpu', 'r1')).toBeNull();
      const neg = makeConfig({ overrides: { r1: malformedOverride({ cpu: -5 }) } });
      expect(resolveMetricDisplayThresholds(neg, 'guest', 'cpu', 'r1')).toBeNull();
    });

    it('returns {critical: numeric, warning: numeric - margin} for a positive numeric value', () => {
      const config = makeConfig({ overrides: { r1: malformedOverride({ cpu: 88 }) } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 83,
        critical: 88,
      });
    });

    it('clamps a numeric warning (numeric - margin) to 0', () => {
      // numeric 3, margin 5 → max(0, -2) = 0.
      const config = makeConfig({ overrides: { r1: malformedOverride({ cpu: 3 }) } });
      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'r1')).toEqual({
        warning: 0,
        critical: 3,
      });
    });

    it('returns null when value and fallback are both absent', () => {
      // pbs has no disk default and FACTORY_PBS_DEFAULTS has no disk → null.
      expect(resolveMetricDisplayThresholds(makeConfig(), 'pbs', 'disk')).toBeNull();
    });

    it('returns null when no usable fallback is available', () => {
      // storage + cpu: getFallbackCritical returns undefined → resolveThreshold null.
      expect(resolveMetricDisplayThresholds(makeConfig(), 'storage', 'cpu')).toBeNull();
    });

    it('builds thresholds from a valid fallback when value is absent', () => {
      // guest cpu: no override/base → fallback FACTORY_GUEST_DEFAULTS.cpu = 80.
      expect(resolveMetricDisplayThresholds(makeConfig(), 'guest', 'cpu')).toEqual({
        warning: 75,
        critical: 80,
      });
    });
  });

  describe('getDefaultMetricDisplayThresholds', () => {
    it('returns the static generic thresholds for the "generic" bar metric', () => {
      expect(getDefaultMetricDisplayThresholds('generic')).toEqual({
        warning: 75,
        critical: 90,
      });
    });

    it('derives cpu/memory/disk from the guest factory defaults via resolveThreshold', () => {
      expect(getDefaultMetricDisplayThresholds('cpu')).toEqual({ warning: 75, critical: 80 });
      expect(getDefaultMetricDisplayThresholds('memory')).toEqual({ warning: 80, critical: 85 });
      expect(getDefaultMetricDisplayThresholds('disk')).toEqual({ warning: 85, critical: 90 });
    });
  });

  describe('getFallbackSeverityThresholds (via getMetricSeverity without thresholds)', () => {
    it('uses METRIC_THRESHOLDS for cpu/memory/disk', () => {
      // cpu: warning 80 / critical 90
      expect(getMetricSeverity(79, 'cpu')).toBe('normal');
      expect(getMetricSeverity(80, 'cpu')).toBe('warning');
      expect(getMetricSeverity(90, 'cpu')).toBe('critical');
      // memory: warning 75 / critical 85
      expect(getMetricSeverity(74, 'memory')).toBe('normal');
      expect(getMetricSeverity(75, 'memory')).toBe('warning');
      expect(getMetricSeverity(85, 'memory')).toBe('critical');
    });

    it('uses the node temperature default for temperature', () => {
      // getFallbackSeverityThresholds('temperature') → getDefaultDisplayMetricThresholds
      // default scope 'node', fallback 80 → { warning: 75, critical: 80 }.
      expect(getMetricSeverity(74, 'temperature')).toBe('normal');
      expect(getMetricSeverity(75, 'temperature')).toBe('warning');
      expect(getMetricSeverity(80, 'temperature')).toBe('critical');
    });

    it('uses the agent diskTemperature default for diskTemperature', () => {
      // default scope 'agent', fallback 55 → { warning: 50, critical: 55 }.
      expect(getMetricSeverity(49, 'diskTemperature')).toBe('normal');
      expect(getMetricSeverity(50, 'diskTemperature')).toBe('warning');
      expect(getMetricSeverity(55, 'diskTemperature')).toBe('critical');
    });

    it('uses the storage usage default for usage', () => {
      // default scope 'storage', fallback 85 → { warning: 80, critical: 85 }.
      expect(getMetricSeverity(79, 'usage')).toBe('normal');
      expect(getMetricSeverity(80, 'usage')).toBe('warning');
      expect(getMetricSeverity(85, 'usage')).toBe('critical');
    });
  });
});
