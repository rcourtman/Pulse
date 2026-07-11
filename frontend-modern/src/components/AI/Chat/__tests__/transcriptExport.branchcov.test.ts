import { describe, expect, it } from 'vitest';
import type { AssistantTranscriptOptions } from '../transcriptExport';
import { formatAssistantTranscript, hasAssistantTranscriptContent } from '../transcriptExport';
import type { ChatMessage, StreamDisplayEvent } from '../types';

const timestamp = new Date('2026-06-06T12:34:56Z');

const assistantWithEvents = (events: StreamDisplayEvent[]): ChatMessage => ({
  id: 'assistant-1',
  role: 'assistant',
  content: '',
  timestamp,
  streamEvents: events,
});

const render = (
  messages: ChatMessage[],
  options: Partial<AssistantTranscriptOptions> = {},
): string =>
  formatAssistantTranscript({
    messages,
    generatedAt: timestamp,
    ...options,
  });

const readToolInput = (command: string): string =>
  JSON.stringify({ action: 'exec', command, target_host: 'current_resource' });

// ---------------------------------------------------------------------------
// formatTimestamp — reachable through appendMessageMetadata -> formatAssistantTranscript
// ---------------------------------------------------------------------------

describe('formatTimestamp guard branches', () => {
  it('formats a valid Date into the Time metadata line', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Answer text',
        timestamp: new Date('2026-06-06T12:34:56Z'),
      },
    ]);
    expect(transcript).toContain('Time:');
    expect(transcript).toContain('2026');
  });

  it('returns empty for a NaN date, leaving the Time label without a formatted value', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Answer text',
        timestamp: new Date(NaN),
      },
    ]);
    const timeLine = transcript.split('\n').find((line) => line.startsWith('Time:'));
    expect(timeLine).toBe('Time: ');
  });

  it('returns empty when timestamp is not a Date instance', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Answer text',
        timestamp: '2026-06-06T12:00:00Z' as unknown as Date,
      },
    ]);
    const timeLine = transcript.split('\n').find((line) => line.startsWith('Time:'));
    expect(timeLine).toBe('Time: ');
  });
});

// ---------------------------------------------------------------------------
// formatGeneratedAt — reachable through formatAssistantTranscript
// ---------------------------------------------------------------------------

describe('formatGeneratedAt guard branches', () => {
  it('emits a Generated ISO line for a valid date', () => {
    const transcript = render(
      [{ id: 'a1', role: 'assistant', content: 'Hi', timestamp }],
      { generatedAt: new Date('2026-06-06T12:34:56Z') },
    );
    expect(transcript).toContain('Generated: 2026-06-06T12:34:56.000Z');
  });

  it('omits the Generated line for a NaN date', () => {
    const transcript = render(
      [{ id: 'a1', role: 'assistant', content: 'Hi', timestamp }],
      { generatedAt: new Date(NaN) },
    );
    expect(transcript).not.toContain('Generated:');
  });

  it('omits the Generated line when generatedAt is not a Date instance', () => {
    const transcript = render(
      [{ id: 'a1', role: 'assistant', content: 'Hi', timestamp }],
      { generatedAt: 'not-a-date' as unknown as Date },
    );
    expect(transcript).not.toContain('Generated:');
  });

  it('falls back to the current time when generatedAt is omitted', () => {
    const before = new Date().toISOString();
    const transcript = formatAssistantTranscript({
      messages: [{ id: 'a1', role: 'assistant', content: 'Hi', timestamp }],
    });
    const after = new Date().toISOString();
    const generatedLine = transcript.split('\n').find((line) => line.startsWith('Generated:'));
    expect(generatedLine).toBeDefined();
    const iso = (generatedLine ?? '').replace('Generated: ', '');
    expect(iso >= before).toBe(true);
    expect(iso <= after).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// formatToolOutput — reachable through formatCompletedTool when includeToolOutput is true
// ---------------------------------------------------------------------------

describe('formatToolOutput truncation and normalization branches', () => {
  it('returns empty for whitespace-only output', () => {
    const transcript = render(
      [
        assistantWithEvents([
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: readToolInput('true'),
              output: '   \n   ',
              success: true,
            },
          },
        ]),
      ],
      { includeToolOutput: true },
    );
    expect(transcript).toContain('[tool:read]');
    // No ellipsis is appended because formatToolOutput short-circuits to ''
    expect(transcript).not.toContain('...');
  });

  it('preserves six lines without an ellipsis', () => {
    const output = ['one', 'two', 'three', 'four', 'five', 'six'].join('\n');
    const transcript = render(
      [
        assistantWithEvents([
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: readToolInput('echo x'),
              output,
              success: true,
            },
          },
        ]),
      ],
      { includeToolOutput: true },
    );
    expect(transcript).toContain('six');
    expect(transcript).not.toContain('...');
  });

  it('appends an ellipsis when output exceeds six lines', () => {
    const output = ['1', '2', '3', '4', '5', '6', '7', '8'].join('\n');
    const transcript = render(
      [
        assistantWithEvents([
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: readToolInput('echo x'),
              output,
              success: true,
            },
          },
        ]),
      ],
      { includeToolOutput: true },
    );
    expect(transcript).toContain('...');
    // Line 6 is included in the preview; line 7 is not
    expect(transcript).toContain('\n6');
    expect(transcript).not.toContain('\n7');
  });

  it('truncates each line to 160 characters', () => {
    const longLine = 'B'.repeat(200);
    const transcript = render(
      [
        assistantWithEvents([
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: readToolInput('echo x'),
              output: longLine,
              success: true,
            },
          },
        ]),
      ],
      { includeToolOutput: true },
    );
    expect(transcript).toContain('B'.repeat(160));
    expect(transcript).not.toContain('B'.repeat(161));
  });

  it('normalizes lone carriage returns to newlines', () => {
    const transcript = render(
      [
        assistantWithEvents([
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: readToolInput('echo x'),
              output: 'alpha\r\nbravo\rcharlie',
              success: true,
            },
          },
        ]),
      ],
      { includeToolOutput: true },
    );
    expect(transcript).toContain('alpha');
    expect(transcript).toContain('bravo');
    expect(transcript).toContain('charlie');
  });
});

// ---------------------------------------------------------------------------
// formatCompletedTool — reachable through streamEvents tool events and fallback toolCalls
// ---------------------------------------------------------------------------

describe('formatCompletedTool status and summary branches', () => {
  it('reports a failed status when success is false', () => {
    const transcript = render([
      assistantWithEvents([
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: readToolInput('ls /dev | wc -l'),
            output: 'permission denied',
            success: false,
          },
        },
      ]),
    ]);
    expect(transcript).toContain('failed');
    expect(transcript).not.toContain('completed');
  });

  it('omits the summary segment when parseToolInputSummary returns empty', () => {
    const transcript = render([
      assistantWithEvents([
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: '',
            output: '',
            success: true,
          },
        },
      ]),
    ]);
    // compactText joins the bare label and status with " - "
    expect(transcript).toContain('[tool:read] - completed');
    // No command preview since input is empty
    expect(transcript).not.toContain('$');
  });

  it('does not include tool output when includeToolOutput is not set', () => {
    const transcript = render([
      assistantWithEvents([
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: readToolInput('echo x'),
            output: 'sensitive-output-data',
            success: true,
          },
        },
      ]),
    ]);
    expect(transcript).not.toContain('sensitive-output-data');
  });
});

// ---------------------------------------------------------------------------
// formatCanceledTool — reachable through tool_cancel stream events
// ---------------------------------------------------------------------------

describe('formatCanceledTool fallback and reason branches', () => {
  it('falls back to the action label when no parsed summary is available', () => {
    const transcript = render([
      assistantWithEvents([
        {
          type: 'tool_cancel',
          toolId: 't1',
          toolCancel: {
            id: 't1',
            name: 'pulse_read',
            input: '',
          },
        },
      ]),
    ]);
    expect(transcript).toContain('[tool:read] Preparing read...');
    expect(transcript).toContain('skipped');
    expect(transcript).not.toContain('reason:');
  });

  it('omits the reason line when reason is only whitespace', () => {
    const transcript = render([
      assistantWithEvents([
        {
          type: 'tool_cancel',
          toolId: 't2',
          toolCancel: {
            id: 't2',
            name: 'pulse_read',
            input: readToolInput('ls /dev'),
            reason: '   ',
          },
        },
      ]),
    ]);
    expect(transcript).toContain('skipped');
    expect(transcript).not.toContain('reason:');
  });

  it('includes the command preview alongside the skipped status', () => {
    const transcript = render([
      assistantWithEvents([
        {
          type: 'tool_cancel',
          toolId: 't3',
          toolCancel: {
            id: 't3',
            name: 'pulse_read',
            input: readToolInput('ls /dev'),
          },
        },
      ]),
    ]);
    expect(transcript).toContain('$ ls /dev');
    expect(transcript).toContain('skipped');
  });
});

// ---------------------------------------------------------------------------
// hasRenderableEvent — uncovered switch arms: tool, pending_tool, content, default
// ---------------------------------------------------------------------------

describe('hasRenderableEvent uncovered switch arms', () => {
  it('treats a tool event as renderable only when a tool payload exists', () => {
    expect(
      hasAssistantTranscriptContent([
        assistantWithEvents([
          {
            type: 'tool',
            tool: { name: 'pulse_read', input: '', output: '', success: true },
          },
        ]),
      ]),
    ).toBe(true);
    expect(hasAssistantTranscriptContent([assistantWithEvents([{ type: 'tool' }])])).toBe(false);
  });

  it('treats a pending_tool event as renderable only when a pendingTool payload exists', () => {
    expect(
      hasAssistantTranscriptContent([
        assistantWithEvents([
          {
            type: 'pending_tool',
            pendingTool: { id: 'p1', name: 'pulse_read', input: '' },
          },
        ]),
      ]),
    ).toBe(true);
    expect(hasAssistantTranscriptContent([assistantWithEvents([{ type: 'pending_tool' }])])).toBe(
      false,
    );
  });

  it('treats a content event as renderable only when visible content is non-empty', () => {
    expect(
      hasAssistantTranscriptContent([
        assistantWithEvents([{ type: 'content', content: 'Visible answer' }]),
      ]),
    ).toBe(true);
    expect(
      hasAssistantTranscriptContent([assistantWithEvents([{ type: 'content', content: '   ' }])]),
    ).toBe(false);
    expect(hasAssistantTranscriptContent([assistantWithEvents([{ type: 'content' }])])).toBe(false);
  });

  it('returns false for an unknown event type via the default arm', () => {
    expect(
      hasAssistantTranscriptContent([
        assistantWithEvents([{ type: 'bogus' } as unknown as StreamDisplayEvent]),
      ]),
    ).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// hasTranscriptMessageContent — uncovered branches beyond user/empty-content
// ---------------------------------------------------------------------------

describe('hasTranscriptMessageContent uncovered branches', () => {
  it('detects an assistant message with only an error', () => {
    expect(
      hasAssistantTranscriptContent([
        { id: 'a1', role: 'assistant', content: '', timestamp, error: 'Turn failed' },
      ]),
    ).toBe(true);
  });

  it('detects an assistant message with only persisted tool calls', () => {
    expect(
      hasAssistantTranscriptContent([
        {
          id: 'a2',
          role: 'assistant',
          content: '',
          timestamp,
          toolCalls: [{ name: 'pulse_read', input: '', output: '', success: true }],
        },
      ]),
    ).toBe(true);
  });

  it('detects an assistant message with only pending tools', () => {
    expect(
      hasAssistantTranscriptContent([
        {
          id: 'a3',
          role: 'assistant',
          content: '',
          timestamp,
          pendingTools: [{ id: 'p1', name: 'pulse_read', input: '' }],
        },
      ]),
    ).toBe(true);
  });

  it('detects an assistant message with only pending approvals', () => {
    expect(
      hasAssistantTranscriptContent([
        {
          id: 'a4',
          role: 'assistant',
          content: '',
          timestamp,
          pendingApprovals: [
            { toolId: 't1', toolName: '', command: '', runOnHost: false },
          ],
        },
      ]),
    ).toBe(true);
  });

  it('detects an assistant message with only pending questions', () => {
    expect(
      hasAssistantTranscriptContent([
        {
          id: 'a5',
          role: 'assistant',
          content: '',
          timestamp,
          pendingQuestions: [{ questionId: 'q1', questions: [] }],
        },
      ]),
    ).toBe(true);
  });

  it('detects an assistant message with renderable stream events', () => {
    expect(
      hasAssistantTranscriptContent([
        assistantWithEvents([{ type: 'content', content: 'Streamed answer' }]),
      ]),
    ).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// appendMessageMetadata — uncovered branches
// ---------------------------------------------------------------------------

describe('appendMessageMetadata uncovered branches', () => {
  it('skips all metadata when includeAssistantMetadata is explicitly false', () => {
    const transcript = render(
      [
        {
          id: 'a1',
          role: 'assistant',
          content: 'Answer',
          timestamp,
          model: 'openrouter:deepseek/deepseek-chat',
          delivery: 'queued',
          isStreaming: true,
        },
      ],
      { includeAssistantMetadata: false },
    );
    expect(transcript).not.toContain('Time:');
    expect(transcript).not.toContain('Model:');
    expect(transcript).not.toContain('Delivery:');
    expect(transcript).not.toContain('State:');
  });

  it('emits queued delivery and streaming state metadata', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Answer',
        timestamp,
        delivery: 'queued',
        isStreaming: true,
      },
    ]);
    expect(transcript).toContain('Delivery: queued');
    expect(transcript).toContain('State: streaming');
  });

  it('emits Time but not Model metadata for a user message', () => {
    const transcript = render([
      { id: 'u1', role: 'user', content: 'Hello', timestamp },
    ]);
    expect(transcript).toContain('Time:');
    expect(transcript).not.toContain('Model:');
  });

  it('does not push a metadata block when no metadata fields are populated', () => {
    const transcript = render([
      {
        id: 'u1',
        role: 'user',
        content: 'Hello',
        timestamp: undefined as unknown as Date,
      },
    ]);
    expect(transcript).toContain('Hello');
    expect(transcript).not.toContain('Time:');
    expect(transcript).not.toContain('Model:');
    expect(transcript).not.toContain('Delivery:');
    expect(transcript).not.toContain('State:');
  });
});

// ---------------------------------------------------------------------------
// appendAssistantStreamEvents — fallback loops, guard-false arms, error/interruption
// ---------------------------------------------------------------------------

describe('appendAssistantStreamEvents fallback and guard branches', () => {
  it('renders persisted pendingTools when no pending_tool stream event is present', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: '',
        timestamp,
        pendingTools: [
          {
            id: 'p1',
            name: 'pulse_read',
            input: readToolInput('ls /dev'),
          },
        ],
      },
    ]);
    expect(transcript).toContain('[tool:read]');
    expect(transcript).toContain('pending');
  });

  it('renders persisted pendingApprovals when no approval stream event is present', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: '',
        timestamp,
        pendingApprovals: [
          {
            toolId: 't1',
            toolName: 'pulse_run_command',
            command: 'systemctl restart nginx',
            description: 'Restart nginx',
            runOnHost: true,
          },
        ],
      },
    ]);
    expect(transcript).toContain('[approval:cmd]');
    expect(transcript).toContain('approval required');
  });

  it('renders persisted pendingQuestions when no question stream event is present', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: '',
        timestamp,
        pendingQuestions: [
          {
            questionId: 'q1',
            questions: [{ id: 'i1', type: 'text', question: 'Which host?' }],
          },
        ],
      },
    ]);
    expect(transcript).toContain('[question] Which host?');
  });

  it('renders the error block for a failed turn', () => {
    const transcript = render([
      { id: 'a1', role: 'assistant', content: '', timestamp, error: 'Model timed out' },
    ]);
    expect(transcript).toContain('[error] Model timed out');
  });

  it('renders a stopped interruption notice', () => {
    const transcript = render([
      { id: 'a1', role: 'assistant', content: 'partial answer', timestamp, interruption: 'stopped' },
    ]);
    expect(transcript).toContain('[interrupted] Stopped by user');
  });

  it('renders a replaced interruption notice', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'partial answer',
        timestamp,
        interruption: 'replaced',
      },
    ]);
    expect(transcript).toContain('[interrupted] Replaced by a later user message');
  });

  it('skips thinking events when includeThinking is not enabled', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Public answer',
        timestamp,
        streamEvents: [{ type: 'thinking', thinking: 'Private reasoning' }],
      },
    ]);
    expect(transcript).toContain('Public answer');
    expect(transcript).not.toContain('[thinking]');
    expect(transcript).not.toContain('Private reasoning');
  });

  it('does not set toolAppended when a tool event lacks a payload, allowing toolCalls fallback', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: '',
        timestamp,
        streamEvents: [{ type: 'tool' }],
        toolCalls: [
          {
            name: 'pulse_read',
            input: readToolInput('ls /dev'),
            output: '',
            success: true,
          },
        ],
      },
    ]);
    // The guard `if (event.tool)` was false, so the fallback toolCalls loop fires
    expect(transcript).toContain('[tool:read]');
    expect(transcript).toContain('completed');
  });

  it('does not render a pending tool when a pending_tool event lacks a payload', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Visible content',
        timestamp,
        streamEvents: [{ type: 'pending_tool' }],
      },
    ]);
    expect(transcript).toContain('Visible content');
    expect(transcript).not.toContain('[tool:');
  });

  it('does not render a canceled tool when a tool_cancel event lacks a payload', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Visible content',
        timestamp,
        streamEvents: [{ type: 'tool_cancel' }],
      },
    ]);
    expect(transcript).toContain('Visible content');
    expect(transcript).not.toContain('skipped');
    expect(transcript).not.toContain('[tool:');
  });

  it('does not render an approval when an approval event lacks a payload', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Visible content',
        timestamp,
        streamEvents: [{ type: 'approval' }],
      },
    ]);
    expect(transcript).toContain('Visible content');
    expect(transcript).not.toContain('[approval:');
  });

  it('does not render a question when a question event lacks a payload', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Visible content',
        timestamp,
        streamEvents: [{ type: 'question' }],
      },
    ]);
    expect(transcript).toContain('Visible content');
    expect(transcript).not.toContain('[question]');
  });

  it('silently ignores unknown event types via the default arm', () => {
    const transcript = render([
      {
        id: 'a1',
        role: 'assistant',
        content: 'Visible content',
        timestamp,
        streamEvents: [{ type: 'bogus' } as unknown as StreamDisplayEvent],
      },
    ]);
    expect(transcript).toContain('Visible content');
    expect(transcript).not.toContain('[bogus]');
  });
});

// ---------------------------------------------------------------------------
// formatAssistantTranscript — structural edge cases
// ---------------------------------------------------------------------------

describe('formatAssistantTranscript structural edge cases', () => {
  it('returns empty string when no messages have content', () => {
    expect(formatAssistantTranscript({ messages: [] })).toBe('');
    expect(
      formatAssistantTranscript({
        messages: [{ id: 'a1', role: 'assistant', content: '', timestamp }],
      }),
    ).toBe('');
  });

  it('skips messages that have no renderable content but includes those that do', () => {
    const transcript = render([
      { id: 'empty', role: 'assistant', content: '', timestamp },
      { id: 'user-1', role: 'user', content: 'Hello world', timestamp },
    ]);
    expect(transcript).not.toContain('## User\n\n\nHello world');
    expect(transcript).toContain('## User');
    expect(transcript).toContain('Hello world');
    // The empty assistant message is skipped entirely
    expect(transcript).not.toContain('## Pulse Assistant');
  });

  it('omits Session lines when session has no title or id', () => {
    const transcript = render([{ id: 'u1', role: 'user', content: 'Hi', timestamp }], {
      session: {},
    });
    expect(transcript).not.toContain('Session:');
    expect(transcript).not.toContain('Session ID:');
  });

  it('collapses three or more consecutive newlines into two', () => {
    const transcript = render([
      { id: 'u1', role: 'user', content: 'A\n\n\n\n\nB', timestamp },
    ]);
    expect(transcript).not.toMatch(/\n{3,}/);
    expect(transcript).toContain('A');
    expect(transcript).toContain('B');
  });
});
