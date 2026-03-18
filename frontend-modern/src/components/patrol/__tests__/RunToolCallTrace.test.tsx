import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RunToolCallTrace } from '../RunToolCallTrace';

const getPatrolRunWithToolCallsMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/patrol', () => ({
  getPatrolRunWithToolCalls: (...args: unknown[]) => getPatrolRunWithToolCallsMock(...args),
}));

describe('RunToolCallTrace', () => {
  beforeEach(() => {
    getPatrolRunWithToolCallsMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('loads tool calls for the selected run directly by id', async () => {
    getPatrolRunWithToolCallsMock.mockResolvedValue({
      id: 'run-75',
      tool_calls: [
        {
          id: 'call-1',
          tool_name: 'ssh_execute',
          input: '{"command":"uptime"}',
          output: 'ok',
          success: true,
          duration_ms: 42,
        },
      ],
    });

    render(() => <RunToolCallTrace runId="run-75" toolCallCount={1} />);

    fireEvent.click(screen.getByRole('button', { name: /Tool calls \(1\)/ }));

    expect(getPatrolRunWithToolCallsMock).toHaveBeenCalledWith('run-75');
    expect(await screen.findByText('ssh_execute')).toBeInTheDocument();
  });

  it('shows the unavailable state when the selected run has no tool-call detail payload', async () => {
    getPatrolRunWithToolCallsMock.mockResolvedValue(null);

    render(() => <RunToolCallTrace runId="run-missing" toolCallCount={2} />);

    fireEvent.click(screen.getByRole('button', { name: /Tool calls \(2\)/ }));

    expect(
      await screen.findByText('Tool call details not available for this run.'),
    ).toBeInTheDocument();
  });
});
