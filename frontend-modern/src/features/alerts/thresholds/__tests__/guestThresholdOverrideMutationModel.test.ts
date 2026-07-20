import { describe, expect, it } from 'vitest';

import type { RawOverrideConfig } from '@/types/alerts';

import type { Override } from '../types';
import {
  findOverrideForResource,
  findRawOverrideConfigForResource,
  getOverridePersistenceIdentity,
  stripOverrideCandidates,
  stripRawOverrideCandidates,
} from '../guestThresholdOverrideMutationModel';

const override = (id: string): Override =>
  ({
    id,
    name: id,
    type: 'guest',
    thresholds: {},
  }) as Override;

const makeRawConfig = (trigger: number): RawOverrideConfig =>
  ({ cpu: { trigger, clear: trigger - 5 } }) as RawOverrideConfig;

describe('guestThresholdOverrideMutationModel', () => {
  describe('getOverridePersistenceIdentity', () => {
    it('returns an empty identity for a missing resource', () => {
      expect(getOverridePersistenceIdentity(undefined)).toEqual({
        candidateIds: [],
        storageId: '',
      });
    });

    it('uses the exact id for non-guest resources', () => {
      expect(getOverridePersistenceIdentity({ id: 'agent-1', type: 'agent' })).toEqual({
        candidateIds: ['agent-1'],
        storageId: 'agent-1',
      });
    });

    it('returns empty candidates for a non-guest resource with a blank id', () => {
      expect(getOverridePersistenceIdentity({ id: '', type: 'agent' })).toEqual({
        candidateIds: [],
        storageId: '',
      });
    });

    it('builds the full guest candidate set and stable storage id for cluster guests', () => {
      const identity = getOverridePersistenceIdentity({
        id: 'res-1',
        type: 'guest',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 100,
      });

      expect(identity.storageId).toBe('guest:cluster-a:100');
      expect(identity.candidateIds).toEqual([
        'guest:cluster-a:100',
        'cluster-a:node-1:100',
        'res-1',
        'cluster-a-100',
        'cluster-a-node-1-100',
      ]);
    });

    it('uses the canonical storage id for standalone guests', () => {
      const identity = getOverridePersistenceIdentity({
        id: 'res-1',
        type: 'guest',
        instance: 'pve',
        node: 'pve',
        vmid: 100,
      });

      expect(identity.storageId).toBe('pve:pve:100');
      expect(identity.candidateIds).toEqual(['pve:pve:100', 'res-1', 'pve-100']);
    });

    it('falls back to the resource id when no guest identity can be derived', () => {
      expect(getOverridePersistenceIdentity({ id: 'res-1', type: 'guest' })).toEqual({
        candidateIds: ['res-1'],
        storageId: 'res-1',
      });
    });

    it('falls back to a blank-id singleton candidate when no identity and no id exist', () => {
      expect(getOverridePersistenceIdentity({ id: '', type: 'guest' })).toEqual({
        candidateIds: [''],
        storageId: '',
      });
    });
  });

  describe('findOverrideForResource', () => {
    const overrides = [
      override('guest:cluster-a:100'),
      override('cluster-a-node-1-100'),
      override('agent-1'),
    ];

    it('returns undefined for a missing resource', () => {
      expect(findOverrideForResource(overrides, undefined)).toBeUndefined();
    });

    it('matches by exact id for non-guest resources', () => {
      expect(findOverrideForResource(overrides, { id: 'agent-1', type: 'agent' })).toEqual(
        override('agent-1'),
      );
    });

    it('matches the first override against any guest candidate id', () => {
      expect(
        findOverrideForResource(overrides, {
          id: 'res-1',
          type: 'guest',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual(override('guest:cluster-a:100'));
    });

    it('matches a legacy cluster candidate id', () => {
      const legacyOnly = [override('cluster-a-node-1-100')];
      expect(
        findOverrideForResource(legacyOnly, {
          id: 'res-1',
          type: 'guest',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual(override('cluster-a-node-1-100'));
    });

    it('returns undefined when no candidate matches', () => {
      expect(
        findOverrideForResource(overrides, {
          id: 'res-1',
          type: 'guest',
          instance: 'other',
          node: 'node-9',
          vmid: 999,
        }),
      ).toBeUndefined();
    });

    it('returns undefined when candidateIds is empty and overrides is non-empty', () => {
      expect(findOverrideForResource(overrides, { id: '', type: 'agent' })).toBeUndefined();
    });
  });

  describe('findRawOverrideConfigForResource', () => {
    const rawConfig: Record<string, RawOverrideConfig> = {
      'guest:cluster-a:100': { cpu: { trigger: 90, clear: 85 } } as RawOverrideConfig,
      'cluster-a-node-1-100': { cpu: { trigger: 70, clear: 65 } } as RawOverrideConfig,
      'agent-1': { cpu: { trigger: 50, clear: 45 } } as RawOverrideConfig,
    };

    it('returns undefined for a missing resource', () => {
      expect(findRawOverrideConfigForResource(rawConfig, undefined)).toBeUndefined();
    });

    it('matches by exact id for non-guest resources', () => {
      expect(findRawOverrideConfigForResource(rawConfig, { id: 'agent-1', type: 'agent' })).toEqual(
        { cpu: { trigger: 50, clear: 45 } },
      );
    });

    it('returns the first truthy raw config across guest candidate ids in priority order', () => {
      expect(
        findRawOverrideConfigForResource(rawConfig, {
          id: 'res-1',
          type: 'guest',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual({ cpu: { trigger: 90, clear: 85 } });
    });

    it('falls back to a later candidate id when earlier ones are absent', () => {
      const onlyLegacy: Record<string, RawOverrideConfig> = {
        'cluster-a-node-1-100': { cpu: { trigger: 70, clear: 65 } } as RawOverrideConfig,
      };
      expect(
        findRawOverrideConfigForResource(onlyLegacy, {
          id: 'res-1',
          type: 'guest',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual({ cpu: { trigger: 70, clear: 65 } });
    });

    it('returns undefined when no candidate key has a config', () => {
      expect(
        findRawOverrideConfigForResource(rawConfig, {
          id: 'res-1',
          type: 'guest',
          instance: 'other',
          node: 'node-9',
          vmid: 999,
        }),
      ).toBeUndefined();
    });

    it('skips falsy values and returns the next truthy candidate', () => {
      const withGap: Record<string, RawOverrideConfig> = {
        'guest:cluster-a:100': undefined as unknown as RawOverrideConfig,
        'cluster-a:node-1:100': { cpu: { trigger: 80, clear: 75 } } as RawOverrideConfig,
      };
      expect(
        findRawOverrideConfigForResource(withGap, {
          id: 'res-1',
          type: 'guest',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual({ cpu: { trigger: 80, clear: 75 } });
    });
  });

  describe('stripOverrideCandidates', () => {
    const overrides = [
      override('guest:cluster-a:100'),
      override('cluster-a:node-1:100'),
      override('res-1'),
      override('cluster-a-100'),
      override('cluster-a-node-1-100'),
      override('agent-1'),
    ];

    it('returns the same array reference when there are no candidate ids', () => {
      const result = stripOverrideCandidates(overrides, undefined);
      expect(result).toBe(overrides);
    });

    it('returns the same array reference for a non-guest resource with a blank id', () => {
      const result = stripOverrideCandidates(overrides, { id: '', type: 'agent' });
      expect(result).toBe(overrides);
    });

    it('removes only the exact id override for non-guest resources', () => {
      const result = stripOverrideCandidates(overrides, { id: 'agent-1', type: 'agent' });
      expect(result.map((o) => o.id)).not.toContain('agent-1');
      expect(result.map((o) => o.id)).toEqual([
        'guest:cluster-a:100',
        'cluster-a:node-1:100',
        'res-1',
        'cluster-a-100',
        'cluster-a-node-1-100',
      ]);
    });

    it('removes every guest candidate override for a cluster guest', () => {
      const result = stripOverrideCandidates(overrides, {
        id: 'res-1',
        type: 'guest',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 100,
      });
      expect(result.map((o) => o.id)).toEqual(['agent-1']);
    });

    it('does not mutate the input overrides array', () => {
      const snapshot = overrides.map((o) => o.id);
      stripOverrideCandidates(overrides, {
        id: 'res-1',
        type: 'guest',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 100,
      });
      expect(overrides.map((o) => o.id)).toEqual(snapshot);
    });
  });

  describe('stripRawOverrideCandidates', () => {
    const base: Record<string, RawOverrideConfig> = {
      'guest:cluster-a:100': makeRawConfig(90),
      'cluster-a:node-1:100': makeRawConfig(80),
      'cluster-a-100': makeRawConfig(70),
      'cluster-a-node-1-100': makeRawConfig(60),
      'agent-1': makeRawConfig(50),
    };

    it('returns a new object with no candidates removed for a missing resource', () => {
      const result = stripRawOverrideCandidates(base, undefined);
      expect(result).not.toBe(base);
      expect(result).toEqual(base);
    });

    it('removes only the exact id key for non-guest resources', () => {
      const result = stripRawOverrideCandidates(base, { id: 'agent-1', type: 'agent' });
      expect(Object.keys(result).sort()).toEqual([
        'cluster-a-100',
        'cluster-a-node-1-100',
        'cluster-a:node-1:100',
        'guest:cluster-a:100',
      ]);
    });

    it('removes every guest candidate key for a cluster guest', () => {
      const result = stripRawOverrideCandidates(base, {
        id: 'res-1',
        type: 'guest',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 100,
      });
      expect(Object.keys(result)).toEqual(['agent-1']);
    });

    it('does not mutate the input config object', () => {
      const snapshot = { ...base };
      stripRawOverrideCandidates(base, {
        id: 'res-1',
        type: 'guest',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 100,
      });
      expect(base).toEqual(snapshot);
    });

    it('is a no-op on candidate keys that are not present in the config', () => {
      const result = stripRawOverrideCandidates(
        { 'agent-1': makeRawConfig(50) },
        {
          id: 'res-1',
          type: 'guest',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        },
      );
      expect(result).toEqual({ 'agent-1': makeRawConfig(50) });
    });
  });
});
