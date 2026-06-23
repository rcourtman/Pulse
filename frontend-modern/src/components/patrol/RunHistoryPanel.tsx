import { createMemo, createSignal, For, Show } from 'solid-js';
import type { Accessor } from 'solid-js';
import type { PatrolRunRecord } from '@/api/patrol';
import { EmptyState } from '@/components/shared/EmptyState';
import { getRunHistoryEmptyState } from '@/utils/patrolEmptyStatePresentation';
import {
  getRunHistoryLoadingState,
  getRunHistorySelectionHint,
} from '@/utils/patrolRunPresentation';
import { RunHistoryEntry } from './RunHistoryEntry';

import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

const INITIAL_VISIBLE_RUN_HISTORY_COUNT = 8;

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

interface RunHistoryPanelProps {
  runs: PatrolRunRecord[];
  loading: boolean;
  selectedRun: PatrolRunRecord | null;
  onSelectRun: (run: PatrolRunRecord | null) => void;
  patrolStream: PatrolStreamState;
}

export function RunHistoryPanel(props: RunHistoryPanelProps) {
  const [showOlderRuns, setShowOlderRuns] = createSignal(false);
  const visibleRuns = createMemo(() =>
    showOlderRuns() ? props.runs : props.runs.slice(0, INITIAL_VISIBLE_RUN_HISTORY_COUNT),
  );
  const hiddenRunCount = createMemo(() => Math.max(props.runs.length - visibleRuns().length, 0));

  return (
    <div class="bg-surface rounded-md border border-border p-4">
      <div class="mb-4 flex items-center justify-between gap-3">
        <p class="min-w-0 text-xs text-muted">
          {getRunHistorySelectionHint(props.runs, props.selectedRun)}
        </p>
        <Show when={props.selectedRun}>
          <button
            type="button"
            onClick={() => props.onSelectRun(null)}
            class="shrink-0 text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
          >
            Show all runs
          </button>
        </Show>
      </div>

      <Show when={props.loading}>
        <div class="text-xs text-muted">{getRunHistoryLoadingState()}</div>
      </Show>

      <Show when={!props.loading && props.runs.length === 0}>
        <EmptyState
          variant="panel"
          icon={<RefreshCwIcon class="h-5 w-5" />}
          title={getRunHistoryEmptyState().text}
        />
      </Show>

      <Show when={!props.loading && props.runs.length > 0}>
        <div class="space-y-3">
          <div class="max-h-[28rem] space-y-2 overflow-y-auto pr-1">
            <For each={visibleRuns()}>
              {(run) => (
                <RunHistoryEntry
                  run={run}
                  isLive={run.id === '__live__'}
                  patrolStream={props.patrolStream}
                  selected={props.selectedRun?.id === run.id}
                  onSelect={props.onSelectRun}
                />
              )}
            </For>
          </div>

          <Show when={props.runs.length > INITIAL_VISIBLE_RUN_HISTORY_COUNT}>
            <button
              type="button"
              onClick={() => setShowOlderRuns((expanded) => !expanded)}
              class="w-full rounded-md border border-border-subtle bg-surface-alt px-3 py-2 text-xs font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
            >
              <Show
                when={!showOlderRuns()}
                fallback={`Show recent ${INITIAL_VISIBLE_RUN_HISTORY_COUNT} runs`}
              >
                Show {hiddenRunCount()} older runs
              </Show>
            </button>
          </Show>
        </div>
      </Show>
    </div>
  );
}
