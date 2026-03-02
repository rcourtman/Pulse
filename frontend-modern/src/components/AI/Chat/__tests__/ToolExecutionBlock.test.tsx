import { describe, expect, it } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@solidjs/testing-library';
import { afterEach } from 'vitest';
import {
  ToolExecutionBlock,
  PendingToolBlock,
  PendingToolsList,
  ToolExecutionsList,
} from '../ToolExecutionBlock';
import type { ToolExecution, PendingTool } from '../types';

afterEach(cleanup);

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
    render(() => (
      <ToolExecutionBlock tool={makeTool({ name: 'very_long_unknown_tool_name' })} />
    ));
    expect(screen.getByText('very long un')).toBeInTheDocument();
  });

  // --- Status icon ---

  it('shows checkmark for successful tool', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ success: true })} />
    ));
    expect(container.textContent).toContain('✓');
  });

  it('shows cross for failed tool', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ success: false })} />
    ));
    expect(container.textContent).toContain('✗');
  });

  it('applies emerald color class for success', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ success: true })} />
    ));
    const icon = container.querySelector('.text-emerald-600');
    expect(icon).not.toBeNull();
  });

  it('applies red color class for failure', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ success: false })} />
    ));
    const icon = container.querySelector('.text-red-600');
    expect(icon).not.toBeNull();
  });

  // --- Input display ---

  it('renders input text', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ input: 'ls -la' })} />);
    expect(screen.getByText('ls -la')).toBeInTheDocument();
  });

  it('truncates input longer than 60 chars', () => {
    const longInput = 'A'.repeat(70);
    render(() => <ToolExecutionBlock tool={makeTool({ input: longInput })} />);
    expect(screen.getByText('A'.repeat(60) + '...')).toBeInTheDocument();
  });

  it('shows "{}" when input is empty', () => {
    render(() => <ToolExecutionBlock tool={makeTool({ input: '' })} />);
    expect(screen.getByText('{}')).toBeInTheDocument();
  });

  // --- Output display ---

  it('shows output when non-empty', () => {
    render(() => (
      <ToolExecutionBlock tool={makeTool({ output: 'hello world' })} />
    ));
    expect(screen.getByText('hello world')).toBeInTheDocument();
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

  it('shows last 3 lines by default when output has more lines', () => {
    const output = 'line1\nline2\nline3\nline4\nline5';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    const pre = screen.getByText(/line5/);
    expect(pre.textContent).toContain('...');
    expect(pre.textContent).toContain('line3');
    expect(pre.textContent).toContain('line4');
    expect(pre.textContent).toContain('line5');
  });

  it('shows all lines when 3 or fewer non-empty lines', () => {
    const output = 'line1\nline2\nline3';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    const pre = screen.getByText(/line1/);
    expect(pre.textContent).not.toContain('...');
  });

  // --- Expand/collapse behavior ---

  it('shows "Show full output" button when output has more than 3 lines', () => {
    const output = 'l1\nl2\nl3\nl4\nl5';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    expect(screen.getByText('Show full output')).toBeInTheDocument();
  });

  it('does not show expand button when output has 3 or fewer lines', () => {
    const output = 'l1\nl2\nl3';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    expect(screen.queryByText('Show full output')).toBeNull();
  });

  it('shows full output when "Show full output" button is clicked', async () => {
    const output = 'line1\nline2\nline3\nline4\nline5';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    const btn = screen.getByText('Show full output');
    fireEvent.click(btn);
    // After expanding, the button should say "Show less"
    expect(screen.getByText('Show less')).toBeInTheDocument();
    // And full output should be visible
    const pre = screen.getByText(/line1/);
    expect(pre.textContent).toContain('line1');
    expect(pre.textContent).toContain('line5');
  });

  it('collapses back when "Show less" is clicked', async () => {
    const output = 'line1\nline2\nline3\nline4\nline5';
    render(() => <ToolExecutionBlock tool={makeTool({ output })} />);
    fireEvent.click(screen.getByText('Show full output'));
    expect(screen.getByText('Show less')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Show less'));
    expect(screen.getByText('Show full output')).toBeInTheDocument();
  });

  it('clicking header row toggles output when has more lines', async () => {
    const output = 'l1\nl2\nl3\nl4\nl5';
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output })} />
    ));
    // The header row has cursor-pointer class when collapsible
    const header = container.querySelector('.cursor-pointer');
    expect(header).not.toBeNull();
    fireEvent.click(header!);
    expect(screen.getByText('Show less')).toBeInTheDocument();
  });

  it('header row is not clickable when output has 3 or fewer lines', () => {
    const output = 'l1\nl2';
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output })} />
    ));
    expect(container.querySelector('.cursor-pointer')).toBeNull();
  });

  // --- Expand chevron icon ---

  it('renders expand chevron SVG when output is collapsible', () => {
    const output = 'l1\nl2\nl3\nl4';
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output })} />
    ));
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
  });

  it('rotates chevron when expanded', async () => {
    const output = 'l1\nl2\nl3\nl4';
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output })} />
    ));
    fireEvent.click(screen.getByText('Show full output'));
    const svg = container.querySelector('svg');
    expect(svg?.classList.contains('rotate-180')).toBe(true);
  });

  // --- Edge cases ---

  it('handles empty output string', () => {
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output: '' })} />
    ));
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
    const { container } = render(() => (
      <ToolExecutionBlock tool={makeTool({ output })} />
    ));
    // hasOutput returns false when output contains "not available"
    expect(container.querySelector('pre')).toBeNull();
    // The expand button should also not appear since output section is hidden
    expect(screen.queryByText('Show full output')).toBeNull();
  });

  it('does not truncate input at exactly 60 chars', () => {
    const input60 = 'X'.repeat(60);
    render(() => <ToolExecutionBlock tool={makeTool({ input: input60 })} />);
    // Exactly 60 chars should NOT be truncated (condition is > 60)
    expect(screen.getByText(input60)).toBeInTheDocument();
  });

  it('truncates input at 61 chars', () => {
    const input61 = 'Y'.repeat(61);
    render(() => <ToolExecutionBlock tool={makeTool({ input: input61 })} />);
    expect(screen.getByText('Y'.repeat(60) + '...')).toBeInTheDocument();
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

  it('falls through to fallback for pulse_get_storage_config (not mapped in pending)', () => {
    // PendingToolBlock does NOT have pulse_get_storage_config mapped unlike ToolExecutionBlock
    // Fallback: strip pulse_, replace underscores, truncate to 12 → "get storage "
    const { container } = render(() => (
      <PendingToolBlock tool={makePending({ name: 'pulse_get_storage_config' })} />
    ));
    // Use container query since getByText trims trailing whitespace
    const label = container.querySelector('span.uppercase');
    expect(label).not.toBeNull();
    expect(label!.textContent).toBe('get storage ');
  });

  // --- Input display ---

  it('renders input text', () => {
    render(() => <PendingToolBlock tool={makePending({ input: 'df -h' })} />);
    expect(screen.getByText('df -h')).toBeInTheDocument();
  });

  it('truncates input longer than 50 chars', () => {
    const longInput = 'B'.repeat(55);
    render(() => <PendingToolBlock tool={makePending({ input: longInput })} />);
    expect(screen.getByText('B'.repeat(50) + '...')).toBeInTheDocument();
  });

  it('does not truncate input at exactly 50 chars', () => {
    const input50 = 'C'.repeat(50);
    render(() => <PendingToolBlock tool={makePending({ input: input50 })} />);
    expect(screen.getByText(input50)).toBeInTheDocument();
  });

  it('truncates input at 51 chars', () => {
    const input51 = 'D'.repeat(51);
    render(() => <PendingToolBlock tool={makePending({ input: input51 })} />);
    expect(screen.getByText('D'.repeat(50) + '...')).toBeInTheDocument();
  });

  // --- Spinner ---

  it('renders a spinner SVG with animate-spin class', () => {
    const { container } = render(() => <PendingToolBlock tool={makePending()} />);
    const svg = container.querySelector('svg.animate-spin');
    expect(svg).not.toBeNull();
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
    expect(screen.getByText('{}')).toBeInTheDocument();
  });

  it('collapses when more than 3 tools, showing first 2', () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` })
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
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` })
    );
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.getByText('+ 3 more tools running...')).toBeInTheDocument();
  });

  it('expands all tools when expand button is clicked', async () => {
    const tools = Array.from({ length: 5 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` })
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
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` })
    );
    render(() => <PendingToolsList tools={tools} />);
    expect(screen.queryByText(/more tools running/)).toBeNull();
  });

  it('shows correct hidden count for 4 tools', () => {
    const tools = Array.from({ length: 4 }, (_, i) =>
      makePending({ id: `t${i}`, name: 'run_command', input: `cmd-${i}` })
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
      makeTool({ name: 'run_command', input: `exec-${i}`, output: `out-${i}` })
    );
    render(() => <ToolExecutionsList tools={tools} />);
    for (let i = 0; i < 5; i++) {
      expect(screen.getByText(`exec-${i}`)).toBeInTheDocument();
    }
  });

  it('collapses when more than 5 tools, showing first 3', () => {
    const tools = Array.from({ length: 8 }, (_, i) =>
      makeTool({ name: 'run_command', input: `exec-${i}`, output: '' })
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
        makeTool({ name: 'run_command', input: `ok-${i}`, output: '', success: true })
      ),
      ...Array.from({ length: 3 }, (_, i) =>
        makeTool({ name: 'run_command', input: `fail-${i}`, output: '', success: false })
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
      makeTool({ name: 'run_command', input: `exec-${i}`, output: '' })
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
      makeTool({ name: 'run_command', input: `exec-${i}`, output: '' })
    );
    render(() => <ToolExecutionsList tools={tools} />);
    expect(screen.queryByText(/more tools/)).toBeNull();
  });

  it('counts all-success correctly in stats', () => {
    const tools = Array.from({ length: 6 }, (_, i) =>
      makeTool({ name: 'run_command', input: `s-${i}`, output: '', success: true })
    );
    render(() => <ToolExecutionsList tools={tools} />);
    expect(screen.getByText(/6 ✓/)).toBeInTheDocument();
    expect(screen.getByText(/0 ✗/)).toBeInTheDocument();
  });

  it('counts all-failure correctly in stats', () => {
    const tools = Array.from({ length: 6 }, (_, i) =>
      makeTool({ name: 'run_command', input: `f-${i}`, output: '', success: false })
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
