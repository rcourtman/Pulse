import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { IncidentTimelineEventCard } from '../IncidentTimelineEventCard';
import type { IncidentEvent } from '@/types/api';

function makeEvent(overrides: Partial<IncidentEvent> = {}): IncidentEvent {
  return {
    id: 'event-1',
    type: 'ai_analysis',
    timestamp: '2026-03-18T12:00:00Z',
    summary: 'Alert investigated',
    details: {
      note: 'updated thresholds',
      command: 'systemctl restart alert',
      output_excerpt: 'stdout: ok',
    },
    ...overrides,
  };
}

describe('IncidentTimelineEventCard', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders the shared event card content for the alt variant', () => {
    const { container } = render(() => (
      <IncidentTimelineEventCard event={makeEvent()} variant="alt" />
    ));

    expect(container.firstElementChild).toHaveClass('bg-surface-alt');
    expect(screen.getByText('Alert investigated')).toBeInTheDocument();
    expect(screen.getByText('updated thresholds')).toBeInTheDocument();
    expect(screen.getByText('systemctl restart alert')).toBeInTheDocument();
    expect(screen.getByText('stdout: ok')).toBeInTheDocument();
  });

  it('renders the surface variant without optional detail lines', () => {
    const { container } = render(() => (
      <IncidentTimelineEventCard
        event={makeEvent({ details: undefined, summary: 'Incident event' })}
        variant="surface"
      />
    ));

    expect(container.firstElementChild).toHaveClass('bg-surface');
    expect(screen.getByText('Incident event')).toBeInTheDocument();
    expect(screen.queryByText('updated thresholds')).not.toBeInTheDocument();
    expect(screen.queryByText('systemctl restart alert')).not.toBeInTheDocument();
    expect(screen.queryByText('stdout: ok')).not.toBeInTheDocument();
  });
  it('preserves observation and occurrence provenance behind a disclosure', () => {
    const { container } = render(() => (
      <IncidentTimelineEventCard
        variant="surface"
        event={makeEvent({
          evidence: {
            id: 'canonical-record',
            resourceId: 'resource-a',
            kind: 'alert_fired',
            observedAt: '2026-03-18T12:10:00Z',
            occurredAt: '2026-03-18T12:00:00Z',
            sourceType: 'platform_event',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'high',
            actor: 'operator',
          },
        })}
      />
    ));
    const details = container.querySelector('details');
    expect(details).not.toHaveAttribute('open');
    expect(screen.getByText('Evidence details')).toBeInTheDocument();
    expect(screen.getByText('Observed')).toBeInTheDocument();
    expect(screen.getByText('Occurred')).toBeInTheDocument();
    expect(screen.getByText('canonical-record')).toBeInTheDocument();
    expect(screen.getByText('operator')).toBeInTheDocument();
  });

  it('does not fabricate a date for missing evidence time', () => {
    render(() => (
      <IncidentTimelineEventCard
        variant="surface"
        event={makeEvent({ timestamp: '0001-01-01T00:00:00Z' })}
      />
    ));
    expect(screen.getByText('Time unavailable')).toBeInTheDocument();
    expect(screen.queryByText(/1\/1\/1/)).not.toBeInTheDocument();
  });
});
