import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { BackupActivityChart } from '../BackupActivityChart';
import {
  buildBackupActivityTimeline,
  type BackupActivityMetricMode,
  type BackupActivityRangeDays,
} from '../proxmoxBackupActivityPresentation';

const initialNow = new Date(2026, 9, 19, 12);
const backupTime = new Date(2026, 9, 19, 11).getTime();
const dayButtons = () => screen.getAllByRole('button', { name: /: \d/ });
const tabStops = () => dayButtons().filter((button) => button.tabIndex === 0);

function mount(initialSelection: string | null = null) {
  const [range, setRange] = createSignal<BackupActivityRangeDays>(365);
  const [mode, setMode] = createSignal<BackupActivityMetricMode>('count');
  const [now, setNow] = createSignal(initialNow);
  const [selected, setSelected] = createSignal(initialSelection);
  const [populated, setPopulated] = createSignal(true);
  render(() => (
    <>
      <button>Before chart</button>
      <BackupActivityChart
        title="Backups per day"
        noun="backup"
        segmentKinds={['archive']}
        range={range}
        onRangeChange={setRange}
        timeline={() =>
          buildBackupActivityTimeline(
            range(),
            populated() ? [backupTime] : [],
            (time) => time,
            () => 'archive',
            { now: now(), getValue: () => (mode() === 'count' ? 1 : 1024) },
          )
        }
        selectedDateKey={selected}
        onToggleDay={(key) => setSelected(selected() === key ? null : key)}
        metricToggle={{ mode, onChange: setMode }}
      />
      <button>After chart</button>
    </>
  ));
  return { setMode, setNow, setRange, setPopulated, selected };
}

afterEach(cleanup);

describe('backup activity day keyboard navigation', () => {
  it('puts only the newest day in the Tab sequence, even for a year of activity', () => {
    mount();
    expect(dayButtons()).toHaveLength(365);
    expect(tabStops()).toEqual([dayButtons().at(-1)]);
    expect(screen.getByText(/Left and Right arrow keys/)).toBeInTheDocument();
  });

  it('starts with the selected day when a saved filter is in the visible window', () => {
    mount('2026-10-18');
    expect(tabStops()).toEqual([dayButtons().at(-2)]);
    expect(tabStops()[0]).toHaveAttribute('aria-pressed', 'true');
  });

  it('moves focus with arrows and Home/End without selecting a day or wrapping at the edges', () => {
    const controls = mount();
    const days = dayButtons();
    days.at(-1)!.focus();
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowLeft' });
    expect(document.activeElement).toBe(days.at(-2));
    expect(tabStops()).toEqual([days.at(-2)]);
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(days.at(-1));
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(days.at(-1));
    fireEvent.keyDown(document.activeElement!, { key: 'Home' });
    expect(document.activeElement).toBe(days[0]);
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowLeft' });
    expect(document.activeElement).toBe(days[0]);
    fireEvent.keyDown(document.activeElement!, { key: 'End' });
    expect(document.activeElement).toBe(days.at(-1));
    expect(controls.selected()).toBeNull();
  });

  it('keeps the focused day and updates its accessible evidence when the metric or clock changes', () => {
    const controls = mount();
    const latest = dayButtons().at(-1)!;
    latest.focus();
    controls.setMode('volume');
    expect(document.activeElement).toBe(latest);
    expect(latest).toHaveAccessibleName(/1\.00 KB/);
    expect(tabStops()).toEqual([latest]);
    controls.setNow(new Date(2026, 9, 20, 12));
    expect(document.activeElement).toBe(latest);
    expect(dayButtons().at(-2)).toBe(latest);
    expect(tabStops()).toEqual([latest]);
    fireEvent.click(latest);
    expect(controls.selected()).toBe('2026-10-19');
    expect(latest).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(latest);
    expect(controls.selected()).toBeNull();
  });

  it('recovers one Tab stop when a range no longer includes the previously focused day', () => {
    const controls = mount();
    dayButtons()[0].focus();
    controls.setRange(7);
    expect(dayButtons()).toHaveLength(7);
    expect(tabStops()).toEqual([dayButtons().at(-1)]);
  });

  it('lets pointer activation choose the re-entry day without changing the day filter contract', () => {
    const controls = mount();
    const day = dayButtons().at(-3)!;
    fireEvent.click(day);
    expect(controls.selected()).toBe('2026-10-17');
    expect(tabStops()).toEqual([day]);
    expect(day).toHaveAttribute('aria-pressed', 'true');
  });

  it('preserves browser shortcut keys and has no hidden focus stops for an empty chart', () => {
    const controls = mount();
    const latest = dayButtons().at(-1)!;
    latest.focus();
    fireEvent.keyDown(latest, { key: 'ArrowLeft', altKey: true });
    expect(document.activeElement).toBe(latest);
    controls.setPopulated(false);
    expect(screen.queryByRole('group', { name: 'Backup activity days' })).not.toBeInTheDocument();
    expect(screen.getByText(/No completed restore-point activity/)).toBeInTheDocument();
  });
});
