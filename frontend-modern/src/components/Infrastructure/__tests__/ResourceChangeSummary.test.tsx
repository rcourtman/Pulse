import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { createSignal } from 'solid-js';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

import { ResourceChangeSummary } from '../ResourceChangeSummary';

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
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('held change-summary ages', () => {
  it('ages an unchanged record without rebuilding its link or changing the evidence', () => {
    const start = Date.parse('2026-10-11T11:00:00Z');
    vi.useFakeTimers({ now: start });
    const change = {
      id: 'held-change',
      resourceId: 'guest-a',
      kind: 'config_update' as const,
      observedAt: new Date(start - 60_000).toISOString(),
      reason: 'Changed retention',
      sourceType: 'platform_event' as const,
      confidence: 'high' as const,
    };
    const [changes, setChanges] = createSignal([change]);
    const view = render(() => (
      <ResourceChangeSummary
        title="Recent changes"
        changes={changes()}
        buildResourceHref={(id) => `/infrastructure/${id}`}
      />
    ));
    const link = screen.getByRole('link', { name: 'Open resource guest-a in Infrastructure' });
    link.focus();
    expect(view.container).toHaveTextContent('1m ago');
    vi.advanceTimersByTime(2 * 3_600_000);
    expect(view.container).toHaveTextContent('2h ago');
    expect(link).toHaveFocus();
    expect(screen.getByRole('link', { name: link.getAttribute('aria-label')! })).toBe(link);
    expect(changes()).toEqual([change]);
    setChanges([{ ...change, observedAt: new Date(Date.now()).toISOString() }]);
    expect(view.container).toHaveTextContent('just now');
    vi.advanceTimersByTime(2 * RELATIVE_TIME_TICK_MS);
    expect(view.container).toHaveTextContent('1m ago');
    setChanges([]);
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(
      screen.getByText('No canonical changes were recorded in the last 24 hours.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Changed retention')).not.toBeInTheDocument();
  });
});
