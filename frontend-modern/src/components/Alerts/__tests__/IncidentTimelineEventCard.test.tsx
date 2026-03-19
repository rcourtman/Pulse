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
});
