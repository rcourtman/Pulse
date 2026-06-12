import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import runToolCallTraceSource from '../RunToolCallTrace.tsx?raw';
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

  it('keeps tool-call result chips on the shared MetadataBadge primitive', () => {
    expect(runToolCallTraceSource).toContain('MetadataBadge');
    expect(runToolCallTraceSource).toContain('RUN_TOOL_CALL_BADGE_PROPS');
    expect(runToolCallTraceSource).toContain('getToolCallResultBadgeTone');
    expect(runToolCallTraceSource).not.toContain('getToolCallResultBadgeClass');
    expect(runToolCallTraceSource).not.toMatch(/px-1\.5 py-0\.5 rounded text-\[10px\] font-medium/);
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

  it('renders the verified column with the right state per tool call', async () => {
    getPatrolRunWithToolCallsMock.mockResolvedValue({
      id: 'run-verify',
      tool_calls: [
        {
          id: 'call-verified',
          tool_name: 'pulse_control',
          input: '{}',
          output: 'restart dispatched',
          success: true,
          duration_ms: 12,
          verification: { status: 'verified', evidenceSummary: 'vm:42 status=running in 8s' },
        },
        {
          id: 'call-failed',
          tool_name: 'pulse_control',
          input: '{}',
          output: 'restart dispatched',
          success: true,
          duration_ms: 15,
          verification: { status: 'failed', evidenceSummary: 'vm:42 still stopped after 2m' },
        },
        {
          id: 'call-unknown',
          tool_name: 'pulse_read',
          input: '{}',
          output: 'ok',
          success: true,
          duration_ms: 9,
        },
        {
          id: 'call-inconclusive',
          tool_name: 'pulse_control',
          input: '{}',
          output: 'restart dispatched',
          success: true,
          duration_ms: 11,
          verification: { status: 'unverified', evidenceSummary: 'agent offline during window' },
        },
      ],
    });

    render(() => <RunToolCallTrace runId="run-verify" toolCallCount={4} />);

    fireEvent.click(screen.getByRole('button', { name: /Tool calls \(4\)/ }));

    const verified = await screen.findByTestId('tool-call-verified-call-verified');
    expect(verified.getAttribute('data-verification-status')).toBe('verified');
    expect(verified.getAttribute('aria-label')).toContain('Verified');
    expect(verified.getAttribute('aria-label')).toContain('vm:42 status=running');

    const failed = await screen.findByTestId('tool-call-verified-call-failed');
    expect(failed.getAttribute('data-verification-status')).toBe('failed');
    expect(failed.getAttribute('aria-label')).toContain('Verification failed');

    const unknown = await screen.findByTestId('tool-call-verified-call-unknown');
    expect(unknown.getAttribute('data-verification-status')).toBe('unknown');
    expect(unknown.getAttribute('aria-label')).toContain('Verification not run');

    const inconclusive = await screen.findByTestId('tool-call-verified-call-inconclusive');
    expect(inconclusive.getAttribute('data-verification-status')).toBe('unverified');
    expect(inconclusive.getAttribute('aria-label')).toContain('inconclusive');
  });
});
