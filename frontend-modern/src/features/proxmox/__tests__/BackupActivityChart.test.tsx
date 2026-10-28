import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { getRecoveryPrettyDateLabel } from '@/utils/recoveryDatePresentation';
import { BackupActivityChart } from '../BackupActivityChart';
import type {
  BackupActivityMetricMode,
  BackupActivityPoint,
  BackupActivityTimeline,
} from '../proxmoxBackupActivityPresentation';

vi.mock('@/components/shared/hoverCapability', () => ({ supportsHoverTooltips: () => true }));

const point = (key: string, total: number, unknown = 0): BackupActivityPoint => ({
  key,
  total,
  counts: { archive: total, pbs: 0, snapshot: 0, ok: 0, failed: 0, running: 0 },
  ...(unknown ? { unknownCounts: { pbs: unknown } } : {}),
});
const timeline = (points: BackupActivityPoint[]): BackupActivityTimeline => ({
  points,
  axisMax: Math.max(2, ...points.map((point) => point.total)),
  labelEvery: 1,
});
const day = (key: string): HTMLButtonElement =>
  document.querySelector(`[data-backup-activity-day="${key}"]`)!;
const tooltip = () => document.querySelector<HTMLElement>('[data-tooltip-portal]');

function mount() {
  const [data, setData] = createSignal(timeline([point('2026-10-27', 1), point('2026-10-28', 2)]));
  const [selected, setSelected] = createSignal<string | null>(null);
  const [mode, setMode] = createSignal<BackupActivityMetricMode>('count');
  const onToggleDay = vi.fn((key: string) => setSelected(selected() === key ? null : key));
  render(() => (
    <>
      <button type="button">Outside chart</button>
      <BackupActivityChart
        title="Restore points"
        noun="restore point"
        segmentKinds={['archive', 'pbs', 'snapshot']}
        range={() => 30}
        onRangeChange={() => {}}
        timeline={data}
        selectedDateKey={selected}
        onToggleDay={onToggleDay}
        metricToggle={{ mode, onChange: setMode }}
      />
    </>
  ));
  return { data, setData, selected, setMode, onToggleDay };
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('backup activity interaction continuity', () => {
  it('keeps the exact focused day across fresh inventory objects and updates its reading', () => {
    const { setData, selected } = mount();
    const target = day('2026-10-28');
    target.focus();
    expect(tooltip()).toHaveTextContent('2 restore points');

    setData(timeline([point('2026-10-27', 1), point('2026-10-28', 5)]));

    expect(day('2026-10-28')).toBe(target);
    expect(target).toHaveFocus();
    expect(target).toHaveAccessibleName(/5 restore points/);
    expect(tooltip()).toHaveTextContent('5 restore points');
    expect(tooltip()).not.toHaveTextContent('2 restore points');
    fireEvent.click(target);
    expect(selected()).toBe('2026-10-28');
    expect(target).toHaveFocus();
    expect(target).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(target);
    expect(selected()).toBeNull();
    expect(target).toHaveAttribute('aria-pressed', 'false');
  });

  it('updates a hovered day to mixed, unknown-only and known volume without retaining old counts', () => {
    const { setData, setMode } = mount();
    const target = day('2026-10-28');
    fireEvent.mouseEnter(target);
    expect(tooltip()).toHaveTextContent('2 restore points');
    setMode('volume');
    setData(timeline([point('2026-10-27', 1), point('2026-10-28', 2048, 1)]));
    expect(day('2026-10-28')).toBe(target);
    expect(tooltip()).toHaveTextContent('2.00 KB known · 1 size unknown');
    expect(target).toHaveAccessibleName(/2.00 KB known · 1 size unknown/);
    expect(target.querySelector('[data-unknown-volume]')).not.toBeNull();

    setData(timeline([point('2026-10-27', 1), point('2026-10-28', 0, 2)]));
    expect(tooltip()).toHaveTextContent('2 sizes unknown');
    expect(tooltip()).not.toHaveTextContent('2.00 KB');
    setData(timeline([point('2026-10-27', 1), point('2026-10-28', 4096)]));
    expect(tooltip()).toHaveTextContent('4.00 KB');
    expect(tooltip()).not.toHaveTextContent('unknown');
    expect(target.querySelector('[data-unknown-volume]')).toBeNull();
    fireEvent.mouseLeave(target);
    expect(tooltip()).toBeNull();
  });

  it('reanchors the retained reading after inventory layout changes', async () => {
    const { setData } = mount();
    const target = day('2026-10-28');
    let top = 200;
    vi.spyOn(target, 'getBoundingClientRect').mockImplementation(
      () => new DOMRect(100, top, 20, 80),
    );
    target.focus();
    const portal = () => document.querySelector('foreignObject');
    await waitFor(() => expect(portal()).toHaveAttribute('y', '192'));
    top = 250;
    setData(timeline([point('2026-10-27', 1), point('2026-10-28', 3)]));
    await waitFor(() => expect(portal()).toHaveAttribute('y', '242'));
    expect(target).toHaveFocus();
    expect(tooltip()).toHaveTextContent('3 restore points');
  });

  it('keeps a surviving calendar day, not its array index, when the window rolls over', () => {
    const { setData } = mount();
    const target = day('2026-10-28');
    target.focus();
    setData(timeline([point('2026-10-28', 3), point('2026-10-29', 1)]));
    expect(day('2026-10-28')).toBe(target);
    expect(target).toHaveFocus();
    expect(target).toHaveAccessibleName(
      `${getRecoveryPrettyDateLabel('2026-10-28')}: 3 restore points`,
    );
    expect(day('2026-10-29')).not.toBe(target);
  });

  it.each([false, true])(
    'returns a retired focused day to the selected range (empty=%s)',
    (empty) => {
      const { setData, selected, onToggleDay } = mount();
      const target = day('2026-10-27');
      target.focus();
      fireEvent.click(target);
      setData(timeline(empty ? [] : [point('2026-10-28', 2), point('2026-10-29', 1)]));
      expect(target).not.toBeInTheDocument();
      expect(
        within(screen.getByRole('group', { name: 'Activity range' })).getByRole('button', {
          name: '30d',
        }),
      ).toHaveFocus();
      expect(tooltip()).toBeNull();
      expect(selected()).toBe('2026-10-27');
      expect(onToggleDay).toHaveBeenCalledTimes(1);
    },
  );

  it('withdraws a retired hover reading without stealing focus or mutating the caller data', () => {
    const { setData } = mount();
    const outside = screen.getByRole('button', { name: 'Outside chart' });
    outside.focus();
    fireEvent.mouseEnter(day('2026-10-27'));
    expect(tooltip()).toHaveTextContent('1 restore point');
    const incoming = timeline([point('2026-10-28', 9), point('2026-10-29', 1)]);
    const saved = JSON.stringify(incoming);
    setData(incoming);
    expect(tooltip()).toBeNull();
    expect(outside).toHaveFocus();
    setData(timeline([point('2026-10-28', 4), point('2026-10-29', 1)]));
    expect(JSON.stringify(incoming)).toBe(saved);
    expect(outside).toHaveFocus();
  });
});
