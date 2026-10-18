import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import type { HostRAIDArray } from '@/types/api';
import { RaidCard } from '../RaidCard';

afterEach(() => {
  cleanup();
});

const degradedArray = (): HostRAIDArray => ({
  device: '/dev/md0',
  name: 'md0',
  level: 'raid1',
  state: 'clean, degraded, recovering',
  totalDevices: 2,
  activeDevices: 1,
  workingDevices: 2,
  failedDevices: 0,
  spareDevices: 1,
  devices: [
    { device: '/dev/sda1', state: 'active sync', slot: 0 },
    { device: '/dev/sdb1', state: 'spare rebuilding', slot: 1 },
  ],
  rebuildPercent: 45.4,
  rebuildSpeed: '98000K/sec',
});

const LAST_KNOWN_TITLE = 'Last known reading, not current: host agent stopped reporting';

describe('RaidCard', () => {
  it('shows a current array with live state colour and rebuild progress', () => {
    render(() => <RaidCard arrays={[degradedArray()]} />);

    const array = screen.getByTestId('raid-card-array');
    expect(array).toHaveAttribute('data-raid-reading', 'current');
    const state = within(array).getByText('clean, degraded, recovering');
    expect(state).toHaveClass('text-amber-600');
    expect(
      within(array).getByRole('img', { name: 'RAID state: clean, degraded, recovering' }),
    ).toHaveClass('bg-amber-500');
    expect(within(array).getByTestId('raid-card-rebuild')).toHaveTextContent(
      'Rebuild: 45% · 98000K/sec',
    );
    const [healthy, rebuilding] = within(array).getAllByTestId('raid-card-device');
    expect(healthy).toHaveTextContent('/dev/sda1');
    expect(healthy.className).toContain('emerald');
    expect(rebuilding).toHaveTextContent('/dev/sdb1');
    expect(rebuilding.className).toContain('amber');
  });

  it("keeps a silent agent's array state as last known evidence without live colour", () => {
    render(() => (
      <RaidCard arrays={[degradedArray()]} lastKnownReason="host agent stopped reporting" />
    ));

    const array = screen.getByTestId('raid-card-array');
    expect(array).toHaveAttribute('data-raid-reading', 'last-known');
    expect(array.innerHTML).not.toMatch(/emerald|amber|text-red|bg-red/);

    const state = within(array).getByTestId('raid-card-state');
    expect(state).toHaveAttribute('title', LAST_KNOWN_TITLE);
    expect(state).toHaveTextContent('clean, degraded, recovering (last known)');
    expect(
      within(state).getByRole('img', {
        name: 'Last known RAID state: clean, degraded, recovering',
      }),
    ).toHaveClass('bg-slate-400');

    // A retained rebuild figure is not progress: past tense, no speed.
    const rebuild = within(array).getByTestId('raid-card-rebuild');
    expect(rebuild).toHaveTextContent('Rebuild was at 45%');
    expect(rebuild).toHaveAttribute('title', LAST_KNOWN_TITLE);
    expect(array).not.toHaveTextContent('98000K/sec');

    const [healthy, rebuilding] = within(array).getAllByTestId('raid-card-device');
    expect(healthy).toHaveTextContent(/^\/dev\/sda1$/);
    expect(rebuilding).toHaveTextContent('/dev/sdb1 · spare rebuilding');
    expect(rebuilding).toHaveAttribute('title', 'slot 1 • spare rebuilding');
  });

  it('treats a blank reason as current', () => {
    render(() => <RaidCard arrays={[degradedArray()]} lastKnownReason="  " />);

    expect(screen.getByTestId('raid-card-array')).toHaveAttribute('data-raid-reading', 'current');
  });

  it('follows the reading between current and last known', () => {
    const [reason, setReason] = createSignal<string | undefined>();
    render(() => <RaidCard arrays={[degradedArray()]} lastKnownReason={reason()} />);

    const array = screen.getByTestId('raid-card-array');
    expect(array).toHaveAttribute('data-raid-reading', 'current');

    setReason('host agent stopped reporting');
    expect(array).toHaveAttribute('data-raid-reading', 'last-known');
    expect(within(array).getByTestId('raid-card-rebuild')).toHaveTextContent('Rebuild was at 45%');
    expect(array.innerHTML).not.toMatch(/emerald|amber|text-red|bg-red/);

    setReason(undefined);
    expect(array).toHaveAttribute('data-raid-reading', 'current');
    expect(within(array).getByTestId('raid-card-rebuild')).toHaveTextContent('98000K/sec');
  });

  it('appears when the first RAID report arrives after mount', () => {
    const [arrays, setArrays] = createSignal<HostRAIDArray[] | undefined>();
    render(() => <RaidCard arrays={arrays()} />);
    expect(screen.queryByTestId('raid-card-array')).not.toBeInTheDocument();

    setArrays([degradedArray()]);
    expect(screen.getByTestId('raid-card-array')).toHaveTextContent('md0');

    setArrays([]);
    expect(screen.queryByTestId('raid-card-array')).not.toBeInTheDocument();
  });

  it('renders nothing without arrays', () => {
    const { container } = render(() => (
      <RaidCard arrays={[]} lastKnownReason="host agent stopped reporting" />
    ));

    expect(container).toBeEmptyDOMElement();
  });
});
