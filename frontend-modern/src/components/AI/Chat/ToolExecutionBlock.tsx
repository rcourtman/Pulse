import { Component, Show, createSignal, createMemo, createEffect, onCleanup, For } from 'solid-js';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import ClockIcon from 'lucide-solid/icons/clock';
import LoaderCircleIcon from 'lucide-solid/icons/loader-circle';
import XCircleIcon from 'lucide-solid/icons/x-circle';
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

const normalizedToolName = (name?: string) => (name || '').trim().replace(/^pulse_/, '');

const escapePartialJSONKey = (key: string) => key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const extractPartialJSONStringField = (rawInput: string, keys: string[]) => {
  const raw = rawInput.trim();
  if (!raw) return '';

  for (const key of keys) {
    const escapedKey = escapePartialJSONKey(key);
    const match = raw.match(new RegExp(`"${escapedKey}"\\s*:\\s*"((?:\\\\.|[^"\\\\])*)`));
    if (match?.[1]) {
      return match[1].replace(/\\"/g, '"').replace(/\\\\/g, '\\').trim();
    }
  }
  return '';
};

const stringField = (record: Record<string, unknown>, keys: string[]) => {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
    if (typeof value === 'number' && Number.isFinite(value)) {
      return String(value);
    }
  }
  return '';
};

const booleanField = (record: Record<string, unknown>, key: string) => record[key] === true;

const inlineValue = (value: string, maxLength = 24) =>
  value
    .replace(/["\r\n]+/g, ' ')
    .trim()
    .substring(0, maxLength);

const targetSuffix = (record: Record<string, unknown>) => {
  const target = stringField(record, ['target_host', 'targetHost', 'resource_id', 'resourceId']);
  if (!target) return '';
  return ` on ${formatIdentifierLabel(target, { maxLength: 18 })}`;
};

const formatPartialRawInputSummary = (rawInput: string | undefined, toolName?: string) => {
  if (!rawInput?.trim()) return '';

  const action = extractPartialJSONStringField(rawInput, ['action', 'type']).toLowerCase();
  const command = extractPartialJSONStringField(rawInput, ['command', 'cmd']);
  const path = extractPartialJSONStringField(rawInput, ['path', 'file', 'file_path', 'filePath']);
  const pattern = extractPartialJSONStringField(rawInput, ['pattern', 'grep', 'grep_pattern', 'grepPattern']);
  const query = extractPartialJSONStringField(rawInput, ['query', 'search', 'name', 'resource_name']);
  const target = extractPartialJSONStringField(rawInput, [
    'target_host',
    'targetHost',
    'resource_id',
    'resourceId',
  ]);
  const record: Record<string, unknown> = {
    ...(action ? { action } : {}),
    ...(command ? { command } : {}),
    ...(path ? { path } : {}),
    ...(pattern ? { pattern } : {}),
    ...(query ? { query } : {}),
    ...(target ? { target_host: target } : {}),
  };
  const tool = normalizedToolName(toolName);

  if ((tool === 'read' || tool === 'run_command' || tool === 'control') && command) {
    return `$ ${inlineValue(command, 64)}${targetSuffix(record)}`;
  }
  if (tool === 'read') {
    if (action === 'file' && path) return `read ${inlineValue(path, 36)}${targetSuffix(record)}`;
    if (action === 'tail' && path) return `tail ${inlineValue(path, 36)}${targetSuffix(record)}`;
    if (action === 'find' && pattern) return `find "${inlineValue(pattern, 28)}"${targetSuffix(record)}`;
    if (action === 'logs') return `read logs${targetSuffix(record)}`;
  }
  if (tool === 'query') {
    if (action === 'search' && query) return `search "${inlineValue(query, 28)}"`;
    if (action === 'list') return 'list resources';
    if (action) return formatIdentifierLabel(action, { maxLength: 28 });
  }
  if (action) return formatIdentifierLabel(action, { maxLength: 28 });
  return 'receiving input';
};

const formatCommandSummary = (record: Record<string, unknown>) => {
  const command = stringField(record, ['command', 'cmd']);
  if (!command) return '';
  return `$ ${inlineValue(command, 64)}${targetSuffix(record)}`;
};

const formatPulseReadInputSummary = (record: Record<string, unknown>) => {
  const action = stringField(record, ['action', 'type']).toLowerCase();
  const path = inlineValue(stringField(record, ['path', 'file', 'file_path', 'filePath']), 36);
  const pattern = inlineValue(
    stringField(record, ['pattern', 'grep', 'grep_pattern', 'grepPattern']),
    28,
  );
  const container = inlineValue(stringField(record, ['container', 'service', 'unit']), 24);
  const source = stringField(record, ['source']).toLowerCase();

  if (action === 'exec') {
    return formatCommandSummary(record) || `run read-only command${targetSuffix(record)}`;
  }
  if (action === 'file') {
    return path ? `read ${path}${targetSuffix(record)}` : `read file${targetSuffix(record)}`;
  }
  if (action === 'tail') {
    return path ? `tail ${path}${targetSuffix(record)}` : `tail file${targetSuffix(record)}`;
  }
  if (action === 'find') {
    if (pattern && path) return `find "${pattern}" in ${path}${targetSuffix(record)}`;
    return pattern
      ? `find "${pattern}"${targetSuffix(record)}`
      : `find files${targetSuffix(record)}`;
  }
  if (action === 'logs') {
    if (container) return `logs ${container}${targetSuffix(record)}`;
    return source
      ? `${formatIdentifierLabel(source, { maxLength: 18 })} logs${targetSuffix(record)}`
      : `read logs${targetSuffix(record)}`;
  }
  return (
    formatCommandSummary(record) || (action ? formatIdentifierLabel(action, { maxLength: 28 }) : '')
  );
};

const formatQueryInputSummary = (record: Record<string, unknown>) => {
  const action = stringField(record, ['action', 'type']).toLowerCase();
  const resourceType = stringField(record, ['resource_type', 'resourceType', 'kind', 'include']);
  const listType = stringField(record, ['type', 'resource_type', 'resourceType', 'kind']);
  const query = inlineValue(stringField(record, ['query', 'search', 'name', 'resource_name']));
  const resourceId = inlineValue(stringField(record, ['resource_id', 'resourceId', 'id', 'vmid']));
  const node = inlineValue(stringField(record, ['node', 'host']));

  switch (action) {
    case 'search':
      return query ? `search "${query}"` : 'search resources';
    case 'list':
      return listType
        ? `list ${formatIdentifierLabel(listType, { maxLength: 22 })}`
        : 'list resources';
    case 'get':
      return resourceId ? `get ${resourceId}` : 'get resource';
    case 'config':
      if (resourceId && node) return `config ${resourceId} on ${node}`;
      return resourceId ? `config ${resourceId}` : 'resource config';
    case 'topology':
      return booleanField(record, 'summary_only') ? 'topology summary' : 'topology';
    case 'health':
      return 'health summary';
    default:
      if (action) return formatIdentifierLabel(action, { maxLength: 28 });
      return resourceType
        ? `query ${formatIdentifierLabel(resourceType, { maxLength: 22 })}`
        : 'query resources';
  }
};

const formatAlertsInputSummary = (record: Record<string, unknown>) => {
  const action = stringField(record, ['action']).toLowerCase();
  const severity = stringField(record, ['severity', 'level']);
  const resource = stringField(record, ['resource', 'resource_id', 'resourceId', 'name']);

  if (action === 'list') {
    return severity
      ? `list ${formatIdentifierLabel(severity, { maxLength: 18 })} alerts`
      : 'list active alerts';
  }
  if (action === 'get') {
    return resource ? `get alert for ${inlineValue(resource, 18)}` : 'get alert';
  }
  return action ? formatIdentifierLabel(action, { maxLength: 28 }) : 'read alerts';
};

const parseToolInputSummary = (input: string, toolName?: string, rawInput?: string) => {
  const trimmed = input.trim();
  if (!trimmed) return '';

  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const record = parsed as Record<string, unknown>;
      if (Object.keys(record).length === 0) {
        return formatPartialRawInputSummary(rawInput, toolName) || 'request';
      }
      const tool = normalizedToolName(toolName);
      if (tool === 'read') {
        return formatPulseReadInputSummary(record) || 'read resource';
      }
      if (tool === 'run_command' || tool === 'control') {
        return formatCommandSummary(record) || 'run command';
      }
      if (tool === 'query') {
        return formatQueryInputSummary(record);
      }
      if (tool === 'alerts') {
        return formatAlertsInputSummary(record);
      }
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

const stripAnsiControlCodes = (value: string) =>
  value.replace(/\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])/g, '');

const looksLikeStructuredOutput = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return false;
  try {
    JSON.parse(trimmed);
    return true;
  } catch {
    return false;
  }
};

const hasBinaryControlCharacters = (value: string) =>
  /[\u0000-\u0008\u000B\u000C\u000E-\u001F]/.test(value);

const trimPreviewLine = (line: string, maxLength: number) => {
  const trimmed = line.trimEnd();
  if (trimmed.length <= maxLength) return trimmed;
  return `${trimmed.slice(0, maxLength).trimEnd()}...`;
};

const formatOutputPreview = (output: string) => {
  const normalized = stripAnsiControlCodes(output).replace(/\r\n/g, '\n').trim();
  if (!hasReadableToolOutput(normalized)) return '';
  if (looksLikeStructuredOutput(normalized)) return '';
  if (hasBinaryControlCharacters(normalized)) return '';

  const maxLines = 4;
  const maxLineLength = 120;
  const lines = normalized.split('\n');
  const previewLines = lines
    .slice(0, maxLines)
    .map((line) => trimPreviewLine(line, maxLineLength));
  const fullPreview = previewLines.join('\n').trim();
  if (!fullPreview) return '';

  const linesTruncated = lines.length > maxLines;
  const charsTruncated = lines
    .slice(0, maxLines)
    .some((line) => line.trimEnd().length > maxLineLength);
  return linesTruncated || charsTruncated ? `${fullPreview}\n...` : fullPreview;
};

const toolValueText = (value: unknown) => {
  if (typeof value === 'string') return value;
  if (value === null || value === undefined) return '';
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
};

/**
 * ToolExecutionBlock - Displays completed tool executions in a compact terminal-like style.
 */
export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);

  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const outputText = createMemo(() => toolValueText(props.tool.output));
  const inputSummary = createMemo(() =>
    parseToolInputSummary(inputText(), props.tool.name, props.tool.rawInput),
  );
  const outputPreview = createMemo(() => formatOutputPreview(outputText()));
  const hasInput = createMemo(() => inputText().trim().length > 0);
  const hasOutput = createMemo(() => hasReadableToolOutput(outputText()));
  const hasDetails = createMemo(() => hasInput() || hasOutput());

  const statusLabel = () => (props.tool.success ? 'completed' : 'failed');

  return (
    <div class="my-1 font-mono text-[11px]">
      <div class="flex items-center gap-1.5 rounded px-2 py-1">
        <Show
          when={props.tool.success}
          fallback={
            <XCircleIcon
              class={`${getToolCallResultTextClass(props.tool.success)} h-3 w-3 shrink-0`}
              aria-label={statusLabel()}
            />
          }
        >
          <CheckCircleIcon
            class={`${getToolCallResultTextClass(props.tool.success)} h-3 w-3 shrink-0`}
            aria-label={statusLabel()}
          />
        </Show>

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

      <Show when={outputPreview()}>
        <pre
          class="ml-[calc(0.75rem+58px)] mr-2 mb-1 max-h-24 overflow-hidden whitespace-pre-wrap break-words rounded bg-surface-alt px-2 py-1 text-[10px] leading-relaxed text-base-content"
          aria-label="Tool output preview"
        >
          {outputPreview()}
        </pre>
      </Show>

      <Show when={showDetails() && hasDetails()}>
        <div class="ml-4 mt-1 mb-2 pl-2 border-l-2 border-border overflow-hidden">
          <Show when={hasInput()}>
            <div class="mb-1 text-[9px] font-semibold uppercase tracking-wide text-muted">
              Input
            </div>
            <pre class="mb-2 max-h-32 overflow-y-auto overflow-x-hidden rounded bg-surface-alt p-2 text-[10px] leading-relaxed text-muted whitespace-pre-wrap break-all">
              {inputText().trim()}
            </pre>
          </Show>
          <Show when={hasOutput()}>
            <div class="mb-1 text-[9px] font-semibold uppercase tracking-wide text-muted">
              Output
            </div>
            <pre class="max-h-64 overflow-y-auto overflow-x-hidden rounded bg-surface-alt p-2 text-[10px] leading-relaxed text-muted whitespace-pre-wrap break-all">
              {outputText().trim()}
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
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const inputSummary = createMemo(() =>
    parseToolInputSummary(inputText(), props.tool.name, props.tool.rawInput),
  );
  const status = createMemo(() => props.tool.status || 'pending');
  const [now, setNow] = createSignal(Date.now());
  const statusLabel = createMemo(() => {
    if (status() === 'waiting') return 'waiting';
    if (status() === 'running') return 'running';
    return 'pending';
  });
  const progressText = createMemo(() => (props.tool.progress || '').trim());
  const activityIconClass = createMemo(() => {
    if (status() === 'waiting') return 'h-3 w-3 shrink-0 text-amber-500 dark:text-amber-300';
    return 'h-3 w-3 shrink-0 animate-spin text-blue-500 dark:text-blue-400';
  });
  createEffect(() => {
    if (!props.tool.startedAt || status() === 'waiting') return;
    setNow(Date.now());
    const interval = window.setInterval(() => setNow(Date.now()), 1000);
    onCleanup(() => window.clearInterval(interval));
  });
  const elapsedLabel = createMemo(() => {
    if (!props.tool.startedAt || status() === 'waiting') return '';
    const elapsedSeconds = Math.max(0, Math.floor((now() - props.tool.startedAt) / 1000));
    return elapsedSeconds >= 2 ? `${elapsedSeconds}s` : '';
  });

  return (
    <div class="my-0.5 font-mono text-[11px] rounded border border-border bg-surface-alt px-2 py-1">
      <div class="flex min-w-0 items-center gap-1.5">
        <Show
          when={status() === 'waiting'}
          fallback={<LoaderCircleIcon class={activityIconClass()} aria-label={statusLabel()} />}
        >
          <ClockIcon class={activityIconClass()} aria-label={statusLabel()} />
        </Show>

        <span class="text-muted uppercase text-[9px] font-medium tracking-wider min-w-[50px]">
          {toolLabel()}
        </span>

        <span class="min-w-0 flex-1 truncate text-base-content">{inputSummary()}</span>
        <span class="shrink-0 text-[10px] text-muted">{statusLabel()}</span>
        <Show when={elapsedLabel()}>
          <span class="shrink-0 text-[10px] text-muted">{elapsedLabel()}</span>
        </Show>
      </div>
      <Show when={progressText()}>
        <div class="mt-0.5 min-w-0 pl-[calc(0.75rem+56px)] text-[10px] leading-snug text-muted">
          <span class="block truncate" title={progressText()}>
            {progressText()}
          </span>
        </div>
      </Show>
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
