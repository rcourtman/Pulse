import { Component, Show, createSignal, createMemo, For } from 'solid-js';
import type { ToolExecution, PendingTool } from './types';

interface ToolExecutionBlockProps {
  tool: ToolExecution;
}

/**
 * ToolExecutionBlock - Displays completed tool executions in a compact terminal-like style.
 */
export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [showOutput, setShowOutput] = createSignal(false); // Collapsed by default like Claude Code

  // Get display name for tool
  const toolLabel = createMemo(() => {
    const name = props.tool.name;
    if (name === 'run_command' || name === 'pulse_run_command') return 'cmd';
    if (name === 'fetch_url' || name === 'pulse_fetch_url') return 'fetch';
    if (name === 'get_infrastructure_state' || name === 'pulse_get_infrastructure_state') return 'infra';
    if (name === 'get_active_alerts' || name === 'pulse_get_active_alerts') return 'alerts';
    if (name === 'get_metrics_history' || name === 'pulse_get_metrics_history') return 'metrics';
    if (name === 'get_baselines' || name === 'pulse_get_baselines') return 'baselines';
    if (name === 'get_patterns' || name === 'pulse_get_patterns') return 'patterns';
    if (name === 'get_disk_health' || name === 'pulse_get_disk_health') return 'disks';
    if (name === 'get_storage' || name === 'pulse_get_storage') return 'storage';
    if (name === 'pulse_get_storage_config') return 'storage cfg';
    if (name === 'get_resource_details' || name === 'pulse_get_resource_details') return 'resource';
    if (name.includes('finding')) return 'finding';
    return name.replace(/^pulse_/, '').replace(/_/g, ' ').substring(0, 12);
  });

  // Check if output is non-empty and interesting
  const hasOutput = createMemo(() => {
    const output = props.tool.output || '';
    return output.trim().length > 0 && !output.includes('not available');
  });

  // Show only last few lines by default, full output when expanded
  const displayOutput = createMemo(() => {
    const output = props.tool.output || '';
    if (showOutput()) {
      // Show full output when expanded
      return output;
    }
    // Show last 3 lines by default
    const lines = output.split('\n').filter(line => line.trim());
    if (lines.length <= 3) {
      return output.trim();
    }
    const lastLines = lines.slice(-3).join('\n');
    return '...\n' + lastLines;
  });

  const hasMoreOutput = createMemo(() => {
    const output = props.tool.output || '';
    const lines = output.split('\n').filter(line => line.trim());
    return lines.length > 3;
  });

  const statusIcon = () => props.tool.success ? '✓' : '✗';
  const statusColor = () => props.tool.success
    ? 'text-emerald-600 dark:text-emerald-400'
    : 'text-red-600 dark:text-red-400';

  return (
    <div class="my-1 font-mono text-[11px]">
      {/* Compact single-line header */}
      <div
        class={`flex items-center gap-1.5 px-2 py-1 rounded ${hasMoreOutput() ? 'cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800' : ''
          } ${showOutput() ? 'bg-slate-50 dark:bg-slate-800/50' : ''}`}
        onClick={() => hasMoreOutput() && setShowOutput(!showOutput())}
      >
        {/* Status icon */}
        <span class={`${statusColor()} font-bold`}>{statusIcon()}</span>

        {/* Tool label */}
        <span class="text-slate-500 dark:text-slate-400 uppercase text-[9px] font-medium tracking-wider min-w-[50px]">
          {toolLabel()}
        </span>

        {/* Command/input - truncated */}
        <code class="text-slate-700 dark:text-slate-300 truncate flex-1">
          {(props.tool.input || '').length > 60 ? (props.tool.input || '').substring(0, 60) + '...' : (props.tool.input || '{}')}
        </code>

        {/* Expand indicator if has more output */}
        <Show when={hasMoreOutput()}>
          <svg
            class={`w-3 h-3 text-slate-400 transition-transform ${showOutput() ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </Show>
      </div>

      {/* Output - always show last few lines, expandable for full output */}
      <Show when={hasOutput()}>
        <div class="ml-4 mt-1 mb-2 pl-2 border-l-2 border-slate-200 dark:border-slate-700 overflow-hidden">
          <pre class={`text-[10px] text-slate-600 dark:text-slate-400 whitespace-pre-wrap break-all leading-relaxed overflow-y-auto overflow-x-hidden bg-slate-50 dark:bg-slate-900/50 rounded p-2 ${showOutput() ? 'max-h-64' : 'max-h-20'}`}>
            {displayOutput()}
          </pre>
          <Show when={hasMoreOutput()}>
            <button
              onClick={(e) => { e.stopPropagation(); setShowOutput(!showOutput()); }}
              class="mt-1 text-[9px] text-purple-600 dark:text-purple-400 hover:underline"
            >
              {showOutput() ? 'Show less' : 'Show full output'}
            </button>
          </Show>
        </div>
      </Show>
    </div>
  );
};

/**
 * PendingToolBlock - Compact single-line display for running tools
 */
interface PendingToolBlockProps {
  tool: PendingTool;
}

export const PendingToolBlock: Component<PendingToolBlockProps> = (props) => {
  const toolLabel = createMemo(() => {
    const name = props.tool.name;
    if (name === 'run_command' || name === 'pulse_run_command') return 'cmd';
    if (name === 'fetch_url' || name === 'pulse_fetch_url') return 'fetch';
    if (name === 'get_infrastructure_state' || name === 'pulse_get_infrastructure_state') return 'infra';
    if (name === 'get_active_alerts' || name === 'pulse_get_active_alerts') return 'alerts';
    if (name === 'get_metrics_history' || name === 'pulse_get_metrics_history') return 'metrics';
    if (name === 'get_baselines' || name === 'pulse_get_baselines') return 'baselines';
    if (name === 'get_patterns' || name === 'pulse_get_patterns') return 'patterns';
    if (name === 'get_disk_health' || name === 'pulse_get_disk_health') return 'disks';
    if (name === 'get_storage' || name === 'pulse_get_storage') return 'storage';
    if (name === 'get_resource_details' || name === 'pulse_get_resource_details') return 'resource';
    if (name.includes('finding')) return 'finding';
    return name.replace(/^pulse_/, '').replace(/_/g, ' ').substring(0, 12);
  });

  return (
    <div class="my-0.5 font-mono text-[11px] flex items-center gap-1.5 px-2 py-1 rounded bg-purple-50 dark:bg-purple-900/20">
      {/* Spinner */}
      <svg class="w-3 h-3 text-purple-500 dark:text-purple-400 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
      </svg>

      {/* Tool label */}
      <span class="text-purple-600 dark:text-purple-400 uppercase text-[9px] font-medium tracking-wider min-w-[50px]">
        {toolLabel()}
      </span>

      {/* Command - truncated */}
      <code class="text-purple-700 dark:text-purple-300 truncate flex-1">
        {props.tool.input.length > 50 ? props.tool.input.substring(0, 50) + '...' : props.tool.input}
      </code>
    </div>
  );
};

/**
 * PendingToolsList - Groups multiple pending tools into a compact list
 */
interface PendingToolsListProps {
  tools: PendingTool[];
}

export const PendingToolsList: Component<PendingToolsListProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);

  // If 3 or fewer, show all. Otherwise show collapsed.
  const shouldCollapse = () => props.tools.length > 3;
  const visibleTools = () => {
    if (!shouldCollapse() || expanded()) return props.tools;
    return props.tools.slice(0, 2);
  };
  const hiddenCount = () => props.tools.length - 2;

  return (
    <div class="my-1">
      <For each={visibleTools()}>
        {(tool) => <PendingToolBlock tool={tool} />}
      </For>

      <Show when={shouldCollapse() && !expanded()}>
        <button
          onClick={() => setExpanded(true)}
          class="w-full mt-0.5 py-1 text-[10px] text-purple-600 dark:text-purple-400 hover:bg-purple-50 dark:hover:bg-purple-900/20 rounded text-center font-medium"
        >
          + {hiddenCount()} more tools running...
        </button>
      </Show>
    </div>
  );
};

/**
 * ToolExecutionsList - Compact list for multiple completed tools
 */
interface ToolExecutionsListProps {
  tools: ToolExecution[];
}

export const ToolExecutionsList: Component<ToolExecutionsListProps> = (props) => {
  const [showAll, setShowAll] = createSignal(false);

  // If more than 5 tools, collapse them
  const shouldCollapse = () => props.tools.length > 5;
  const visibleTools = () => {
    if (!shouldCollapse() || showAll()) return props.tools;
    return props.tools.slice(0, 3);
  };
  const hiddenCount = () => props.tools.length - 3;

  // Count successes/failures
  const stats = createMemo(() => {
    const success = props.tools.filter(t => t.success).length;
    const failed = props.tools.length - success;
    return { success, failed };
  });

  return (
    <div class="my-1">
      <For each={visibleTools()}>
        {(tool) => <ToolExecutionBlock tool={tool} />}
      </For>

      <Show when={shouldCollapse() && !showAll()}>
        <button
          onClick={() => setShowAll(true)}
          class="w-full mt-0.5 py-1 text-[10px] text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 rounded text-center font-medium"
        >
          + {hiddenCount()} more tools ({stats().success} ✓ / {stats().failed} ✗)
        </button>
      </Show>
    </div>
  );
};
