import { describe, expect, it } from 'vitest';

import {
  CLOUD_PLAN_DEFINITIONS,
  CLOUD_PLAN_LABELS,
  DEFAULT_CLOUD_TIER,
  parseCloudTier,
} from '@/utils/cloudPlans';

describe('parseCloudTier', () => {
  it('returns each tier key when given its canonical lowercase name', () => {
    expect(parseCloudTier('starter')).toBe('starter');
    expect(parseCloudTier('power')).toBe('power');
    expect(parseCloudTier('max')).toBe('max');
  });

  it('normalizes mixed-case input through toLowerCase', () => {
    expect(parseCloudTier('STARTER')).toBe('starter');
    expect(parseCloudTier('Power')).toBe('power');
    expect(parseCloudTier('MAX')).toBe('max');
    expect(parseCloudTier('MaX')).toBe('max');
    expect(parseCloudTier('sTaRtEr')).toBe('starter');
  });

  it('trims surrounding whitespace before matching', () => {
    expect(parseCloudTier(' power ')).toBe('power');
    expect(parseCloudTier('\tmax\n')).toBe('max');
    expect(parseCloudTier('  starter  ')).toBe('starter');
    expect(parseCloudTier('\t POWER \n')).toBe('power');
  });

  it('falls back to DEFAULT_CLOUD_TIER for empty, whitespace-only, null, and undefined', () => {
    expect(parseCloudTier('')).toBe(DEFAULT_CLOUD_TIER);
    expect(parseCloudTier('   ')).toBe(DEFAULT_CLOUD_TIER);
    expect(parseCloudTier('\t\n')).toBe(DEFAULT_CLOUD_TIER);
    expect(parseCloudTier(null)).toBe(DEFAULT_CLOUD_TIER);
    expect(parseCloudTier(undefined)).toBe(DEFAULT_CLOUD_TIER);
  });

  it('falls back to the starter tier for any unrecognized non-empty string', () => {
    expect(parseCloudTier('enterprise')).toBe('starter');
    expect(parseCloudTier('free')).toBe('starter');
    expect(parseCloudTier('foo')).toBe('starter');
    expect(parseCloudTier('starter-annual')).toBe('starter');
    expect(parseCloudTier('power_pro')).toBe('starter');
    expect(parseCloudTier('123')).toBe('starter');
  });

  it('does not treat cloud planVersion identifiers as tier names', () => {
    // planVersions use the cloud_<tier> form; they must not collide with bare tier keys
    expect(parseCloudTier('cloud_starter')).toBe('starter');
    expect(parseCloudTier('cloud_power')).toBe('starter');
    expect(parseCloudTier('cloud_max')).toBe('starter');
    expect(parseCloudTier('cloud_founding')).toBe('starter');
  });

  it('does not treat MSP plan identifiers as cloud tiers', () => {
    expect(parseCloudTier('msp_starter')).toBe('starter');
    expect(parseCloudTier('msp_growth')).toBe('starter');
    expect(parseCloudTier('msp_scale')).toBe('starter');
  });
});

describe('CLOUD_PLAN_DEFINITIONS', () => {
  it('defines exactly the three canonical cloud tiers in canonical order', () => {
    expect(CLOUD_PLAN_DEFINITIONS.map((d) => d.tier)).toEqual(['starter', 'power', 'max']);
  });

  it('keeps monitored-system capacity strictly increasing and distinct across tiers', () => {
    const capacities = CLOUD_PLAN_DEFINITIONS.map((d) => d.monitoredSystems);
    const sorted = [...capacities].sort((a, b) => a - b);
    expect(capacities).toEqual(sorted);
    expect(new Set(capacities).size).toBe(capacities.length);
    for (const capacity of capacities) {
      expect(capacity).toBeGreaterThan(0);
    }
  });

  it('gives every definition a non-empty planVersion that has a human-readable label', () => {
    for (const definition of CLOUD_PLAN_DEFINITIONS) {
      expect(definition.planVersion.length).toBeGreaterThan(0);
      expect(CLOUD_PLAN_LABELS[definition.planVersion]).toBeTruthy();
      expect(typeof CLOUD_PLAN_LABELS[definition.planVersion]).toBe('string');
    }
  });

  it('keeps planVersions and names unique across definitions', () => {
    const planVersions = CLOUD_PLAN_DEFINITIONS.map((d) => d.planVersion);
    const names = CLOUD_PLAN_DEFINITIONS.map((d) => d.name);
    expect(new Set(planVersions).size).toBe(planVersions.length);
    expect(new Set(names).size).toBe(names.length);
  });

  it('keeps every definition structurally complete with valid support level', () => {
    for (const definition of CLOUD_PLAN_DEFINITIONS) {
      expect(definition.name.length).toBeGreaterThan(0);
      expect(definition.monthlyPrice.length).toBeGreaterThan(0);
      expect(definition.annualSummary.length).toBeGreaterThan(0);
      expect(['Community', 'Priority']).toContain(definition.support);
    }
  });
});

describe('CLOUD_PLAN_LABELS', () => {
  it('provides a label for every planVersion used by the cloud plan definitions', () => {
    const usedPlanVersions = CLOUD_PLAN_DEFINITIONS.map((d) => d.planVersion);
    for (const planVersion of usedPlanVersions) {
      expect(CLOUD_PLAN_LABELS).toHaveProperty(planVersion);
    }
  });

  it('only maps non-empty string keys to non-empty string labels', () => {
    for (const [key, label] of Object.entries(CLOUD_PLAN_LABELS)) {
      expect(typeof label).toBe('string');
      expect(label.length).toBeGreaterThan(0);
      expect(key.length).toBeGreaterThan(0);
    }
  });

  it('keeps display labels distinct from their raw keys', () => {
    for (const [key, label] of Object.entries(CLOUD_PLAN_LABELS)) {
      expect(label).not.toBe(key);
    }
  });
});

describe('DEFAULT_CLOUD_TIER', () => {
  it('points at the starter tier', () => {
    expect(DEFAULT_CLOUD_TIER).toBe('starter');
  });

  it('is one of the tiers defined in CLOUD_PLAN_DEFINITIONS', () => {
    expect(CLOUD_PLAN_DEFINITIONS.map((d) => d.tier)).toContain(DEFAULT_CLOUD_TIER);
  });

  it('survives a parseCloudTier round-trip', () => {
    expect(parseCloudTier(DEFAULT_CLOUD_TIER)).toBe(DEFAULT_CLOUD_TIER);
  });
});
