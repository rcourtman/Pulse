import { createSignal } from 'solid-js';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { MonitoringAPI } from '@/api/monitoring';
import { DockerHostDrawerManagement } from '../DockerHostDrawerOverview';

vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => true,
}));
vi.mock('@/api/monitoring', () => ({ MonitoringAPI: { checkDockerUpdates: vi.fn() } }));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('mounted Docker update evidence', () => {
  it('ages the retained check without dispatch and keeps policy and command admission', () => {
    const start = Date.parse('2026-10-10T12:00:00Z');
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
    const original = {
      id: 'docker-lab',
      type: 'docker-host',
      docker: {
        hostSourceId: 'docker-lab',
        updatesAvailableCount: 3,
        updatesLastCheckedAt: new Date(start - 60_000).toISOString(),
        command: { type: 'update_check', status: 'queued' },
      },
    } as Resource;
    const [host, setHost] = createSignal(original);
    render(() => <DockerHostDrawerManagement host={host()} />);
    const check = screen.getByRole('button', { name: 'Check updates' });
    expect(check).toBeDisabled();
    expect(screen.getByText('1 min ago')).toBeInTheDocument();
    vi.advanceTimersByTime(60_000);
    expect(screen.getByText('2 mins ago')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Check updates' })).toBe(check);
    expect(check).toBeDisabled();
    expect(screen.queryByRole('button', { name: /Update all/ })).not.toBeInTheDocument();
    expect(MonitoringAPI.checkDockerUpdates).not.toHaveBeenCalled();
    expect(host()).toBe(original);
    setHost({
      ...original,
      docker: { ...original.docker!, collectionMode: 'typed-helper-summary' },
    });
    expect(screen.queryByText('Last checked')).not.toBeInTheDocument();
    vi.advanceTimersByTime(30_000);
    expect(screen.queryByRole('button', { name: 'Check updates' })).not.toBeInTheDocument();
  });
});
