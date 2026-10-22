import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { VersionInfo } from '@/api/updates';

const controls = vi.hoisted(() => ({ runtime: vi.fn(), check: vi.fn() }));
vi.mock('@/stores/license', () => ({ runtimeCapabilities: () => controls.runtime() }));
vi.mock('@/stores/updates', () => ({
  updateStore: { lastCheckedAt: () => null, lastError: () => null },
}));
vi.mock('@/api/updates', () => ({ UpdatesAPI: { checkProRuntimeSwitch: () => controls.check() } }));
vi.mock('../UpdateHistorySection', () => ({ UpdateHistorySection: () => null }));
import { UpdatesSettingsPanel } from '../UpdatesSettingsPanel';

function mount(initial: VersionInfo | null) {
  const [version, setVersion] = createSignal(initial);
  const [channel, setChannel] = createSignal<'stable' | 'rc'>('stable');
  const [auto, setAuto] = createSignal(false);
  render(() => (
    <UpdatesSettingsPanel
      versionInfo={version}
      updateInfo={() => null}
      checkingForUpdates={() => false}
      updateChannel={channel}
      setUpdateChannel={setChannel}
      autoUpdateEnabled={auto}
      setAutoUpdateEnabled={setAuto}
      checkForUpdates={async () => undefined}
      setHasUnsavedChanges={() => false}
      updatePlan={() => null}
      onInstallUpdate={() => undefined}
      isInstalling={() => false}
    />
  ));
  return setVersion;
}

const release = { version: '6.5.0', isSourceBuild: false } as VersionInfo;
describe('Updates panel runtime switch admission', () => {
  beforeEach(() => {
    controls.runtime.mockReturnValue({ runtime: { build: 'community' } });
    controls.check.mockReset();
    controls.check.mockResolvedValue({
      available: true,
      installInPlace: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
    });
  });
  afterEach(cleanup);

  it('waits for a known release build and withdraws the offer for source builds', async () => {
    const setVersion = mount(null);
    await Promise.resolve();
    expect(controls.check).not.toHaveBeenCalled();
    setVersion(release);
    await screen.findByRole('button', { name: /Switch to Pulse Pro/ });
    expect(controls.check).toHaveBeenCalledTimes(1);
    setVersion({ ...release, isSourceBuild: true });
    await waitFor(() => expect(screen.queryByTestId('pro-runtime-switch')).not.toBeInTheDocument());
  });

  it.each(['pro', 'unknown'])('does not ask for a switch on a %s runtime', async (build) => {
    controls.runtime.mockReturnValue({ runtime: { build } });
    mount(release);
    await Promise.resolve();
    expect(controls.check).not.toHaveBeenCalled();
    expect(screen.queryByTestId('pro-runtime-switch')).not.toBeInTheDocument();
  });
});
