import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { ResourceDetailDrawerDebugTab } from '../ResourceDetailDrawerDebugTab';
import type { UseResourceDetailDrawerStateResult } from '../useResourceDetailDrawerState';

// The debug tab reads only copy state, identity matching and the per-source
// sections from the drawer state.
const drawer = {
  handleCopyJson: vi.fn(),
  copied: () => false,
  identityMatchInfo: () => null,
  sourceSections: () => [{ id: 'proxmox', label: 'Proxmox', payload: { node: 'pve1' } }],
  sourceStatus: () => ({
    proxmox: { status: 'stale', lastSeen: '2026-08-30T11:58:00Z' },
  }),
} as unknown as UseResourceDetailDrawerStateResult;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('ResourceDetailDrawerDebugTab relative ages', () => {
  it("keeps a quiet source's last-seen age moving while the tab stays open", () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-08-30T12:00:00Z'),
    });

    render(() => (
      <ResourceDetailDrawerDebugTab resource={{ id: 'node-1' } as Resource} drawer={drawer} />
    ));

    expect(screen.getByText('stale • 2 mins ago')).toBeInTheDocument();

    // The source stays quiet: its status never changes and only the clock moves.
    vi.advanceTimersByTime(60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(screen.getByText('stale • 1 hour ago')).toBeInTheDocument();
  });
});
