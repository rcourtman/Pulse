import { describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@solidjs/testing-library';
import { afterEach } from 'vitest';
import {
  ToolExecutionBlock,
  PendingToolBlock,
  ToolCancellationBlock,
  PendingToolsList,
  ToolExecutionsList,
} from '../ToolExecutionBlock';
import type { ToolExecution, PendingTool } from '../types';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// --- Factories ---

function makeTool(overrides?: Partial<ToolExecution>): ToolExecution {
  return {
    name: 'run_command',
    input: 'uptime',
    output: 'up 42 days',
    success: true,
    ...overrides,
  };
}

function makePending(overrides?: Partial<PendingTool>): PendingTool {
  return {
    id: 'tool-1',
    name: 'run_command',
    input: 'uptime',
    ...overrides,
  };
}

const getToolDetailsTrigger = () => screen.getByTitle('Show tool details');
const FAST_TOOL_SETTLE_TEST_MS = 500;

// ============================================================
// ToolExecutionBlock
// ============================================================

describe('ToolExecutionBlock', () => {
  // --- Tool label mapping ---

  it.each([
    ['run_command', 'cmd'],
    ['pulse_run_command', 'cmd'],
    ['fetch_url', 'fetch'],
    ['pulse_fetch_url', 'fetch'],
    ['get_infrastructure_state', 'infra'],
    ['pulse_get_infrastructure_state', 'infra'],
    ['get_active_alerts', 'alerts'],
    ['pulse_get_active_alerts', 'alerts'],
    ['get_metrics_history', 'metrics'],
    ['pulse_get_metrics_history', 'metrics'],
    ['get_baselines', 'baselines'],
    ['pulse_get_baselines', 'baselines'],
    ['get_patterns', 'patterns'],
    ['pulse_get_patterns', 'patterns'],
    ['get_disk_health', 'disks'],
    ['pulse_get_disk_health', 'disks'],
    ['get_storage', 'storage'],
    ['pulse_get_storage', 'storage'],
    ['pulse_get_storage_config', 'storage cfg'],
    ['get_resource_details', 'resource'],
    ['pulse_get_resource_details', 'resource'],
    ['report_finding', 'finding'],
    ['patrol_report_finding', 'finding'],
  ])('maps tool name "%s" to label "%s"', (name, label) => {
    render(() => <ToolExecutionBlock tool={makeTool({ name })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('strips pulse_ prefix and underscores for unknown tool names', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ name: 'pulse_some_tool' })} />);
    expect(screen.getByText('some tool')).toBeInTheDocument();
  });

  it('truncates fallback label to 12 characters', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ name: 'very_long_unknown_tool_name' })} />);
    expect(screen.getByText('very long un')).toBeInTheDocument();
  });

  // --- Status icon ---

  it('shows completed icon for successful tool', () => {
    const { container } = render(() => <ToolExecutionBlock tool={makeTool({ success: true })} />);
    expect(container.querySelector('svg[aria-label="completed"]')).not.toBeNull();
  });

  it('shows failed icon for failed tool', () => {
    const { container } = render(() => <ToolExecutionBlock tool={makeTool({ success: false })} />);
    expect(container.querySelector('svg[aria-label="failed"]')).not.toBeNull();
  });

  it('applies emerald color class for success', () => {
    const { container } = render(() => <ToolExecutionBlock tool={makeTool({ success: true })} />);
    const icon = container.querySelector('.text-emerald-600');
    expect(icon).not.toBeNull();
  });

  it('applies red color class for failure', () => {
    const { container } = render(() => <ToolExecutionBlock tool={makeTool({ success: false })} />);
    const icon = container.querySelector('.text-red-600');
    expect(icon).not.toBeNull();
  });

  it('shows completed tool duration when stream timing is available', () => {
    render(() => (
      <ToolExecutionBlock tool={makeTool()} startedAt={1_000} completedAt={4_200} />
    ));

    expect(screen.getByLabelText('Tool duration 3s')).toHaveTextContent('3s');
  });

  it('keeps very fast completed tools visible as sub-second work', () => {
    render(() => (
      <ToolExecutionBlock tool={makeTool()} startedAt={1_000} completedAt={1_450} />
    ));

    expect(screen.getByLabelText('Tool duration <1s')).toHaveTextContent('<1s');
  });

  it('briefly presents fresh live fast completions as running before settling', async () => {
    vi.useFakeTimers();

    render(() => (
      <ToolExecutionBlock tool={makeTool()} startedAt={1_000} completedAt={1_040} live />
    ));

    expect(screen.getByLabelText('Assistant tool running')).toBeInTheDocument();
    expect(screen.getByLabelText('running')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.queryByText('completed')).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Tool duration/)).not.toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(FAST_TOOL_SETTLE_TEST_MS);

    expect(screen.queryByLabelText('Assistant tool running')).not.toBeInTheDocument();
    expect(screen.getByLabelText('completed')).toBeInTheDocument();
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool duration <1s')).toHaveTextContent('<1s');
  });

  it('does not defer failed fast completions behind a running state', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({ success: false })}
        startedAt={1_000}
        completedAt={1_040}
        live
      />
    ));

    expect(screen.getByLabelText('failed')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.queryByText('running')).not.toBeInTheDocument();
  });

  it('does not invent completed tool duration without a valid start and end', () => {
    render(() => <ToolExecutionBlock tool={makeTool()} startedAt={4_000} completedAt={3_000} />);

    expect(screen.queryByLabelText(/Tool duration/)).not.toBeInTheDocument();
  });

  // --- Input display ---

  it('renders input text', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ input: 'ls -la' })} />);
    expect(screen.getByText('ls -la')).toBeInTheDocument();
  });

  it('truncates input summaries longer than 28 chars', () => {
    const longInput = 'A'.repeat(70);
    render(() => <ToolExecutionBlock tool={makeTool({ input: longInput })} />);
    expect(screen.getByText('A'.repeat(28))).toBeInTheDocument();
  });

  it('summarizes JSON action input without showing raw JSON by default', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'query',
          input: '{"action":"topology","include":"all","summary_only":true}',
          output: '{"summary":{"total_nodes":3}}',
        })}
      />
    ));

    expect(screen.getByText('topology summary')).toBeInTheDocument();
    expect(screen.queryByText(/summary_only/)).not.toBeInTheDocument();
    expect(screen.queryByText(/total_nodes/)).not.toBeInTheDocument();
  });

  it('renders Pulse query search input as a readable action', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_query',
          input: '{"action":"search","query":"prowlarr"}',
          output: '{"matches":[]}',
        })}
      />
    ));

    expect(screen.getByText('search "prowlarr"')).toBeInTheDocument();
    expect(screen.queryByText(/"query"/)).not.toBeInTheDocument();
  });

  it('tolerates structured Pulse query input from persisted transcripts', () => {
    render(() => (
      <ToolExecutionBlock
        tool={
          {
            ...makeTool({
              name: 'pulse_query',
            }),
            input: { action: 'search', query: 'prowlarr' },
            output: { matches: [] },
          } as unknown as ToolExecution
        }
      />
    ));

    expect(screen.getByText('search "prowlarr"')).toBeInTheDocument();
    fireEvent.click(getToolDetailsTrigger());
    expect(screen.getByText(/"action":"search"/)).toBeInTheDocument();
  });

  it('renders Pulse alerts list input as active alerts', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_alerts',
          input: '{"action":"list"}',
          output: '{"alerts":[]}',
        })}
      />
    ));

    expect(screen.getByText('list active alerts')).toBeInTheDocument();
  });

  it('renders Pulse read exec input as a readable activity', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
          output: '42',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev | wc -l');
    expect(screen.queryByText(/"target_host"/)).not.toBeInTheDocument();
  });

  it('wraps completed command summaries instead of truncating the visible action row', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
          output: '42',
        })}
      />
    ));

    const summary = screen
      .getByText('Inspect devices on current resource')
      .closest('[data-testid="tool-input-summary"]');
    expect(summary).toBeInTheDocument();
    expect(summary?.className).toContain('whitespace-pre-wrap');
    expect(summary?.className).toContain('break-words');
    expect(summary?.className).not.toContain('truncate');
  });

  it('renders provider-style Pulse read function input as a readable activity', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input: 'pulse_read(target_host="current_resource", command="ls /dev | wc -l")',
          output: '42',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev | wc -l');
    expect(screen.queryByText(/pulse_read\(/)).not.toBeInTheDocument();
  });

  it('redacts obvious secrets from completed read command previews', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input:
            '{"action":"exec","command":"curl -H \\"Authorization: Bearer secret-token\\" --password hunter2 https://example.local"}',
          output: '200',
        })}
      />
    ));

    const preview = screen.getByLabelText('Tool command');
    expect(preview).toHaveTextContent(
      '$ curl -H "Authorization: Bearer [redacted-secret]" --password [redacted-secret] https://example.local',
    );
    expect(preview).not.toHaveTextContent('secret-token');
    expect(preview).not.toHaveTextContent('hunter2');
  });

  it('renders Pulse read log input as a readable log action', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input: '{"action":"logs","target_host":"jellyfin","source":"systemd","unit":"jellyfin"}',
          output: 'log output',
        })}
      />
    ));

    expect(screen.getByText('logs jellyfin on jellyfin')).toBeInTheDocument();
  });

  it('renders governed command input as the command being run', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_run_command',
          input: '{"target_host":"tower","command":"systemctl restart nginx"}',
          output: 'queued',
        })}
      />
    ));

    expect(screen.getByText('$ systemctl restart nginx on tower')).toBeInTheDocument();
  });

  it('uses structured raw input over backend command display strings', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_run_command',
          input: 'Running: systemctl restart nginx',
          rawInput: '{"target_host":"tower","command":"systemctl restart nginx"}',
          output: 'queued',
        })}
      />
    ));

    expect(screen.getByText('$ systemctl restart nginx on tower')).toBeInTheDocument();
    expect(screen.queryByText('Running: systemctl restart nginx')).not.toBeInTheDocument();
  });

  // --- Output display ---

  it('previews successful short plain-text output while keeping full details available', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ output: 'hello world' })} />);
    expect(screen.getByLabelText('Tool output preview')).toHaveTextContent('hello world');
    expect(screen.queryByLabelText(/Tool output available/)).not.toBeInTheDocument();
    fireEvent.click(getToolDetailsTrigger());
    expect(screen.getByText('Output')).toBeInTheDocument();
    expect(screen.getAllByText('hello world')).toHaveLength(2);
  });

  it('does not preview structured JSON output by default', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_query',
          input: '{"action":"topology"}',
          output: '{"summary":{"total_nodes":3}}',
        })}
      />
    ));

    expect(screen.queryByText(/total_nodes/)).not.toBeInTheDocument();
    expect(screen.getByLabelText('Tool output available: structured output')).toHaveTextContent(
      'structured output',
    );
    expect(getToolDetailsTrigger()).toBeInTheDocument();
  });

  it('keeps successful long plain-text output behind details', () => {
    const { container } = render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          output: ['line 1', 'line 2', 'line 3', 'line 4', 'line 5'].join('\n'),
        })}
      />
    ));

    const text = container.textContent || '';
    expect(screen.queryByLabelText('Tool output preview')).not.toBeInTheDocument();
    expect(text).not.toContain('line 1');
    expect(text).not.toContain('line 5');
    expect(screen.getByLabelText('Tool output available: 5 lines output')).toHaveTextContent(
      '5 lines output',
    );
    expect(getToolDetailsTrigger()).toBeInTheDocument();
  });

  it('summarizes suppressed single-line output by character count', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ output: 'x'.repeat(160) })} />);

    expect(screen.queryByLabelText('Tool output preview')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Tool output available: 160 chars output')).toHaveTextContent(
      '160 chars output',
    );
  });

  it('previews failed plain-text output while keeping full details available', () => {
    render(() => (
      <ToolExecutionBlock tool={makeTool({ success: false, output: 'permission denied' })} />
    ));

    expect(screen.getByLabelText('Tool output preview')).toHaveTextContent('permission denied');
    expect(getToolDetailsTrigger()).toBeInTheDocument();
  });

  it('uses raw streamed input to summarize sparse completed Pulse read tools', () => {
    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input: '{}',
          rawInput:
            '{"action": "exec", "command": "ls /dev | wc -l", "target_host": "current_resource',
          output: '42',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev | wc -l');
    expect(screen.getByLabelText('Tool output preview')).toHaveTextContent('42');
  });

  it('hides output that is only whitespace', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output: '   \n\t  ' })} />
    ));
    // Should not render the output section (no <pre> element)
    expect(container.querySelector('pre')).toBeNull();
  });

  it('hides output containing "not available"', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output: 'data not available yet' })} />
    ));
    expect(container.querySelector('pre')).toBeNull();
  });

  // --- Expand/collapse behavior ---

  it('makes the completed tool row the details trigger when raw input or output is available', () => {
    const output = 'l1\nl2\nl3\nl4\nl5';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    const trigger = getToolDetailsTrigger();
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveAttribute('role', 'button');
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
  });

  it('does not make the row expandable when there are no details', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ input: '', output: '' })} />);
    expect(screen.queryByTitle('Show tool details')).toBeNull();
  });

  it('shows raw input and output when details are opened', async () => {
    const output = 'line1\nline2\nline3\nline4\nline5';
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ input: '{"action":"list"}', output })} />
    ));
    const trigger = getToolDetailsTrigger();
    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(trigger).toHaveAttribute('title', 'Hide tool details');
    expect(screen.getByText('Input')).toBeInTheDocument();
    expect(screen.getByText('Output')).toBeInTheDocument();
    expect(screen.getByText('{"action":"list"}')).toBeInTheDocument();
    const rawDetails = Array.from(container.querySelectorAll('pre'))
      .map((pre) => pre.textContent || '')
      .join('\n');
    expect(rawDetails).toContain('line5');
  });

  it('copies raw tool output from the details panel without closing it', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });
    const output = ['line1', 'line2', 'line3', 'line4', 'line5'].join('\n');

    render(() => <ToolExecutionBlock tool={makeTool({ input: '{"action":"list"}', output })} />);
    const trigger = getToolDetailsTrigger();
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('button', { name: 'Copy tool output' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(output);
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Copied tool output' })).toBeInTheDocument();
    });
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('Output')).toBeInTheDocument();
  });

  it('copies raw tool input from the details panel', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });
    const input = '{"action":"exec","command":"ls /dev | wc -l"}';

    render(() => <ToolExecutionBlock tool={makeTool({ input, output: '42' })} />);
    fireEvent.click(getToolDetailsTrigger());
    fireEvent.click(screen.getByRole('button', { name: 'Copy tool input' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(input);
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Copied tool input' })).toBeInTheDocument();
    });
  });

  it('keeps raw tool details readable without break-all formatting', async () => {
    const output = 'alpha beta gamma delta epsilon';
    const { container } = render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          input: '{"action":"exec","command":"ls /dev | wc -l"}',
          output,
        })}
      />
    ));

    fireEvent.click(getToolDetailsTrigger());
    const detailBlocks = Array.from(container.querySelectorAll('pre'));
    expect(detailBlocks.length).toBeGreaterThanOrEqual(2);
    for (const block of detailBlocks) {
      expect(block.className).toContain('whitespace-pre-wrap');
      expect(block.className).toContain('break-words');
      expect(block.className).not.toContain('break-all');
    }
  });

  it('hides raw details when toggled closed', async () => {
    const output = '{"value":"line1"}';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    const trigger = getToolDetailsTrigger();
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText(/line1/)).not.toBeInTheDocument();
  });

  it('toggles raw details from the keyboard on the completed tool row', async () => {
    render(() => <ToolExecutionBlock tool={makeTool({ output: '{"value":"keyboard output"}' })} />);
    const trigger = getToolDetailsTrigger();
    fireEvent.keyDown(trigger, { key: 'Enter' });
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('{"value":"keyboard output"}')).toBeInTheDocument();

    fireEvent.keyDown(trigger, { key: ' ' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('{"value":"keyboard output"}')).not.toBeInTheDocument();
  });

  // --- Edge cases ---

  it('handles empty output string', () => {
    const { container } = render(() => <ToolExecutionBlock tool={makeTool({ output: '' })} />);
    expect(container.querySelector('pre')).toBeNull();
  });

  it('handles output with only blank lines', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output: '\n\n\n' })} />
    ));
    expect(container.querySelector('pre')).toBeNull();
  });

  it('hides output for multi-line "not available" response', () => {
    const output = 'line1\nnot available\nline3\nline4';
    const { container } = render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    // hasOutput returns false when output contains "not available"
    expect(container.querySelector('pre')).toBeNull();
    expect(getToolDetailsTrigger()).toBeInTheDocument();
  });

  it('does not truncate input summaries at exactly 28 chars', () => {
    const input28 = 'X'.repeat(28);
    render(() => <ToolExecutionBlock tool={makeTool({ input: input28 })} />);
    expect(screen.getByText(input28)).toBeInTheDocument();
  });

  it('truncates input summaries at 29 chars', () => {
    const input29 = 'Y'.repeat(29);
    render(() => <ToolExecutionBlock tool={makeTool({ input: input29 })} />);
    expect(screen.getByText('Y'.repeat(28))).toBeInTheDocument();
  });
});

// ============================================================
// PendingToolBlock
// ============================================================

describe('PendingToolBlock', () => {
  // --- Tool label mapping (same logic) ---

  it.each([
    ['run_command', 'cmd'],
    ['pulse_run_command', 'cmd'],
    ['fetch_url', 'fetch'],
    ['get_infrastructure_state', 'infra'],
    ['get_active_alerts', 'alerts'],
    ['get_metrics_history', 'metrics'],
    ['get_baselines', 'baselines'],
    ['get_patterns', 'patterns'],
    ['get_disk_health', 'disks'],
    ['get_storage', 'storage'],
    ['get_resource_details', 'resource'],
    ['report_finding', 'finding'],
  ])('maps tool name "%s" to label "%s"', (name, label) => {
    render(() => <PendingToolBlock tool={makePending({ name })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('strips pulse_ prefix for unknown names', () => {
    render(() => <PendingToolBlock tool={makePending({ name: 'pulse_custom_op' })} />);
    expect(screen.getByText('custom op')).toBeInTheDocument();
  });

  it('maps pulse_get_storage_config consistently while pending', () => {
    render(() => <PendingToolBlock tool={makePending({ name: 'pulse_get_storage_config' })} />);
    expect(screen.getByText('storage cfg')).toBeInTheDocument();
  });

  // --- Input display ---

  it('renders input text', () => {
    render(() => <PendingToolBlock tool={makePending({ input: 'df -h' })} />);
    expect(screen.getByText('df -h')).toBeInTheDocument();
  });

  it('truncates input summaries longer than 28 chars', () => {
    const longInput = 'B'.repeat(55);
    render(() => <PendingToolBlock tool={makePending({ input: longInput })} />);
    expect(screen.getByText('B'.repeat(28))).toBeInTheDocument();
  });

  it('does not truncate input summaries at exactly 28 chars', () => {
    const input28 = 'C'.repeat(28);
    render(() => <PendingToolBlock tool={makePending({ input: input28 })} />);
    expect(screen.getByText(input28)).toBeInTheDocument();
  });

  it('truncates input summaries at 29 chars', () => {
    const input29 = 'D'.repeat(29);
    render(() => <PendingToolBlock tool={makePending({ input: input29 })} />);
    expect(screen.getByText('D'.repeat(28))).toBeInTheDocument();
  });

  it('summarizes JSON action input without showing raw JSON', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({ name: 'query', input: '{"action":"topology","include":"all"}' })}
      />
    ));

    expect(screen.getByText('topology')).toBeInTheDocument();
    expect(screen.queryByText(/include/)).not.toBeInTheDocument();
  });

  it('renders an action-specific pending label before query arguments arrive', () => {
    render(() => <PendingToolBlock tool={makePending({ name: 'pulse_query', input: '{}' })} />);

    expect(screen.getByText('Preparing query...')).toBeInTheDocument();
    expect(screen.getByText('preparing')).toBeInTheDocument();
    expect(screen.queryByText('request')).not.toBeInTheDocument();
  });

  it('renders pending Pulse query list input as a readable action', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({ name: 'pulse_query', input: '{"action":"list","type":"vms"}' })}
      />
    ));

    expect(screen.getByText('list vms')).toBeInTheDocument();
    expect(screen.queryByText(/"type"/)).not.toBeInTheDocument();
  });

  it('renders pending Pulse read exec input as readable activity', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_read',
          input:
            '{"action":"exec","target_host":"current_resource","command":"lsblk -o NAME,SIZE"}',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ lsblk -o NAME,SIZE');
    expect(screen.queryByText(/"command"/)).not.toBeInTheDocument();
  });

  it('wraps pending command summaries so live tool activity stays readable', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_read',
          input:
            '{"action":"exec","target_host":"current_resource","command":"lsblk -o NAME,SIZE"}',
        })}
      />
    ));

    const summary = screen
      .getByText('Inspect devices on current resource')
      .closest('[data-testid="tool-input-summary"]');
    expect(summary).toBeInTheDocument();
    expect(summary?.className).toContain('whitespace-pre-wrap');
    expect(summary?.className).toContain('break-words');
    expect(summary?.className).not.toContain('truncate');
  });

  it('renders pending provider-style Pulse read function input as readable activity', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_read',
          input: 'pulse_read(target_host="current_resource", command="lsblk -o NAME,SIZE")',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ lsblk -o NAME,SIZE');
    expect(screen.queryByText(/pulse_read\(/)).not.toBeInTheDocument();
  });

  it('uses raw partial provider-style input while pending Pulse read arguments are still streaming', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_read',
          input: '{}',
          rawInput: 'pulse_read(target_host="current_resource", command="ls /dev | wc',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev | wc');
    expect(screen.queryByText(/pulse_read\(/)).not.toBeInTheDocument();
  });

  it('uses raw partial input while pending Pulse read command JSON is still streaming', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_read',
          input: '{}',
          rawInput:
            '{"action": "exec", "command": "ls /dev | wc -l", "target_host": "current_resource',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev | wc -l');
    expect(screen.queryByText(/"target_host"/)).not.toBeInTheDocument();
  });

  it('turns streamed command fragments into readable activity before valid JSON arrives', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_read',
          input: '{}',
          rawInput: '{"action": "exec", "command": "ls /dev |',
        })}
      />
    ));

    expect(screen.getByText('Inspect devices')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev |');
  });

  it('uses structured raw input over pending backend command display strings', () => {
    render(() => (
      <PendingToolBlock
        tool={makePending({
          name: 'pulse_run_command',
          input: 'Running: systemctl restart nginx',
          rawInput: '{"target_host":"tower","command":"systemctl restart nginx"}',
        })}
      />
    ));

    expect(screen.getByText('$ systemctl restart nginx on tower')).toBeInTheDocument();
    expect(screen.queryByText('Running: systemctl restart nginx')).not.toBeInTheDocument();
  });

  it('renders a command-specific pending label before governed command arguments arrive', () => {
    render(() => (
      <PendingToolBlock tool={makePending({ name: 'pulse_run_command', input: '{}' })} />
    ));

    expect(screen.getByText('Writing command...')).toBeInTheDocument();
    expect(screen.getByText('writing')).toBeInTheDocument();
    expect(screen.queryByText('request')).not.toBeInTheDocument();
  });

  it('renders a read-specific pending label before Pulse read arguments arrive', () => {
    render(() => <PendingToolBlock tool={makePending({ name: 'pulse_read', input: '{}' })} />);

    expect(screen.getByText('Preparing read...')).toBeInTheDocument();
    expect(screen.getByText('reading')).toBeInTheDocument();
    expect(screen.queryByText('request')).not.toBeInTheDocument();
  });

  // --- Activity state ---

  it('renders a spinner SVG with animate-spin class while pending', () => {
    const { container } = render(() => <PendingToolBlock tool={makePending()} />);
    const svg = container.querySelector('svg.animate-spin');
    expect(svg).not.toBeNull();
    expect(screen.getByText('pending')).toBeInTheDocument();
  });

  it('renders running status and compact progress text', () => {
    render(() => (
      <PendingToolBlock tool={makePending({ status: 'running', progress: 'Running command.' })} />
    ));

    expect(screen.getByText('running')).toBeInTheDocument();
    const progress = screen.getByText('Running command.');
    expect(progress).toBeInTheDocument();
    expect(progress).toHaveAttribute('title', 'Running command.');
    expect(progress.className).not.toContain('hidden');
    expect(progress.className).toContain('break-words');
    expect(progress.className).not.toContain('truncate');
  });

  it('renders waiting status without a spinner', () => {
    const { container } = render(() => (
      <PendingToolBlock
        tool={makePending({ status: 'waiting', progress: 'Waiting for approval.' })}
      />
    ));

    expect(screen.getByText('waiting')).toBeInTheDocument();
    expect(screen.getByText('Waiting for approval.')).toBeInTheDocument();
    expect(container.querySelector('svg.animate-spin')).toBeNull();
  });
});

describe('ToolCancellationBlock', () => {
  it('renders skipped tool activity with the cancellation reason', () => {
    render(() => (
      <ToolCancellationBlock
        tool={{
          id: 'tool-1',
          name: 'pulse_read',
          input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
          reason: 'current_resource unavailable',
        }}
      />
    ));

    expect(screen.getByRole('status', { name: 'Assistant tool canceled' })).toHaveTextContent(
      'skipped',
    );
    expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ ls /dev | wc -l');
    expect(screen.getByText('current_resource unavailable')).toBeInTheDocument();
  });
});

// ============================================================
// PendingToolsList
// ============================================================

describe('PendingToolsList', () => {
  it('renders all tools when 3 or fewer', () => {
    const tools = [
      makePending({ id: '1', name: 'run_command', input: 'cmd1' }),
      makePending({ id: '2', name: 'fetch_url', input: 'url1' }),
      makePending({ id: '3', name: 'get_storage', input: '{}' }),
    ];
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.getByText('cmd1')).toBeInTheDocument();
    expect(screen.getByText('url1')).toBeInTheDocument();
    expect(screen.getByText('Reading storage...')).toBeInTheDocument();
  });

  it('collapses when more than 3 tools, showing first 2', () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` }),
    );
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.getByText('cmd-0')).toBeInTheDocument();
    expect(screen.getByText('cmd-1')).toBeInTheDocument();
    expect(screen.queryByText('cmd-2')).toBeNull();
    expect(screen.queryByText('cmd-3')).toBeNull();
    expect(screen.queryByText('cmd-4')).toBeNull();
  });

  it('shows "+ N more tools running..." button when collapsed', () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` }),
    );
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.getByText('+ 3 more tools running...')).toBeInTheDocument();
  });

  it('expands all tools when expand button is clicked', async () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` }),
    );
    render(() => <PendingToolsList tools={tools} />);
    fireEvent.click(screen.getByText('+ 3 more tools running...'));
    // All tools should now be visible
    for (let i = 0; i < 5; i++) {
      expect(screen.getByText(`cmd-${i}`)).toBeInTheDocument();
    }
  });

  it('does not show expand button when exactly 3 tools', () => {
    const tools = Array.from({ length: 3 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` }),
    );
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.queryByText(/more tools running/)).toBeNull();
  });

  it('shows correct hidden count for 4 tools', () => {
    const tools = Array.from({ length: 4 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` }),
    );
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.getByText('+ 2 more tools running...')).toBeInTheDocument();
  });

  it('renders without crashing for empty tools array', () => {
    const { container } = render(() => <PendingToolsList tools={[]} />);
    // Should render the wrapper div with no children
    expect(container.firstElementChild).not.toBeNull();
    expect(container.querySelector('svg')).toBeNull();
  });
});

// ============================================================
// ToolExecutionsList
// ============================================================

describe('ToolExecutionsList', () => {
  it('renders all tools when 5 or fewer', () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makeTool({ name: 'run_command', input: `exec-${i}`, output: `out-${i}` }),
    );
    render(() => <ToolExecutionsList tools={tools} />);
    for (let i = 0; i < 5; i++) {
      expect(screen.getByText(`exec-${i}`)).toBeInTheDocument();
    }
  });

  it('collapses when more than 5 tools, showing first 3', () => {
    const tools = Array.from({ length: 8 }, (_, i) =>
      makeTool({ name: 'run_command', input: `exec-${i}`, output: '' }),
    );
    render(() => <ToolExecutionsList tools={tools} />);
    expect(screen.getByText('exec-0')).toBeInTheDocument();
    expect(screen.getByText('exec-1')).toBeInTheDocument();
    expect(screen.getByText('exec-2')).toBeInTheDocument();
    expect(screen.queryByText('exec-3')).toBeNull();
  });

  it('shows expand button with correct stats when collapsed', () => {
    const tools = [
      ...Array.from({ length: 5 }, (_, i) =>
        makeTool({ name: 'run_command', input: `ok-${i}`, output: '', success: true }),
      ),
      ...Array.from({ length: 3 }, (_, i) =>
        makeTool({ name: 'run_command', input: `fail-${i}`, output: '', success: false }),
      ),
    ];
    render(() => <ToolExecutionsList tools={tools} />);
    // 8 total - 3 visible = 5 hidden, 5 success / 3 failed
    expect(screen.getByText(/\+ 5 more tools/)).toBeInTheDocument();
    expect(screen.getByText(/5 ✓/)).toBeInTheDocument();
    expect(screen.getByText(/3 ✗/)).toBeInTheDocument();
  });

  it('expands all tools when expand button is clicked', async () => {
    const tools = Array.from({ length: 7 }, (_, i) =>
      makeTool({ name: 'run_command', input: `exec-${i}`, output: '' }),
    );
    render(() => <ToolExecutionsList tools={tools} />);
    const btn = screen.getByText(/more tools/);
    fireEvent.click(btn);
    for (let i = 0; i < 7; i++) {
      expect(screen.getByText(`exec-${i}`)).toBeInTheDocument();
    }
  });

  it('does not show expand button when exactly 5 tools', () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makeTool({ name: 'run_command', input: `exec-${i}`, output: '' }),
    );
    render(() => <ToolExecutionsList tools={tools} />);
    expect(screen.queryByText(/more tools/)).toBeNull();
  });

  it('counts all-success correctly in stats', () => {
    const tools = Array.from({ length: 6 }, (_, i) =>
      makeTool({ name: 'run_command', input: `s-${i}`, output: '', success: true }),
    );
    render(() => <ToolExecutionsList tools={tools} />);
    expect(screen.getByText(/6 ✓/)).toBeInTheDocument();
    expect(screen.getByText(/0 ✗/)).toBeInTheDocument();
  });

  it('counts all-failure correctly in stats', () => {
    const tools = Array.from({ length: 6 }, (_, i) =>
      makeTool({ name: 'run_command', input: `f-${i}`, output: '', success: false }),
    );
    render(() => <ToolExecutionsList tools={tools} />);
    expect(screen.getByText(/0 ✓/)).toBeInTheDocument();
    expect(screen.getByText(/6 ✗/)).toBeInTheDocument();
  });

  it('renders without crashing for empty tools array', () => {
    const { container } = render(() => <ToolExecutionsList tools={[]} />);
    expect(container.firstElementChild).not.toBeNull();
    expect(screen.queryByText(/more tools/)).toBeNull();
  });
});
