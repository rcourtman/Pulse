import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadMetadataId } from '@/utils/workloads';
import {
  buildWorkloadWebLinkRows,
  filterWorkloadWebLinkRows,
  getWorkloadWebLinkChanges,
  getWorkloadWebLinkPlaceholder,
  keepStableWorkloadWebLinkRows,
  validateWorkloadWebLinkChanges,
} from '../workloadWebLinksModel';
import { getWorkloadGuestMetadataRecord } from '../workloadGuestMetadataRecord';

const makeGuest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest =>
  ({
    id: 'inst1-node1-100',
    vmid: 100,
    name: 'test-vm',
    node: 'node1',
    instance: 'inst1',
    status: 'running',
    type: 'qemu',
    ...overrides,
  }) as WorkloadGuest;

describe('workloadWebLinksModel', () => {
  it('reads stable metadata first and falls back to the legacy instance:node:vmid key', () => {
    const guest = makeGuest();
    const stableId = getWorkloadMetadataId(guest);

    expect(
      getWorkloadGuestMetadataRecord(guest, {
        'inst1:node1:100': { id: 'inst1:node1:100', customUrl: 'https://legacy.example' },
      })?.customUrl,
    ).toBe('https://legacy.example');
    expect(
      getWorkloadGuestMetadataRecord(guest, {
        'inst1:node1:100': { id: 'inst1:node1:100', customUrl: 'https://legacy.example' },
        [stableId]: { id: stableId, customUrl: 'https://stable.example' },
      })?.customUrl,
    ).toBe('https://stable.example');
  });

  it('builds name-sorted rows keyed by the save identity with type, ID and node detail', () => {
    const alpha = makeGuest({
      id: 'a',
      vmid: 101,
      name: 'alpha-10',
      ipAddresses: ['', '10.0.0.5'],
    });
    const beta = makeGuest({ id: 'b', vmid: 102, name: 'alpha-9', type: 'lxc', node: 'node2' });
    const rows = buildWorkloadWebLinkRows([alpha, beta, alpha], {
      [getWorkloadMetadataId(alpha)]: {
        id: getWorkloadMetadataId(alpha),
        customUrl: ' https://alpha.example ',
      },
    });

    expect(rows.map((row) => row.name)).toEqual(['alpha-9', 'alpha-10']);
    expect(rows[1]).toMatchObject({
      metadataId: getWorkloadMetadataId(alpha),
      savedUrl: 'https://alpha.example',
      addressHint: '10.0.0.5',
    });
    expect(rows[1].detail).toContain('101');
    expect(rows[1].detail).toContain('node1');
    expect(rows[0].detail).toContain('node2');
    expect(rows[0].savedUrl).toBe('');
  });

  it('reports only real changes, treating an emptied field as a removal', () => {
    const rows = buildWorkloadWebLinkRows(
      [makeGuest({ id: 'a', name: 'a', vmid: 1 }), makeGuest({ id: 'b', name: 'b', vmid: 2 })],
      {},
    );
    const [a, b] = rows;
    const withSaved = [{ ...a, savedUrl: 'https://a.example' }, b];

    expect(
      getWorkloadWebLinkChanges(withSaved, {
        [a.metadataId]: '  https://a.example ',
        [b.metadataId]: '',
      }),
    ).toEqual([]);
    expect(
      getWorkloadWebLinkChanges(withSaved, {
        [a.metadataId]: '',
        [b.metadataId]: ' https://b.example',
      }),
    ).toEqual([
      { metadataId: a.metadataId, name: 'a', url: '' },
      { metadataId: b.metadataId, name: 'b', url: 'https://b.example' },
    ]);
  });

  it('validates added links with the shared web-interface rules but never blocks a removal', () => {
    expect(
      validateWorkloadWebLinkChanges([
        { metadataId: 'ok', name: 'ok', url: 'http://10.0.0.5:8080' },
        { metadataId: 'bad', name: 'bad', url: 'admin page' },
        { metadataId: 'ftp', name: 'ftp', url: 'ftp://files.example' },
        { metadataId: 'gone', name: 'gone', url: '' },
      ]),
    ).toEqual({
      bad: 'Enter a valid URL (for example: https://198.51.100.100:8080).',
      ftp: 'URL must start with http:// or https://.',
    });
  });

  it('filters to guests without a saved link, ignoring unsaved drafts', () => {
    const rows = buildWorkloadWebLinkRows(
      [makeGuest({ id: 'a', name: 'a', vmid: 1 }), makeGuest({ id: 'b', name: 'b', vmid: 2 })],
      {},
    );
    const withSaved = [{ ...rows[0], savedUrl: 'https://a.example' }, rows[1]];

    expect(filterWorkloadWebLinkRows(withSaved, 'missing').map((row) => row.name)).toEqual(['b']);
    expect(filterWorkloadWebLinkRows(withSaved, 'all')).toHaveLength(2);
  });

  it('keeps unchanged row objects across inventory ticks so inputs are not remounted', () => {
    const guest = makeGuest({ ipAddresses: ['10.0.0.5'] });
    const first = buildWorkloadWebLinkRows([guest], {});
    const tick = buildWorkloadWebLinkRows([{ ...guest, cpu: 0.9 } as WorkloadGuest], {});

    expect(tick[0]).not.toBe(first[0]);
    expect(keepStableWorkloadWebLinkRows(first, tick)).toBe(first);

    const renamed = buildWorkloadWebLinkRows([{ ...guest, name: 'renamed' }], {});
    const merged = keepStableWorkloadWebLinkRows(first, renamed);
    expect(merged).not.toBe(first);
    expect(merged[0].name).toBe('renamed');
  });

  it('suggests the guest address as a placeholder, bracketing IPv6', () => {
    const [row] = buildWorkloadWebLinkRows([makeGuest()], {});

    expect(getWorkloadWebLinkPlaceholder(row)).toBe('https://');
    expect(getWorkloadWebLinkPlaceholder({ ...row, addressHint: '10.0.0.5' })).toBe(
      'http://10.0.0.5',
    );
    expect(getWorkloadWebLinkPlaceholder({ ...row, addressHint: 'fd00::5' })).toBe(
      'http://[fd00::5]',
    );
  });
});
