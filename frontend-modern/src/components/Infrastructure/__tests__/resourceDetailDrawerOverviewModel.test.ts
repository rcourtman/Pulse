import { describe, expect, it } from 'vitest';

import type { DetailSection } from '@/components/shared/detailSectionModel';
import {
  buildResourceDrawerOverviewInventory,
  drawerContentCardTestId,
  isDrawerContentCard,
} from '../resourceDetailDrawerOverviewModel';

const runtime: DetailSection = {
  label: 'Runtime context',
  rows: [
    { label: 'Observed state', value: 'Online' },
    { label: 'Uptime', value: '3d' },
  ],
};

const poolHealth: DetailSection = {
  label: 'Health',
  testId: drawerContentCardTestId('truenas-pool-health'),
  rows: [
    { label: 'State', value: 'DEGRADED', tone: 'warning' },
    { label: 'ZFS errors', value: 'Read 1 · Write 0 · Checksum 2', tone: 'warning' },
    { label: 'Risk summary', value: 'ZFS pool tank is DEGRADED', tone: 'warning' },
  ],
};

const placement: DetailSection = {
  label: 'State',
  rows: [
    { label: 'Accessible', value: 'No', tone: 'warning' },
    { label: 'Datastore type', value: 'VMFS' },
  ],
};

describe('buildResourceDrawerOverviewInventory', () => {
  it('lifts warning rows from platform sections as well as the summary', () => {
    const inventory = buildResourceDrawerOverviewInventory({
      summarySections: [runtime],
      platformSections: [placement],
    });
    expect(inventory.lifted).toEqual([{ label: 'Accessible', value: 'No', tone: 'warning' }]);
    // A lifted row moves rather than repeats; the rest stays under Details.
    expect(inventory.details).toEqual([
      runtime,
      { label: 'State', rows: [{ label: 'Datastore type', value: 'VMFS' }] },
    ]);
    expect(inventory.cards).toEqual([]);
  });

  it('keeps a content card whole and out of Details', () => {
    const inventory = buildResourceDrawerOverviewInventory({
      summarySections: [runtime],
      platformSections: [poolHealth],
    });
    expect(isDrawerContentCard(poolHealth)).toBe(true);
    expect(inventory.cards).toEqual([poolHealth]);
    expect(inventory.details).toEqual([runtime]);
    expect(inventory.lifted.map((row) => row.label)).toEqual([
      'State',
      'ZFS errors',
      'Risk summary',
    ]);
  });

  it('does not lift a sentence the alerts block already states', () => {
    const inventory = buildResourceDrawerOverviewInventory({
      summarySections: [],
      platformSections: [poolHealth],
      statedMessages: ['Pool tank needs attention. ZFS pool tank is DEGRADED.'],
    });
    expect(inventory.lifted.map((row) => row.label)).toEqual(['State', 'ZFS errors']);
    // The card still holds the stated row: it is the pool's full reading.
    expect(inventory.cards[0].rows).toHaveLength(3);
  });

  it('leaves a row an alert states by label in Details instead of lifting it', () => {
    const inventory = buildResourceDrawerOverviewInventory({
      summarySections: [
        {
          label: 'Runtime context',
          rows: [{ label: 'Observed state', value: 'Offline', tone: 'danger' }],
        },
      ],
      platformSections: [],
      statedRowLabels: new Set(['Observed state']),
    });
    expect(inventory.lifted).toEqual([]);
    expect(inventory.details[0].rows).toEqual([
      { label: 'Observed state', value: 'Offline', tone: 'danger' },
    ]);
  });

  it('keeps a message-stated row in Details so the record stays complete', () => {
    const inventory = buildResourceDrawerOverviewInventory({
      summarySections: [],
      platformSections: [
        {
          label: 'Storage Health',
          rows: [{ label: 'Storage summary', value: 'Pool archive is degraded', tone: 'warning' }],
        },
      ],
      statedMessages: ['Pool archive is degraded'],
    });
    expect(inventory.lifted).toEqual([]);
    expect(inventory.details[0].rows.map((row) => row.label)).toEqual(['Storage summary']);
  });
});
