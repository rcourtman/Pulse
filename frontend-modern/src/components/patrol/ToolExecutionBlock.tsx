import {
  Component,
  Show,
  createSignal,
  createMemo,
  onCleanup,
  For,
  createUniqueId,
} from 'solid-js';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import XCircleIcon from 'lucide-solid/icons/x-circle';
import { copyToClipboard } from '@/utils/clipboard';
import { ButtonLink, CopyValueButton } from '@/components/shared/Button';
import { getToolCallResultTextClass } from '@/utils/patrolRunPresentation';
import {
  getToolLabel,
  canonicalToolActionURL,
  parseToolCommandPreview,
  parseToolInputSummary,
  toolValueText,
} from './toolPresentation';

/** One completed tool call from a Patrol investigation transcript. */
export interface ToolExecution {
  name: string;
  input: string;
  rawInput?: string;
  output: string;
  success: boolean;
}

interface ToolExecutionBlockProps {
  tool: ToolExecution;
}

interface ToolInputSummaryProps {
  summary: string;
}

interface ToolCommandPreviewProps {
  preview: string;
}

type ToolDetailKind = 'input' | 'output';

interface ToolDetailsPanelProps {
  id: string;
  input?: string;
  output?: string;
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

const ToolInputSummary: Component<ToolInputSummaryProps> = (props) => {
  const isShellSummary = createMemo(() => props.summary.trim().startsWith('$ '));
  const className = createMemo(
    () =>
      `mt-1 block whitespace-pre-wrap wrap-break-word leading-5 text-base-content ${
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

  const copyCommand = async (value: string) => {
    const text = value.trim();
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
        class="min-w-0 flex-1 whitespace-pre-wrap wrap-break-word font-mono text-[11px] leading-5 text-muted"
      >
        {props.preview}
      </code>
      <CopyValueButton
        value={commandText()}
        copied={copied()}
        onCopyValue={copyCommand}
        label={copyLabel()}
        variant="neutral"
        size="sm"
        stopPropagation
        class="mt-0.5"
        onMouseDown={(event) => event.stopPropagation()}
        onKeyDown={handleKeyDown}
      />
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

const detailTitle = (kind: ToolDetailKind) => (kind === 'input' ? 'Input' : 'Output');

const detailCopyTarget = (kind: ToolDetailKind) =>
  kind === 'input' ? 'tool input' : 'tool output';

const detailPreClass = (kind: ToolDetailKind) =>
  `${kind === 'output' ? 'max-h-72' : 'max-h-36'} overflow-auto rounded-sm bg-surface-alt p-2 font-mono text-[10px] leading-5 text-muted whitespace-pre-wrap wrap-break-word`;

const ToolDetailsPanel: Component<ToolDetailsPanelProps> = (props) => {
  const [copiedDetail, setCopiedDetail] = createSignal<ToolDetailKind | null>(null);
  let copiedResetTimer: number | undefined;

  const details = createMemo(() => {
    const items: Array<{ kind: ToolDetailKind; text: string }> = [];
    const input = normalizeDetailText(props.input);
    const output = normalizeDetailText(props.output);

    if (input) items.push({ kind: 'input', text: input });
    if (output) items.push({ kind: 'output', text: output });
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
              <CopyValueButton
                value={detail.text}
                copied={copiedDetail() === detail.kind}
                onCopyValue={(value) => copyDetail(detail.kind, value)}
                label={detailCopyLabel(detail.kind)}
                variant="neutral"
                size="sm"
                stopPropagation
              />
            </div>
            <pre class={detailPreClass(detail.kind)}>{detail.text}</pre>
          </div>
        )}
      </For>
    </div>
  );
};

/**
 * ToolExecutionBlock - Displays one completed tool call in a compact terminal-like style.
 */
export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [showDetails, setShowDetails] = createSignal(false);
  const detailsId = createUniqueId();

  const toolLabel = createMemo(() => getToolLabel(props.tool.name));
  const inputText = createMemo(() => toolValueText(props.tool.input));
  const detailInputText = createMemo(() => toolDetailInputText(inputText(), props.tool.rawInput));
  const outputText = createMemo(() => toolValueText(props.tool.output));
  const actionURL = createMemo(() =>
    canonicalToolActionURL(props.tool.name, props.tool.success, outputText()),
  );
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
  const showInlineOutputPreview = createMemo(() => outputPreview().length > 0);
  const hiddenOutputSummary = createMemo(() =>
    showInlineOutputPreview() ? '' : formatHiddenOutputSummary(outputText()),
  );
  const hasInput = createMemo(() => detailInputText().trim().length > 0);
  const hasOutput = createMemo(() => hasReadableToolOutput(outputText()));
  const hasDetails = createMemo(() => hasInput() || hasOutput());

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

  return (
    <div class="my-2 overflow-hidden rounded-md border border-border-subtle bg-surface text-[11px] shadow-xs">
      <div
        class={`flex min-w-0 items-start gap-2 px-2.5 py-2 ${
          hasDetails()
            ? 'cursor-pointer transition-colors hover:bg-surface-hover focus:outline-hidden focus:ring-2 focus:ring-blue-500/30 focus:ring-inset'
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
              class={`shrink-0 rounded-sm border px-1.5 py-0.5 text-[9px] font-medium ${statusPillClass()}`}
            >
              {statusLabel()}
            </span>
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
                class="shrink-0 rounded-sm border border-border-subtle bg-surface-alt px-1.5 py-0.5 text-[9px] font-medium text-muted"
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

      <Show when={actionURL()}>
        <div class="border-t border-border-subtle px-3 py-2">
          <ButtonLink href={actionURL()} size="sm" variant="secondary">
            Review action
          </ButtonLink>
        </div>
      </Show>

      <Show when={showInlineOutputPreview()}>
        <div class="border-t border-border-subtle bg-surface-alt">
          <div class="px-3 pt-2 text-[9px] font-semibold uppercase tracking-wide text-muted">
            Output preview
          </div>
          <pre
            class="px-3 pb-2 pt-1 font-mono text-[11px] leading-5 text-base-content whitespace-pre-wrap wrap-break-word"
            aria-label="Tool output preview"
          >
            {outputPreview()}
          </pre>
        </div>
      </Show>

      <Show when={showDetails() && hasDetails()}>
        <ToolDetailsPanel
          id={detailsId}
          input={hasInput() ? detailInputText() : ''}
          output={hasOutput() ? outputText() : ''}
        />
      </Show>
    </div>
  );
};
