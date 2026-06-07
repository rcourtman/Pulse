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
import type { ToolExecution, PendingTool, ToolCancellation } from './types';
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
import { getAssistantFastToolCompletionSettleUntil } from './streamActivityTiming';

interface ToolExecutionBlockProps {
  tool: ToolExecution;
  startedAt?: number;
  completedAt?: number;
  live?: boolean;
  settleUntil?: number;
  compact?: boolean;
}

interface ToolInputSummaryProps {
  summary: string;
}

interface ToolCommandPreviewProps {
  preview: string;
}

interface ToolCancellationBlockProps {
  tool: ToolCancellation;
}

type ToolDetailKind = 'input' | 'output' | 'progress' | 'reason';

interface ToolDetailsPanelProps {
  id: string;
  input?: string;
  output?: string;
  progress?: string;
  reason?: string;
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

const formatOutputPreview = (output: string) => {
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
  <VisibleToolCommandPreview preview={props.preview} />
);

const VisibleToolCommandPreview: Component<ToolCommandPreviewProps> = (props) => {
  const [copied, setCopied] = createSignal(false);
  let copiedResetTimer: number | undefined;
  const copyLabel = () => (copied() ? 'Copied tool command' : 'Copy tool command');
  const commandText = () => props.preview.replace(/^\$\s*/, '').trim() || props.preview.trim();

  const copyCommand = async (event: MouseEvent) => {
    event.stopPropagation();
    const text = commandText();
    if (!text) return;
    const ok = await copyToClipboard(text);
    if (!ok) return;
    setCopied(true);
    if (copiedResetTimer) window.clearTimeout(copiedResetTimer);
    copiedResetTimer = window.setTimeout(() => setCopied(false), 1500);
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    event.stopPropagation();
  };

  onCleanup(() => {
    if (copiedResetTimer) window.clearTimeout(copiedResetTimer);
  });

  return (
    <div class="mt-1 flex min-w-0 items-start gap-1.5">
      <code
        data-testid="tool-command-preview"
        aria-label="Tool command"
        class="min-w-0 flex-1 whitespace-pre-wrap break-words font-mono text-[11px] leading-5 text-muted"
      >
        {props.preview}
      </code>
      <button
        type="button"
        class="mt-0.5 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30"
        onClick={(event) => void copyCommand(event)}
        onMouseDown={(event) => event.stopPropagation()}
        onKeyDown={handleKeyDown}
        title={copyLabel()}
        aria-label={copyLabel()}
      >
        <Show when={copied()} fallback={<CopyIcon class="h-3 w-3" aria-hidden="true" />}>
          <CheckIcon class="h-3 w-3 text-emerald-600" aria-hidden="true" />
        </Show>
      </button>
    </div>
  );
};

const normalizeDetailText = (value?: string) => value?.trim() ?? '';

const toolDetailInputText = (input: string, rawInput?: string) => {
  const text = input.trim();
  const raw = rawInput?.trim() ?? '';
  if (raw && (!text || text === '{}' || text === '[]')) return raw;
  return text || raw;
};

const detailTitle = (kind: ToolDetailKind) => {
  if (kind === 'input') return 'Input';
  if (kind === 'output') return 'Output';
  if (kind === 'progress') return 'Progress';
  return 'Reason';
};

const detailCopyTarget = (kind: ToolDetailKind) => {
  if (kind === 'input') return 'tool input';
  if (kind === 'output') return 'tool output';
  if (kind === 'progress') return 'tool progress';
  return 'skip reason';
};

const detailPreClass = (kind: ToolDetailKind) =>
  `${kind === 'output' ? 'max-h-72' : 'max-h-36'} overflow-auto rounded bg-surface-alt p-2 font-mono text-[10px] leading-5 text-muted whitespace-pre-wrap break-words`;

const ToolDetailsPanel: Component<ToolDetailsPanelProps> = (props) => {
  const [copiedDetail, setCopiedDetail] = createSignal<ToolDetailKind | null>(null);
  let copiedResetTimer: number | undefined;

  const details = createMemo(() => {
    const items: Array<{ kind: ToolDetailKind; text: string }> = [];
    const input = normalizeDetailText(props.input);
    const output = normalizeDetailText(props.output);
    const progress = normalizeDetailText(props.progress);
    const reason = normalizeDetailText(props.reason);

    if (input) items.push({ kind: 'input', text: input });
    if (output) items.push({ kind: 'output', text: output });
    if (progress) items.push({ kind: 'progress', text: progress });
    if (reason) items.push({ kind: 'reason', text: reason });
    return items;
  });

  const detailCopyLabel = (kind: ToolDetailKind) => {
    const target = detailCopyTarget(kind);
    return copiedDetail() === kind ? `Copied ${target}` : `Copy ${target}`;
  };

  const copyDetail = async (kind: ToolDetailKind, value: string) => {
    const text = value.trim();
    if (!text) return;
    const copied = await copyToClipboard(text);
    if (!copied) return;
    setCopiedDetail(kind);
    if (copiedResetTimer) window.clearTimeout(copiedResetTimer);
    copiedResetTimer = window.setTimeout(() => setCopiedDetail(null), 1500);
  };

  onCleanup(() => {
    if (copiedResetTimer) window.clearTimeout(copiedResetTimer);
  });

  return (
    <div id={props.id} class="border-t border-border-subtle px-3 py-2">
      <For each={details()}>
        {(detail) => (
          <div class="mb-2 last:mb-0">
            <div class="mb-1 flex items-center justify-between gap-2">
              <div class="text-[9px] font-semibold uppercase tracking-wide text-muted">
                {detailTitle(detail.kind)}
              </div>
              <button
                type="button"
                class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                onClick={() => void copyDetail(detail.kind, detail.text)}
                title={detailCopyLabel(detail.kind)}
                aria-label={detailCopyLabel(detail.kind)}
              >
                <Show
                  when={copiedDetail() === detail.kind}
                  fallback={<CopyIcon class="h-3 w-3" aria-hidden="true" />}
                >
                  <CheckIcon class="h-3 w-3 text-emerald-600" aria-hidden="true" />
                </Show>
              </button>
            </div>
            <pre class={detailPreClass(detail.kind)}>{detail.text}</pre>
          </div>
        )}
      </For>
    </div>
  );
};

/**
 * ToolExecutionBlock - Displays completed tool executions in a compact terminal-like style.
 */
export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);
  const [settlingFastCompletion, setSettlingFastCompletion] = createSignal(false);
  const detailsId = createUniqueId();

  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const detailInputText = createMemo(() => toolDetailInputText(inputText(), props.tool.rawInput));
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
  const outputPreview = createMemo(() => formatOutputPreview(outputText()));
  const showInlineOutputPreview = createMemo(
    () => props.tool.success === false && outputPreview().length > 0,
  );
  const hiddenOutputSummary = createMemo(() =>
    showInlineOutputPreview() ? '' : formatHiddenOutputSummary(outputText()),
  );
  const hiddenOutputBadgeSummary = createMemo(() =>
    settlingFastCompletion() ? '' : hiddenOutputSummary(),
  );
  const hiddenOutputBadgeLabel = createMemo(() =>
    hiddenOutputBadgeSummary() ? 'output available' : '',
  );
  const hasInput = createMemo(() => detailInputText().trim().length > 0);
  const hasOutput = createMemo(() => hasReadableToolOutput(outputText()));
  const hasDetails = createMemo(() => hasInput() || hasOutput());
  createEffect(() => {
    if (!props.tool.success) {
      setSettlingFastCompletion(false);
      return;
    }

    const now = Date.now();
    const explicitSettleUntil =
      Number.isFinite(props.settleUntil) && (props.settleUntil || 0) > now
        ? props.settleUntil
        : undefined;
    const liveSettleUntil =
      props.live === true
        ? getAssistantFastToolCompletionSettleUntil(props.startedAt, props.completedAt, now)
        : undefined;
    const settleUntil = explicitSettleUntil || liveSettleUntil;
    if (!settleUntil) {
      setSettlingFastCompletion(false);
      return;
    }

    setSettlingFastCompletion(true);
    const timeout = window.setTimeout(() => setSettlingFastCompletion(false), settleUntil - now);
    onCleanup(() => window.clearTimeout(timeout));
  });

  const durationLabel = createMemo(() =>
    settlingFastCompletion() ? '' : formatCompletedToolDuration(props.startedAt, props.completedAt),
  );

  const statusLabel = () =>
    settlingFastCompletion() ? 'running' : props.tool.success ? 'completed' : 'failed';
  const statusPillClass = () =>
    settlingFastCompletion()
      ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-300'
      : props.tool.success
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

  return (
    <Show
      when={!props.compact}
      fallback={
        <div
          class="my-1 inline-flex max-w-full items-start gap-2 rounded-md border border-border-subtle bg-surface-alt px-2.5 py-1.5 text-[11px] text-muted"
          role="status"
          aria-label={
            settlingFastCompletion()
              ? 'Assistant tool running'
              : 'Assistant completed tool activity'
          }
          title={inputSummary()}
        >
          <Show
            when={!settlingFastCompletion()}
            fallback={
              <LoaderCircleIcon
                class="mt-0.5 h-3.5 w-3.5 shrink-0 animate-spin text-blue-500 dark:text-blue-400"
                aria-label="running"
              />
            }
          >
            <CheckCircleIcon
              class="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-300"
              aria-label="completed"
            />
          </Show>
          <span class="shrink-0 font-mono text-[9px] font-semibold uppercase tracking-wider">
            {toolLabel()}
          </span>
          <span class="shrink-0 text-[10px] font-medium">{statusLabel()}</span>
          <span class="min-w-0 truncate text-base-content">{inputSummary()}</span>
          <Show when={durationLabel()}>
            <span class="shrink-0 text-[10px]" aria-label={`Tool duration ${durationLabel()}`}>
              {durationLabel()}
            </span>
          </Show>
          <Show when={hiddenOutputBadgeSummary()}>
            <span
              class="shrink-0 rounded border border-border-subtle bg-surface px-1.5 py-0.5 text-[9px] font-medium"
              title="Open the completed turn to inspect tool output"
              aria-label={`Tool output available: ${hiddenOutputBadgeSummary()}`}
            >
              {hiddenOutputBadgeLabel()}
            </span>
          </Show>
        </div>
      }
    >
    <div
      class="my-2 overflow-hidden rounded-md border border-border-subtle bg-surface text-[11px] shadow-sm"
      role={settlingFastCompletion() ? 'status' : undefined}
      aria-label={settlingFastCompletion() ? 'Assistant tool running' : undefined}
    >
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
            when={!settlingFastCompletion()}
            fallback={
              <LoaderCircleIcon
                class="h-3.5 w-3.5 shrink-0 animate-spin text-blue-500 dark:text-blue-400"
                aria-label="running"
              />
            }
          >
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
            <Show when={hiddenOutputBadgeSummary()}>
              <span
                class="shrink-0 rounded border border-border-subtle bg-surface-alt px-1.5 py-0.5 text-[9px] font-medium text-muted"
                title="Open tool details to inspect output"
                aria-label={`Tool output available: ${hiddenOutputBadgeSummary()}`}
              >
                {hiddenOutputBadgeLabel()}
              </span>
            </Show>
          </div>
          <ToolInputSummary summary={inputSummary()} />
          <Show when={commandPreview()}>
            <ToolCommandPreview preview={commandPreview()} />
          </Show>
        </div>
      </div>

      <Show when={showInlineOutputPreview()}>
        <pre
          class="border-t border-border-subtle bg-surface-alt px-3 py-2 font-mono text-[11px] leading-5 text-base-content whitespace-pre-wrap break-words"
          aria-label="Tool output preview"
        >
          {outputPreview()}
        </pre>
      </Show>

      <Show when={showDetails() && hasDetails()}>
        <ToolDetailsPanel
          id={detailsId}
          input={hasInput() ? detailInputText() : ''}
          output={hasOutput() ? outputText() : ''}
        />
      </Show>
    </div>
    </Show>
  );
};

/**
 * PendingToolBlock - Compact single-line display for running tools
 */
interface PendingToolBlockProps {
  tool: PendingTool;
}

export const PendingToolBlock: Component<PendingToolBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);
  const detailsId = createUniqueId();
  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const detailInputText = createMemo(() => toolDetailInputText(inputText(), props.tool.rawInput));
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
  const hasInput = createMemo(() => detailInputText().trim().length > 0);
  const hasProgress = createMemo(() => progressText().length > 0);
  const hasDetails = createMemo(() => hasInput() || hasProgress());
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
    <div class="my-1 overflow-hidden rounded-md border border-blue-200 bg-blue-50/70 text-[11px] dark:border-blue-900/60 dark:bg-blue-950/20">
      <div
        class={`px-2.5 py-2 ${
          hasDetails()
            ? 'cursor-pointer transition-colors hover:bg-blue-100/60 focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:ring-inset dark:hover:bg-blue-950/30'
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
              <Show when={hasDetails()}>
                <ChevronRightIcon
                  class={`h-3.5 w-3.5 shrink-0 text-muted transition-transform ${
                    showDetails() ? 'rotate-90' : ''
                  }`}
                  aria-hidden="true"
                />
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
      <Show when={showDetails() && hasDetails()}>
        <ToolDetailsPanel
          id={detailsId}
          input={hasInput() ? detailInputText() : ''}
          progress={hasProgress() ? progressText() : ''}
        />
      </Show>
    </div>
  );
};

/**
 * ToolCancellationBlock - Durable row for tool calls skipped by policy/runtime boundaries.
 */
export const ToolCancellationBlock: Component<ToolCancellationBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);
  const detailsId = createUniqueId();
  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const detailInputText = createMemo(() => toolDetailInputText(inputText(), props.tool.rawInput));
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
  const reason = createMemo(() => props.tool.reason?.trim() || 'Skipped before execution');
  const hasInput = createMemo(() => detailInputText().trim().length > 0);
  const hasReason = createMemo(() => reason().trim().length > 0);
  const hasDetails = createMemo(() => hasInput() || hasReason());
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

  return (
    <div
      class="my-1 overflow-hidden rounded-md border border-amber-200 bg-amber-50/70 text-[11px] dark:border-amber-900/60 dark:bg-amber-950/20"
      role="status"
      aria-label="Assistant tool canceled"
      title={reason()}
    >
      <div
        class={`flex min-w-0 items-start gap-2 px-2.5 py-2 ${
          hasDetails()
            ? 'cursor-pointer transition-colors hover:bg-amber-100/60 focus:outline-none focus:ring-2 focus:ring-amber-500/30 focus:ring-inset dark:hover:bg-amber-950/30'
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
          <XCircleIcon
            class="h-3.5 w-3.5 shrink-0 text-amber-600 dark:text-amber-300"
            aria-label="skipped"
          />
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <span class="shrink-0 font-mono text-[9px] font-semibold uppercase tracking-wider text-muted">
              {toolLabel()}
            </span>
            <span class="shrink-0 rounded border border-amber-200 bg-amber-100 px-1.5 py-0.5 text-[9px] font-medium text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200">
              skipped
            </span>
            <Show when={hasDetails()}>
              <ChevronRightIcon
                class={`h-3.5 w-3.5 shrink-0 text-muted transition-transform ${
                  showDetails() ? 'rotate-90' : ''
                }`}
                aria-hidden="true"
              />
            </Show>
          </div>
          <ToolInputSummary summary={inputSummary()} />
          <Show when={commandPreview()}>
            <ToolCommandPreview preview={commandPreview()} />
          </Show>
          <div class="mt-1 min-w-0 text-[10px] leading-snug text-muted">
            <span class="block whitespace-pre-wrap break-words">{reason()}</span>
          </div>
        </div>
      </div>
      <Show when={showDetails() && hasDetails()}>
        <ToolDetailsPanel
          id={detailsId}
          input={hasInput() ? detailInputText() : ''}
          reason={hasReason() ? reason() : ''}
        />
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
