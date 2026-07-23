import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import type { Override } from '../types';
import {
  buildAgentHeaderMeta,
  dockerContainerOverrideIdCandidates,
  findOverrideByCandidates,
  hasThresholdDiff,
} from '../thresholdsResourceModel';

/**
 * Build a minimal Resource fixture. `as unknown as Resource` keeps strict
 * TypeScript clean while letting us exercise the runtime guards with
 * deliberately partial / malformed payloads.
 */
const makeAgent = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-id',
    type: 'agent',
    name: 'agent-name',
    displayName: 'agent-display',
    platformId: 'platform-id',
    platformType: 'generic',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 0,
    ...overrides,
  }) as unknown as Resource;

const makeOverride = (id: string, thresholds: Override['thresholds'] = {}): Override =>
  ({
    id,
    name: id,
    type: 'guest',
    thresholds,
  }) as Override;

describe('thresholdsResourceModel branch coverage 0723pm', () => {
  describe('hasThresholdDiff', () => {
    it('returns false when the override is undefined (?.thresholds short-circuit)', () => {
      expect(hasThresholdDiff(undefined, { cpu: 80 })).toBe(false);
    });

    it('returns false when every threshold equals its default', () => {
      const override = makeOverride('a', { cpu: 80, memory: 70 });
      expect(hasThresholdDiff(override, { cpu: 80, memory: 70 })).toBe(false);
    });

    it('returns true when one threshold diverges while a sibling matches', () => {
      const override = makeOverride('a', { cpu: 80, memory: 90 });
      expect(hasThresholdDiff(override, { cpu: 80, memory: 70 })).toBe(true);
    });

    it('returns true when the first keyed threshold already diverges (some early-exit)', () => {
      const override = makeOverride('a', { cpu: 99, memory: 99 });
      expect(hasThresholdDiff(override, { cpu: 80, memory: 70 })).toBe(true);
    });

    it('returns false for an empty thresholds object (some() over an empty key list)', () => {
      const override = makeOverride('a', {});
      expect(hasThresholdDiff(override, { cpu: 80 })).toBe(false);
    });

    it('ignores keys whose override value is undefined even when a default exists', () => {
      // The predicate's `!== undefined` guard must filter out absent values.
      const override = makeOverride('a', { cpu: undefined });
      expect(hasThresholdDiff(override, { cpu: 80 })).toBe(false);
    });
  });

  describe('dockerContainerOverrideIdCandidates', () => {
    it('prefixes every host candidate as docker:<hostId>/<shortId>, preserving order', () => {
      const host = makeAgent({
        id: 'runtime-1',
        discoveryTarget: {
          resourceType: 'app-container',
          resourceId: 'container-123',
          agentId: 'agent-7',
        },
        platformData: {
          docker: { hostSourceId: 'docker-platform' },
          hostSourceId: 'host-platform',
        },
      });
      expect(dockerContainerOverrideIdCandidates(host, 'a1b2c3d4e5f6')).toEqual([
        'docker:container-123/a1b2c3d4e5f6',
        'docker:docker-platform/a1b2c3d4e5f6',
        'docker:host-platform/a1b2c3d4e5f6',
        'docker:agent-7/a1b2c3d4e5f6',
        'docker:runtime-1/a1b2c3d4e5f6',
      ]);
    });

    it('produces a single prefixed entry when the host exposes only its resource id', () => {
      const host = makeAgent({ id: 'lone-host' });
      expect(dockerContainerOverrideIdCandidates(host, 'deadbeef')).toEqual([
        'docker:lone-host/deadbeef',
      ]);
    });

    it('falls back to platformData.docker.hostSourceId when discoveryTarget is absent', () => {
      const host = makeAgent({
        id: 'host-1',
        platformData: { docker: { hostSourceId: 'docker-src' } },
      });
      expect(dockerContainerOverrideIdCandidates(host, 'cid')).toEqual([
        'docker:docker-src/cid',
        'docker:host-1/cid',
      ]);
    });

    it('returns an empty array when the host has no usable id signal at all', () => {
      // resource.id is whitespace -> readString drops it -> uniqueIds yields [].
      const host = makeAgent({ id: '   ' });
      expect(dockerContainerOverrideIdCandidates(host, 'cid')).toEqual([]);
    });
  });

  describe('findOverrideByCandidates', () => {
    it('returns the override held under the first candidate (early return)', () => {
      const first = makeOverride('first', { cpu: 1 });
      const map = new Map<string, Override>([
        ['c1', first],
        ['c2', makeOverride('second', { cpu: 2 })],
      ]);
      expect(findOverrideByCandidates(map, ['c1', 'c2'])).toBe(first);
    });

    it('skips missing candidates and returns the first hit further down the list', () => {
      const later = makeOverride('later', { cpu: 5 });
      const map = new Map<string, Override>([['c3', later]]);
      expect(findOverrideByCandidates(map, ['c1', 'c2', 'c3'])).toBe(later);
    });

    it('returns undefined when no candidate is present in the map', () => {
      const map = new Map<string, Override>([['unrelated', makeOverride('x')]]);
      expect(findOverrideByCandidates(map, ['c1', 'c2'])).toBeUndefined();
    });

    it('returns undefined for an empty candidate list (loop body never executes)', () => {
      const map = new Map<string, Override>([['c1', makeOverride('x')]]);
      expect(findOverrideByCandidates(map, [])).toBeUndefined();
    });

    it('skips a candidate whose map value is explicitly undefined (truthy guard)', () => {
      // Map.get yields undefined for both missing keys and keys set to undefined;
      // the `if (override)` guard must treat them identically and keep scanning.
      const real = makeOverride('real', { cpu: 9 });
      const map = new Map<string, Override | undefined>([
        ['hole', undefined],
        ['real', real],
      ]) as unknown as Map<string, Override>;
      expect(findOverrideByCandidates(map, ['hole', 'real'])).toBe(real);
    });
  });

  describe('buildAgentHeaderMeta', () => {
    it('uses identity.hostname for rawName and collects distinct display/hostname/id keys', () => {
      const agent = makeAgent({
        id: 'agent-7',
        name: 'titan-host',
        displayName: 'Titan Server',
        status: 'warning',
        identity: { hostname: 'titan.example.com' },
      });
      const { headerMeta, keys } = buildAgentHeaderMeta(agent);
      expect(headerMeta.type).toBe('agent');
      expect(headerMeta.status).toBe('warning');
      expect(headerMeta.rawName).toBe('titan.example.com');
      expect(headerMeta.displayName).toBe('Titan Server');
      expect([...keys]).toEqual(['Titan Server', 'titan.example.com', 'agent-7']);
    });

    it('falls back to the trimmed agent.name for rawName when no hostname signal is present', () => {
      const agent = makeAgent({
        id: 'agent-2',
        name: 'box-2',
        displayName: undefined,
      });
      const { headerMeta, keys } = buildAgentHeaderMeta(agent);
      // getPreferredResourceHostname bottoms out at asTrimmedString(resource.name).
      expect(headerMeta.rawName).toBe('box-2');
      // displayName collapses onto the same value, so keys dedupe to two entries.
      expect(headerMeta.displayName).toBe('box-2');
      expect([...keys]).toEqual(['box-2', 'agent-2']);
    });

    it('falls back to agent.id for rawName when both hostname and name are absent', () => {
      const agent = makeAgent({
        id: 'agent-3',
        name: '',
        displayName: undefined,
        platformId: undefined,
      });
      const { headerMeta, keys } = buildAgentHeaderMeta(agent);
      expect(headerMeta.rawName).toBe('agent-3');
      expect(headerMeta.displayName).toBe('agent-3');
      expect([...keys]).toEqual(['agent-3']);
    });

    it('uses a whitespace-only name for rawName but keeps it out of the keys set', () => {
      // asTrimmedString drops the whitespace name (and platformId is cleared),
      // so getPreferredResourceHostname is undefined and the `|| agent.name` arm
      // fires, making rawName the raw "   " string. The keys loop then skips it
      // via its `value.trim()` guard.
      const agent = makeAgent({
        id: 'real-id',
        name: '   ',
        displayName: undefined,
        platformId: undefined,
      });
      const { headerMeta, keys } = buildAgentHeaderMeta(agent);
      expect(headerMeta.rawName).toBe('   ');
      expect([...keys]).toEqual(['real-id']);
    });
  });
});
