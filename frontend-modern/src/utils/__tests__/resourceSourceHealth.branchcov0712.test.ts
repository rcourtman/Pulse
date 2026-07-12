/**
 * Branch-coverage tests for resourceSourceHealth.ts — second pass.
 *
 * Focused exclusively on branches of `getResourceSourceStatus` and the
 * module-private `normalizeStatus` that the sibling
 * resourceSourceHealth.test.ts does not yet reach.
 *
 * `normalizeStatus` is not exported, so it is exercised through
 * `getResourceSourceHealth` (its only caller) with status values chosen to
 * drive each arm of its `typeof value === 'string'` guard — non-string
 * primitives/objects and whitespace/case-mixed strings the sibling test never
 * feeds in.
 *
 * Fixtures mirror the sibling test's `as Resource` cast pattern.
 */
import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { getResourceSourceHealth, getResourceSourceStatus } from '@/utils/resourceSourceHealth';

const makeResource = (platformData: Record<string, unknown>): Resource =>
  ({
    id: 'resource-1',
    type: 'agent',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'lab',
    platformType: 'vmware-vsphere',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    platformData,
  }) as Resource;

const withSourceStatus = (sourceStatus: unknown): Resource => makeResource({ sourceStatus });

describe('getResourceSourceStatus (branch coverage)', () => {
  it('returns undefined when the source argument is an empty string (early guard)', () => {
    // `normalizeSourcePlatformKey('') || ''.trim().toLowerCase()` -> '' -> !normalizedSource.
    const resource = withSourceStatus({ truenas: { status: 'online' } });
    expect(getResourceSourceStatus(resource, '')).toBeUndefined();
  });

  it('returns undefined when the source argument is only whitespace (early guard)', () => {
    // normalizeSourcePlatformKey('   ') -> null; '   '.trim().toLowerCase() -> ''.
    const resource = withSourceStatus({ truenas: { status: 'online' } });
    expect(getResourceSourceStatus(resource, '   ')).toBeUndefined();
  });

  it('returns the full status entry when the source is a known platform id (truthy || arm)', () => {
    // normalizeSourcePlatformKey('truenas') -> 'truenas' (truthy) short-circuits the ||.
    const resource = withSourceStatus({ truenas: { status: 'online', lastSeen: 5 } });
    expect(getResourceSourceStatus(resource, 'truenas')).toStrictEqual({
      status: 'online',
      lastSeen: 5,
    });
  });

  it('matches a sourceStatus key written as a platform alias (key-side normalize arm)', () => {
    // key 'vmware' normalizes to 'vmware-vsphere', matching source 'vmware-vsphere'.
    const resource = withSourceStatus({ vmware: { status: 'healthy', error: null } });
    expect(getResourceSourceStatus(resource, 'vmware-vsphere')).toStrictEqual({
      status: 'healthy',
      error: null,
    });
  });

  it('matches an unknown source key through the trim/lowercase fallback (both || falsy arms)', () => {
    // source and key are both unknown to the manifest, so both fall to `.trim().toLowerCase()`.
    // Also exercises the loop `continue` for a non-matching preceding key ('truenas').
    const resource = withSourceStatus({
      truenas: { status: 'online' },
      'Custom-Source': { status: 'offline', error: 'timeout' },
    });
    expect(getResourceSourceStatus(resource, 'custom-source')).toStrictEqual({
      status: 'offline',
      error: 'timeout',
    });
  });

  it('returns undefined when no sourceStatus key matches the requested source (loop fall-through)', () => {
    // Loop iterates 'truenas' (normalizes to 'truenas' !== 'docker'), then the bottom return.
    const resource = withSourceStatus({ truenas: { status: 'online' } });
    expect(getResourceSourceStatus(resource, 'docker')).toBeUndefined();
  });

  it('returns undefined when platformData has no sourceStatus field (optional-chain arm)', () => {
    // resource.platformData?.sourceStatus -> undefined -> asRecord(undefined) -> undefined.
    expect(getResourceSourceStatus(makeResource({}), 'truenas')).toBeUndefined();
  });

  it('returns undefined when sourceStatus is a primitive string (asRecord reject arm)', () => {
    // typeof 'truenas' === 'string' -> asRecord returns undefined -> !sourceStatus guard.
    expect(getResourceSourceStatus(withSourceStatus('truenas'), 'truenas')).toBeUndefined();
  });

  it('returns undefined when sourceStatus is null (asRecord falsy guard)', () => {
    // `value && typeof value === 'object'` — null is falsy, so asRecord returns undefined.
    expect(getResourceSourceStatus(withSourceStatus(null), 'truenas')).toBeUndefined();
  });

  it('returns undefined when sourceStatus is an array with no matching keys', () => {
    // asRecord([]) returns the array (typeof 'object'), Object.entries([]) is empty,
    // the loop never matches and the function falls through to `return undefined`.
    expect(getResourceSourceStatus(withSourceStatus([]), 'truenas')).toBeUndefined();
  });

  it('returns undefined when the matched status value is itself a primitive', () => {
    // Loop matches 'custom-source', but asRecord('flat-status') -> undefined (string),
    // which is then cast and returned as undefined.
    const resource = withSourceStatus({ 'custom-source': 'flat-status' });
    expect(getResourceSourceStatus(resource, 'custom-source')).toBeUndefined();
  });
});

describe('normalizeStatus — exercised via getResourceSourceHealth (branch coverage)', () => {
  // normalizeStatus is module-private and only reachable through getResourceSourceHealth.

  it('normalizes a whitespace-padded, case-mixed status into a connected keyword', () => {
    // typeof === 'string' arm with non-trivial trim()+toLowerCase(): '  ONLINE  ' -> 'online'.
    const resource = withSourceStatus({ truenas: { status: '  ONLINE  ' } });
    expect(getResourceSourceHealth(resource, 'truenas')).toBe('connected');
  });

  it.each(['ONLINE', 'Running', 'HEALTHY', 'Connected', 'Ok'])(
    'recognizes every connected keyword after case normalization (%s)',
    (status) => {
      // String arm + Set hit for each member of CONNECTED_SOURCE_STATUSES via upper/mixed case.
      const resource = withSourceStatus({ truenas: { status } });
      expect(getResourceSourceHealth(resource, 'truenas')).toBe('connected');
    },
  );

  it('treats an empty status string as impaired (string arm yields empty normalized value)', () => {
    // ''.trim().toLowerCase() -> '' which is not in the connected set.
    const resource = withSourceStatus({ truenas: { status: '' } });
    expect(getResourceSourceHealth(resource, 'truenas')).toBe('impaired');
  });

  it('treats a numeric status as impaired (non-string arm returns empty string)', () => {
    // typeof 123 !== 'string' -> '' -> not connected.
    const resource = withSourceStatus({ truenas: { status: 123 } });
    expect(getResourceSourceHealth(resource, 'truenas')).toBe('impaired');
  });

  it('treats a boolean status as impaired (non-string arm)', () => {
    // typeof true === 'boolean' !== 'string' -> ''.
    const resource = withSourceStatus({ truenas: { status: true } });
    expect(getResourceSourceHealth(resource, 'truenas')).toBe('impaired');
  });

  it('treats an object status as impaired (non-string arm)', () => {
    // typeof {} === 'object' !== 'string' -> ''.
    const resource = withSourceStatus({ truenas: { status: { healthy: true } } });
    expect(getResourceSourceHealth(resource, 'truenas')).toBe('impaired');
  });

  it('treats a null status as impaired (typeof null === "object", non-string arm)', () => {
    // null hits the non-string arm even though typeof null === 'object', yielding ''.
    const resource = withSourceStatus({ truenas: { status: null } });
    expect(getResourceSourceHealth(resource, 'truenas')).toBe('impaired');
  });
});
