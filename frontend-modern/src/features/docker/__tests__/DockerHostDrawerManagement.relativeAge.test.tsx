import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { DockerHostDrawerManagement } from '../DockerHostDrawerOverview';

const host = (): Resource =>
  ({
    id: 'agent:docker-1',
    name: 'docker-1',
    displayName: 'Docker 1',
    type: 'agent',
    platformId: 'docker-1',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.parse('2026-08-30T12:00:00Z'),
    docker: {
      hostSourceId: 'docker-source-1',
      updatesAvailableCount: 2,
      updatesLastCheckedAt: '2026-08-30T11:58:00Z',
    },
  }) as Resource;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('DockerHostDrawerManagement relative ages', () => {
  it('keeps the container updates Last checked age moving while the drawer stays open', () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-08-30T12:00:00Z'),
    });

    render(() => <DockerHostDrawerManagement host={host()} />);
    const card = screen.getByTestId('docker-host-management-actions');

    expect(card).toHaveTextContent('Last checked2 mins ago');

    // No new update check arrives: the host never changes and only the clock
    // moves.
    vi.advanceTimersByTime(2 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(card).toHaveTextContent('Last checked2 hours ago');
  });
});
