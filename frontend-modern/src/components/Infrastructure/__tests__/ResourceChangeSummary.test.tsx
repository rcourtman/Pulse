import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { ResourceChangeSummary } from '../ResourceChangeSummary';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('ResourceChangeSummary', () => {
  it('sorts recent changes canonically and truncates the feed', () => {
    render(() => (
      <ResourceChangeSummary
        class="mt-2"
        title="Recent changes"
        subtitle="Canonical 24h timeline"
        resolveResourceLabel={(resourceId) =>
          resourceId === 'vm-200' ? 'VM 200' : resourceId === 'vm-300' ? 'VM 300' : resourceId
        }
        changes={[
          {
            id: 'older-change',
            resourceId: 'storage-1',
            kind: 'config_update',
            reason: 'Updated retention policy',
            observedAt: '2026-03-18T10:00:00Z',
            sourceType: 'pulse_diff',
            confidence: 'medium',
            actor: 'agent:ops-helper',
            relatedResources: ['vm-100'],
          },
          {
            id: 'newer-change',
            resourceId: 'storage-2',
            kind: 'restart',
            from: 'running',
            to: 'restarting',
            reason: 'Restart after maintenance',
            observedAt: '2026-03-18T12:00:00Z',
            sourceType: 'platform_event',
            sourceAdapter: 'docker_adapter',
            confidence: 'high',
            actor: 'agent:ops-helper',
            relatedResources: ['vm-200', 'vm-300'],
          },
        ]}
        maxChanges={1}
      />
    ));

    expect(screen.getByText('Recent changes')).toBeInTheDocument();
    expect(screen.getByText('Canonical 24h timeline')).toBeInTheDocument();
    expect(screen.getByText('Restart: running → restarting')).toBeInTheDocument();
    expect(screen.getByText('Platform event')).toBeInTheDocument();
    expect(screen.getByText('Docker adapter')).toBeInTheDocument();
    // Cross-jump links to /infrastructure?resource=... were retired with the
    // legacy Infrastructure surface; the change feed now renders resource
    // labels as plain text by default.
    expect(screen.queryAllByRole('link')).toHaveLength(0);
    expect(screen.getByText('storage-2')).toBeInTheDocument();
    expect(screen.getByText('VM 200')).toBeInTheDocument();
    expect(screen.getByText('By agent:ops-helper')).toBeInTheDocument();
    expect(screen.getByText('Restart after maintenance')).toBeInTheDocument();
    expect(screen.queryByText('storage-1')).toBeNull();
  });

  it('renders the empty state when no recent changes are available', () => {
    render(() => (
      <ResourceChangeSummary
        title="Latest canonical change"
        emptyText="No canonical changes were recorded."
        changes={[]}
      />
    ));

    expect(screen.getByText('Latest canonical change')).toBeInTheDocument();
    expect(screen.getByText('No canonical changes were recorded.')).toBeInTheDocument();
  });

  it('can suppress metadata badges for compact operator context', () => {
    render(() => (
      <ResourceChangeSummary
        title="Nearby activity"
        changes={[
          {
            id: 'change-1',
            resourceId: 'storage-1',
            kind: 'alert_resolved',
            reason: 'Alert resolved: ZFS pool recovered',
            observedAt: '2026-03-18T12:00:00Z',
            sourceType: 'platform_event',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'medium',
          },
        ]}
        showMetadataBadges={false}
      />
    ));

    expect(screen.getByText('Nearby activity')).toBeInTheDocument();
    expect(screen.getByText('Alert resolved: ZFS pool recovered')).toBeInTheDocument();
    expect(screen.getByText('storage-1')).toBeInTheDocument();
    expect(screen.queryByText('Platform event')).toBeNull();
    expect(screen.queryByText('Proxmox adapter')).toBeNull();
    expect(screen.queryByText('Heuristic')).toBeNull();
  });

  it('keeps a change age moving while the open summary data does not change', () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-03-18T12:05:00Z'),
    });

    render(() => (
      <ResourceChangeSummary
        title="Latest canonical change"
        changes={[
          {
            id: 'restart',
            resourceId: 'storage-2',
            kind: 'restart',
            observedAt: '2026-03-18T12:00:00Z',
            sourceType: 'platform_event',
            confidence: 'high',
          },
        ]}
        compact
      />
    ));
    const meta = () => screen.getByText('storage-2').parentElement!;

    expect(meta()).toHaveTextContent('storage-2·5m ago');

    // No new change arrives: the summary keeps the same change and only the
    // clock moves.
    vi.advanceTimersByTime(3 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(meta()).toHaveTextContent('storage-2·3h ago');
  });
});
