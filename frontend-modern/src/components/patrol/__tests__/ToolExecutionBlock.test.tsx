import { describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@solidjs/testing-library';
import { afterEach } from 'vitest';
import { ToolExecutionBlock, type ToolExecution } from '../ToolExecutionBlock';
import toolExecutionBlockSource from '../ToolExecutionBlock.tsx?raw';
import { Router, Route } from '@solidjs/router';

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

const getToolDetailsTrigger = () => screen.getByTitle('Show tool details');

// ============================================================
// ToolExecutionBlock
// ============================================================

describe('ToolExecutionBlock', () => {
  it('links canonical action records to the action review', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <ToolExecutionBlock
              tool={makeTool({
                name: 'pulse_query',
                output: JSON.stringify({
                  source: 'canonical_action_audit',
                  action_id: 'act-proof',
                  plan: { actionId: 'act-proof' },
                }),
              })}
            />
          )}
        />
      </Router>
    ));
    const link = screen.getByRole('link', { name: 'Review action' });
    expect(link).toHaveAttribute('href', '/actions?action=act-proof');
  });

  it('keeps tool copy controls on the shared CopyValueButton primitive', () => {
    expect(toolExecutionBlockSource).toContain('@/components/shared/Button');
    expect(toolExecutionBlockSource).toContain('CopyValueButton');
    expect(toolExecutionBlockSource).not.toContain('lucide-solid/icons/copy');
    expect(toolExecutionBlockSource).not.toContain("lucide-solid/icons/check';");
    expect(toolExecutionBlockSource).not.toContain(
      'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-sm border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-hidden focus:ring-2 focus:ring-blue-500/30',
    );
  });

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
    expect(screen.getByLabelText('Tool output preview')).toHaveTextContent('42');
    expect(screen.queryByLabelText(/Tool output available/)).not.toBeInTheDocument();
    expect(screen.queryByText(/"target_host"/)).not.toBeInTheDocument();
  });

  it('copies the sanitized visible command preview without opening raw details', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });

    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_read',
          input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
          output: '42',
        })}
      />
    ));

    const trigger = getToolDetailsTrigger();
    fireEvent.click(screen.getByRole('button', { name: 'Copy tool command' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith('ls /dev | wc -l');
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Copied tool command' })).toBeInTheDocument();
    });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
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
    expect(summary?.className).toContain('wrap-break-word');
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

  it('copies the redacted visible command preview rather than the raw command', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });

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

    fireEvent.click(screen.getByRole('button', { name: 'Copy tool command' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(
        'curl -H "Authorization: Bearer [redacted-secret]" --password [redacted-secret] https://example.local',
      );
    });
    expect(writeText).not.toHaveBeenCalledWith(expect.stringContaining('secret-token'));
    expect(writeText).not.toHaveBeenCalledWith(expect.stringContaining('hunter2'));
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

  it('renders governed command input as readable activity with a copyable command', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });

    render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          name: 'pulse_run_command',
          input: '{"target_host":"tower","command":"systemctl restart nginx"}',
          output: 'queued',
        })}
      />
    ));

    expect(screen.getByText('Run command on tower')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ systemctl restart nginx');
    expect(screen.queryByText(/"target_host"/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Copy tool command' }));
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith('systemctl restart nginx');
    });
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

    expect(screen.getByText('Run command on tower')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool command')).toHaveTextContent('$ systemctl restart nginx');
    expect(screen.queryByText('Running: systemctl restart nginx')).not.toBeInTheDocument();
  });

  // --- Output display ---

  it('previews successful short plain-text output while keeping raw details available', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ output: 'hello world' })} />);
    expect(screen.getByText('Output preview')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool output preview')).toHaveTextContent('hello world');
    expect(screen.queryByLabelText(/Tool output available/)).not.toBeInTheDocument();
    fireEvent.click(getToolDetailsTrigger());
    expect(screen.getByText('Output')).toBeInTheDocument();
    expect(screen.getAllByText('hello world').length).toBeGreaterThanOrEqual(2);
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

  it('previews successful multi-line plain-text output without dumping every line', () => {
    const { container } = render(() => (
      <ToolExecutionBlock
        tool={makeTool({
          output: ['line 1', 'line 2', 'line 3', 'line 4', 'line 5'].join('\n'),
        })}
      />
    ));

    const text = container.textContent || '';
    expect(screen.getByLabelText('Tool output preview')).toHaveTextContent(
      /line 1\s+line 2\s+line 3\s+line 4\s+\.\.\./,
    );
    expect(screen.queryByLabelText(/Tool output available/)).not.toBeInTheDocument();
    expect(text).toContain('line 1');
    expect(text).toContain('line 4');
    expect(text).not.toContain('line 5');
    expect(getToolDetailsTrigger()).toBeInTheDocument();
  });

  it('previews long successful single-line output with truncation', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ output: 'x'.repeat(160) })} />);

    const preview = screen.getByLabelText('Tool output preview');
    expect(preview.textContent?.length).toBeLessThan(160);
    expect(preview).toHaveTextContent('...');
    expect(screen.queryByLabelText(/Tool output available/)).not.toBeInTheDocument();
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
    expect(screen.queryByLabelText(/Tool output available/)).not.toBeInTheDocument();
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
      expect(block.className).toContain('wrap-break-word');
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
