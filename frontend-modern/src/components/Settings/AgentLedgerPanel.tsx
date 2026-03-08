import { createResource, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { formatRelativeTime } from '@/utils/format';
import { AgentLedgerAPI } from '@/api/agentLedger';
import type { AgentLedgerEntry } from '@/api/agentLedger';
import { getSimpleStatusIndicator } from '@/utils/status';

function usagePercent(total: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((total / limit) * 100));
}

export function AgentLedgerPanel() {
  const [ledger, { refetch }] = createResource(() => AgentLedgerAPI.getLedger());

  const total = () => ledger()?.total ?? 0;
  const limit = () => ledger()?.limit ?? 0;
  const agents = () => ledger()?.agents ?? [];
  const hasLimit = () => limit() > 0;
  const overLimit = () => hasLimit() && total() > limit();
  const pct = () => usagePercent(total(), limit());

  return (
    <Card padding="lg">
      <div class="space-y-4">
        {/* Summary */}
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-base-content">Installed Agents</h3>
          <Show when={!ledger.error && ledger()}>
            <span
              class="text-sm font-medium"
              classList={{
                'text-base-content': !overLimit(),
                'text-red-600 dark:text-red-400': overLimit(),
              }}
            >
              {total()}
              <Show when={hasLimit()}>{` / ${limit()}`}</Show>
            </span>
          </Show>
        </div>

        {/* Loading state */}
        <Show when={ledger.loading}>
          <p class="text-sm text-muted py-4 text-center">Loading agent ledger...</p>
        </Show>

        {/* Error state */}
        <Show when={ledger.error}>
          <div class="text-sm text-red-600 dark:text-red-400 py-4 text-center">
            <p>Failed to load agent ledger.</p>
            <button
              type="button"
              class="mt-2 text-xs text-primary hover:underline disabled:opacity-50"
              disabled={ledger.loading}
              onClick={() => refetch()}
            >
              {ledger.loading ? 'Retrying\u2026' : 'Retry'}
            </button>
          </div>
        </Show>

        {/* Loaded content */}
        <Show when={!ledger.loading && !ledger.error && ledger()}>
          {/* Progress bar — only shown when there is a limit */}
          <Show when={hasLimit()}>
            <div class="h-2 w-full rounded-full bg-surface-alt overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-300"
                classList={{
                  'bg-blue-500': !overLimit(),
                  'bg-red-500': overLimit(),
                }}
                style={{ width: `${pct()}%` }}
              />
            </div>
          </Show>

          {/* Table */}
          <Show
            when={agents().length > 0}
            fallback={<p class="text-sm text-muted py-4 text-center">No agents installed.</p>}
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Last Seen</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <For each={agents()}>
                  {(agent: AgentLedgerEntry) => {
                    const indicator = getSimpleStatusIndicator(agent.status);
                    return (
                      <TableRow>
                        <TableCell>
                          <span class="text-sm font-medium text-base-content">{agent.name}</span>
                        </TableCell>
                        <TableCell>
                          <span class="inline-flex items-center gap-1.5">
                            <StatusDot variant={indicator.variant} size="sm" />
                            <span class="text-xs text-muted">{indicator.label}</span>
                          </span>
                        </TableCell>
                        <TableCell>
                          <span class="text-xs text-muted">
                            {agent.last_seen
                              ? formatRelativeTime(agent.last_seen, { compact: true })
                              : '—'}
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </Show>
        </Show>
      </div>
    </Card>
  );
}
