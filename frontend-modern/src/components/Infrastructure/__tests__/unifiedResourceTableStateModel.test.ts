import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { ResourceGroup } from '@/components/Infrastructure/infrastructureSelectors';
import {
  buildHostRowIndexById,
  buildHostTableItems,
  buildResourceLabelById,
  getHostRevealTargetIndex,
  getHostSpacerHeights,
  getNextUnifiedResourceTableSortState,
  getUnifiedResourceTableColumnStyles,
  getUnifiedResourceTableSortIndicator,
  getUnifiedSources,
  getVisibleHostTableItems,
  shouldShowUnifiedResourceHostTable,
} from '@/components/Infrastructure/unifiedResourceTableStateModel';

const makeResource = (id: string, overrides: Partial<Resource> = {}): Resource =>
  ({
    id,
    type: 'agent',
    name: `name-${id}`,
    displayName: `Display ${id}`,
    platformId: `platform-${id}`,
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { sources: ['proxmox'] },
    ...overrides,
  }) as Resource;

describe('unifiedResourceTableStateModel', () => {
  it('builds resource labels from the canonical preferred display name', () => {
    const labels = buildResourceLabelById([
      makeResource('a', { displayName: 'Alpha' }),
      makeResource('b', { displayName: 'Beta' }),
    ]);

    expect(labels.get('a')).toBe('Alpha');
    expect(labels.get('b')).toBe('Beta');
  });

  it('builds host table items with group headers only in grouped mode', () => {
    const groups: ResourceGroup[] = [
      {
        cluster: 'cluster-a',
        resources: [makeResource('a'), makeResource('b')],
      },
    ];

    expect(buildHostTableItems(groups, 'grouped').map((item) => item.type)).toEqual([
      'header',
      'row',
      'row',
    ]);
    expect(buildHostTableItems(groups, 'flat').map((item) => item.type)).toEqual(['row', 'row']);
  });

  it('tracks reveal indexes only for row items', () => {
    const items = buildHostTableItems(
      [{ cluster: 'cluster-a', resources: [makeResource('a'), makeResource('b')] }],
      'grouped',
    );
    const rowIndexById = buildHostRowIndexById(items);

    expect(getHostRevealTargetIndex(rowIndexById, 'b', null)).toBe(2);
    expect(getHostRevealTargetIndex(rowIndexById, null, 'a')).toBe(1);
    expect(getHostRevealTargetIndex(rowIndexById, null, null)).toBeNull();
  });

  it('computes visible items and spacer heights from the active window', () => {
    const items = buildHostTableItems(
      [{ cluster: '', resources: [makeResource('a'), makeResource('b'), makeResource('c')] }],
      'flat',
    );

    expect(getVisibleHostTableItems(items, false, 1, 2)).toEqual(items);
    expect(getVisibleHostTableItems(items, true, 1, 3).map((item) =>
      item.type === 'row' ? item.resource.id : item.type,
    )).toEqual(['b', 'c']);
    expect(getHostSpacerHeights(items.length, 1, 3, true, 40)).toEqual({
      top: 40,
      bottom: 0,
    });
  });

  it('cycles sort state through asc, desc, then default', () => {
    expect(getNextUnifiedResourceTableSortState('default', 'asc', 'cpu')).toEqual({
      key: 'cpu',
      direction: 'desc',
    });
    expect(getNextUnifiedResourceTableSortState('cpu', 'desc', 'cpu')).toEqual({
      key: 'default',
      direction: 'asc',
    });
    expect(getNextUnifiedResourceTableSortState('name', 'asc', 'name')).toEqual({
      key: 'name',
      direction: 'desc',
    });
    expect(getUnifiedResourceTableSortIndicator('name', 'asc', 'name')).toBe('▲');
    expect(getUnifiedResourceTableSortIndicator('cpu', 'desc', 'cpu')).toBe('▼');
    expect(getUnifiedResourceTableSortIndicator('default', 'asc', 'cpu')).toBeNull();
  });

  it('derives responsive column styles and host-table visibility as pure layout policy', () => {
    const mobileStyles = getUnifiedResourceTableColumnStyles(true);
    const desktopStyles = getUnifiedResourceTableColumnStyles(false);

    expect(mobileStyles.resourceColumnStyle.width).toBe('100%');
    expect(desktopStyles.resourceColumnStyle['min-width']).toBe('220px');
    expect(shouldShowUnifiedResourceHostTable(0, 0)).toBe(true);
    expect(shouldShowUnifiedResourceHostTable(0, 2)).toBe(false);
    expect(shouldShowUnifiedResourceHostTable(3, 2)).toBe(true);
  });

  it('reads unified source tags from platform data without hook state', () => {
    expect(getUnifiedSources(makeResource('a', { platformData: { sources: ['proxmox', 'agent'] } }))).toEqual([
      'proxmox',
      'agent',
    ]);
    expect(getUnifiedSources(makeResource('b', { platformData: {} }))).toEqual([]);
  });
});
