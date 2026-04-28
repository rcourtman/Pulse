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
  getUnifiedResourceTableColumnPresentations,
  getUnifiedResourceTableHeaderLabels,
  getUnifiedResourceTableLayoutMode,
  getUnifiedResourceTableShellClass,
  getUnifiedResourceTableSortIndicator,
  getUnifiedSources,
  getVisibleHostTableItems,
  isUnifiedResourceHostDiskIoVisible,
  isUnifiedResourceServiceColumnVisible,
  isUnifiedResourceTableColumnVisible,
  normalizeUnifiedResourceTableLayoutWidth,
  shouldShowUnifiedResourceHostTable,
  shouldUseUnifiedResourceTableMobileLayout,
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
    expect(
      getVisibleHostTableItems(items, true, 1, 3).map((item) =>
        item.type === 'row' ? item.resource.id : item.type,
      ),
    ).toEqual(['b', 'c']);
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

  it('derives responsive column presentations and host-table visibility as pure layout policy', () => {
    const mobileColumns = getUnifiedResourceTableColumnPresentations('mobile');
    const narrowTabletColumns = getUnifiedResourceTableColumnPresentations('tablet', 760);
    const tabletColumns = getUnifiedResourceTableColumnPresentations('tablet', 820);
    const compactColumns = getUnifiedResourceTableColumnPresentations('compact');
    const wideColumns = getUnifiedResourceTableColumnPresentations('wide');

    expect(getUnifiedResourceTableShellClass('mobile')).toContain('table-fixed');
    expect(getUnifiedResourceTableShellClass('mobile')).toContain('min-w-full');
    expect(getUnifiedResourceTableShellClass('compact')).toContain('min-w-full');
    expect(getUnifiedResourceTableShellClass('wide')).toContain('min-w-full');
    expect(getUnifiedResourceTableShellClass('wide')).not.toContain('min-w-[max-content]');
    // Mobile and tablet use percentage widths so the visible-column set fills
    // the table surface without horizontal overflow. Wider modes keep all
    // host columns visible while compressing their tracks before any
    // lower-priority columns are dropped.
    expect(mobileColumns.resourceColumn.width).toBe('40%');
    expect(mobileColumns.metricColumn.width).toBe('20%');
    expect(mobileColumns.serviceResourceColumn.width).toBe('28%');
    expect(mobileColumns.serviceSourceColumn.width).toBe('10%');
    expect(mobileColumns.serviceCountColumn.width).toBe('11%');
    expect(mobileColumns.serviceHealthColumn.width).toBe('13%');
    expect(mobileColumns.serviceActionColumn.width).toBe('14%');
    expect(narrowTabletColumns.resourceColumn.width).toBe('34%');
    expect(narrowTabletColumns.ioColumn.width).toBe('14%');
    expect(narrowTabletColumns.sourceColumn.width).toBe('13%');
    expect(isUnifiedResourceHostDiskIoVisible(799)).toBe(false);
    expect(isUnifiedResourceHostDiskIoVisible(800)).toBe(true);
    expect(tabletColumns.resourceColumn.width).toBe('28%');
    expect(tabletColumns.serviceResourceColumn.width).toBe('24%');
    expect(tabletColumns.metricColumn.width).toBe('12%');
    expect(tabletColumns.ioColumn.width).toBe('12%');
    expect(tabletColumns.sourceColumn.width).toBe('12%');
    expect(tabletColumns.serviceSourceColumn.width).toBe('8%');
    expect(compactColumns.resourceColumn.width).toBe('16%');
    expect(compactColumns.serviceResourceColumn.width).toBe('18%');
    expect(compactColumns.metricColumn.width).toBe('10%');
    expect(compactColumns.sourceColumn.width).toBe('13%');
    expect(compactColumns.serviceSourceColumn.width).toBe('9.5%');
    expect(compactColumns.tempColumn.width).toBe('6.5%');
    expect(wideColumns.resourceColumn.width).toBe('18%');
    expect(wideColumns.serviceResourceColumn.width).toBe('18%');
    expect(wideColumns.serviceSourceColumn.width).toBe('10%');
    expect(wideColumns.ioColumn.width).toBe('12.5%');
    expect(wideColumns.serviceActionColumn.width).toBe('16%');
    expect(getUnifiedResourceTableHeaderLabels('wide').memory).toBe('Memory');
    expect(getUnifiedResourceTableHeaderLabels('wide').source).toBe('Platform');
    expect(getUnifiedResourceTableHeaderLabels('compact').memory).toBe('Mem');
    expect(getUnifiedResourceTableHeaderLabels('compact').source).toBe('Platform');
    expect(getUnifiedResourceTableHeaderLabels('tablet').network).toBe('Net');
    expect(getUnifiedResourceTableHeaderLabels('tablet').source).toBe('Plat');
    expect(getUnifiedResourceTableHeaderLabels('mobile').datastores).toBe('Store');
    expect(shouldShowUnifiedResourceHostTable(0, 0)).toBe(true);
    expect(shouldShowUnifiedResourceHostTable(0, 2)).toBe(false);
    expect(shouldShowUnifiedResourceHostTable(3, 2)).toBe(true);
  });

  it('derives infrastructure table breakpoints from the measured table surface width', () => {
    expect(normalizeUnifiedResourceTableLayoutWidth(820.4)).toBe(820);
    expect(normalizeUnifiedResourceTableLayoutWidth(null, 700)).toBe(700);
    expect(getUnifiedResourceTableLayoutMode(699)).toBe('mobile');
    expect(getUnifiedResourceTableLayoutMode(700)).toBe('tablet');
    expect(getUnifiedResourceTableLayoutMode(899)).toBe('tablet');
    expect(getUnifiedResourceTableLayoutMode(900)).toBe('compact');
    expect(getUnifiedResourceTableLayoutMode(1159)).toBe('compact');
    expect(getUnifiedResourceTableLayoutMode(1160)).toBe('wide');
    expect(shouldUseUnifiedResourceTableMobileLayout(699)).toBe(true);
    expect(shouldUseUnifiedResourceTableMobileLayout(700)).toBe(false);
    expect(isUnifiedResourceTableColumnVisible('primary', 640)).toBe(true);
    expect(isUnifiedResourceTableColumnVisible('secondary', 699)).toBe(false);
    expect(isUnifiedResourceTableColumnVisible('secondary', 700)).toBe(true);
    expect(isUnifiedResourceTableColumnVisible('supplementary', 899)).toBe(false);
    expect(isUnifiedResourceTableColumnVisible('supplementary', 900)).toBe(true);
    expect(isUnifiedResourceTableColumnVisible('detailed', 1159)).toBe(false);
    expect(isUnifiedResourceTableColumnVisible('detailed', 1160)).toBe(true);
    expect(isUnifiedResourceServiceColumnVisible('primary', 499)).toBe(false);
    expect(isUnifiedResourceServiceColumnVisible('primary', 500)).toBe(true);
    expect(isUnifiedResourceServiceColumnVisible('secondary', 579)).toBe(false);
    expect(isUnifiedResourceServiceColumnVisible('secondary', 580)).toBe(true);
    expect(isUnifiedResourceServiceColumnVisible('supplementary', 639)).toBe(false);
    expect(isUnifiedResourceServiceColumnVisible('supplementary', 640)).toBe(true);
    expect(isUnifiedResourceServiceColumnVisible('detailed', 899)).toBe(false);
    expect(isUnifiedResourceServiceColumnVisible('detailed', 900)).toBe(true);
  });

  it('reads unified source tags from platform data without hook state', () => {
    expect(
      getUnifiedSources(makeResource('a', { platformData: { sources: ['proxmox', 'agent'] } })),
    ).toEqual(['proxmox', 'agent']);
    expect(getUnifiedSources(makeResource('b', { platformData: {} }))).toEqual([]);
  });
});
