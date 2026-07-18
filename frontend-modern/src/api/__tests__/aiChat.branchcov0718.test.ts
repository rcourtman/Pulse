/**
 * Branch-coverage tests for currently-uncovered AIChatAPI methods and the
 * chat() stream's onTrailingParseError handler:
 *   - AIChatAPI.getStatus
 *   - AIChatAPI.createSession
 *   - AIChatAPI.deleteSession
 *   - AIChatAPI.abortSession
 *   - AIChatAPI.denyCommand (reason present vs default)
 *   - AIChatAPI.listAgents
 *   - AIChatAPI.forkSession
 *   - chat()'s onTrailingParseError callback (parseable vs unparseable trailing
 *     buffer)
 *
 * The transport is mocked with the same harness used by aiChat.test.ts
 * (vi.mock('@/utils/apiClient', ...) plus vi.mock('@/utils/logger', ...)).
 * These tests intentionally do NOT re-assert anything aiChat.test.ts already
 * covers (listSessions normalization, renameSession, undo/redo, steerSession,
 * summarizeSession, chat happy-path streaming, fixtures, etc.).
 *
 * Branch arms exercised here:
 *   - getStatus returns the apiFetchJSON payload cast to AIStatus
 *   - createSession issues a bare POST with no body and no Content-Type
 *   - deleteSession routes through apiFetch (NOT apiFetchJSON), DELETE method
 *   - deleteSession URL-encodes '/' in the session id (encodeURIComponent arm)
 *   - abortSession routes through apiFetch, POST to the /abort suffix
 *   - abortSession URL-encodes '/' in the session id before appending /abort
 *   - denyCommand truthy-reason branch -> body uses the supplied reason
 *   - denyCommand falsy-reason branch -> body falls back to 'User skipped':
 *        * no second argument supplied
 *        * empty-string reason
 *   - denyCommand URL-encodes '/' in the approval id
 *   - listAgents returns the apiFetchJSON payload cast to Agent[]
 *   - forkSession POSTs to the /fork suffix and URL-encodes the session id
 *   - onTrailingParseError parseable arm: a trailing 'data:' line without a
 *     terminating blank line is parsed and dispatched; onEvent receives the
 *     terminal event and the synthetic close-error is NOT emitted
 *   - onTrailingParseError unparseable arm: a trailing 'data:' line that fails
 *     to parse as JSON triggers logger.warn('[AI Chat] Could not parse
 *     remaining buffer') and then the onComplete handler surfaces the
 *     synthetic 'closed before the response finished' interruption
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { AIChatAPI } from '@/api/aiChat';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

describe('AIChatAPI branch coverage', () => {
  const apiFetchMock = vi.mocked(apiFetch);
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchMock.mockReset();
    apiFetchJSONMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  describe('getStatus', () => {
    it('GETs /api/ai/status through apiFetchJSON and returns the AIStatus payload verbatim', async () => {
      const status = { running: true, engine: 'pulse-assistant' };
      apiFetchJSONMock.mockResolvedValueOnce(status);

      await expect(AIChatAPI.getStatus()).resolves.toEqual(status);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/status');
      expect(apiFetchMock).not.toHaveBeenCalled();
    });
  });

  describe('createSession', () => {
    it('POSTs to /api/ai/sessions with no body and no headers (only method)', async () => {
      const created = {
        id: 'session-7',
        title: 'New Chat',
        created_at: '2026-07-18T10:00:00Z',
        updated_at: '2026-07-18T10:00:00Z',
        message_count: 0,
      };
      apiFetchJSONMock.mockResolvedValueOnce(created);

      await expect(AIChatAPI.createSession()).resolves.toEqual(created);

      // The module passes ONLY { method: 'POST' } — no body, no Content-Type.
      // (apiFetchJSON adds its own Content-Type downstream; that is the
      // wrapper's concern, not the module's, so it is not asserted here.)
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/sessions', {
        method: 'POST',
      });
    });
  });

  describe('deleteSession', () => {
    it('issues a plain DELETE through apiFetch (no JSON parsing) and resolves void', async () => {
      // deleteSession uses apiFetch, NOT apiFetchJSON, because the backend
      // returns 204 No Content. Asserting the resolved value is undefined
      // confirms the function swallows the response rather than returning it.
      apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

      await expect(AIChatAPI.deleteSession('session-1')).resolves.toBeUndefined();

      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/sessions/session-1', {
        method: 'DELETE',
      });
      expect(apiFetchJSONMock).not.toHaveBeenCalled();
    });

    it('URL-encodes slash-bearing session ids in the path', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

      await AIChatAPI.deleteSession('session/root');

      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/sessions/session%2Froot', {
        method: 'DELETE',
      });
    });
  });

  describe('abortSession', () => {
    it('POSTs to the /abort suffix through apiFetch and resolves void', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

      await expect(AIChatAPI.abortSession('session-1')).resolves.toBeUndefined();

      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/sessions/session-1/abort', {
        method: 'POST',
      });
      expect(apiFetchJSONMock).not.toHaveBeenCalled();
    });

    it('URL-encodes slash-bearing session ids before appending /abort', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

      await AIChatAPI.abortSession('session/root');

      // Only the session id segment is encoded — the literal '/abort' suffix
      // must remain unencoded so the backend route still matches.
      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/sessions/session%2Froot/abort', {
        method: 'POST',
      });
    });
  });

  describe('denyCommand', () => {
    it('posts the supplied reason body to the deny endpoint', async () => {
      const result = { denied: true, message: 'Command denied.' };
      apiFetchJSONMock.mockResolvedValueOnce(result);

      await expect(AIChatAPI.denyCommand('approval-1', 'Wrong host')).resolves.toEqual(
        result,
      );

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/approvals/approval-1/deny', {
        method: 'POST',
        body: JSON.stringify({ reason: 'Wrong host' }),
      });
    });

    it('substitutes the "User skipped" default when no reason is provided', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ denied: true, message: 'ok' });

      await AIChatAPI.denyCommand('approval-1');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/approvals/approval-1/deny', {
        method: 'POST',
        body: JSON.stringify({ reason: 'User skipped' }),
      });
    });

    it('treats an empty-string reason as absent (falls back to the default)', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ denied: true, message: 'ok' });

      await AIChatAPI.denyCommand('approval-1', '');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/approvals/approval-1/deny', {
        method: 'POST',
        body: JSON.stringify({ reason: 'User skipped' }),
      });
    });

    it('URL-encodes slash-bearing approval ids in the path', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ denied: true, message: 'ok' });

      await AIChatAPI.denyCommand('approval/with/slash');

      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/ai/approvals/approval%2Fwith%2Fslash/deny',
        expect.objectContaining({ method: 'POST' }),
      );
    });
  });

  describe('listAgents', () => {
    it('GETs /api/ai/agents and returns the parsed agent array verbatim', async () => {
      const agents = [
        {
          name: 'build',
          mode: 'subagent',
          native: true,
          model: { providerID: 'openrouter', modelID: 'deepseek/deepseek-chat' },
        },
        { name: 'code', mode: 'primary', hidden: false },
      ];
      apiFetchJSONMock.mockResolvedValueOnce(agents);

      await expect(AIChatAPI.listAgents()).resolves.toEqual(agents);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/agents');
    });
  });

  describe('forkSession', () => {
    it('POSTs to the /fork suffix through apiFetchJSON and returns the forked session', async () => {
      const forked = {
        id: 'session-fork',
        title: 'Fork of session/root',
        created_at: '2026-07-18T11:00:00Z',
        updated_at: '2026-07-18T11:00:00Z',
        message_count: 4,
      };
      apiFetchJSONMock.mockResolvedValueOnce(forked);

      await expect(AIChatAPI.forkSession('session/root')).resolves.toEqual(forked);

      // The session id is encoded; the literal '/fork' suffix is not.
      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/ai/sessions/session%2Froot/fork',
        { method: 'POST' },
      );
    });
  });

  describe('chat() onTrailingParseError arm', () => {
    // Stream chunks are authored WITHOUT a trailing '\n\n' so that
    // consumeJSONEventStream leaves them in `buffer` at EOF, which is exactly
    // what activates the trailing-buffer branch in src/api/streaming.ts. The
    // aiChat.ts onTrailingParseError callback is then either bypassed (when the
    // trailing line parses) or invoked (when it does not).

    it('parses a trailing "data:" line with no terminator as a final event and skips the synthetic close error', async () => {
      const encoder = new TextEncoder();
      const read = vi
        .fn()
        .mockResolvedValueOnce({
          done: false,
          value: encoder.encode('data: {"type":"done","data":{"session_id":"s-1"}}'),
        })
        .mockResolvedValueOnce({ done: true, value: undefined });
      const releaseLock = vi.fn();
      const onEvent = vi.fn();

      apiFetchMock.mockResolvedValueOnce({
        ok: true,
        body: { getReader: () => ({ read, releaseLock }) },
      } as unknown as Response);

      await AIChatAPI.chat('hello', undefined, undefined, onEvent);

      // The trailing line is parsed by the trailing-buffer arm and dispatched
      // as a normal event; because 'done' is terminal the stream early-returns
      // and the onComplete synthetic-close error must NOT fire.
      expect(onEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          type: 'done',
          data: { session_id: 's-1' },
        }),
      );
      const errorCalls = onEvent.mock.calls.filter(([event]) => event.type === 'error');
      expect(errorCalls).toHaveLength(0);
      expect(logger.warn).not.toHaveBeenCalledWith('[AI Chat] Could not parse remaining buffer');
      expect(releaseLock).toHaveBeenCalledTimes(1);
    });

    it('invokes onTrailingParseError (and surfaces a synthetic close error) when the trailing line is unparseable', async () => {
      const encoder = new TextEncoder();
      const read = vi
        .fn()
        .mockResolvedValueOnce({
          done: false,
          value: encoder.encode('data: not valid json'),
        })
        .mockResolvedValueOnce({ done: true, value: undefined });
      const releaseLock = vi.fn();
      const onEvent = vi.fn();

      apiFetchMock.mockResolvedValueOnce({
        ok: true,
        body: { getReader: () => ({ read, releaseLock }) },
      } as unknown as Response);

      await AIChatAPI.chat('hello', undefined, undefined, onEvent);

      // The unparseable trailing line triggers aiChat.ts's onTrailingParseError
      // handler, whose only effect is a logger.warn call.
      expect(logger.warn).toHaveBeenCalledWith('[AI Chat] Could not parse remaining buffer');
      // Because the trailing parse failed, no terminal event was delivered and
      // onComplete fires, surfacing the synthetic "closed before the response
      // finished" interruption that lets the user retry the turn.
      expect(onEvent).toHaveBeenCalledWith({
        type: 'error',
        data: { message: expect.stringContaining('closed before the response finished') },
      });
      expect(releaseLock).toHaveBeenCalledTimes(1);
    });
  });
});
