import { describe, expect, it } from 'vitest';

import {
  getDockerContainerColumnWidthStyle,
  getDockerContainerTableMinWidthClass,
  getDockerContainerVisibleColumnsForLayout,
} from '../dockerContainerTableModel';

describe('dockerContainerTableModel', () => {
  it('keeps the mobile container table on identity, state, live metrics, and update action', () => {
    const columns = getDockerContainerVisibleColumnsForLayout('mobile', true, true, true);
    const ids = columns.map((column) => column.id);

    expect(ids).toEqual(['container', 'state', 'cpu', 'memory', 'updates']);
    expect(getDockerContainerTableMinWidthClass()).toBe('min-w-full');
    expect(getDockerContainerColumnWidthStyle('container', 'mobile', ids)).toEqual({
      width: '32%',
    });
    expect(getDockerContainerColumnWidthStyle('memory', 'mobile', ids)).toEqual({
      width: '22%',
    });
  });

  it('adds host before slower forensic fields on tablet', () => {
    expect(
      getDockerContainerVisibleColumnsForLayout('tablet', true, false, false).map(
        (column) => column.id,
      ),
    ).toEqual(['container', 'host', 'cpu', 'memory', 'updates']);
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
      'ports',
      'updates',
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
});
