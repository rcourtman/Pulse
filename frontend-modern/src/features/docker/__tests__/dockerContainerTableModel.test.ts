import { describe, expect, it } from 'vitest';

import type { WorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import {
  getDockerContainerColumnWidthStyle,
  getDockerContainerTableMinWidthClass,
  getDockerContainerVisibleColumnsForLayout,
} from '../dockerContainerTableModel';

describe('dockerContainerTableModel', () => {
  it('drops host and engine columns when rows are grouped under their host', () => {
    const flat = getDockerContainerVisibleColumnsForLayout('wide', true, true, true).map(
      (column) => column.id,
    );
    const grouped = getDockerContainerVisibleColumnsForLayout('wide', true, true, true, {
      groupedByHost: true,
    }).map((column) => column.id);
    expect(flat).toEqual(expect.arrayContaining(['host', 'runtime']));
    // The group header names the host and its engine once for every row.
    expect(grouped).not.toContain('host');
    expect(grouped).not.toContain('runtime');
    expect(grouped).toEqual(flat.filter((id) => id !== 'host' && id !== 'runtime'));
  });

  it('keeps five readable container signals in ultra-narrow containers', () => {
    const columns = getDockerContainerVisibleColumnsForLayout('narrow', true, true, true);
    const ids = columns.map((column) => column.id);

    expect(ids).toEqual(['container', 'state', 'cpu', 'memory', 'updates']);
    expect(getDockerContainerColumnWidthStyle('container', 'narrow', ids)).toEqual({
      width: '40%',
    });
    expect(getDockerContainerColumnWidthStyle('cpu', 'narrow', ids)).toEqual({ width: '15%' });
  });

  it('keeps the five-field narrow projection stable when every container is running', () => {
    const ids = getDockerContainerVisibleColumnsForLayout('narrow', true, false, false).map(
      (column) => column.id,
    );

    expect(ids).toEqual(['container', 'state', 'cpu', 'memory', 'updates']);
  });

  it('keeps phone rows focused on container identity and live health signals', () => {
    const columns = getDockerContainerVisibleColumnsForLayout('phone', true, true, true);
    const ids = columns.map((column) => column.id);

    // Restarts waits for mobile width: a phone row could not fit its header
    // beside the Update control, and the count stays in the row expansion.
    expect(ids).toEqual(['container', 'state', 'cpu', 'memory', 'updates']);
    expect(getDockerContainerColumnWidthStyle('container', 'phone', ids)).toEqual({
      width: '30%',
    });
  });

  it('keeps the mobile container table on identity, state, live metrics, and governed actions', () => {
    const columns = getDockerContainerVisibleColumnsForLayout('mobile', true, true, true);
    const ids = columns.map((column) => column.id);

    // Restarts waits for the tablet layout (its header clips below 720px).
    expect(ids).toEqual(['container', 'state', 'cpu', 'memory', 'updates', 'actions']);
    expect(getDockerContainerTableMinWidthClass()).toBe('min-w-full');
    expect(getDockerContainerColumnWidthStyle('container', 'mobile', ids)).toEqual({
      width: '30%',
    });
    // State spells out a problem ("Exited (139)", about 77px with padding), so
    // it takes the room the memory bar did not need.
    expect(getDockerContainerColumnWidthStyle('state', 'mobile', ids)).toEqual({
      width: '16.371%',
    });
    expect(getDockerContainerColumnWidthStyle('memory', 'mobile', ids)).toEqual({
      width: '15.2419%',
    });
  });

  it('adds host before slower forensic fields on tablet', () => {
    expect(
      getDockerContainerVisibleColumnsForLayout('tablet', true, false, false).map(
        (column) => column.id,
      ),
    ).toEqual(['container', 'host', 'cpu', 'memory', 'updates', 'actions']);
  });

  it('drops the Host column when every container runs on one host', () => {
    const ids = getDockerContainerVisibleColumnsForLayout('compact', false, true, true, {
      singleHost: true,
    }).map((column) => column.id);
    expect(ids).not.toContain('host');
    expect(ids).toContain('image');
  });

  it('gives the State words and the Uptime header room where the table shows them', () => {
    // "Exited (139)" is about 70px at 12px and the cell pads 8px a side; the
    // Uptime header is about 45px with 6px of padding a side. Checked at each
    // layout's narrowest table, for grouped and single-host tables (a host per
    // row only appears in a flat multi-host view the user chose).
    const narrowest: [WorkloadTableLayoutMode, number][] = [
      ['compact', 900],
      ['wide', 1440],
    ];
    for (const [layoutMode, tableWidth] of narrowest) {
      for (const includeRestarts of [false, true]) {
        for (const hostOption of [{ groupedByHost: true }, { singleHost: true }]) {
          const ids = getDockerContainerVisibleColumnsForLayout(
            layoutMode,
            false,
            includeRestarts,
            true,
            hostOption,
          ).map((column) => column.id);
          const pixels = (columnId: 'state' | 'uptime') =>
            (Number.parseFloat(
              String(getDockerContainerColumnWidthStyle(columnId, layoutMode, ids).width),
            ) /
              100) *
            tableWidth;
          expect(pixels('state')).toBeGreaterThanOrEqual(86);
          expect(pixels('uptime')).toBeGreaterThanOrEqual(57);
        }
      }
    }
  });

  it('gives the Restarts header room on tablets when one host or groups hide Host', () => {
    // "Restarts" is about 61px at 11px uppercase with 6px of padding a side.
    for (const hostOption of [{ groupedByHost: true }, { singleHost: true }]) {
      const ids = getDockerContainerVisibleColumnsForLayout(
        'tablet',
        false,
        true,
        true,
        hostOption,
      ).map((column) => column.id);
      const width = Number.parseFloat(
        String(getDockerContainerColumnWidthStyle('restarts', 'tablet', ids).width),
      );
      expect((width / 100) * 720).toBeGreaterThanOrEqual(73);
    }
  });

  it('drops uptime when no container in view reports one', () => {
    const ids = getDockerContainerVisibleColumnsForLayout('compact', true, true, true, {
      includeUptime: false,
    }).map((column) => column.id);
    expect(ids).not.toContain('uptime');
    expect(
      getDockerContainerVisibleColumnsForLayout('tablet', true, true, true).map(
        (column) => column.id,
      ),
    ).not.toContain('uptime');
  });

  it('adds restarts only when the current row set has restart signal to scan', () => {
    const withoutRestarts = getDockerContainerVisibleColumnsForLayout(
      'compact',
      true,
      false,
      true,
    ).map((column) => column.id);
    const withRestarts = getDockerContainerVisibleColumnsForLayout('compact', true, true, true).map(
      (column) => column.id,
    );

    expect(withoutRestarts).not.toContain('restarts');
    expect(withRestarts).toContain('restarts');
  });

  it('adds state only when the current row set has non-running state to scan', () => {
    const withoutState = getDockerContainerVisibleColumnsForLayout(
      'compact',
      true,
      true,
      false,
    ).map((column) => column.id);
    const withState = getDockerContainerVisibleColumnsForLayout('compact', true, true, true).map(
      (column) => column.id,
    );

    expect(withoutState).not.toContain('state');
    expect(withState).toContain('state');
  });

  it('keeps compact desktop scan-focused and hides wide forensic columns', () => {
    const columns = getDockerContainerVisibleColumnsForLayout('compact', true, true, true);
    const ids = columns.map((column) => column.id);

    expect(ids).toEqual([
      'container',
      'host',
      'runtime',
      'image',
      'state',
      'cpu',
      'memory',
      'restarts',
      'uptime',
      'ports',
      'updates',
      'actions',
    ]);
    expect(ids).not.toContain('health');
    expect(ids).not.toContain('networks');
    expect(ids).not.toContain('mounts');
  });

  it('shows runtime only for mixed Docker and Podman fleets', () => {
    const compactIds = getDockerContainerVisibleColumnsForLayout('compact', false, true, true).map(
      (column) => column.id,
    );
    const wideIds = getDockerContainerVisibleColumnsForLayout('wide', false, true, true).map(
      (column) => column.id,
    );

    expect(compactIds).not.toContain('runtime');
    expect(wideIds).not.toContain('runtime');
    expect(wideIds).toContain('networks');
    expect(wideIds).toContain('mounts');
  });

  it('keeps the update control unclipped in every layout and optional column set', () => {
    // Update and Current render about 74px wide, and the cell pads 6px a side
    // below a 640px viewport and 8px above. Each layout is checked at its
    // narrowest table: the 34rem phone container for mobile (below it the
    // badge wraps instead), then 720, 900 and 1440px. Wide rows expand the
    // lifecycle controls to about 92px.
    const narrowest: [WorkloadTableLayoutMode, number, number][] = [
      ['mobile', 544, 12],
      ['tablet', 720, 16],
      ['compact', 900, 16],
      ['wide', 1440, 16],
    ];
    const pixels = (
      columnId: 'updates' | 'actions',
      layoutMode: WorkloadTableLayoutMode,
      ids: ReturnType<typeof getDockerContainerVisibleColumnsForLayout>,
      tableWidth: number,
    ) =>
      (Number.parseFloat(
        String(
          getDockerContainerColumnWidthStyle(
            columnId,
            layoutMode,
            ids.map((column) => column.id),
          ).width,
        ),
      ) /
        100) *
      tableWidth;
    for (const [layoutMode, tableWidth, cellPadding] of narrowest) {
      for (const includeRuntime of [false, true]) {
        for (const includeRestarts of [false, true]) {
          for (const includeState of [false, true]) {
            for (const groupedByHost of [false, true]) {
              const columns = getDockerContainerVisibleColumnsForLayout(
                layoutMode,
                includeRuntime,
                includeRestarts,
                includeState,
                { groupedByHost },
              );
              expect(pixels('updates', layoutMode, columns, tableWidth)).toBeGreaterThanOrEqual(
                74 + cellPadding,
              );
              if (layoutMode === 'wide') {
                expect(pixels('actions', layoutMode, columns, tableWidth)).toBeGreaterThanOrEqual(
                  92 + cellPadding,
                );
              }
            }
          }
        }
      }
    }
  });
});

describe('docker container alert override identity', () => {
  it('resolves alert thresholds through the shared name-first override candidate chain', async () => {
    // Overrides key on docker:{host}/{containerName} so they survive
    // container recreates (#1601). The table must hand the full container
    // resource to the shared candidate builder instead of parsing an id
    // tail locally.
    const source = (await import('../DockerContainersTable.tsx?raw')).default;
    expect(source).toContain(
      "import { dockerContainerOverrideIdCandidates } from '@/features/alerts/alertOverridesModel'",
    );
    expect(source).toContain('dockerContainerOverrideIdCandidates(hostResource, resource)');
    expect(source).not.toContain("split('/')");
  });
});
