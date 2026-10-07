import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { UpdatesSettingsPanel } from '../UpdatesSettingsPanel';

vi.mock('@/stores/updates', () => ({
  updateStore: {
    lastCheckedAt: () => Date.parse('2026-08-10T09:00:00Z'),
    lastError: () => null,
    versionInfo: () => null,
    rollbackUpdate: vi.fn(),
  },
}));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('UpdatesSettingsPanel relative ages', () => {
  it('keeps the age of the check behind Up to date moving while the panel stays open', () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-08-10T12:00:00Z'),
    });

    render(() => (
      <UpdatesSettingsPanel
        versionInfo={() => ({
          version: '6.4.0',
          build: 'community',
          runtime: 'server',
          isDocker: false,
          isSourceBuild: false,
          isDevelopment: false,
        })}
        updateInfo={() => null}
        checkingForUpdates={() => false}
        updateChannel={() => 'stable'}
        setUpdateChannel={vi.fn()}
        autoUpdateEnabled={() => false}
        setAutoUpdateEnabled={vi.fn()}
        checkForUpdates={vi.fn().mockResolvedValue(undefined)}
        setHasUnsavedChanges={vi.fn()}
        updatePlan={() => null}
        onInstallUpdate={vi.fn()}
        isInstalling={() => false}
      />
    ));

    expect(screen.getByText('Checked 3 hours ago')).toBeInTheDocument();

    // No new check runs: the stored check time never changes and only the
    // clock moves.
    vi.advanceTimersByTime(2 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(screen.getByText('Checked 5 hours ago')).toBeInTheDocument();
  });
});
