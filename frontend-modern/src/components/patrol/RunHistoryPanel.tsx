import { For, Show } from 'solid-js';
import type { Accessor } from 'solid-js';
import type { PatrolRunRecord } from '@/api/patrol';
import { RunHistoryEntry } from './RunHistoryEntry';

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

interface RunHistoryPanelProps {
 runs: PatrolRunRecord[];
 loading: boolean;
 selectedRun: PatrolRunRecord | null;
 onSelectRun: (run: PatrolRunRecord | null) => void;
 patrolStream: PatrolStreamState;
}

export function RunHistoryPanel(props: RunHistoryPanelProps) {
 return (
 <div class="bg-surface rounded-md border border-border p-4">
 <div class="flex items-center justify-between mb-4">
 <div>
 <h2 class="text-sm font-semibold text-base-content">Patrol Run History</h2>
 <p class="text-xs text-muted">
 Select a run to filter findings to that snapshot
 </p>
 </div>
 <Show when={props.selectedRun}>
 <button
 type="button"
 onClick={() => props.onSelectRun(null)}
 class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
 >
 Clear filter
 </button>
 </Show>
 </div>

 <Show when={props.loading}>
 <div class="text-xs text-muted">Loading run history…</div>
 </Show>

 <Show when={!props.loading && props.runs.length === 0}>
 <div class="text-center py-8">
 <RefreshCwIcon class="w-12 h-12 mx-auto text-slate-300 mb-3" />
 <p class="text-sm text-muted">
 No patrol runs yet. Trigger a run to populate history.
 </p>
 </div>
 </Show>

 <Show when={!props.loading && props.runs.length > 0}>
 <div class="space-y-2">
 <For each={props.runs}>
 {(run) => (
 <RunHistoryEntry
 run={run}
 isLive={run.id ==='__live__'}
                patrolStream={props.patrolStream}
                selected={props.selectedRun?.id === run.id}
                onSelect={props.onSelectRun}
              />
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}
