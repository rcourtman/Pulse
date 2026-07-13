import { describe, expect, it } from 'vitest';

import {
  alertResourceSupportsMetric,
  buildAlertResourceEditPayload,
  flattenAlertResourceTableResources,
  getAlertResourceColumnHeaderTooltip,
  getAlertResourceColumnKind,
  getAlertResourceEnabledDefault,
  getAlertResourceLabel,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDelayOverride,
  getAlertResourceMetricDisplayValue,
  getAlertResourceMetricStep,
  hasAlertResourceTableRows,
  hasCustomAlertResourceGlobalDefaults,
  isAlertResourceMetricOverridden,
  normalizeAlertResourceMetricKey,
  type AlertResourceTableResourceLike,
  type AlertResourceThresholdMap,
} from '../alertResourceTableModel';

function makeResource(
  overrides: Partial<AlertResourceTableResourceLike> = {},
): AlertResourceTableResourceLike {
  return {
    id: 'res-1',
    name: 'Test VM',
    ...overrides,
  };
}

describe('isAlertResourceMetricOverridden', () => {
  it('returns true when the metric has a positive numeric override', () => {
    const resource = makeResource({ thresholds: { cpu: 80 } });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(true);
  });

  it('returns true when the override is 0 (not collapsed by undefined/null guards)', () => {
    const resource = makeResource({ thresholds: { cpu: 0 } });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(true);
  });

  it('returns true for a negative override value', () => {
    const resource = makeResource({ thresholds: { cpu: -1 } });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(true);
  });

  it('returns true when the value is a non-null/undefined string', () => {
    const thresholds = { cpu: '90' } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ thresholds });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(true);
  });

  it('returns false when thresholds is undefined (optional-chain short-circuit)', () => {
    const resource = makeResource({ thresholds: undefined });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(false);
  });

  it('returns false when thresholds is an empty object', () => {
    const resource = makeResource({ thresholds: {} });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(false);
  });

  it('returns false when the metric is absent from a populated thresholds map', () => {
    const resource = makeResource({ thresholds: { memory: 90 } });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(false);
  });

  it('returns false when the metric value is explicitly undefined', () => {
    const resource = makeResource({ thresholds: { cpu: undefined } });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(false);
  });

  it('returns false when the metric value is null (second guard arm)', () => {
    const thresholds = { cpu: null } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ thresholds });
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(false);
  });

  it('checks a different metric than the one present', () => {
    const resource = makeResource({ thresholds: { cpu: 80 } });
    expect(isAlertResourceMetricOverridden(resource, 'memory')).toBe(false);
    expect(isAlertResourceMetricOverridden(resource, 'cpu')).toBe(true);
  });
});

describe('buildAlertResourceEditPayload', () => {
  it('returns shallow copies of thresholds and defaults plus the string note', () => {
    const resource = makeResource({
      thresholds: { cpu: 80, memory: 90 },
      defaults: { cpu: 70 },
      note: 'adjusted thresholds',
    });
    const payload = buildAlertResourceEditPayload(resource);
    expect(payload.thresholds).toEqual({ cpu: 80, memory: 90 });
    expect(payload.defaults).toEqual({ cpu: 70 });
    expect(payload.note).toBe('adjusted thresholds');
  });

  it('returns empty objects for thresholds and defaults when they are absent', () => {
    const payload = buildAlertResourceEditPayload(makeResource());
    expect(payload.thresholds).toEqual({});
    expect(payload.defaults).toEqual({});
  });

  it('returns undefined note when note is not set on the resource', () => {
    const payload = buildAlertResourceEditPayload(makeResource({ note: undefined }));
    expect(payload.note).toBeUndefined();
  });

  it('returns undefined note when note is a number (typeof string check fails)', () => {
    const resource = makeResource({ note: 42 as unknown as string });
    expect(buildAlertResourceEditPayload(resource).note).toBeUndefined();
  });

  it('preserves an empty-string note because typeof "" === "string"', () => {
    const resource = makeResource({ note: '' });
    expect(buildAlertResourceEditPayload(resource).note).toBe('');
  });

  it('produces a thresholds copy that is not a reference to the source', () => {
    const resource = makeResource({ thresholds: { cpu: 80 } });
    const payload = buildAlertResourceEditPayload(resource);
    payload.thresholds.cpu = 999;
    expect(resource.thresholds?.cpu).toBe(80);
  });

  it('produces a defaults copy that is not a reference to the source', () => {
    const resource = makeResource({ defaults: { cpu: 70 } });
    const payload = buildAlertResourceEditPayload(resource);
    payload.defaults.cpu = 999;
    expect(resource.defaults?.cpu).toBe(70);
  });

  it('copies empty thresholds and defaults objects into new object instances', () => {
    const resource = makeResource({ thresholds: {}, defaults: {} });
    const payload = buildAlertResourceEditPayload(resource);
    expect(payload.thresholds).toEqual({});
    expect(payload.defaults).toEqual({});
    expect(payload.thresholds).not.toBe(resource.thresholds);
    expect(payload.defaults).not.toBe(resource.defaults);
  });

  it('always returns an object with thresholds, defaults, and note keys', () => {
    const payload = buildAlertResourceEditPayload(makeResource());
    expect(Object.keys(payload).sort()).toEqual(['defaults', 'note', 'thresholds']);
  });
});

describe('hasAlertResourceTableRows — uncovered groupedResources arms', () => {
  it('returns true when groupedResources is empty-object and globalDefaults is populated', () => {
    expect(hasAlertResourceTableRows(undefined, {}, { cpu: 80 })).toBe(true);
  });

  it('returns false when groupedResources is empty-object and globalDefaults is undefined', () => {
    expect(hasAlertResourceTableRows(undefined, {}, undefined)).toBe(false);
  });

  it('returns true for empty-object groupedDefaults with empty-object globalDefaults', () => {
    expect(hasAlertResourceTableRows(undefined, {}, {})).toBe(true);
  });
});

describe('hasCustomAlertResourceGlobalDefaults — uncovered factoryDefaults arms', () => {
  it('returns false when factoryDefaults is an empty object (no keys to compare)', () => {
    expect(hasCustomAlertResourceGlobalDefaults({ cpu: 80 }, {})).toBe(false);
  });

  it('returns false when both arguments are empty objects', () => {
    expect(hasCustomAlertResourceGlobalDefaults({}, {})).toBe(false);
  });

  it('returns false when globalDefaults is an empty object but factoryDefaults has keys', () => {
    expect(hasCustomAlertResourceGlobalDefaults({}, { cpu: 80 })).toBe(false);
  });
});

describe('normalizeAlertResourceMetricKey — compound replace chains', () => {
  it('chains " %" strip with "disk w" replacement to produce "diskWrite"', () => {
    expect(normalizeAlertResourceMetricKey('disk w %')).toBe('diskWrite');
  });

  it('chains " %" strip with "net in" replacement to produce "networkIn"', () => {
    expect(normalizeAlertResourceMetricKey('net in %')).toBe('networkIn');
  });

  it('chains " %" strip with "net out" replacement to produce "networkOut"', () => {
    expect(normalizeAlertResourceMetricKey('net out %')).toBe('networkOut');
  });

  it('strips both " %" and " °c" suffixes in one pass', () => {
    expect(normalizeAlertResourceMetricKey('foo °c %')).toBe('foo');
  });

  it('strips " mb/s" from an unmapped key without further replacements', () => {
    expect(normalizeAlertResourceMetricKey('zzz mb/s')).toBe('zzz');
  });
});

describe('getAlertResourceMetricBounds — edge inputs', () => {
  it('returns the default bounds for an empty-string metric', () => {
    expect(getAlertResourceMetricBounds('')).toStrictEqual({ min: -1, max: 10000 });
  });

  it('returns the default bounds for a whitespace metric', () => {
    expect(getAlertResourceMetricBounds('   ')).toStrictEqual({ min: -1, max: 10000 });
  });
});

describe('getAlertResourceMetricStep — edge inputs', () => {
  it('returns 1 for an empty-string metric', () => {
    expect(getAlertResourceMetricStep('')).toBe(1);
  });

  it('returns 1 for a whitespace-only metric', () => {
    expect(getAlertResourceMetricStep('   ')).toBe(1);
  });
});

describe('getAlertResourceEnabledDefault — edge inputs', () => {
  it('returns the fallback of 80 for an empty-string metric', () => {
    expect(getAlertResourceEnabledDefault('')).toBe(80);
  });

  it('returns 55 for diskTemperature (distinct from temperature)', () => {
    expect(getAlertResourceEnabledDefault('diskTemperature')).toBe(55);
  });
});

describe('getAlertResourceMetricDelayOverride — uncovered normalization arms', () => {
  it('normalizes leading/trailing whitespace AND mixed case together', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: 42 }, '  CPU  ')).toBe(42);
  });

  it('prefers the normalized-key value when both normalized and original keys exist', () => {
    const map = { cpu: 10, CPU: 20 } as Record<string, number>;
    expect(getAlertResourceMetricDelayOverride(map, 'CPU')).toBe(10);
  });

  it('returns undefined for a whitespace-only metric with no matching key', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: 30 }, '   ')).toBeUndefined();
  });
});

describe('getAlertResourceColumnHeaderTooltip — edge inputs', () => {
  it('returns undefined for an empty-string column', () => {
    expect(getAlertResourceColumnHeaderTooltip('')).toBeUndefined();
  });

  it('returns undefined for a whitespace-only column', () => {
    expect(getAlertResourceColumnHeaderTooltip('   ')).toBeUndefined();
  });
});

describe('getAlertResourceColumnKind — compound normalization edges', () => {
  it('returns "badge" for "Backup %" via compound strip-then-Map-miss path', () => {
    expect(getAlertResourceColumnKind('Backup %')).toBe('badge');
  });

  it('returns "badge" for "Snapshot" (case-insensitive Map hit)', () => {
    expect(getAlertResourceColumnKind('Snapshot')).toBe('badge');
  });
});

describe('alertResourceSupportsMetric — supplementary metric arms', () => {
  it('returns true for disk on node (non-throughput metric bypasses the node guard)', () => {
    expect(alertResourceSupportsMetric('node', 'disk')).toBe(true);
  });

  it('returns false for diskRead on agent (throughput blocked for agents)', () => {
    expect(alertResourceSupportsMetric('agent', 'diskRead')).toBe(false);
  });

  it('returns false for temperature on storage (only usage supported)', () => {
    expect(alertResourceSupportsMetric('storage', 'temperature')).toBe(false);
  });

  it('returns false for usage on kubernetesCluster (not in the cluster metric set)', () => {
    expect(alertResourceSupportsMetric('kubernetesCluster', 'usage')).toBe(false);
  });

  it('returns true for memory on kubernetesPod', () => {
    expect(alertResourceSupportsMetric('kubernetesPod', 'memory')).toBe(true);
  });

  it('returns true for diskWrite on vmwareVm', () => {
    expect(alertResourceSupportsMetric('vmwareVm', 'diskWrite')).toBe(true);
  });

  it('returns false for memory on vmwareDatastore (only usage)', () => {
    expect(alertResourceSupportsMetric('vmwareDatastore', 'memory')).toBe(false);
  });

  it('returns false for cpu on vmwareNetwork (nothing supported)', () => {
    expect(alertResourceSupportsMetric('vmwareNetwork', 'cpu')).toBe(false);
  });

  it('returns false for disk on dockerContainer (not in restart/memory set)', () => {
    expect(alertResourceSupportsMetric('dockerContainer', 'disk')).toBe(false);
  });

  it('returns false for diskRead on kubernetesNode (not in cpu/memory/disk set)', () => {
    expect(alertResourceSupportsMetric('kubernetesNode', 'diskRead')).toBe(false);
  });
});

describe('getAlertResourceLabel — uncovered displayName/name arms', () => {
  it('falls through an empty-string displayName to the name', () => {
    const resource = makeResource({ name: 'web-01', displayName: '' });
    expect(getAlertResourceLabel(resource)).toBe('web-01');
  });

  it('falls through absent displayName and empty-string name to the resource id', () => {
    const resource = makeResource({ id: 'vm-42', name: '' });
    expect(getAlertResourceLabel(resource)).toBe('vm-42');
  });
});

describe('getAlertResourceMetricDisplayValue — boolean defaults coercion', () => {
  it('coerces a defaults boolean true to 1 via Number() in live mode', () => {
    const defaults = { cpu: true } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ defaults });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(1);
  });

  it('coerces a defaults boolean false to 0 via Number() in live mode', () => {
    const defaults = { cpu: false } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ defaults });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
  });

  it('coerces a defaults boolean true to 1 in edit mode when editingThresholds is empty', () => {
    const defaults = { cpu: true } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ defaults });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu', {}, true)).toBe(1);
  });
});

describe('flattenAlertResourceTableResources — nullish edge arms', () => {
  it('returns an empty array when resources is null (nullish coalescing treats null as nullish)', () => {
    const resources = null as unknown as AlertResourceTableResourceLike[];
    expect(flattenAlertResourceTableResources(resources)).toEqual([]);
  });

  it('falls back to resources when groupedResources is null (falsy, fails truthy guard)', () => {
    const a = makeResource({ id: 'a', name: 'A' });
    const grouped = null as unknown as Record<string, AlertResourceTableResourceLike[]>;
    expect(flattenAlertResourceTableResources([a], grouped)).toEqual([a]);
  });

  it('returns the single-item array from a single-key groupedResources', () => {
    const a = makeResource({ id: 'only', name: 'Only' });
    expect(flattenAlertResourceTableResources(undefined, { g: [a] })).toEqual([a]);
  });
});
