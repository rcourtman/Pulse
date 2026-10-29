import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal, Suspense } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import investigationMessagesSource from '../InvestigationMessages.tsx?raw';
import InvestigationMessages from '../InvestigationMessages';

const getMessages = vi.hoisted(() => vi.fn());
vi.mock('@/api/patrol', () => ({
  getInvestigationMessages: getMessages,
  formatTimestamp: (value: string) => value,
}));

beforeEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  getMessages.mockReset();
});
afterEach(cleanup);

describe('InvestigationMessages', () => {
  it('reads the thread through the non-suspending query', () => {
    // The nearest Suspense boundary above a review is the app's route
    // boundary, whose fallback replaces the page and loses the reader's place.
    expect(investigationMessagesSource).not.toContain('createResource');
    expect(investigationMessagesSource).toContain('createNonSuspendingQuery');
  });

  it('keeps the page around a finding review while the thread loads', async () => {
    let finishRead: (value: unknown) => void = () => {};
    getMessages.mockReturnValue(
      new Promise((resolve) => {
        finishRead = resolve;
      }),
    );

    render(() => (
      <Suspense fallback={<div>Loading view...</div>}>
        <InvestigationMessages findingId="finding-3" />
      </Suspense>
    ));

    expect(await screen.findByText('Loading messages...')).toBeInTheDocument();
    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();

    finishRead({
      messages: [
        { id: 'turn-1', role: 'assistant', content: 'Checked the service log.', timestamp: '1' },
      ],
    });
    expect(await screen.findByText('Checked the service log.')).toBeInTheDocument();
    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();
  });

  it("reads a new run's thread instead of keeping the last run's", async () => {
    getMessages.mockResolvedValueOnce({
      messages: [
        { id: 'a-1', role: 'assistant', content: 'First run read the log.', timestamp: '1' },
      ],
    });
    let finishSecondRead: (value: unknown) => void = () => {};
    getMessages.mockReturnValueOnce(
      new Promise((resolve) => {
        finishSecondRead = resolve;
      }),
    );
    const [sessionId, setSessionId] = createSignal('session-a');

    render(() => <InvestigationMessages findingId="finding-4" sessionId={sessionId()} />);
    expect(await screen.findByText('First run read the log.')).toBeInTheDocument();

    // While the new run's thread loads, the last run's is not shown as its.
    setSessionId('session-b');
    expect(await screen.findByText('Loading messages...')).toBeInTheDocument();
    expect(screen.queryByText('First run read the log.')).not.toBeInTheDocument();

    finishSecondRead({
      messages: [
        { id: 'b-1', role: 'assistant', content: 'Second run read memory.', timestamp: '1' },
      ],
    });
    expect(await screen.findByText('Second run read memory.')).toBeInTheDocument();
    expect(screen.queryByText('First run read the log.')).not.toBeInTheDocument();
  });

  it('reads the thread again 30s after a failed read', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      getMessages.mockRejectedValueOnce(new Error('Pulse is restarting'));
      getMessages.mockResolvedValueOnce({
        messages: [{ id: 'r-1', role: 'assistant', content: 'Read on retry.', timestamp: '1' }],
      });
      render(() => <InvestigationMessages findingId="finding-7" sessionId="session-retry" />);
      expect(
        await screen.findByRole('alert', { name: 'Investigation thread read failed' }),
      ).toHaveTextContent('This does not mean it is empty.');
      expect(screen.queryByText('No investigation messages available.')).not.toBeInTheDocument();

      vi.advanceTimersByTime(30_000);
      expect(await screen.findByText('Read on retry.')).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it('lets a read-only retry recover an unavailable thread without exposing its error payload', async () => {
    getMessages.mockRejectedValueOnce(Object.assign(new Error('private path'), { status: 503 }));
    let finishRead: (value: unknown) => void = () => {};
    getMessages.mockReturnValueOnce(
      new Promise((resolve) => {
        finishRead = resolve;
      }),
    );
    render(() => <InvestigationMessages findingId="unavailable" sessionId="unavailable" />);
    await screen.findByRole('alert', { name: 'Investigation thread read failed' });
    expect(screen.queryByText('private path')).not.toBeInTheDocument();
    expect(screen.queryByText('No investigation messages available.')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry messages' }));
    expect(screen.getByRole('button', { name: 'Retrying messages...' })).toBeDisabled();
    finishRead({
      messages: [
        {
          id: 'recovered',
          role: 'assistant',
          content: 'Recovered thread evidence.',
          timestamp: '1',
        },
      ],
    });
    expect(await screen.findByText('Recovered thread evidence.')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it.each([404, 200])(
    'shows genuine empty messages for HTTP %s, not a read failure',
    async (status) => {
      if (status === 404)
        getMessages.mockRejectedValueOnce(Object.assign(new Error('not found'), { status }));
      else getMessages.mockResolvedValueOnce({ messages: [] });
      render(() => <InvestigationMessages findingId="absent" sessionId="absent" />);
      expect(await screen.findByText('No investigation messages available.')).toBeInTheDocument();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    },
  );

  it('labels retained messages after failure and keeps the same rows through successful recovery', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      const message = {
        id: 'retained',
        role: 'assistant',
        content: 'Retained message evidence.',
        timestamp: '1',
      };
      getMessages.mockResolvedValueOnce({ messages: [message] });
      getMessages.mockRejectedValueOnce(Object.assign(new Error('unavailable'), { status: 503 }));
      getMessages.mockResolvedValueOnce({
        messages: [
          message,
          { id: 'new', role: 'assistant', content: 'New evidence after recovery.', timestamp: '2' },
        ],
      });
      render(() => <InvestigationMessages findingId="retained" sessionId="retained" live />);
      const row = await screen.findByText('Retained message evidence.');
      vi.advanceTimersByTime(5_000);
      expect(
        await screen.findByRole('alert', { name: 'Investigation thread read failed' }),
      ).toHaveTextContent('Newer messages may be missing.');
      expect(screen.getByText('Retained message evidence.')).toBe(row);
      fireEvent.click(screen.getByRole('button', { name: 'Retry messages' }));
      expect(await screen.findByText('New evidence after recovery.')).toBeInTheDocument();
      expect(screen.getByText('Retained message evidence.')).toBe(row);
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it.each([401, 403])('withdraws retained thread messages after HTTP %s', async (status) => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      getMessages.mockResolvedValueOnce({
        messages: [
          {
            id: 'private',
            role: 'assistant',
            content: 'Previously authorised message.',
            timestamp: '1',
          },
        ],
      });
      getMessages.mockRejectedValueOnce(Object.assign(new Error('secret payload'), { status }));
      render(() => <InvestigationMessages findingId="private" sessionId="private" live />);
      await screen.findByText('Previously authorised message.');
      vi.advanceTimersByTime(5_000);
      const alert = await screen.findByRole('alert', { name: 'Investigation thread read failed' });
      expect(alert).toHaveTextContent(status === 401 ? 'Sign in again' : 'Access denied');
      expect(alert).not.toHaveTextContent('secret payload');
      expect(screen.queryByText('Previously authorised message.')).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not borrow the previous session messages when the new session is unavailable', async () => {
    getMessages.mockResolvedValueOnce({
      messages: [
        { id: 'old', role: 'assistant', content: 'Old session evidence.', timestamp: '1' },
      ],
    });
    getMessages.mockRejectedValueOnce(Object.assign(new Error('unavailable'), { status: 503 }));
    const [sessionId, setSessionId] = createSignal('old');
    render(() => <InvestigationMessages findingId="same-finding" sessionId={sessionId()} />);
    await screen.findByText('Old session evidence.');
    setSessionId('new');
    await screen.findByRole('alert', { name: 'Investigation thread read failed' });
    expect(screen.queryByText('Old session evidence.')).not.toBeInTheDocument();
    expect(screen.queryByText('No investigation messages available.')).not.toBeInTheDocument();
  });

  it('does not read a finished thread again while it stays open', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      getMessages.mockResolvedValue({
        messages: [{ id: 'f-1', role: 'assistant', content: 'Done.', timestamp: '1' }],
      });
      render(() => <InvestigationMessages findingId="finding-6" sessionId="session-done" />);
      expect(await screen.findByText('Done.')).toBeInTheDocument();

      vi.advanceTimersByTime(120_000);
      expect(getMessages).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('reads the thread again while its run is live', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      const first = {
        id: 'm-1',
        role: 'assistant',
        content: 'Started on the service.',
        timestamp: '1',
      };
      // Each read parses new objects, as the API client does.
      getMessages.mockResolvedValueOnce({ messages: [{ ...first }] });
      getMessages.mockResolvedValueOnce({
        messages: [
          { ...first },
          { id: 'm-2', role: 'assistant', content: 'Found the crash in the log.', timestamp: '2' },
        ],
      });

      render(() => <InvestigationMessages findingId="finding-5" sessionId="session-live" live />);
      const firstRow = await screen.findByText('Started on the service.');

      vi.advanceTimersByTime(5_000);
      expect(await screen.findByText('Found the crash in the log.')).toBeInTheDocument();
      // The unchanged first message keeps its row.
      expect(screen.getByText('Started on the service.')).toBe(firstRow);
    } finally {
      vi.useRealTimers();
    }
  });

  it('reads the thread once more when its run finishes', async () => {
    getMessages.mockResolvedValueOnce({
      messages: [{ id: 'e-1', role: 'assistant', content: 'Checking the log.', timestamp: '1' }],
    });
    getMessages.mockResolvedValueOnce({
      messages: [
        { id: 'e-1', role: 'assistant', content: 'Checking the log.', timestamp: '1' },
        { id: 'e-2', role: 'assistant', content: 'The disk filled up.', timestamp: '2' },
      ],
    });
    const [live, setLive] = createSignal(true);

    render(() => (
      <InvestigationMessages findingId="finding-8" sessionId="session-ending" live={live()} />
    ));
    expect(await screen.findByText('Checking the log.')).toBeInTheDocument();

    // The run finished between two live reads.
    setLive(false);
    expect(await screen.findByText('The disk filled up.')).toBeInTheDocument();
    expect(getMessages).toHaveBeenCalledTimes(2);
  });

  it('retains merged failed tool evidence and exposes it through the shared disclosure', async () => {
    getMessages.mockResolvedValue({
      messages: [
        {
          id: 'turn-1',
          role: 'assistant',
          content: '',
          timestamp: '2026-09-06',
          tool_calls: [
            {
              id: 'read-1',
              name: 'pulse_read',
              input: { resource_id: 'app-container-1' },
              output: '{"error":"NO_AGENT","message":"Command agent is not connected"}',
              success: false,
            },
          ],
        },
      ],
    });
    render(() => <InvestigationMessages findingId="finding-1" />);
    const disclosure = await screen.findByRole('button', { name: /failed/i });
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');
    fireEvent.keyDown(disclosure, { key: 'Enter' });
    expect(disclosure).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText(/"error":\s*"NO_AGENT"/)).toBeVisible();
    fireEvent.keyDown(disclosure, { key: ' ' });
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');
  });

  it('does not infer completion from historical calls that have no recorded result status', async () => {
    getMessages.mockResolvedValue({
      messages: [
        {
          id: 'turn-2',
          role: 'assistant',
          content: '',
          timestamp: '2026-09-06',
          tool_calls: [
            {
              id: 'query-1',
              name: 'pulse_query',
              input: { action: 'metrics' },
              output: 'Historical output',
            },
          ],
        },
      ],
    });
    render(() => <InvestigationMessages findingId="finding-2" />);
    expect(await screen.findByText('Historical output')).toBeVisible();
    expect(screen.queryByText('completed')).not.toBeInTheDocument();
    expect(screen.queryByText('failed')).not.toBeInTheDocument();
  });
});
