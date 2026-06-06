import { describe, expect, it, vi } from 'vitest';
import type { ChatMessage } from '../types';
import {
  buildAssistantTranscriptFilename,
  downloadAssistantTranscriptFile,
  formatAssistantTranscript,
  hasAssistantTranscriptContent,
} from '../transcriptExport';

const timestamp = new Date('2026-06-06T12:34:56Z');

const readBlobText = (blob: Blob): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(reader.error);
    reader.readAsText(blob);
  });

describe('Assistant transcript export', () => {
  it('formats a clean session transcript with status and tool summaries', () => {
    const messages: ChatMessage[] = [
      {
        id: 'user-1',
        role: 'user',
        content: 'how many devices in this',
        timestamp,
      },
      {
        id: 'assistant-1',
        role: 'assistant',
        content: 'There are 4,358 entries in /dev.',
        timestamp,
        completedAt: new Date('2026-06-06T12:35:02Z'),
        model: 'openrouter:deepseek/deepseek-chat',
        streamEvents: [
          {
            type: 'workflow_status',
            workflowStatus: {
              message: 'Checking device nodes',
              phase: 'tool',
            },
          },
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: JSON.stringify({
                action: 'exec',
                command: 'ls /dev | wc -l',
                target_host: 'current_resource',
              }),
              output: '4358\n',
              success: true,
            },
          },
          {
            type: 'content',
            content: 'There are 4,358 entries in /dev.',
          },
        ],
      },
    ];

    const transcript = formatAssistantTranscript({
      messages,
      session: { id: 'session-123456789', title: 'Device count' },
      generatedAt: timestamp,
      getModelRouteLabel: () => 'DeepSeek via OpenRouter',
    });

    expect(transcript).toContain('# Pulse Assistant Transcript');
    expect(transcript).toContain('Session: Device count');
    expect(transcript).toContain('Session ID: session-123456789');
    expect(transcript).toContain('## User');
    expect(transcript).toContain('how many devices in this');
    expect(transcript).toContain('## Pulse Assistant');
    expect(transcript).toContain('Model: DeepSeek via OpenRouter');
    expect(transcript).toContain('[status] Checking device nodes');
    expect(transcript).toContain('[tool:read]');
    expect(transcript).toContain('ls /dev | wc -l');
    expect(transcript).toContain('completed');
    expect(transcript).toContain('There are 4,358 entries in /dev.');
    expect(transcript).not.toContain('4358\n\n##');
  });

  it('keeps hidden reasoning and raw tool-call leaks out of default transcripts', () => {
    const messages: ChatMessage[] = [
      {
        id: 'assistant-1',
        role: 'assistant',
        content:
          "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.pulse_read(target_host=\"current_resource\",command=\"ls/dev|wc-l\")",
        thinking: 'Private chain of thought',
        timestamp,
        streamEvents: [
          {
            type: 'thinking',
            thinking: 'Private chain of thought',
          },
        ],
      },
    ];

    const transcript = formatAssistantTranscript({
      messages,
      generatedAt: timestamp,
    });

    expect(transcript).toBe('');
    expect(transcript).not.toContain('Private chain of thought');
    expect(transcript).not.toContain('pulse_read');
    expect(transcript).not.toContain('ls/dev');
  });

  it('can include thinking and tool output when explicitly requested', () => {
    const messages: ChatMessage[] = [
      {
        id: 'assistant-1',
        role: 'assistant',
        content: '',
        timestamp,
        streamEvents: [
          {
            type: 'thinking',
            thinking: 'Checking the live state.',
          },
          {
            type: 'tool',
            tool: {
              name: 'pulse_get_active_alerts',
              input: JSON.stringify({ action: 'list' }),
              output: 'No active alerts',
              success: true,
            },
          },
        ],
      },
    ];

    const transcript = formatAssistantTranscript({
      messages,
      generatedAt: timestamp,
      includeThinking: true,
      includeToolOutput: true,
    });

    expect(transcript).toContain('[thinking]');
    expect(transcript).toContain('Checking the live state.');
    expect(transcript).toContain('No active alerts');
  });

  it('reports whether a transcript has content', () => {
    expect(hasAssistantTranscriptContent([])).toBe(false);
    expect(
      hasAssistantTranscriptContent([
        {
          id: 'empty-assistant',
          role: 'assistant',
          content: '',
          timestamp,
        },
      ]),
    ).toBe(false);
    expect(
      hasAssistantTranscriptContent([
        {
          id: 'user-1',
          role: 'user',
          content: 'hello',
          timestamp,
        },
      ]),
    ).toBe(true);
  });

  it('builds stable markdown filenames', () => {
    expect(buildAssistantTranscriptFilename('session-1234/abcd', timestamp)).toBe(
      'pulse-assistant-session-1234-abcd.md',
    );
    expect(buildAssistantTranscriptFilename('', timestamp)).toBe('pulse-assistant-2026-06-06.md');
  });

  it('downloads the transcript as a markdown blob', async () => {
    const originalCreateObjectURL = URL.createObjectURL;
    const originalRevokeObjectURL = URL.revokeObjectURL;
    const createObjectURL = vi.fn((blob: Blob) => {
      void blob;
      return 'blob:transcript';
    });
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, 'createObjectURL', {
      value: createObjectURL,
      configurable: true,
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      value: revokeObjectURL,
      configurable: true,
    });
    const originalCreateElement = document.createElement.bind(document);
    let createdAnchor: HTMLAnchorElement | null = null;

    const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tagName) => {
      const element = originalCreateElement(tagName);
      if (tagName.toLowerCase() === 'a') {
        createdAnchor = element as HTMLAnchorElement;
        vi.spyOn(createdAnchor, 'click').mockImplementation(() => undefined);
      }
      return element;
    });

    try {
      downloadAssistantTranscriptFile('transcript body', 'pulse-assistant-session.md');

      expect(createObjectURL).toHaveBeenCalled();
      const createdBlob = createObjectURL.mock.calls[0]?.[0] as unknown as Blob;
      expect(await readBlobText(createdBlob)).toBe('transcript body');
      expect(createdAnchor).not.toBeNull();
      const anchor = createdAnchor as unknown as HTMLAnchorElement;
      expect(anchor.href).toBe('blob:transcript');
      expect(anchor.download).toBe('pulse-assistant-session.md');
      expect(anchor.click).toHaveBeenCalled();
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:transcript');
    } finally {
      createElementSpy.mockRestore();
      if (originalCreateObjectURL) {
        Object.defineProperty(URL, 'createObjectURL', {
          value: originalCreateObjectURL,
          configurable: true,
        });
      }
      if (originalRevokeObjectURL) {
        Object.defineProperty(URL, 'revokeObjectURL', {
          value: originalRevokeObjectURL,
          configurable: true,
        });
      }
    }
  });
});
