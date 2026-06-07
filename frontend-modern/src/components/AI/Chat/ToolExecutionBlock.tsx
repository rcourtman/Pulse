import {
  Component,
  Show,
  createSignal,
  createMemo,
  createEffect,
  onCleanup,
  For,
  createUniqueId,
} from 'solid-js';
import CheckIcon from 'lucide-solid/icons/check';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import ClockIcon from 'lucide-solid/icons/clock';
import CopyIcon from 'lucide-solid/icons/copy';
import LoaderCircleIcon from 'lucide-solid/icons/loader-circle';
import XCircleIcon from 'lucide-solid/icons/x-circle';
import type { ToolExecution, PendingTool } from './types';
import { copyToClipboard } from '@/utils/clipboard';
import { getToolCallResultTextClass } from '@/utils/patrolRunPresentation';
import {
  getToolLabel,
  isPlaceholderToolInputSummary,
  parseToolCommandPreview,
  parseToolInputSummary,
  pendingToolActionLabel,
  pendingToolActionState,
  toolValueText,
} from './toolPresentation';

interface ToolExecutionBlockProps {
  tool: ToolExecution;
  startedAt?: number;
  completedAt?: number;
}

interface ToolInputSummaryProps {
  summary: string;
}

interface ToolCommandPreviewProps {
  preview: string;
}

const hasReadableToolOutput = (output: string) => {
  const trimmed = output.trim();
  return trimmed.length > 0 && !trimmed.toLowerCase().includes('not available');
};

const ansiControlCodePattern = new RegExp(String.raw`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`, 'g');

const stripAnsiControlCodes = (value: string) => value.replace(ansiControlCodePattern, '');

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

const hasBinaryControlCharacters = (value: string) => {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code <= 8 || code === 11 || code === 12 || (code >= 14 && code <= 31)) {
      return true;
    }
  }
  return false;
};

const trimPreviewLine = (line: string, maxLength: number) => {
  const trimmed = line.trimEnd();
  if (trimmed.length <= maxLength) return trimmed;
  return `${trimmed.slice(0, maxLength).trimEnd()}...`;
};

const formatOutputPreview = (output: string, success: boolean) => {
  const normalized = stripAnsiControlCodes(output).replace(/\r\n/g, '\n').trim();
  if (!hasReadableToolOutput(normalized)) return '';
  if (looksLikeStructuredOutput(normalized)) return '';
  if (hasBinaryControlCharacters(normalized)) return '';

  const maxLines = 4;
  const maxLineLength = 120;
  const lines = normalized.split('\n');
  const previewLines = lines.slice(0, maxLines).map((line) => trimPreviewLine(line, maxLineLength));
  const fullPreview = previewLines.join('\n').trim();
  if (!fullPreview) return '';

  const linesTruncated = lines.length > maxLines;
  const charsTruncated = lines
    .slice(0, maxLines)
    .some((line) => line.trimEnd().length > maxLineLength);
  if (success && (linesTruncated || charsTruncated)) return '';
  return linesTruncated || charsTruncated ? `${fullPreview}\n...` : fullPreview;
};

const formatHiddenOutputSummary = (output: string) => {
  const normalized = stripAnsiControlCodes(output).replace(/\r\n/g, '\n').trim();
  if (!hasReadableToolOutput(normalized)) return '';
  if (looksLikeStructuredOutput(normalized)) return 'structured output';
  if (hasBinaryControlCharacters(normalized)) return 'binary output';

  const lines = normalized.split('\n');
  if (lines.length > 1) return `${lines.length} lines output`;

  const characterCount = normalized.length;
  return `${characterCount} ${characterCount === 1 ? 'char' : 'chars'} output`;
};

const formatCompletedToolDuration = (startedAt?: number, completedAt?: number): string => {
  if (!startedAt || !completedAt) return '';
  const durationMs = completedAt - startedAt;
  if (!Number.isFinite(durationMs) || durationMs < 0) return '';
  if (durationMs < 1000) return '<1s';

  const totalSeconds = Math.max(1, Math.round(durationMs / 1000));
  if (totalSeconds < 60) return `${totalSeconds}s`;

  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes < 60) return seconds ? `${minutes}m ${seconds}s` : `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
};

const ToolInputSummary: Component<ToolInputSummaryProps> = (props) => {
  const isShellSummary = createMemo(() => props.summary.trim().startsWith('$ '));
  const className = createMemo(
    () =>
      `mt-1 block whitespace-pre-wrap break-words leading-5 text-base-content ${
        isShellSummary() ? 'font-mono text-[11px]' : 'text-[12px] font-medium'
      }`,
  );

  return (
    <Show
      when={isShellSummary()}
      fallback={
        <span data-testid="tool-input-summary" class={className()}>
          {props.summary}
        </span>
      }
    >
      <code data-testid="tool-input-summary" class={className()}>
        {props.summary}
      </code>
    </Show>
  );
};

const ToolCommandPreview: Component<ToolCommandPreviewProps> = (props) => (
  <code
    data-testid="tool-command-preview"
    aria-label="Tool command"
    class="mt-1 block whitespace-pre-wrap break-words font-mono text-[11px] leading-5 text-muted"
  >
    {props.preview}
  </code>
);

/**
 * ToolExecutionBlock - Displays completed tool executions in a compact terminal-like style.
 */
export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);
  const [copiedDetail, setCopiedDetail] = createSignal<'input' | 'output' | null>(null);
  const detailsId = createUniqueId();
  let copiedResetTimer: number | undefined;

  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const outputText = createMemo(() => toolValueText(props.tool.output));
  const inputSummary = createMemo(() =>
    parseToolInputSummary(inputText(), props.tool.name, props.tool.rawInput),
  );
  const commandPreview = createMemo(() => {
    const preview = parseToolCommandPreview(inputText(), props.tool.name, props.tool.rawInput);
    const summary = inputSummary().trim();
    if (!preview || summary.startsWith('$ ') || preview === summary) return '';
    return preview;
  });
  const outputPreview = createMemo(() => formatOutputPreview(outputText(), props.tool.success));
  const hiddenOutputSummary = createMemo(() =>
    outputPreview() ? '' : formatHiddenOutputSummary(outputText()),
  );
  const hasInput = createMemo(() => inputText().trim().length > 0);
  const hasOutput = createMemo(() => hasReadableToolOutput(outputText()));
  const hasDetails = createMemo(() => hasInput() || hasOutput());
  const durationLabel = createMemo(() =>
    formatCompletedToolDuration(props.startedAt, props.completedAt),
  );

  const statusLabel = () => (props.tool.success ? 'completed' : 'failed');
  const statusPillClass = () =>
    props.tool.success
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300'
      : 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300';
  const summaryControlTitle = () => (showDetails() ? 'Hide tool details' : 'Show tool details');
  const toggleDetails = () => {
    if (!hasDetails()) return;
    setShowDetails(!showDetails());
  };
  const handleSummaryKeyDown = (event: KeyboardEvent) => {
    if (!hasDetails()) return;
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    toggleDetails();
  };
  const copyDetail = async (kind: 'input' | 'output', value: string) => {
    const text = value.trim();
    if (!text) return;
    const copied = await copyToClipboard(text);
    if (!copied) return;
    setCopiedDetail(kind);
    if (copiedResetTimer) window.clearTimeout(copiedResetTimer);
    copiedResetTimer = window.setTimeout(() => setCopiedDetail(null), 1500);
  };
  const detailCopyLabel = (kind: 'input' | 'output') =>
    copiedDetail() === kind ? `Copied tool ${kind}` : `Copy tool ${kind}`;

  onCleanup(() => {
    if (copiedResetTimer) window.clearTimeout(copiedResetTimer);
  });

  return (
    <div class="my-2 overflow-hidden rounded-md border border-border-subtle bg-surface text-[11px] shadow-sm">
      <div
        class={`flex min-w-0 items-start gap-2 px-2.5 py-2 ${
          hasDetails()
            ? 'cursor-pointer transition-colors hover:bg-surface-hover focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:ring-inset'
            : ''
        }`}
        role={hasDetails() ? 'button' : undefined}
        tabIndex={hasDetails() ? 0 : undefined}
        aria-expanded={hasDetails() ? showDetails() : undefined}
        aria-controls={hasDetails() ? detailsId : undefined}
        title={hasDetails() ? summaryControlTitle() : undefined}
        onClick={toggleDetails}
        onKeyDown={handleSummaryKeyDown}
      >
        <div class="pt-0.5">
          <Show
            when={props.tool.success}
            fallback={
              <XCircleIcon
                class={`${getToolCallResultTextClass(props.tool.success)} h-3.5 w-3.5 shrink-0`}
                aria-label={statusLabel()}
              />
            }
          >
            <CheckCircleIcon
              class={`${getToolCallResultTextClass(props.tool.success)} h-3.5 w-3.5 shrink-0`}
              aria-label={statusLabel()}
            />
          </Show>
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <span class="shrink-0 font-mono text-[9px] font-semibold uppercase tracking-wider text-muted">
              {toolLabel()}
            </span>
            <span
              class={`shrink-0 rounded border px-1.5 py-0.5 text-[9px] font-medium ${statusPillClass()}`}
            >
              {statusLabel()}
            </span>
            <Show when={durationLabel()}>
              <span
                class="inline-flex shrink-0 items-center gap-1 text-[10px] font-medium text-muted"
                title="Tool duration"
                aria-label={`Tool duration ${durationLabel()}`}
              >
                <ClockIcon class="h-3 w-3" aria-hidden="true" />
                {durationLabel()}
              </span>
            </Show>
            <Show when={hasDetails()}>
              <ChevronRightIcon
                class={`h-3.5 w-3.5 shrink-0 text-muted transition-transform ${
                  showDetails() ? 'rotate-90' : ''
                }`}
                aria-hidden="true"
              />
            </Show>
            <Show when={hiddenOutputSummary()}>
              <span
                class="shrink-0 rounded border border-border-subtle bg-surface-alt px-1.5 py-0.5 text-[9px] font-medium text-muted"
                title="Open tool details to inspect output"
                aria-label={`Tool output available: ${hiddenOutputSummary()}`}
              >
                {hiddenOutputSummary()}
              </span>
            </Show>
          </div>
          <ToolInputSummary summary={inputSummary()} />
          <Show when={commandPreview()}>
            <ToolCommandPreview preview={commandPreview()} />
          </Show>
        </div>
      </div>

      <Show when={outputPreview()}>
        <pre
          class="border-t border-border-subtle bg-surface-alt px-3 py-2 font-mono text-[11px] leading-5 text-base-content whitespace-pre-wrap break-words"
          aria-label="Tool output preview"
        >
          {outputPreview()}
        </pre>
      </Show>

      <Show when={showDetails() && hasDetails()}>
        <div id={detailsId} class="border-t border-border-subtle px-3 py-2">
          <Show when={hasInput()}>
            <div class="mb-1 flex items-center justify-between gap-2">
              <div class="text-[9px] font-semibold uppercase tracking-wide text-muted">Input</div>
              <button
                type="button"
                class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                onClick={() => void copyDetail('input', inputText())}
                title={detailCopyLabel('input')}
                aria-label={detailCopyLabel('input')}
              >
                <Show
                  when={copiedDetail() === 'input'}
                  fallback={<CopyIcon class="h-3 w-3" aria-hidden="true" />}
                >
                  <CheckIcon class="h-3 w-3 text-emerald-600" aria-hidden="true" />
                </Show>
              </button>
            </div>
            <pre class="mb-2 max-h-36 overflow-auto rounded bg-surface-alt p-2 font-mono text-[10px] leading-5 text-muted whitespace-pre-wrap break-words">
              {inputText().trim()}
            </pre>
          </Show>
          <Show when={hasOutput()}>
            <div class="mb-1 flex items-center justify-between gap-2">
              <div class="text-[9px] font-semibold uppercase tracking-wide text-muted">Output</div>
              <button
                type="button"
                class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                onClick={() => void copyDetail('output', outputText())}
                title={detailCopyLabel('output')}
                aria-label={detailCopyLabel('output')}
              >
                <Show
                  when={copiedDetail() === 'output'}
                  fallback={<CopyIcon class="h-3 w-3" aria-hidden="true" />}
                >
                  <CheckIcon class="h-3 w-3 text-emerald-600" aria-hidden="true" />
                </Show>
              </button>
            </div>
            <pre class="max-h-72 overflow-auto rounded bg-surface-alt p-2 font-mono text-[10px] leading-5 text-muted whitespace-pre-wrap break-words">
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
  const parsedInputSummary = createMemo(() =>
    parseToolInputSummary(inputText(), props.tool.name, props.tool.rawInput),
  );
  const inputSummary = createMemo(() => {
    const summary = parsedInputSummary();
    return isPlaceholderToolInputSummary(summary)
      ? pendingToolActionLabel(props.tool.name)
      : summary;
  });
  const commandPreview = createMemo(() => {
    const preview = parseToolCommandPreview(inputText(), props.tool.name, props.tool.rawInput);
    const summary = inputSummary().trim();
    if (!preview || summary.startsWith('$ ') || preview === summary) return '';
    return preview;
  });
  const status = createMemo(() => props.tool.status || 'pending');
  const [now, setNow] = createSignal(Date.now());
  const statusLabel = createMemo(() => {
    if (status() === 'waiting') return 'waiting';
    if (status() === 'running') return 'running';
    if (isPlaceholderToolInputSummary(parsedInputSummary()))
      return pendingToolActionState(props.tool.name);
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
    <div class="my-1 rounded-md border border-blue-200 bg-blue-50/70 px-2.5 py-2 text-[11px] dark:border-blue-900/60 dark:bg-blue-950/20">
      <div class="flex min-w-0 items-start gap-2">
        <div class="pt-0.5">
          <Show
            when={status() === 'waiting'}
            fallback={<LoaderCircleIcon class={activityIconClass()} aria-label={statusLabel()} />}
          >
            <ClockIcon class={activityIconClass()} aria-label={statusLabel()} />
          </Show>
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <span class="shrink-0 font-mono text-[9px] font-semibold uppercase tracking-wider text-muted">
              {toolLabel()}
            </span>
            <span class="shrink-0 text-[10px] font-medium text-muted">{statusLabel()}</span>
            <Show when={elapsedLabel()}>
              <span class="shrink-0 text-[10px] text-muted">{elapsedLabel()}</span>
            </Show>
          </div>
          <ToolInputSummary summary={inputSummary()} />
          <Show when={commandPreview()}>
            <ToolCommandPreview preview={commandPreview()} />
          </Show>
        </div>
      </div>
      <Show when={progressText()}>
        <div class="mt-1 min-w-0 pl-[calc(0.875rem+0.5rem)] text-[10px] leading-snug text-muted">
          <span class="block whitespace-pre-wrap break-words" title={progressText()}>
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
