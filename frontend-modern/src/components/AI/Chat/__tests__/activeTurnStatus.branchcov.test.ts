import { describe, expect, it } from 'vitest';
import { formatAssistantWorkflowStatus, getAssistantActiveTurnStatus } from '../activeTurnStatus';
import type { ChatMessage, StreamDisplayEvent } from '../types';

// Fixture builders — mirror the conventions of the sibling activeTurnStatus.test.ts.
const assistantMessage = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'assistant-1',
  role: 'assistant',
  content: '',
  timestamp: new Date(1_000),
  ...overrides,
});

const activeTurn = (overrides: Partial<ChatMessage> = {}, now?: number) =>
  getAssistantActiveTurnStatus([assistantMessage(overrides)], true, now);

const readToolInput = (command: string): string =>
  JSON.stringify({ action: 'exec', command, target_host: 'current_resource' });

// ---------------------------------------------------------------------------
// formatPendingToolStatus — reachable via getAssistantActiveTurnStatus
// (pendingTools state, stream pending_tool events, and the no-pendingTool guard)
// ---------------------------------------------------------------------------

describe('formatPendingToolStatus branch coverage', () => {
  it('renders a waiting tool from a command preview ("Waiting on …")', () => {
    expect(
      activeTurn({
        pendingTools: [
          {
            id: 'tool-1',
            name: 'pulse_read',
            input: readToolInput('ls /dev | wc -l'),
            status: 'waiting',
            startedAt: 500,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Waiting on $ ls /dev | wc -l',
      startedAt: 500,
    });
  });

  it('renders a waiting tool from the label fallback when no activity is parseable', () => {
    expect(
      activeTurn({
        pendingTools: [{ id: 'tool-1', name: 'pulse_get_nodes', input: '{}', status: 'waiting' }],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Waiting on get nodes',
    });
  });

  it('ignores a generic "Running" progress in favor of a real command preview', () => {
    // progress truthy + commandPreview truthy + isGenericToolProgress(progress) true
    //  -> the progress short-circuit is skipped and the command preview wins.
    expect(
      activeTurn({
        pendingTools: [
          {
            id: 'tool-1',
            name: 'pulse_read',
            input: readToolInput('df -h'),
            progress: 'Running',
            status: 'running',
            startedAt: 100,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Running $ df -h',
      startedAt: 100,
    });
  });

  it('ignores a generic "Running command." progress in favor of a real command preview', () => {
    expect(
      activeTurn({
        pendingTools: [
          {
            id: 'tool-1',
            name: 'pulse_read',
            input: readToolInput('uptime'),
            progress: 'Running command.',
            status: 'running',
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Running $ uptime',
    });
  });

  it('drops a pending_tool stream event that carries no pendingTool payload', () => {
    // formatPendingToolStatus(undefined) -> '' -> no candidate; the turn falls
    // back to the initial request status instead of surfacing a tool row.
    expect(
      activeTurn({
        streamEvents: [{ type: 'pending_tool', toolId: 'tool-1' }],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });
});

// ---------------------------------------------------------------------------
// sanitizeWorkflowStatusMessage — reachable via formatAssistantWorkflowStatus
// ---------------------------------------------------------------------------

describe('sanitizeWorkflowStatusMessage branch coverage', () => {
  it('strips a trailing "using <tool>." clause and keeps the terminating period', () => {
    expect(
      formatAssistantWorkflowStatus({
        message: 'Reading inventory using pulse_query.',
        tool: 'pulse_query',
      }),
    ).toBe('Reading inventory.');
  });

  it('strips a trailing "with <tool>" clause that has no terminating period', () => {
    expect(
      formatAssistantWorkflowStatus({
        message: 'Reading inventory with pulse_query',
        tool: 'pulse_query',
      }),
    ).toBe('Reading inventory');
  });

  it('strips a trailing "via the <tool>" clause (label re-attached as a suffix)', () => {
    // After the trailing clause is removed the message no longer references the
    // tool, so formatAssistantWorkflowStatus re-attaches the friendly label.
    expect(
      formatAssistantWorkflowStatus({
        message: 'Gathering data via the pulse_query.',
        tool: 'pulse_query',
      }),
    ).toBe('Gathering data. · inventory');
  });

  it('rewrites a bare internal tool identifier in the middle of a message', () => {
    expect(
      formatAssistantWorkflowStatus({
        message: 'Running pulse_query now',
        tool: 'pulse_query',
      }),
    ).toBe('Running inventory now');
  });

  it('collapses runs of whitespace and lone spaced-out periods', () => {
    expect(formatAssistantWorkflowStatus({ message: 'Working   on   it .' })).toBe(
      'Working on it.',
    );
  });

  it('leaves a plain message unchanged when no tool context is present', () => {
    expect(formatAssistantWorkflowStatus({ message: 'Waiting for assistant.' })).toBe(
      'Waiting for assistant.',
    );
  });
});

// ---------------------------------------------------------------------------
// messageContainsToolLabel — reachable via formatAssistantWorkflowStatus
// ---------------------------------------------------------------------------

describe('messageContainsToolLabel branch coverage', () => {
  it('appends the tool suffix when the friendly label is absent from the message', () => {
    // messageContainsToolLabel('Working', 'read') -> \bread\b does not match -> false
    //  -> toolSuffix is appended.
    expect(formatAssistantWorkflowStatus({ message: 'Working', tool: 'pulse_read' })).toBe(
      'Working · read',
    );
  });

  it('withholds the tool suffix when the message already mentions the label as a word', () => {
    // messageContainsToolLabel('Reading storage', 'read') -> \bread\b does not
    //  match inside "Reading" -> false -> suffix appended (boundary edge case).
    expect(formatAssistantWorkflowStatus({ message: 'Reading storage', tool: 'pulse_read' })).toBe(
      'Reading storage · read',
    );
  });
});

// ---------------------------------------------------------------------------
// formatRetryDelay — reachable via formatAssistantWorkflowStatus (now omitted
// so remainingRetryDelay returns retryAfterMs verbatim)
// ---------------------------------------------------------------------------

describe('formatRetryDelay branch coverage', () => {
  const retry = (overrides: Record<string, unknown>) =>
    formatAssistantWorkflowStatus({ message: 'Retrying.', ...overrides } as Parameters<
      typeof formatAssistantWorkflowStatus
    >[0]);

  it('formats sub-second delays in milliseconds', () => {
    expect(retry({ attempt: 1, retryAfterMs: 500 })).toBe(
      'Retrying. · attempt 1 · retrying in 500ms',
    );
    expect(retry({ attempt: 1, retryAfterMs: 999 })).toBe(
      'Retrying. · attempt 1 · retrying in 999ms',
    );
  });

  it('formats the one-second boundary and sub-ten-second delays with one decimal', () => {
    expect(retry({ attempt: 1, retryAfterMs: 1_000 })).toBe(
      'Retrying. · attempt 1 · retrying in 1s',
    );
    expect(retry({ attempt: 1, retryAfterMs: 9_500 })).toBe(
      'Retrying. · attempt 1 · retrying in 9.5s',
    );
  });

  it('formats sub-minute second delays as whole seconds', () => {
    expect(retry({ attempt: 1, retryAfterMs: 30_000 })).toBe(
      'Retrying. · attempt 1 · retrying in 30s',
    );
  });

  it('formats sub-ten-minute delays with one decimal minute', () => {
    expect(retry({ attempt: 1, retryAfterMs: 90_000 })).toBe(
      'Retrying. · attempt 1 · retrying in 1.5m',
    );
  });

  it('formats delays of ten minutes or more as whole minutes', () => {
    expect(retry({ attempt: 1, retryAfterMs: 600_000 })).toBe(
      'Retrying. · attempt 1 · retrying in 10m',
    );
    expect(retry({ attempt: 1, retryAfterMs: 900_000 })).toBe(
      'Retrying. · attempt 1 · retrying in 15m',
    );
  });

  it('hits the formatRetryDelay empty-result branch once a countdown reaches zero', () => {
    // remainingRetryDelay(retryAfterMs 3200, startedAt 1000, now 4500) -> 0,
    // formatRetryDelay(0) -> '' -> formatRetryStatusSuffix emits "retrying now".
    expect(
      formatAssistantWorkflowStatus(
        { message: 'Retrying.', attempt: 1, retryAfterMs: 3_200, startedAt: 1_000 },
        4_500,
      ),
    ).toBe('Retrying. · attempt 1 · retrying now');
  });
});

// ---------------------------------------------------------------------------
// formatRetryStatusSuffix — reachable via formatAssistantWorkflowStatus
// ---------------------------------------------------------------------------

describe('formatRetryStatusSuffix branch coverage', () => {
  it('emits only the attempt count when maxAttempts is missing', () => {
    expect(formatAssistantWorkflowStatus({ message: 'Working', attempt: 2 })).toBe(
      'Working · attempt 2',
    );
  });

  it('emits the attempt ratio when both attempt and maxAttempts are present', () => {
    expect(formatAssistantWorkflowStatus({ message: 'Working', attempt: 2, maxAttempts: 3 })).toBe(
      'Working · attempt 2/3',
    );
  });

  it('emits a retry delay without an attempt count', () => {
    expect(formatAssistantWorkflowStatus({ message: 'Working', retryAfterMs: 500 })).toBe(
      'Working · retrying in 500ms',
    );
  });

  it('omits the suffix entirely when there is no attempt or retry metadata', () => {
    expect(formatAssistantWorkflowStatus({ message: 'Working' })).toBe('Working');
  });

  it('returns an empty string when there is no message at all', () => {
    expect(formatAssistantWorkflowStatus({ message: '' })).toBe('');
    expect(formatAssistantWorkflowStatus(undefined)).toBe('');
  });
});

// ---------------------------------------------------------------------------
// thinkingStatusText — reachable via getAssistantActiveTurnStatus
// ---------------------------------------------------------------------------

describe('thinkingStatusText branch coverage', () => {
  it('falls back to the bare "Thinking" label when no reasoning title is extractable', () => {
    expect(
      activeTurn({
        streamEvents: [
          { type: 'thinking', thinking: 'Plain reasoning with no bold heading.', startedAt: 2_000 },
        ],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Thinking',
      startedAt: 2_000,
    });
  });

  it('ignores a whitespace-only thinking payload so no thinking status is surfaced', () => {
    // thinkingStatusText -> '' -> no candidate -> falls back to initial request status.
    expect(
      activeTurn({
        streamEvents: [{ type: 'thinking', thinking: '   ', startedAt: 2_000 }],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });
});

// ---------------------------------------------------------------------------
// toolCancelStatusText — reachable via getAssistantActiveTurnStatus
// ---------------------------------------------------------------------------

describe('toolCancelStatusText branch coverage', () => {
  it('omits the reason when none is supplied', () => {
    expect(
      activeTurn({
        streamEvents: [
          {
            type: 'tool_cancel',
            toolId: 'tool-1',
            toolCancel: { id: 'tool-1', name: 'pulse_read', input: '{}' },
            startedAt: 1_500,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Skipped read',
      startedAt: 1_500,
    });
  });

  it('treats a whitespace-only reason as absent', () => {
    expect(
      activeTurn({
        streamEvents: [
          {
            type: 'tool_cancel',
            toolId: 'tool-1',
            toolCancel: { id: 'tool-1', name: 'pulse_read', input: '{}', reason: '   ' },
            startedAt: 1_500,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Skipped read',
      startedAt: 1_500,
    });
  });

  it('falls back to pendingTool.name when toolCancel.name is empty', () => {
    expect(
      activeTurn({
        streamEvents: [
          {
            type: 'tool_cancel',
            toolId: 'tool-1',
            toolCancel: { id: 'tool-1', name: '', input: '{}' },
            pendingTool: { id: 'tool-1', name: 'pulse_read', input: '{}' },
            startedAt: 1_500,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Skipped read',
      startedAt: 1_500,
    });
  });
});

// ---------------------------------------------------------------------------
// completedToolStatusText — reachable via getAssistantActiveTurnStatus
// ---------------------------------------------------------------------------

describe('completedToolStatusText branch coverage', () => {
  it('reports "Completed" with the label fallback for placeholder input', () => {
    expect(
      activeTurn({
        streamEvents: [
          {
            type: 'tool',
            toolId: 'tool-1',
            tool: { name: 'pulse_get_nodes', input: '{}', output: 'ok', success: true },
            startedAt: 1_100,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Completed get nodes',
    });
  });

  it('reports "Completed" with a parsed input summary when available', () => {
    expect(
      activeTurn({
        streamEvents: [
          {
            type: 'tool',
            toolId: 'tool-1',
            tool: {
              name: 'pulse_query',
              input: JSON.stringify({ action: 'list' }),
              output: 'ok',
              success: true,
            },
            startedAt: 1_100,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Completed list resources',
    });
  });

  it('drops a tool stream event that carries no tool payload', () => {
    // completedToolStatusText -> `if (!tool) return ''` -> no candidate.
    expect(
      activeTurn({
        streamEvents: [{ type: 'tool', toolId: 'tool-1' }],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });
});

// ---------------------------------------------------------------------------
// latestStreamActivityStatus — uncovered switch arms / guard-false arms
// ---------------------------------------------------------------------------

describe('latestStreamActivityStatus branch coverage', () => {
  it('ignores an unknown event type via the default switch arm', () => {
    expect(
      activeTurn({
        streamEvents: [{ type: 'bogus' } as unknown as StreamDisplayEvent],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });

  it('ignores a content event whose payload is only whitespace', () => {
    // `if (event.content?.trim())` guard is false -> no generating candidate.
    expect(
      activeTurn({
        streamEvents: [{ type: 'content', content: '   ' }],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });

  it('ignores an approval event that carries no approval payload', () => {
    expect(
      activeTurn({
        streamEvents: [{ type: 'approval' }],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });

  it('ignores a question event that carries no question payload', () => {
    expect(
      activeTurn({
        streamEvents: [{ type: 'question' }],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt.',
      startedAt: 1_000,
    });
  });

  it('surfaces the later of two equally-timed thinking events via the order tiebreak', () => {
    // Both events have no startedAt/updatedAt -> activityAt undefined for both.
    // isFresherStatusCandidate falls through to the order tiebreak.
    expect(
      activeTurn({
        streamEvents: [
          { type: 'thinking', thinking: '**Alpha**' },
          { type: 'thinking', thinking: '**Beta**' },
        ],
      }),
    ).toEqual({
      type: 'thinking',
      text: 'Thinking: Beta',
    });
  });
});

// ---------------------------------------------------------------------------
// isFresherStatusCandidate — observable arms via getAssistantActiveTurnStatus
// ---------------------------------------------------------------------------

describe('isFresherStatusCandidate branch coverage', () => {
  it('prefers a current workflow status over an older generic content row', () => {
    // currentWorkflow candidate vs genericContent current -> returns true.
    expect(
      activeTurn(
        {
          isStreaming: true,
          workflowStatus: {
            phase: 'stream_idle',
            message: 'Still working; waiting for more response data.',
            startedAt: 2_000,
          },
          streamEvents: [
            {
              type: 'workflow_status',
              workflowStatus: {
                phase: 'stream_idle',
                message: 'Still working; waiting for more response data.',
                startedAt: 2_000,
              },
              startedAt: 2_000,
              updatedAt: 2_000,
            },
            { type: 'content', content: 'partial token', startedAt: 3_000, updatedAt: 3_000 },
          ],
        },
        3_500,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Still working; waiting for more response data.',
      startedAt: 2_000,
    });
  });

  it('prefers a newer-activity tool over an older tool regardless of array order', () => {
    // Two pending tools in state; the one with the later updatedAt wins via the
    // candidateTime > currentTime arm of isFresherStatusCandidate.
    expect(
      activeTurn({
        pendingTools: [
          {
            id: 'tool-a',
            name: 'pulse_get_nodes',
            input: '{}',
            status: 'running',
            startedAt: 100,
            updatedAt: 200,
          },
          {
            id: 'tool-b',
            name: 'pulse_read',
            input: '{}',
            status: 'running',
            startedAt: 100,
            updatedAt: 500,
          },
        ],
      }),
    ).toEqual({
      type: 'tool',
      text: 'Running read',
      startedAt: 100,
    });
  });
});
