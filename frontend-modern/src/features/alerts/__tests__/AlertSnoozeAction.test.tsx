import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Alert } from '@/types/api';
import type { AlertOverviewState } from '../useAlertOverviewState';
import type { AlertIncidentTimelineState } from '../useAlertIncidentTimelineState';

import { AlertSnoozeAction } from '../AlertSnoozeAction';

function makeAlert(): Alert {
  return {
    id: 'cpu:vm/100',
    type: 'cpu',
    level: 'critical',
    resourceId: 'vm/100',
    resourceName: 'vm-100',
    message: 'CPU is critical',
    startTime: '2026-08-27T11:00:00Z',
    acknowledged: false,
  } as Alert;
}

describe('AlertSnoozeAction', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('uses accessible targets and prevents duplicate preset submission', async () => {
    let finishSnooze: (() => void) | undefined;
    const handleSnooze = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finishSnooze = resolve;
        }),
    );
    const loadIncidentTimeline = vi.fn().mockResolvedValue(undefined);
    const state = {
      snoozeProcessingAlerts: () => new Set<string>(),
      processingAlerts: () => new Set<string>(),
      handleSnooze,
      handleUnsnooze: vi.fn(),
    } as unknown as AlertOverviewState;
    const timelineState = {
      expandedIncidents: () => new Set(['cpu:vm/100']),
      loadIncidentTimeline,
    } as unknown as AlertIncidentTimelineState;

    render(() => (
      <AlertSnoozeAction alert={makeAlert()} state={state} timelineState={timelineState} />
    ));

    const trigger = screen.getByRole('button', { name: 'Snooze' });
    expect(trigger).toHaveAttribute('type', 'button');
    expect(trigger).toHaveClass('min-h-11', 'focus:ring-2');
    fireEvent.click(trigger);

    const dialog = screen.getByRole('dialog', { name: 'Snooze this alert' });
    const oneHour = within(dialog).getByRole('button', { name: 'For 1 hour' });
    expect(oneHour).toHaveClass('min-h-11', 'focus:ring-2');
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveClass(
      'min-h-11',
      'focus:ring-2',
    );

    fireEvent.click(oneHour);
    expect(handleSnooze).toHaveBeenCalledOnce();
    expect(oneHour).toBeDisabled();
    expect(oneHour.parentElement).toHaveAttribute('aria-busy', 'true');
    fireEvent.click(oneHour);
    expect(handleSnooze).toHaveBeenCalledOnce();

    finishSnooze?.();
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(loadIncidentTimeline).toHaveBeenCalledWith(
      'cpu:vm/100',
      'cpu:vm/100',
      '2026-08-27T11:00:00Z',
    );
  });
});
