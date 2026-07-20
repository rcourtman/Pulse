import { describe, expect, it } from 'vitest';

import {
  getGuestOverrideIdentity,
  guestOverrideIdCandidates,
  guestOverrideStorageId,
  normalizeGuestOverrideKey,
} from '../guestOverrideIdentity';

describe('guestOverrideIdentity', () => {
  describe('getGuestOverrideIdentity', () => {
    it('returns undefined when no identity can be derived', () => {
      expect(getGuestOverrideIdentity({})).toBeUndefined();
      expect(getGuestOverrideIdentity({ id: 'res-1' })).toBeUndefined();
      expect(getGuestOverrideIdentity({ node: 'pve' })).toBeUndefined();
      expect(getGuestOverrideIdentity({ instance: 'cluster-a' })).toBeUndefined();
    });

    it('derives identity from direct resource fields', () => {
      expect(
        getGuestOverrideIdentity({
          id: 'res-1',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual({
        id: 'res-1',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 100,
      });
    });

    it('parses a string vmid into a positive integer', () => {
      expect(
        getGuestOverrideIdentity({ instance: 'cluster-a', node: 'node-1', vmid: '100' }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('trims whitespace on string components', () => {
      expect(
        getGuestOverrideIdentity({
          id: '  res-1  ',
          instance: '  cluster-a  ',
          node: '  node-1  ',
          vmid: '  100  ',
        }),
      ).toEqual({ id: 'res-1', instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('rejects node values that contain a slash and falls through to the next source', () => {
      expect(
        getGuestOverrideIdentity({
          node: 'cluster/node',
          proxmox: { node: 'node-1' },
          instance: 'cluster-a',
          vmid: 100,
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it.each([
      ['zero number', 0],
      ['negative number', -5],
      ['float number', 1.5],
      ['NaN', Number.NaN],
      ['zero string', '0'],
      ['negative string', '-1'],
      ['float string', '1.5'],
      ['non-numeric string', 'abc'],
      ['empty string', ''],
    ])('rejects an invalid vmid (%s)', (_label, vmid) => {
      expect(
        getGuestOverrideIdentity({ instance: 'cluster-a', node: 'node-1', vmid }),
      ).toBeUndefined();
    });

    it.each([
      ['Infinity', Number.POSITIVE_INFINITY],
      ['negative Infinity', Number.NEGATIVE_INFINITY],
    ])('rejects an infinite vmid (%s)', (_label, vmid) => {
      expect(
        getGuestOverrideIdentity({ instance: 'cluster-a', node: 'node-1', vmid }),
      ).toBeUndefined();
    });

    it('resolves node and vmid from the direct proxmox block when top-level fields are absent', () => {
      expect(
        getGuestOverrideIdentity({
          instance: 'cluster-a',
          proxmox: { node: 'node-1', vmid: 100 },
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('resolves node from proxmox.nodeName as a fallback', () => {
      expect(
        getGuestOverrideIdentity({
          instance: 'cluster-a',
          proxmox: { nodeName: 'node-1', vmid: 100 },
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('resolves identity from platformData.proxmox when proxmox is absent', () => {
      expect(
        getGuestOverrideIdentity({
          platformData: { proxmox: { instance: 'cluster-a', node: 'node-1', vmid: 100 } },
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('resolves identity from platformData directly when no proxmox blocks exist', () => {
      expect(
        getGuestOverrideIdentity({
          platformData: { instance: 'cluster-a', node: 'node-1', vmid: 100 },
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('prefers direct resource.node over nested sources', () => {
      expect(
        getGuestOverrideIdentity({
          node: 'direct-node',
          proxmox: { node: 'proxmox-node', nodeName: 'proxmox-node-name' },
          platformData: { proxmox: { node: 'platform-node' }, node: 'platform-data-node' },
          instance: 'cluster-a',
          vmid: 100,
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'direct-node', vmid: 100 });
    });

    it('prefers proxmox.node over proxmox.nodeName and platform sources', () => {
      expect(
        getGuestOverrideIdentity({
          proxmox: { node: 'proxmox-node', nodeName: 'proxmox-node-name' },
          platformData: { proxmox: { node: 'platform-node' }, node: 'platform-data-node' },
          instance: 'cluster-a',
          vmid: 100,
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'proxmox-node', vmid: 100 });
    });

    it('prefers platformData.proxmox.node over platformData.node', () => {
      expect(
        getGuestOverrideIdentity({
          platformData: { proxmox: { node: 'platform-proxmox-node' }, node: 'platform-data-node' },
          instance: 'cluster-a',
          vmid: 100,
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'platform-proxmox-node', vmid: 100 });
    });

    it('falls instance back to the resolved node when no instance source is available', () => {
      expect(getGuestOverrideIdentity({ node: 'pve', vmid: 100 })).toEqual({
        id: undefined,
        instance: 'pve',
        node: 'pve',
        vmid: 100,
      });
    });

    it('ignores non-object proxmox and platformData values', () => {
      expect(
        getGuestOverrideIdentity({
          proxmox: 'not-an-object',
          platformData: null,
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });

    it('returns undefined id when resource.id is blank', () => {
      expect(
        getGuestOverrideIdentity({ id: '   ', instance: 'cluster-a', node: 'node-1', vmid: 100 }),
      ).toEqual({ id: undefined, instance: 'cluster-a', node: 'node-1', vmid: 100 });
    });
  });

  describe('guestOverrideStorageId', () => {
    it('returns the trimmed resource id when no identity can be derived', () => {
      expect(guestOverrideStorageId({ id: 'res-1' })).toBe('res-1');
      expect(guestOverrideStorageId({ id: '  res-1  ' })).toBe('res-1');
    });

    it('returns an empty string when no identity and no usable id exist', () => {
      expect(guestOverrideStorageId({})).toBe('');
      expect(guestOverrideStorageId({ id: '   ' })).toBe('');
      expect(guestOverrideStorageId({ id: undefined })).toBe('');
    });

    it('uses the stable guest:instance:vmid key for cluster guests (instance !== node)', () => {
      expect(guestOverrideStorageId({ instance: 'cluster-a', node: 'node-1', vmid: 100 })).toBe(
        'guest:cluster-a:100',
      );
    });

    it('uses the canonical instance:node:vmid key for standalone guests (instance === node)', () => {
      expect(guestOverrideStorageId({ instance: 'pve', node: 'pve', vmid: 100 })).toBe(
        'pve:pve:100',
      );
    });

    it('ignores resource.id when a guest identity is available', () => {
      expect(
        guestOverrideStorageId({
          id: 'res-1',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toBe('guest:cluster-a:100');
    });
  });

  describe('guestOverrideIdCandidates', () => {
    it('returns only the resource id when no identity can be derived', () => {
      expect(guestOverrideIdCandidates({ id: 'res-1' })).toEqual(['res-1']);
      expect(guestOverrideIdCandidates({ id: '  res-1  ' })).toEqual(['res-1']);
    });

    it('returns an empty array when no identity and no usable id exist', () => {
      expect(guestOverrideIdCandidates({})).toEqual([]);
      expect(guestOverrideIdCandidates({ id: '   ' })).toEqual([]);
    });

    it('builds the full candidate set for cluster guests including legacy keys', () => {
      expect(
        guestOverrideIdCandidates({
          id: 'res-1',
          instance: 'cluster-a',
          node: 'node-1',
          vmid: 100,
        }),
      ).toEqual([
        'guest:cluster-a:100',
        'cluster-a:node-1:100',
        'res-1',
        'cluster-a-100',
        'cluster-a-node-1-100',
      ]);
    });

    it('omits the legacy cluster key for standalone guests and dedupes stable/canonical', () => {
      expect(
        guestOverrideIdCandidates({ id: 'res-1', instance: 'pve', node: 'pve', vmid: 100 }),
      ).toEqual(['pve:pve:100', 'res-1', 'pve-100']);
    });

    it('omits the id candidate when resource.id is missing or blank', () => {
      expect(
        guestOverrideIdCandidates({ instance: 'cluster-a', node: 'node-1', vmid: 100 }),
      ).toEqual([
        'guest:cluster-a:100',
        'cluster-a:node-1:100',
        'cluster-a-100',
        'cluster-a-node-1-100',
      ]);
      expect(
        guestOverrideIdCandidates({ id: '  ', instance: 'pve', node: 'pve', vmid: 100 }),
      ).toEqual(['pve:pve:100', 'pve-100']);
    });
  });

  describe('normalizeGuestOverrideKey', () => {
    it.each([
      ['canonical cluster key', 'cluster-a:node-1:100', 'guest:cluster-a:100'],
      ['stable guest key', 'guest:cluster-a:100', 'guest:cluster-a:100'],
      ['canonical standalone key', 'pve:pve:100', 'pve:pve:100'],
      ['whitespace-padded canonical', '  cluster-a:node-1:100  ', 'guest:cluster-a:100'],
      ['whitespace-padded stable', '  guest:cluster-a:100  ', 'guest:cluster-a:100'],
      ['too few parts', 'cluster-a:100', 'cluster-a:100'],
      ['too many parts', 'cluster-a:node-1:100:extra', 'cluster-a:node-1:100:extra'],
      ['zero vmid', 'cluster-a:node-1:0', 'cluster-a:node-1:0'],
      ['negative vmid', 'cluster-a:node-1:-1', 'cluster-a:node-1:-1'],
      ['node with slash', 'cluster-a:node/1:100', 'cluster-a:node/1:100'],
      ['capital-G Guest prefix', 'Guest:cluster-a:100', 'Guest:cluster-a:100'],
      ['empty string', '', ''],
      ['only whitespace', '   ', ''],
    ])('normalizes %s to the expected form', (_label, input, expected) => {
      expect(normalizeGuestOverrideKey(input)).toBe(expected);
    });

    it('is idempotent for an already-stable cluster key', () => {
      const stable = normalizeGuestOverrideKey('cluster-a:node-1:100');
      expect(normalizeGuestOverrideKey(stable)).toBe(stable);
    });
  });
});
