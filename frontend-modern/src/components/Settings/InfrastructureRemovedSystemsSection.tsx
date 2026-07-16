import { For, Show, createSignal, type Accessor, type Component } from 'solid-js';
import { ChevronDown, ChevronRight, RotateCcw } from 'lucide-solid';
import { Button } from '@/components/shared/Button';
import {
  getRemovedUnifiedAgentItemLabel,
  getUnifiedAgentLastSeenLabel,
} from '@/utils/unifiedAgentInventoryPresentation';
import { getReconnectActionLabel, type UnifiedAgentRow } from './infrastructureOperationsModel';

interface InfrastructureRemovedSystemsSectionProps {
  rows: Accessor<readonly UnifiedAgentRow[]>;
  readOnly: boolean;
  onAllowReconnect?: (row: UnifiedAgentRow) => Promise<void>;
}

// Removed systems stay out of the main connected-systems table on purpose:
// removal was a deliberate operator action, so these rows are only relevant
// when the user is looking for a machine that vanished (#1581). The band is
// collapsed by default and renders nothing at all when there is nothing
// removed, keeping the healthy-state panel unchanged.
export const InfrastructureRemovedSystemsSection: Component<
  InfrastructureRemovedSystemsSectionProps
> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const [pendingRowKeys, setPendingRowKeys] = createSignal<ReadonlySet<string>>(new Set());

  const rowPending = (row: UnifiedAgentRow) => pendingRowKeys().has(row.rowKey);

  const handleAllowReconnect = async (row: UnifiedAgentRow) => {
    if (!props.onAllowReconnect || rowPending(row)) return;
    setPendingRowKeys((current) => new Set(current).add(row.rowKey));
    try {
      await props.onAllowReconnect(row);
    } finally {
      setPendingRowKeys((current) => {
        const next = new Set(current);
        next.delete(row.rowKey);
        return next;
      });
    }
  };

  return (
    <Show when={props.rows().length > 0}>
      <section aria-label="Removed systems" class="border-t border-border bg-surface-alt/35">
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-3 text-left transition-colors hover:bg-surface-alt/60"
          aria-expanded={expanded()}
          onClick={() => setExpanded((value) => !value)}
        >
          <Show
            when={expanded()}
            fallback={<ChevronRight class="h-4 w-4 flex-shrink-0 text-muted" />}
          >
            <ChevronDown class="h-4 w-4 flex-shrink-0 text-muted" />
          </Show>
          <span class="text-sm font-semibold text-base-content">Removed systems</span>
          <span class="inline-flex items-center rounded-full bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-muted">
            {props.rows().length}
          </span>
        </button>

        <Show when={expanded()}>
          <div class="space-y-3 px-4 pb-4">
            <p class="max-w-4xl text-xs leading-5 text-muted">
              You removed these systems, so Pulse ignores any reports they still send. If the agent
              is still installed and you want the system back, allow it to reconnect. To detach a
              machine completely, uninstall the agent on it.
            </p>

            <div class="space-y-2">
              <For each={props.rows()}>
                {(row) => (
                  <div class="flex flex-col gap-2 rounded-md border border-border-subtle bg-surface px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
                    <div class="min-w-0">
                      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                        <span
                          class="min-w-0 truncate text-[13px] font-medium text-base-content"
                          title={
                            row.hostname && row.hostname !== row.name
                              ? `${row.name} · ${row.hostname}`
                              : row.name
                          }
                        >
                          {row.name}
                        </span>
                        <span class="inline-flex flex-shrink-0 items-center rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-muted whitespace-nowrap">
                          {getRemovedUnifiedAgentItemLabel(row)}
                        </span>
                      </div>
                      <div class="mt-0.5 text-[12px] text-muted">
                        {getUnifiedAgentLastSeenLabel(row, 'Monitoring stopped')}
                      </div>
                    </div>

                    <Show when={!props.readOnly && props.onAllowReconnect}>
                      <Button
                        type="button"
                        variant="outline"
                        size="xs"
                        class="gap-1.5 self-start sm:self-center"
                        onClick={() => void handleAllowReconnect(row)}
                        disabled={rowPending(row)}
                        title="Accept reports from this system again. The installed agent reconnects on its own; nothing is reinstalled."
                      >
                        <RotateCcw class="h-3.5 w-3.5" />
                        {rowPending(row) ? 'Allowing…' : getReconnectActionLabel(row)}
                      </Button>
                    </Show>
                  </div>
                )}
              </For>
            </div>
          </div>
        </Show>
      </section>
    </Show>
  );
};

export default InfrastructureRemovedSystemsSection;
