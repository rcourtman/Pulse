import { describe, expect, it } from 'vitest';
import { pacedWorkflowStatusForDisplay, WORKFLOW_STATUS_PACE_MS } from '../workflowStatusPacing';
import type { StreamDisplayEvent, WorkflowStatus } from '../types';

const burstStatuses = (): WorkflowStatus[] => [
  {
    phase: 'request_start',
    message: 'Preparing Pulse context.',
    startedAt: 1_000,
  },
  {
    phase: 'context',
    message: 'Reading current Pulse inventory with pulse_query.',
    tool: 'pulse_query',
    startedAt: 1_100,
  },
  {
    phase: 'provider_start',
    message: 'OpenRouter is starting the response.',
    startedAt: 1_200,
  },
];

describe('workflowStatusPacing', () => {
  it('paces burst workflow statuses instead of jumping straight to the latest status', () => {
    const history = burstStatuses();
    const events: StreamDisplayEvent[] = [
      {
        type: 'workflow_status',
        workflowStatus: history[2],
        startedAt: 1_200,
        updatedAt: 1_200,
      },
    ];

    expect(
      pacedWorkflowStatusForDisplay(history, history[2], events, 1_200),
    ).toBe(history[0]);
    expect(
      pacedWorkflowStatusForDisplay(
        history,
        history[2],
        events,
        1_000 + WORKFLOW_STATUS_PACE_MS,
      ),
    ).toBe(history[1]);
    expect(
      pacedWorkflowStatusForDisplay(
        history,
        history[2],
        events,
        1_000 + WORKFLOW_STATUS_PACE_MS * 2,
      ),
    ).toBe(history[2]);
  });

  it('keeps workflow rows tied to their own status after durable transcript evidence', () => {
    const history = burstStatuses();
    const events: StreamDisplayEvent[] = [
      {
        type: 'workflow_status',
        workflowStatus: history[0],
        startedAt: 1_000,
        updatedAt: 1_000,
      },
      {
        type: 'tool',
        tool: {
          name: 'pulse_query',
          input: '{}',
          output: '3 devices found',
          success: true,
        },
        startedAt: 1_150,
        updatedAt: 1_300,
      },
      {
        type: 'workflow_status',
        workflowStatus: history[2],
        startedAt: 1_200,
        updatedAt: 1_200,
      },
    ];

    expect(
      pacedWorkflowStatusForDisplay(history, history[2], events, 1_200),
    ).toBe(history[2]);
  });

  it('does not pace status history that was not delivered as a burst', () => {
    const history: WorkflowStatus[] = [
      {
        phase: 'request_start',
        message: 'Preparing Pulse context.',
        startedAt: 1_000,
      },
      {
        phase: 'provider_start',
        message: 'OpenRouter is starting the response.',
        startedAt: 2_500,
      },
    ];

    expect(
      pacedWorkflowStatusForDisplay(
        history,
        history[1],
        [{ type: 'workflow_status', workflowStatus: history[1] }],
        2_500,
      ),
    ).toBe(history[1]);
  });
});
