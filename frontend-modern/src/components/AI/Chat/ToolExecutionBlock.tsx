import { Component, Show, createSignal, createMemo, For } from 'solid-js';
import type { ToolExecution, PendingTool } from './types';
import { getToolCallResultTextClass } from '@/utils/patrolRunPresentation';
import { formatIdentifierLabel } from '@/utils/textPresentation';

interface ToolExecutionBlockProps {
  tool: ToolExecution;
}

const getToolLabel = (name: string) => {
  if (name === 'run_command' || name === 'pulse_run_command') return 'cmd';
  if (name === 'fetch_url' || name === 'pulse_fetch_url') return 'fetch';
  if (name === 'get_infrastructure_state' || name === 'pulse_get_infrastructure_state')
    return 'infra';
  if (name === 'get_active_alerts' || name === 'pulse_get_active_alerts') return 'alerts';
  if (name === 'get_metrics_history' || name === 'pulse_get_metrics_history') return 'metrics';
  if (name === 'get_baselines' || name === 'pulse_get_baselines') return 'baselines';
  if (name === 'get_patterns' || name === 'pulse_get_patterns') return 'patterns';
  if (name === 'get_disk_health' || name === 'pulse_get_disk_health') return 'disks';
  if (name === 'get_storage' || name === 'pulse_get_storage') return 'storage';
  if (name === 'pulse_get_storage_config') return 'storage cfg';
  if (name === 'get_resource_details' || name === 'pulse_get_resource_details') return 'resource';
  if (name.includes('finding')) return 'finding';
  return formatIdentifierLabel(name, { stripPrefix: 'pulse_', maxLength: 12 });
};

const parseToolInputSummary = (input: string) => {
  const trimmed = input.trim();
  if (!trimmed) return '';

  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const record = parsed as Record<string, unknown>;
      if (typeof record.action === 'string' && record.action.trim()) {
        return formatIdentifierLabel(record.action, { maxLength: 28 });
      }
      if (typeof record.command === 'string' && record.command.trim()) {
        return formatIdentifierLabel(record.command, { maxLength: 28 });
      }
      return 'request';
    }
  } catch {
    return formatIdentifierLabel(trimmed, { maxLength: 28 });
  }

  return 'request';
};

const hasReadableToolOutput = (output: string) => {
  const trimmed = output.trim();
  return trimmed.length > 0 && !trimmed.toLowerCase().includes('not available');
};

/**
 * ToolExecutionBlock - Displays completed tool executions in a compact terminal-like style.
 */
export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);

  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputSummary = createMemo(() => parseToolInputSummary(props.tool.input || ''));
  const hasInput = createMemo(() => (props.tool.input || '').trim().length > 0);
  const hasOutput = createMemo(() => hasReadableToolOutput(props.tool.output || ''));
  const hasDetails = createMemo(() => hasInput() || hasOutput());

  const statusIcon = () => (props.tool.success ? '✓' : '✗');
  const statusLabel = () => (props.tool.success ? 'completed' : 'failed');

  return (
    <div class="my-1 font-mono text-[11px]">
      <div class="flex items-center gap-1.5 rounded px-2 py-1">
        <span class={`${getToolCallResultTextClass(props.tool.success)} font-bold`}>
          {statusIcon()}
        </span>

        <span class="text-muted uppercase text-[9px] font-medium tracking-wider min-w-[50px]">
          {toolLabel()}
        </span>

        <span class="min-w-0 flex-1 truncate text-base-content">{inputSummary()}</span>
        <span class="text-[10px] text-muted">{statusLabel()}</span>

        <Show when={hasDetails()}>
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              setShowDetails(!showDetails());
            }}
            class="rounded px-1.5 py-0.5 text-[9px] font-medium text-muted hover:bg-surface-hover hover:text-base-content"
          >
            {showDetails() ? 'Hide details' : 'Details'}
          </button>
        </Show>
      </div>

      <Show when={showDetails() && hasDetails()}>
        <div class="ml-4 mt-1 mb-2 pl-2 border-l-2 border-border overflow-hidden">
          <Show when={hasInput()}>
            <div class="mb-1 text-[9px] font-semibold uppercase tracking-wide text-muted">
              Input
            </div>
            <pre class="mb-2 max-h-32 overflow-y-auto overflow-x-hidden rounded bg-surface-alt p-2 text-[10px] leading-relaxed text-muted whitespace-pre-wrap break-all">
              {(props.tool.input || '').trim()}
            </pre>
          </Show>
          <Show when={hasOutput()}>
            <div class="mb-1 text-[9px] font-semibold uppercase tracking-wide text-muted">
              Output
            </div>
            <pre class="max-h-64 overflow-y-auto overflow-x-hidden rounded bg-surface-alt p-2 text-[10px] leading-relaxed text-muted whitespace-pre-wrap break-all">
              {(props.tool.output || '').trim()}
            </pre>
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
  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputSummary = createMemo(() => parseToolInputSummary(props.tool.input || ''));

  return (
    <div class="my-0.5 font-mono text-[11px] flex items-center gap-1.5 px-2 py-1 rounded bg-surface-alt border border-border">
      {/* Spinner */}
      <svg
        class="w-3 h-3 text-blue-500 dark:text-blue-400 animate-spin"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>

      {/* Tool label */}
      <span class="text-muted uppercase text-[9px] font-medium tracking-wider min-w-[50px]">
        {toolLabel()}
      </span>

      <span class="text-base-content truncate flex-1">{inputSummary()}</span>
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
      <For each={visibleTools()}>{(tool) => <PendingToolBlock tool={tool} />}</For>

      <Show when={shouldCollapse() && !expanded()}>
        <button
          onClick={() => setExpanded(true)}
          class="w-full mt-0.5 py-1 text-[10px] text-muted hover:text-base-content hover:bg-surface-hover rounded text-center font-medium"
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
    const success = props.tools.filter((t) => t.success).length;
    const failed = props.tools.length - success;
    return { success, failed };
  });

  return (
    <div class="my-1">
      <For each={visibleTools()}>{(tool) => <ToolExecutionBlock tool={tool} />}</For>

      <Show when={shouldCollapse() && !showAll()}>
        <button
          onClick={() => setShowAll(true)}
          class="w-full mt-0.5 py-1 text-[10px] text-muted hover:bg-surface-hover rounded text-center font-medium"
        >
          + {hiddenCount()} more tools ({stats().success} ✓ / {stats().failed} ✗)
        </button>
      </Show>
    </div>
  );
};
