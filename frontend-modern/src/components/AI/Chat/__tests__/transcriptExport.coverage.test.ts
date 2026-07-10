import { describe, expect, it } from 'vitest';
import type { AssistantTranscriptOptions } from '../transcriptExport';
import { formatAssistantTranscript, hasAssistantTranscriptContent } from '../transcriptExport';
import type { ChatMessage, PendingApproval, Question, StreamDisplayEvent } from '../types';

const timestamp = new Date('2026-06-06T12:34:56Z');

const assistantWithEvents = (events: StreamDisplayEvent[]): ChatMessage => ({
  id: 'assistant-1',
  role: 'assistant',
  content: '',
  timestamp,
  streamEvents: events,
});

const render = (
  events: StreamDisplayEvent[],
  options: Partial<AssistantTranscriptOptions> = {},
): string =>
  formatAssistantTranscript({
    messages: [assistantWithEvents(events)],
    generatedAt: timestamp,
    ...options,
  });

const hasContent = (event: StreamDisplayEvent): boolean =>
  hasAssistantTranscriptContent([assistantWithEvents([event])]);

describe('formatPendingTool', () => {
  it('uses the parsed input summary, default pending status, and command preview', () => {
    const transcript = render([
      {
        type: 'pending_tool',
        pendingTool: {
          id: 't1',
          name: 'pulse_read',
          input: JSON.stringify({
            action: 'exec',
            command: 'ls /dev | wc -l',
            target_host: 'current_resource',
          }),
        },
      },
    ]);

    expect(transcript).toContain('[tool:read] Inspect devices on current resource - pending');
    expect(transcript).toContain('$ ls /dev | wc -l');
  });

  it('falls back to the action label and surfaces status plus progress without a command preview', () => {
    const transcript = render([
      {
        type: 'pending_tool',
        pendingTool: {
          id: 't2',
          name: 'pulse_get_disk_health',
          input: '',
          status: 'waiting',
          progress: 'probing controllers',
        },
      },
    ]);

    expect(transcript).toContain(
      '[tool:disks] Checking disks... - waiting - progress: probing controllers',
    );
    expect(transcript).not.toContain('$');
  });
});

describe('formatApproval', () => {
  it('marks a non-executing high-risk approval as approval required', () => {
    const transcript = render([
      {
        type: 'approval',
        approval: {
          toolId: 't1',
          toolName: 'pulse_run_command',
          description: 'Restart nginx service',
          command: 'systemctl restart nginx',
          risk: 'high',
          runOnHost: true,
        },
      },
    ]);

    expect(transcript).toContain(
      '[approval:cmd] Restart nginx service - approval required - high risk',
    );
  });

  it('falls back to the command and executing status when description is absent', () => {
    const transcript = render([
      {
        type: 'approval',
        approval: {
          toolId: 't2',
          toolName: '',
          command: 'rm -rf /tmp/cache',
          isExecuting: true,
          runOnHost: true,
        },
      },
    ]);

    expect(transcript).toContain('[approval:approval] rm -rf /tmp/cache - executing');
    expect(transcript).not.toContain('risk');
  });

  it('uses the generic action subject when no description or command is present', () => {
    const transcript = render([
      {
        type: 'approval',
        approval: {
          toolId: 't3',
          toolName: 'pulse_read',
          runOnHost: false,
        } as unknown as PendingApproval,
      },
    ]);

    expect(transcript).toContain('[approval:read] action - approval required');
  });
});

describe('formatQuestion', () => {
  it('renders a single prompt', () => {
    const transcript = render([
      {
        type: 'question',
        question: {
          questionId: 'q1',
          questions: [{ id: 'q1a', type: 'text', question: 'Which host should I inspect?' }],
        },
      },
    ]);

    expect(transcript).toContain('[question] Which host should I inspect?');
  });

  it('joins multiple prompts with a slash separator', () => {
    const transcript = render([
      {
        type: 'question',
        question: {
          questionId: 'q2',
          questions: [
            { id: '1', type: 'select', question: 'Pick severity' },
            { id: '2', type: 'select', question: 'Pick scope' },
          ],
        },
      },
    ]);

    expect(transcript).toContain('[question] Pick severity / Pick scope');
  });

  it('falls back to the header when the question text is missing', () => {
    const transcript = render([
      {
        type: 'question',
        question: {
          questionId: 'q3',
          questions: [{ id: '1', type: 'text', header: 'Resource name' } as unknown as Question],
        },
      },
    ]);

    expect(transcript).toContain('[question] Resource name');
  });

  it('omits the question block when every prompt normalizes to empty', () => {
    const transcript = render([
      {
        type: 'question',
        question: {
          questionId: 'q4',
          questions: [{ id: '1', type: 'text', question: '   ', header: '' }],
        },
      },
    ]);

    expect(transcript).toContain('## Pulse Assistant');
    expect(transcript).not.toContain('[question]');
  });
});

describe('formatModelSwitch', () => {
  const labelFor = (id: string): string =>
    (
      ({
        'm-sel': 'Selected Route',
        'm-new': 'New Route',
        'm-old': 'Old Route',
        'm-x': 'X Route',
      }) as Record<string, string>
    )[id] ?? id;

  it('renders a selected route with the using prefix', () => {
    const transcript = render(
      [{ type: 'model_switch', model: 'm-sel', modelEvent: 'selected' }],
      { getModelRouteLabel: labelFor },
    );

    expect(transcript).toContain('[model] using Selected Route');
  });

  it('renders a switch after a failed route', () => {
    const transcript = render(
      [{ type: 'model_switch', model: 'm-new', failedModel: 'm-old', modelEvent: 'switch' }],
      { getModelRouteLabel: labelFor },
    );

    expect(transcript).toContain('[model] New Route after Old Route');
  });

  it('renders a plain switch without a failed route', () => {
    const transcript = render([{ type: 'model_switch', model: 'm-x', modelEvent: 'switch' }], {
      getModelRouteLabel: labelFor,
    });

    expect(transcript).toContain('[model] X Route');
    expect(transcript).not.toContain('after');
  });

  it('does not duplicate the route when the failed route matches the new one', () => {
    const transcript = render(
      [{ type: 'model_switch', model: 'm-x', failedModel: 'm-x', modelEvent: 'switch' }],
      { getModelRouteLabel: labelFor },
    );

    expect(transcript).toContain('[model] X Route');
    expect(transcript).not.toContain('after');
  });
});

describe('hasRenderableEvent switch arms', () => {
  it('treats thinking as renderable only when included and non-empty', () => {
    expect(hasContent({ type: 'thinking', thinking: 'Private reasoning' })).toBe(false);
    expect(
      render([{ type: 'thinking', thinking: 'Private reasoning' }], { includeThinking: true }),
    ).toContain('[thinking]');
    expect(render([{ type: 'thinking', thinking: '   ' }], { includeThinking: true })).toBe('');
  });

  it('renders workflow status only when it formats to a message', () => {
    expect(
      hasContent({
        type: 'workflow_status',
        workflowStatus: { message: 'Reading current inventory.' },
      }),
    ).toBe(true);
    expect(hasContent({ type: 'workflow_status' })).toBe(false);
  });

  it('renders tool cancellation only when a cancel payload exists', () => {
    expect(
      hasContent({
        type: 'tool_cancel',
        toolCancel: { id: 'c1', name: 'pulse_read', input: '' },
      }),
    ).toBe(true);
    expect(hasContent({ type: 'tool_cancel' })).toBe(false);
  });

  it('renders model switch only when a model is present', () => {
    expect(hasContent({ type: 'model_switch', model: 'm' })).toBe(true);
    expect(hasContent({ type: 'model_switch' })).toBe(false);
  });

  it('renders approval only when an approval payload exists', () => {
    expect(
      hasContent({
        type: 'approval',
        approval: { toolId: 'a', toolName: '', command: '', runOnHost: false },
      }),
    ).toBe(true);
    expect(hasContent({ type: 'approval' })).toBe(false);
  });

  it('renders question only when a question payload exists', () => {
    expect(
      hasContent({ type: 'question', question: { questionId: 'q', questions: [] } }),
    ).toBe(true);
    expect(hasContent({ type: 'question' })).toBe(false);
  });
});
