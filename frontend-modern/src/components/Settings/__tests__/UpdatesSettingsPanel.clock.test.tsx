import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UpdateInfo, VersionInfo } from '@/api/updates';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

const store = vi.hoisted(() => ({
  lastCheckedAt: vi.fn(),
  lastError: vi.fn<() => string | null>(() => null),
}));
vi.mock('@/stores/updates', () => ({ updateStore: store }));
vi.mock('../UpdateHistorySection', () => ({ UpdateHistorySection: () => null }));
import { UpdatesSettingsPanel } from '../UpdatesSettingsPanel';

const start = Date.parse('2026-10-11T04:00:00Z');
const mount = (sourceBuild = false) => {
  const [checkedAt, setCheckedAt] = createSignal<number | null>(start - 30_000);
  store.lastCheckedAt.mockImplementation(checkedAt);
  const [channel, setChannel] = createSignal<'stable' | 'rc'>('stable');
  const [auto, setAuto] = createSignal(false);
  const [info, setInfo] = createSignal<UpdateInfo>({
    available: false,
    availability: 'up_to_date',
    latestVersion: '6.5.0',
  } as UpdateInfo);
  const [checking, setChecking] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  store.lastError.mockImplementation(error);
  const check = vi.fn(async () => {});
  const install = vi.fn();
  const dirty = vi.fn();
  render(() => (
    <UpdatesSettingsPanel
      versionInfo={() => ({ version: '6.5.0', isSourceBuild: sourceBuild }) as VersionInfo}
      updateInfo={info}
      checkingForUpdates={checking}
      updateChannel={channel}
      setUpdateChannel={setChannel}
      autoUpdateEnabled={auto}
      setAutoUpdateEnabled={setAuto}
      checkForUpdates={check}
      setHasUnsavedChanges={dirty}
      updatePlan={() => null}
      onInstallUpdate={install}
      isInstalling={() => false}
    />
  ));
  return { setCheckedAt, setChannel, check, install, dirty, setInfo, setChecking, setError };
};

describe('held global update check age', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(start);
    store.lastCheckedAt.mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });
  it('ages the mounted last check without changing the verdict or requesting a check or installation', () => {
    const controls = mount();
    expect(screen.getByText('Checked 30s ago')).toBeInTheDocument();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(screen.getByText('Checked 1 min ago')).toBeInTheDocument();
    vi.advanceTimersByTime(60_000);
    expect(screen.getByText('Checked 2 mins ago')).toBeInTheDocument();
    expect(screen.getByText('Up to date')).toBeInTheDocument();
    expect(controls.check).not.toHaveBeenCalled();
    expect(controls.install).not.toHaveBeenCalled();
    expect(controls.dirty).not.toHaveBeenCalled();
  });
  it('retains missing, future and withdrawn check semantics', () => {
    const { setCheckedAt } = mount();
    setCheckedAt(null);
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(screen.getByText('Not checked yet')).toBeInTheDocument();
    setCheckedAt(Date.now() + 60_000);
    expect(screen.getByText('Checked 0s ago')).toBeInTheDocument();
    setCheckedAt(null);
    vi.advanceTimersByTime(60_000);
    expect(screen.getByText('Not checked yet')).toBeInTheDocument();
  });
  it('keeps source-build checks and preview automatic updates unavailable as the clock advances', () => {
    const controls = mount(true);
    controls.setChannel('rc');
    vi.advanceTimersByTime(60_000);
    expect(screen.getByText('Source build')).toBeInTheDocument();
    expect(screen.queryByText(/Checked .*ago/)).not.toBeInTheDocument();
    expect(screen.getByText('Release updates disabled')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Check Now' })).toBeDisabled();
    expect(controls.check).not.toHaveBeenCalled();
    expect(controls.install).not.toHaveBeenCalled();
  });

  it('shows publication preparation and changes to ready only after an installable offer', () => {
    const controls = mount();
    controls.setInfo({
      available: false,
      availability: 'preparing',
      preparingVersion: '6.8.0-rc.2',
      latestVersion: '6.8.0-rc.1',
    } as UpdateInfo);
    expect(screen.getByText('Update is still being prepared')).toBeInTheDocument();
    expect(screen.getByText(/Version 6.8.0-rc.2 has been released/)).toBeInTheDocument();
    expect(screen.queryByText('Up to date')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /install/i })).not.toBeInTheDocument();
    controls.setChecking(true);
    expect(screen.getByRole('button', { name: 'Checking...' })).toBeDisabled();
    controls.setChecking(false);
    controls.setInfo({
      available: true,
      availability: 'available',
      latestVersion: '6.8.0-rc.2',
    } as UpdateInfo);
    expect(screen.getByText('Update Ready')).toBeInTheDocument();
    expect(screen.queryByText('Update is still being prepared')).not.toBeInTheDocument();
    expect(screen.queryByText(/Its update for this installation/)).not.toBeInTheDocument();
  });

  it('withdraws up-to-date status after a failed check and for old negative data', () => {
    const controls = mount();
    controls.setError('Refresh failed');
    expect(screen.queryByText('Up to date')).not.toBeInTheDocument();
    expect(screen.getByText('Could not confirm update status')).toBeInTheDocument();
    controls.setError(null);
    controls.setInfo({ available: false, latestVersion: '6.5.0' } as UpdateInfo);
    expect(screen.queryByText('Up to date')).not.toBeInTheDocument();
    expect(screen.getByText('Could not confirm update status')).toBeInTheDocument();
  });
});
