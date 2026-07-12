import { describe, expect, it } from 'vitest';

import {
  alertResourceSupportsMetric,
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

describe('flattenAlertResourceTableResources', () => {
  it('returns flattened grouped resources when groupedResources is provided', () => {
    const a = makeResource({ id: 'a', name: 'A' });
    const b = makeResource({ id: 'b', name: 'B' });
    const c = makeResource({ id: 'c', name: 'C' });
    expect(flattenAlertResourceTableResources(undefined, { nodes: [a, b], agents: [c] })).toEqual([
      a,
      b,
      c,
    ]);
  });

  it('returns the resources array when groupedResources is undefined', () => {
    const a = makeResource({ id: 'a', name: 'A' });
    const b = makeResource({ id: 'b', name: 'B' });
    expect(flattenAlertResourceTableResources([a, b])).toEqual([a, b]);
  });

  it('returns an empty array when both arguments are undefined', () => {
    expect(flattenAlertResourceTableResources()).toEqual([]);
  });

  it('prefers groupedResources even when resources is also provided', () => {
    const a = makeResource({ id: 'a', name: 'A' });
    const b = makeResource({ id: 'b', name: 'B' });
    expect(flattenAlertResourceTableResources([b], { group: [a] })).toEqual([a]);
  });

  it('returns an empty array for an empty groupedResources object (truthy but no values)', () => {
    expect(flattenAlertResourceTableResources(undefined, {})).toEqual([]);
  });
});

describe('hasAlertResourceTableRows', () => {
  it('returns true when resources has entries (flatten length > 0)', () => {
    expect(hasAlertResourceTableRows([makeResource({ id: 'a', name: 'A' })])).toBe(true);
  });

  it('returns true when groupedResources has keys even if all groups are empty arrays', () => {
    expect(hasAlertResourceTableRows(undefined, { group: [] })).toBe(true);
  });

  it('returns true when only globalDefaults with a numeric value is provided', () => {
    expect(hasAlertResourceTableRows(undefined, undefined, { cpu: 0 })).toBe(true);
  });

  it('returns false when nothing is provided', () => {
    expect(hasAlertResourceTableRows()).toBe(false);
  });

  it('returns false when resources is empty, groupedResources is undefined, and globalDefaults is undefined', () => {
    expect(hasAlertResourceTableRows([], undefined, undefined)).toBe(false);
  });

  it('returns true for an empty-object globalDefaults because Boolean({}) is truthy', () => {
    expect(hasAlertResourceTableRows(undefined, undefined, {})).toBe(true);
  });

  it('returns true when groupedResources is non-empty and globalDefaults is also set', () => {
    expect(hasAlertResourceTableRows(undefined, { g: [makeResource()] }, { cpu: 80 })).toBe(true);
  });
});

describe('hasCustomAlertResourceGlobalDefaults', () => {
  it('returns false when globalDefaults is undefined', () => {
    expect(hasCustomAlertResourceGlobalDefaults(undefined, { cpu: 80 })).toBe(false);
  });

  it('returns false when factoryDefaults is undefined', () => {
    expect(hasCustomAlertResourceGlobalDefaults({ cpu: 80 }, undefined)).toBe(false);
  });

  it('returns false when both are undefined', () => {
    expect(hasCustomAlertResourceGlobalDefaults(undefined, undefined)).toBe(false);
  });

  it('returns false when all factory keys match the global values exactly', () => {
    expect(
      hasCustomAlertResourceGlobalDefaults({ cpu: 80, memory: 90 }, { cpu: 80, memory: 90 }),
    ).toBe(false);
  });

  it('returns true when at least one factory key differs in global', () => {
    expect(
      hasCustomAlertResourceGlobalDefaults({ cpu: 75, memory: 90 }, { cpu: 80, memory: 90 }),
    ).toBe(true);
  });

  it('returns false when a factory key is absent from globalDefaults (current is undefined)', () => {
    expect(
      hasCustomAlertResourceGlobalDefaults({ memory: 90 }, { cpu: 80, memory: 90 }),
    ).toBe(false);
  });

  it('returns true when exactly one of many keys differs', () => {
    expect(
      hasCustomAlertResourceGlobalDefaults(
        { cpu: 80, memory: 90, disk: 95 },
        { cpu: 80, memory: 90, disk: 85 },
      ),
    ).toBe(true);
  });

  it('treats a global value of 0 as defined and custom when factory is non-zero', () => {
    expect(hasCustomAlertResourceGlobalDefaults({ cpu: 0 }, { cpu: 80 })).toBe(true);
  });

  it('returns false when global value equals factory value of 0', () => {
    expect(hasCustomAlertResourceGlobalDefaults({ cpu: 0 }, { cpu: 0 })).toBe(false);
  });
});

describe('normalizeAlertResourceMetricKey', () => {
  describe('Map direct hits', () => {
    it.each([
      ['cpu %', 'cpu'],
      ['memory %', 'memory'],
      ['disk %', 'disk'],
      ['disk r mb/s', 'diskRead'],
      ['disk w mb/s', 'diskWrite'],
      ['net in mb/s', 'networkIn'],
      ['net out mb/s', 'networkOut'],
      ['usage %', 'usage'],
      ['temp °c', 'temperature'],
      ['temperature °c', 'temperature'],
      ['temperature', 'temperature'],
      ['restart count', 'restartCount'],
      ['restart window', 'restartWindow'],
      ['restart window (s)', 'restartWindow'],
      ['memory warn %', 'memoryWarnPct'],
      ['memory critical %', 'memoryCriticalPct'],
      ['warning size (gib)', 'warningSizeGiB'],
      ['critical size (gib)', 'criticalSizeGiB'],
      ['disk temp °c', 'diskTemperature'],
      ['backup', 'backup'],
      ['snapshot', 'snapshot'],
    ])('maps %s -> %s', (input, expected) => {
      expect(normalizeAlertResourceMetricKey(input)).toBe(expected);
    });
  });

  describe('trim and lowercase normalization before Map lookup', () => {
    it('trims surrounding whitespace before lookup', () => {
      expect(normalizeAlertResourceMetricKey('  cpu %  ')).toBe('cpu');
    });

    it('lowercases before lookup', () => {
      expect(normalizeAlertResourceMetricKey('CPU %')).toBe('cpu');
    });

    it('handles mixed case and surrounding whitespace together', () => {
      expect(normalizeAlertResourceMetricKey('  Memory % ')).toBe('memory');
    });
  });

  describe('replace fallback chain (unmapped inputs)', () => {
    it('strips " %" suffix', () => {
      expect(normalizeAlertResourceMetricKey('foo %')).toBe('foo');
    });

    it('strips " °c" suffix', () => {
      expect(normalizeAlertResourceMetricKey('foo °c')).toBe('foo');
    });

    it('strips " mb/s" suffix', () => {
      expect(normalizeAlertResourceMetricKey('foo mb/s')).toBe('foo');
    });

    it('maps "disk r" to "diskRead" via replace (not in Map)', () => {
      expect(normalizeAlertResourceMetricKey('disk r')).toBe('diskRead');
    });

    it('maps "disk w" to "diskWrite" via replace', () => {
      expect(normalizeAlertResourceMetricKey('disk w')).toBe('diskWrite');
    });

    it('maps "net in" to "networkIn" via replace', () => {
      expect(normalizeAlertResourceMetricKey('net in')).toBe('networkIn');
    });

    it('maps "net out" to "networkOut" via replace', () => {
      expect(normalizeAlertResourceMetricKey('net out')).toBe('networkOut');
    });

    it('returns the trimmed lowercased key unchanged when no pattern matches', () => {
      expect(normalizeAlertResourceMetricKey('Foobar')).toBe('foobar');
    });

    it('chains multiple replaces: "disk r %" -> "diskRead"', () => {
      expect(normalizeAlertResourceMetricKey('disk r %')).toBe('diskRead');
    });
  });
});

describe('getAlertResourceMetricBounds', () => {
  it.each([
    ['temperature', { min: -1, max: 150 }],
    ['diskTemperature', { min: -1, max: 150 }],
    ['diskRead', { min: -1, max: 10000 }],
    ['diskWrite', { min: -1, max: 10000 }],
    ['networkIn', { min: -1, max: 10000 }],
    ['networkOut', { min: -1, max: 10000 }],
    ['cpu', { min: -1, max: 100 }],
    ['memory', { min: -1, max: 100 }],
    ['disk', { min: -1, max: 100 }],
    ['usage', { min: -1, max: 100 }],
    ['memoryWarnPct', { min: -1, max: 100 }],
    ['memoryCriticalPct', { min: -1, max: 100 }],
    ['warningSizeGiB', { min: -1, max: 100000 }],
    ['criticalSizeGiB', { min: -1, max: 100000 }],
    ['restartCount', { min: -1, max: 50 }],
    ['restartWindow', { min: -1, max: 86400 }],
    ['unknownMetric', { min: -1, max: 10000 }],
  ])('returns correct bounds for %s', (metric, expected) => {
    expect(getAlertResourceMetricBounds(metric)).toStrictEqual(expected);
  });
});

describe('getAlertResourceMetricStep', () => {
  it.each([
    ['diskRead', 'any'],
    ['diskWrite', 'any'],
    ['networkIn', 'any'],
    ['networkOut', 'any'],
    ['warningSizeGiB', 'any'],
    ['criticalSizeGiB', 'any'],
    ['cpu', 1],
    ['temperature', 1],
    ['unknownMetric', 1],
  ])('returns correct step for %s', (metric, expected) => {
    expect(getAlertResourceMetricStep(metric)).toBe(expected);
  });
});

describe('getAlertResourceEnabledDefault', () => {
  it.each([
    ['diskRead', 100],
    ['diskWrite', 100],
    ['networkIn', 100],
    ['networkOut', 100],
    ['temperature', 80],
    ['diskTemperature', 55],
    ['restartCount', 3],
    ['restartWindow', 300],
    ['memoryWarnPct', 90],
    ['memoryCriticalPct', 95],
    ['cpu', 80],
    ['memory', 80],
    ['unknownMetric', 80],
  ])('returns correct default for %s', (metric, expected) => {
    expect(getAlertResourceEnabledDefault(metric)).toBe(expected);
  });
});

describe('getAlertResourceMetricDelayOverride', () => {
  it('returns undefined when metricDelaySeconds is undefined', () => {
    expect(getAlertResourceMetricDelayOverride(undefined, 'cpu')).toBeUndefined();
  });

  it('returns the value for a normalized (trimmed+lowercased) key match', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: 30 }, 'CPU')).toBe(30);
  });

  it('returns the value when the metric is already normalized', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: 30 }, 'cpu')).toBe(30);
  });

  it('falls back to the original metric key when normalized lookup misses', () => {
    expect(getAlertResourceMetricDelayOverride({ CPU: 45 } as Record<string, number>, 'CPU')).toBe(
      45,
    );
  });

  it('returns undefined when neither normalized nor original key matches', () => {
    expect(getAlertResourceMetricDelayOverride({ foo: 30 }, 'cpu')).toBeUndefined();
  });

  it('returns undefined when the looked-up value is NaN (not finite)', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: NaN }, 'cpu')).toBeUndefined();
  });

  it('returns undefined when the looked-up value is Infinity (not finite)', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: Infinity }, 'cpu')).toBeUndefined();
  });

  it('returns undefined when the value is not a number type', () => {
    const corrupted = { cpu: '30' } as unknown as Record<string, number>;
    expect(getAlertResourceMetricDelayOverride(corrupted, 'cpu')).toBeUndefined();
  });

  it('returns 0 for a valid finite value of 0', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: 0 }, 'cpu')).toBe(0);
  });

  it('returns a negative finite value as-is', () => {
    expect(getAlertResourceMetricDelayOverride({ cpu: -5 }, 'cpu')).toBe(-5);
  });
});

describe('getAlertResourceColumnHeaderTooltip', () => {
  it('returns the tooltip for an exact column match', () => {
    expect(getAlertResourceColumnHeaderTooltip('cpu %')).toBe(
      'Percent CPU utilization allowed before an alert fires.',
    );
  });

  it('returns the tooltip via normalized lookup when exact misses (case/whitespace)', () => {
    expect(getAlertResourceColumnHeaderTooltip('  CPU %  ')).toBe(
      'Percent CPU utilization allowed before an alert fires.',
    );
  });

  it('returns undefined when no tooltip exists for the column', () => {
    expect(getAlertResourceColumnHeaderTooltip('nonexistent column')).toBeUndefined();
  });

  it('returns the mail-queue tooltip for a known queue column', () => {
    expect(getAlertResourceColumnHeaderTooltip('queue warn')).toBe(
      'Early warning when total mail queue exceeds this message count.',
    );
  });

  it('returns the snapshot-size tooltip via normalized lookup (mixed case)', () => {
    expect(getAlertResourceColumnHeaderTooltip('Warning Size (GiB)')).toBe(
      'Total snapshot size in GiB that raises a warning.',
    );
  });
});

describe('getAlertResourceColumnKind', () => {
  it('returns "badge" for "backup"', () => {
    expect(getAlertResourceColumnKind('backup')).toBe('badge');
  });

  it('returns "badge" for "snapshot"', () => {
    expect(getAlertResourceColumnKind('snapshot')).toBe('badge');
  });

  it('returns "badge" for "Backup" (case-insensitive via normalize)', () => {
    expect(getAlertResourceColumnKind('Backup')).toBe('badge');
  });

  it('returns "numeric-value" for a standard numeric metric', () => {
    expect(getAlertResourceColumnKind('cpu %')).toBe('numeric-value');
  });

  it('returns "numeric-value" for an unmapped column', () => {
    expect(getAlertResourceColumnKind('unknown')).toBe('numeric-value');
  });
});

describe('alertResourceSupportsMetric', () => {
  it('returns true when resourceType is undefined (any metric)', () => {
    expect(alertResourceSupportsMetric(undefined, 'cpu')).toBe(true);
    expect(alertResourceSupportsMetric(undefined, 'temperature')).toBe(true);
  });

  it('returns true for an empty-string resourceType', () => {
    expect(alertResourceSupportsMetric('', 'cpu')).toBe(true);
  });

  describe('node / agent — throughput blocked', () => {
    it('returns false for all throughput metrics on node', () => {
      expect(alertResourceSupportsMetric('node', 'diskRead')).toBe(false);
      expect(alertResourceSupportsMetric('node', 'diskWrite')).toBe(false);
      expect(alertResourceSupportsMetric('node', 'networkIn')).toBe(false);
      expect(alertResourceSupportsMetric('node', 'networkOut')).toBe(false);
    });

    it('returns true for cpu on node (falls through to default)', () => {
      expect(alertResourceSupportsMetric('node', 'cpu')).toBe(true);
    });

    it('returns false for throughput on agent', () => {
      expect(alertResourceSupportsMetric('agent', 'networkIn')).toBe(false);
    });

    it('returns true for memory on agent', () => {
      expect(alertResourceSupportsMetric('agent', 'memory')).toBe(true);
    });
  });

  describe('pbs — cpu/memory only', () => {
    it('supports cpu and memory', () => {
      expect(alertResourceSupportsMetric('pbs', 'cpu')).toBe(true);
      expect(alertResourceSupportsMetric('pbs', 'memory')).toBe(true);
    });

    it('rejects disk and temperature', () => {
      expect(alertResourceSupportsMetric('pbs', 'disk')).toBe(false);
      expect(alertResourceSupportsMetric('pbs', 'temperature')).toBe(false);
    });
  });

  describe('storage — usage only', () => {
    it('supports usage', () => {
      expect(alertResourceSupportsMetric('storage', 'usage')).toBe(true);
    });

    it('rejects cpu', () => {
      expect(alertResourceSupportsMetric('storage', 'cpu')).toBe(false);
    });
  });

  describe('kubernetesNamespace — nothing supported', () => {
    it('returns false for all metrics', () => {
      expect(alertResourceSupportsMetric('kubernetesNamespace', 'cpu')).toBe(false);
      expect(alertResourceSupportsMetric('kubernetesNamespace', 'memory')).toBe(false);
    });
  });

  describe('kubernetes cluster / deployment / pod', () => {
    it('supports cpu/disk/throughput on kubernetesCluster', () => {
      expect(alertResourceSupportsMetric('kubernetesCluster', 'cpu')).toBe(true);
      expect(alertResourceSupportsMetric('kubernetesCluster', 'disk')).toBe(true);
      expect(alertResourceSupportsMetric('kubernetesCluster', 'diskRead')).toBe(true);
    });

    it('rejects temperature on kubernetesCluster', () => {
      expect(alertResourceSupportsMetric('kubernetesCluster', 'temperature')).toBe(false);
    });

    it('supports networkIn on kubernetesDeployment', () => {
      expect(alertResourceSupportsMetric('kubernetesDeployment', 'networkIn')).toBe(true);
    });

    it('rejects restartCount on kubernetesDeployment', () => {
      expect(alertResourceSupportsMetric('kubernetesDeployment', 'restartCount')).toBe(false);
    });

    it('supports disk on kubernetesPod', () => {
      expect(alertResourceSupportsMetric('kubernetesPod', 'disk')).toBe(true);
    });

    it('rejects usage on kubernetesPod', () => {
      expect(alertResourceSupportsMetric('kubernetesPod', 'usage')).toBe(false);
    });
  });

  describe('kubernetesNode — cpu/memory/disk only', () => {
    it('supports cpu/memory/disk', () => {
      expect(alertResourceSupportsMetric('kubernetesNode', 'cpu')).toBe(true);
      expect(alertResourceSupportsMetric('kubernetesNode', 'memory')).toBe(true);
      expect(alertResourceSupportsMetric('kubernetesNode', 'disk')).toBe(true);
    });

    it('rejects diskRead', () => {
      expect(alertResourceSupportsMetric('kubernetesNode', 'diskRead')).toBe(false);
    });
  });

  describe('truenasSystem — extended set including temperature', () => {
    it('supports temperature and cpu', () => {
      expect(alertResourceSupportsMetric('truenasSystem', 'temperature')).toBe(true);
      expect(alertResourceSupportsMetric('truenasSystem', 'cpu')).toBe(true);
    });

    it('rejects restartCount', () => {
      expect(alertResourceSupportsMetric('truenasSystem', 'restartCount')).toBe(false);
    });
  });

  describe('truenasPool / truenasDataset — usage only', () => {
    it('supports usage on truenasPool', () => {
      expect(alertResourceSupportsMetric('truenasPool', 'usage')).toBe(true);
    });

    it('rejects cpu on truenasPool', () => {
      expect(alertResourceSupportsMetric('truenasPool', 'cpu')).toBe(false);
    });

    it('supports usage on truenasDataset', () => {
      expect(alertResourceSupportsMetric('truenasDataset', 'usage')).toBe(true);
    });

    it('rejects temperature on truenasDataset', () => {
      expect(alertResourceSupportsMetric('truenasDataset', 'temperature')).toBe(false);
    });
  });

  describe('truenasDisk — temperature only', () => {
    it('supports temperature', () => {
      expect(alertResourceSupportsMetric('truenasDisk', 'temperature')).toBe(true);
    });

    it('rejects cpu', () => {
      expect(alertResourceSupportsMetric('truenasDisk', 'cpu')).toBe(false);
    });
  });

  describe('vmwareHost — no disk', () => {
    it('supports cpu/memory/throughput', () => {
      expect(alertResourceSupportsMetric('vmwareHost', 'cpu')).toBe(true);
      expect(alertResourceSupportsMetric('vmwareHost', 'memory')).toBe(true);
      expect(alertResourceSupportsMetric('vmwareHost', 'diskRead')).toBe(true);
    });

    it('rejects disk', () => {
      expect(alertResourceSupportsMetric('vmwareHost', 'disk')).toBe(false);
    });
  });

  describe('vmwareVm — disk + throughput', () => {
    it('supports disk and networkOut', () => {
      expect(alertResourceSupportsMetric('vmwareVm', 'disk')).toBe(true);
      expect(alertResourceSupportsMetric('vmwareVm', 'networkOut')).toBe(true);
    });

    it('rejects temperature', () => {
      expect(alertResourceSupportsMetric('vmwareVm', 'temperature')).toBe(false);
    });
  });

  describe('vmwareDatastore — usage only', () => {
    it('supports usage', () => {
      expect(alertResourceSupportsMetric('vmwareDatastore', 'usage')).toBe(true);
    });

    it('rejects cpu', () => {
      expect(alertResourceSupportsMetric('vmwareDatastore', 'cpu')).toBe(false);
    });
  });

  describe('vmwareNetwork — nothing supported', () => {
    it('returns false for any metric', () => {
      expect(alertResourceSupportsMetric('vmwareNetwork', 'cpu')).toBe(false);
    });
  });

  describe('dockerContainer — restart + memory-warn set', () => {
    it('supports restart and memory-warn metrics', () => {
      expect(alertResourceSupportsMetric('dockerContainer', 'restartCount')).toBe(true);
      expect(alertResourceSupportsMetric('dockerContainer', 'restartWindow')).toBe(true);
      expect(alertResourceSupportsMetric('dockerContainer', 'memoryWarnPct')).toBe(true);
      expect(alertResourceSupportsMetric('dockerContainer', 'memoryCriticalPct')).toBe(true);
      expect(alertResourceSupportsMetric('dockerContainer', 'cpu')).toBe(true);
      expect(alertResourceSupportsMetric('dockerContainer', 'memory')).toBe(true);
    });

    it('rejects temperature and disk', () => {
      expect(alertResourceSupportsMetric('dockerContainer', 'temperature')).toBe(false);
      expect(alertResourceSupportsMetric('dockerContainer', 'disk')).toBe(false);
    });
  });

  describe('unknown resource type — default true', () => {
    it('returns true for any metric', () => {
      expect(alertResourceSupportsMetric('mysteryType', 'cpu')).toBe(true);
      expect(alertResourceSupportsMetric('mysteryType', 'anything')).toBe(true);
    });
  });
});

describe('getAlertResourceLabel', () => {
  it('returns the displayName when present', () => {
    const resource = makeResource({ displayName: 'Prod Web Server' });
    expect(getAlertResourceLabel(resource)).toBe('Prod Web Server');
  });

  it('trims surrounding whitespace from displayName', () => {
    const resource = makeResource({ displayName: '  Prod Web Server  ' });
    expect(getAlertResourceLabel(resource)).toBe('Prod Web Server');
  });

  it('falls back to name when displayName is absent', () => {
    const resource = makeResource({ name: 'web-01' });
    expect(getAlertResourceLabel(resource)).toBe('web-01');
  });

  it('falls through whitespace-only displayName to the name', () => {
    const resource = makeResource({ name: 'node-7', displayName: '   ' });
    expect(getAlertResourceLabel(resource)).toBe('node-7');
  });
});

describe('parseAlertMetricNumber (exercised via getAlertResourceMetricDisplayValue)', () => {
  it('passes a negative number through the typeof-number branch unchanged', () => {
    const resource = makeResource({ thresholds: { cpu: -5 } });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(-5);
  });

  it('passes a float through the typeof-number branch unchanged', () => {
    const resource = makeResource({ thresholds: { cpu: 42.5 } });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(42.5);
  });

  it('parses a whitespace-padded numeric string via Number() (finite branch -> number)', () => {
    const thresholds = { cpu: '  42  ' } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ thresholds });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(42);
  });

  it('returns 0 fallback for null (null branch -> undefined -> defaults -> 0)', () => {
    const thresholds = { cpu: null } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ thresholds });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
  });

  it('returns 0 fallback for a non-numeric string (Number() -> NaN -> undefined -> 0)', () => {
    const thresholds = { cpu: 'nope' } as unknown as AlertResourceThresholdMap;
    const resource = makeResource({ thresholds });
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
  });
});
