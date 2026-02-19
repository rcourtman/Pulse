import { createSignal, Show } from 'solid-js';
import type { Accessor } from 'solid-js';
import type { PatrolRunRecord } from '@/api/patrol';
import { FindingsPanel } from '@/components/AI/FindingsPanel';
import { renderMarkdown } from '@/components/AI/aiChatUtils';
import { RunToolCallTrace } from './RunToolCallTrace';
import {
  formatDurationMs,
  formatTriggerReason,
  formatScope,
  sanitizeAnalysis,
} from '@/utils/patrolFormat';
import { formatRelativeTime } from '@/utils/format';

import BrainCircuitIcon from 'lucide-solid/icons/brain-circuit';
import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import ServerIcon from 'lucide-solid/icons/server';
import MonitorIcon from 'lucide-solid/icons/monitor';
import BoxIcon from 'lucide-solid/icons/box';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import GlobeIcon from 'lucide-solid/icons/globe';
import DatabaseIcon from 'lucide-solid/icons/database';
import SearchIcon from 'lucide-solid/icons/search';
import WrenchIcon from 'lucide-solid/icons/wrench';
import ClockIcon from 'lucide-solid/icons/clock';
import ZapIcon from 'lucide-solid/icons/zap';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import FilterXIcon from 'lucide-solid/icons/filter-x';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import MailIcon from 'lucide-solid/icons/mail';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

interface PatrolStreamState {
  phase: Accessor<string>;
  currentTool: Accessor<string>;
  tokens: Accessor<number>;
  resynced: Accessor<boolean>;
  resyncReason: Accessor<string>;
  bufferStartSeq: Accessor<number>;
  bufferEndSeq: Accessor<number>;
  outputTruncated: Accessor<boolean>;
  reconnectCount: Accessor<number>;
  isStreaming: Accessor<boolean>;
  errorMessage: Accessor<string>;
}

interface RunHistoryEntryProps {
  run: PatrolRunRecord;
  isLive: boolean;
  patrolStream: PatrolStreamState;
  selected: boolean;
  onSelect: (run: PatrolRunRecord | null) => void;
}

export function RunHistoryEntry(props: RunHistoryEntryProps) {
  const [showRunAnalysis, setShowRunAnalysis] = createSignal(true);

  // Live in-progress entry
  if (props.isLive) {
    const hasError = () => !!props.patrolStream.errorMessage();
    const resyncTitle = () => {
      const reason = props.patrolStream.resyncReason();
      const start = props.patrolStream.bufferStartSeq();
      const end = props.patrolStream.bufferEndSeq();
      const parts: string[] = [];
      if (reason) parts.push(`reason=${reason}`);
      if (start > 0 || end > 0) parts.push(`buffer=${start || '?'}..${end || '?'}`);
      if (props.patrolStream.outputTruncated()) parts.push('output_truncated=true');
      if (props.patrolStream.reconnectCount() > 0) parts.push(`reconnects=${props.patrolStream.reconnectCount()}`);
      return parts.length ? parts.join(' ') : '';
    };
    return (
      <div class={`rounded-md border transition-colors ${hasError()
        ? 'border-red-300 dark:border-red-700 bg-red-50 dark:bg-red-900/20'
        : 'border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20'
        }`}>
        <div class="px-3 py-2">
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <Show when={!hasError()}>
              <span class="relative flex h-2.5 w-2.5">
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
                <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500" />
              </span>
              <span class="font-medium text-blue-800 dark:text-blue-200">Running now</span>
            </Show>
            <Show when={!hasError() && (props.patrolStream.resynced() || props.patrolStream.reconnectCount() > 0)}>
              <span
                title={resyncTitle()}
                class="inline-flex items-center gap-1 text-[10px] font-medium px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300"
              >
                <RefreshCwIcon class="w-3 h-3" />
                <span>
                  {props.patrolStream.resynced()
                    ? (props.patrolStream.resyncReason() === 'buffer_rotated' ? 'Resynced (truncated)' : 'Resynced')
                    : `Reconnected${props.patrolStream.reconnectCount() > 1 ? ` x${props.patrolStream.reconnectCount()}` : ''}`}
                </span>
              </span>
            </Show>
            <Show when={hasError()}>
              <ShieldAlertIcon class="w-3.5 h-3.5 text-red-500" />
              <span class="font-medium text-red-800 dark:text-red-200">Error</span>
            </Show>
            <Show when={!hasError() && props.patrolStream.phase()}>
              <span class="text-blue-700 dark:text-blue-300">{props.patrolStream.phase()}</span>
            </Show>
            <Show when={!hasError() && props.patrolStream.currentTool()}>
              <span class="font-mono text-[11px] bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400 px-1.5 py-0.5 rounded">
                {props.patrolStream.currentTool()}
              </span>
            </Show>
            <Show when={!hasError() && props.patrolStream.tokens() > 0}>
              <span class="text-blue-500 dark:text-blue-400 ml-auto">
                {props.patrolStream.tokens().toLocaleString()} tokens
              </span>
            </Show>
          </div>
          <Show when={hasError()}>
            <p class="mt-1.5 text-xs text-red-700 dark:text-red-300">
              {props.patrolStream.errorMessage()}
            </p>
          </Show>
        </div>
      </div>
    );
  }

  // Completed run entry
  const run = props.run;
  const scopeSummary = formatScope(run);
  const duration = formatDurationMs(run.duration_ms);

  return (
    <div class={`rounded-md border transition-colors ${props.selected
      ? 'border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20'
      : 'border-slate-200 dark:border-slate-700'
      }`}>
      <button
        type="button"
        onClick={() => props.onSelect(props.selected ? null : run)}
        class={`w-full text-left px-3 py-2 rounded-md transition-colors ${!props.selected ? 'hover:bg-slate-50 dark:hover:bg-slate-700/40' : ''}`}
      >
        <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
          <span class="text-slate-900 dark:text-slate-100 font-medium">
            {formatRelativeTime(run.started_at, { compact: true })}
          </span>
          <span class={`px-1.5 py-0.5 rounded ${run.status === 'critical'
            ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
            : run.status === 'issues_found'
              ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
              : run.status === 'error'
                ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
            }`}>
            {run.status.replace(/_/g, ' ')}
          </span>
          <span>{formatTriggerReason(run.trigger_reason)}</span>
          <Show when={scopeSummary}>
            <span>• {scopeSummary}</span>
          </Show>
          <Show when={duration}>
            <span>• {duration}</span>
          </Show>
          <Show when={run.resources_checked}>
            <span>• {run.resources_checked} resources</span>
          </Show>
          <Show when={run.new_findings}>
            <span>• {run.new_findings} new</span>
          </Show>
          <Show when={run.rejected_findings}>
            <span class="text-slate-400 dark:text-slate-500">• {run.rejected_findings} rejected</span>
          </Show>
        </div>
      </button>

      {/* Inline expansion details */}
      <Show when={props.selected}>
        <div class="px-3 pb-3 border-t border-blue-200 dark:border-blue-800 mt-0">

          {/* Section 1: Narrative Summary */}
          <div class="mt-3 flex items-start gap-2 text-sm text-slate-700 dark:text-slate-200">
            <SparklesIcon class="w-4 h-4 text-blue-500 dark:text-blue-400 mt-0.5 flex-shrink-0" />
            <p>
              {run.resources_checked > 0
                ? <>Scanned <strong>{run.resources_checked}</strong> resource{run.resources_checked !== 1 ? 's' : ''}{' '}
                  {formatDurationMs(run.duration_ms) ? <>in <strong>{formatDurationMs(run.duration_ms)}</strong></> : ''}{' '}
                  {run.tool_call_count > 0 ? <>using <strong>{run.tool_call_count}</strong> tool call{run.tool_call_count !== 1 ? 's' : ''}</> : ''}.{' '}
                </>
                : <>Patrol completed{formatDurationMs(run.duration_ms) ? <> in <strong>{formatDurationMs(run.duration_ms)}</strong></> : ''}.{' '}</>
              }
              {run.new_findings > 0
                ? <>Found <strong>{run.new_findings}</strong> new issue{run.new_findings !== 1 ? 's' : ''}{run.auto_fix_count > 0 ? <>, auto-fixed <strong>{run.auto_fix_count}</strong></> : ''}.</>
                : <span class="text-green-600 dark:text-green-400">All clear — no new issues.</span>
              }
            </p>
          </div>

          {/* Section 2: Resources Scanned */}
          <Show when={run.resources_checked > 0}>
            <div class="mt-3">
              <div class="flex items-center gap-1.5 mb-2">
                <SearchIcon class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
                <span class="text-[10px] font-semibold tracking-wider uppercase text-slate-500 dark:text-slate-400">
                  Resources Scanned ({run.resources_checked})
                </span>
              </div>
              <div class="flex flex-wrap gap-1.5">
                <Show when={run.nodes_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                    <ServerIcon class="w-3 h-3" /> {run.nodes_checked} node{run.nodes_checked !== 1 ? 's' : ''}
                  </span>
                </Show>
                <Show when={run.guests_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">
                    <MonitorIcon class="w-3 h-3" /> {run.guests_checked} VM{run.guests_checked !== 1 ? 's' : ''}
                  </span>
                </Show>
                <Show when={run.docker_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-cyan-50 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300">
                    <BoxIcon class="w-3 h-3" /> {run.docker_checked} container{run.docker_checked !== 1 ? 's' : ''}
                  </span>
                </Show>
                <Show when={run.storage_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                    <HardDriveIcon class="w-3 h-3" /> {run.storage_checked} storage
                  </span>
                </Show>
                <Show when={run.hosts_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300">
                    <GlobeIcon class="w-3 h-3" /> {run.hosts_checked} host{run.hosts_checked !== 1 ? 's' : ''}
                  </span>
                </Show>
                <Show when={run.pbs_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300">
                    <DatabaseIcon class="w-3 h-3" /> {run.pbs_checked} PBS
                  </span>
                </Show>
                <Show when={run.pmg_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300">
                    <MailIcon class="w-3 h-3" /> {run.pmg_checked} PMG
                  </span>
                </Show>
                <Show when={run.kubernetes_checked > 0}>
                  <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
                    <ActivityIcon class="w-3 h-3" /> {run.kubernetes_checked} K8s
                  </span>
                </Show>
              </div>
            </div>
          </Show>

          {/* Section 3: Outcomes */}
          <div class="mt-3">
            <div class="flex items-center gap-1.5 mb-2">
              <ShieldAlertIcon class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
              <span class="text-[10px] font-semibold tracking-wider uppercase text-slate-500 dark:text-slate-400">
                Outcomes
              </span>
            </div>
            <div class="flex flex-wrap gap-1.5">
              <Show when={run.new_findings > 0}>
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300">
                  <AlertTriangleIcon class="w-3 h-3" /> {run.new_findings} new
                </span>
              </Show>
              <Show when={run.existing_findings > 0}>
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300">
                  <ActivityIcon class="w-3 h-3" /> {run.existing_findings} existing
                </span>
              </Show>
              <Show when={run.resolved_findings > 0}>
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300">
                  <CheckCircleIcon class="w-3 h-3" /> {run.resolved_findings} resolved
                </span>
              </Show>
              <Show when={run.auto_fix_count > 0}>
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300">
                  <WrenchIcon class="w-3 h-3" /> {run.auto_fix_count} auto-fixed
                </span>
              </Show>
              <Show when={run.rejected_findings > 0}>
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-slate-50 text-slate-400 dark:bg-slate-800 dark:text-slate-500">
                  <FilterXIcon class="w-3 h-3" /> {run.rejected_findings} rejected
                </span>
              </Show>
              <Show when={run.status === 'healthy' && run.new_findings === 0}>
                <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300">
                  <CheckCircleIcon class="w-3 h-3" /> All clear
                </span>
              </Show>
            </div>
          </div>

          {/* Section 4: AI Effort Bar */}
          <div class="mt-3 flex flex-wrap items-center gap-3 px-3 py-2 rounded-md bg-slate-50 dark:bg-slate-800 text-xs text-slate-500 dark:text-slate-400">
            <Show when={formatDurationMs(run.duration_ms)}>
              <span class="inline-flex items-center gap-1">
                <ClockIcon class="w-3.5 h-3.5" /> {formatDurationMs(run.duration_ms)}
              </span>
            </Show>
            <Show when={run.tool_call_count > 0}>
              <span class="inline-flex items-center gap-1">
                <ZapIcon class="w-3.5 h-3.5" /> {run.tool_call_count} tool call{run.tool_call_count !== 1 ? 's' : ''}
              </span>
            </Show>
            <Show when={(run.input_tokens || 0) + (run.output_tokens || 0) > 0}>
              <span class="inline-flex items-center gap-1">
                <BrainCircuitIcon class="w-3.5 h-3.5" /> {((run.input_tokens || 0) + (run.output_tokens || 0)).toLocaleString()} tokens
              </span>
            </Show>
            <Show when={run.type === 'scoped'}>
              <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400 text-[10px] font-medium">
                {formatScope(run) || 'Scoped'}
              </span>
            </Show>
          </div>

          {/* Section 5: Patrol Analysis */}
          <Show when={run.ai_analysis}>
            <div class="mt-3">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-1.5">
                  <BrainCircuitIcon class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
                  <span class="text-[10px] font-semibold tracking-wider uppercase text-slate-500 dark:text-slate-400">
                    Patrol Analysis
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => setShowRunAnalysis(!showRunAnalysis())}
                  class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                >
                  {showRunAnalysis() ? 'Collapse' : 'Expand'}
                </button>
              </div>
              <Show when={showRunAnalysis()}>
                <div
                  class="mt-2 p-3 rounded bg-white dark:bg-slate-900 text-sm leading-relaxed text-slate-700 dark:text-slate-200 max-h-64 overflow-auto prose prose-sm max-w-none dark:prose-invert prose-headings:text-sm prose-headings:mt-2 prose-headings:mb-1 prose-p:my-1 prose-ul:my-1 prose-li:my-0"
                  // eslint-disable-next-line solid/no-innerhtml
                  innerHTML={renderMarkdown(sanitizeAnalysis(run.ai_analysis))}
                />
              </Show>
            </div>
          </Show>

          {/* Section 6: Tool Call Trace */}
          <RunToolCallTrace runId={run.id} toolCallCount={run.tool_call_count} />

          {/* Section 7: Inline Findings */}
          <Show when={run.finding_ids?.length}>
            <div class="mt-3 pt-3 border-t border-blue-200 dark:border-blue-800">
              <FindingsPanel
                filterFindingIds={run.finding_ids}
                filterOverride="all"
                showControls={false}
              />
            </div>
          </Show>
        </div>
      </Show>
    </div>
  );
}
