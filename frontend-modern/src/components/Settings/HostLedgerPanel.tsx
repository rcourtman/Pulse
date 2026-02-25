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
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { formatRelativeTime } from '@/utils/format';
import { HostLedgerAPI } from '@/api/hostLedger';
import type { HostLedgerEntry } from '@/api/hostLedger';
import type { StatusIndicatorVariant } from '@/utils/status';

function statusVariant(status: string): StatusIndicatorVariant {
  switch (status) {
    case 'online':
      return 'success';
    case 'offline':
      return 'danger';
    default:
      return 'muted';
  }
}

function usagePercent(total: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((total / limit) * 100));
}

function TypeBadge(props: { type: string }) {
  const badge = () => getSourcePlatformBadge(props.type);
  return (
    <Show when={badge()} fallback={<span class="text-xs text-muted">{props.type}</span>}>
      {(b) => <span class={b().classes}>{b().label}</span>}
    </Show>
  );
}

export function HostLedgerPanel() {
  const [ledger] = createResource(() => HostLedgerAPI.getLedger());

  const total = () => ledger()?.total ?? 0;
  const limit = () => ledger()?.limit ?? 0;
  const hosts = () => ledger()?.hosts ?? [];
  const hasLimit = () => limit() > 0;
  const overLimit = () => hasLimit() && total() > limit();
  const pct = () => usagePercent(total(), limit());

  return (
    <Card padding="lg">
      <div class="space-y-4">
        {/* Summary */}
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-base-content">Registered Hosts</h3>
          <Show when={ledger()}>
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
          <p class="text-sm text-muted py-4 text-center">Loading host ledger...</p>
        </Show>

        {/* Error state */}
        <Show when={ledger.error}>
          <p class="text-sm text-red-600 dark:text-red-400 py-4 text-center">
            Failed to load host ledger.
          </p>
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
            when={hosts().length > 0}
            fallback={<p class="text-sm text-muted py-4 text-center">No hosts registered.</p>}
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>First Seen</TableHead>
                  <TableHead>Last Seen</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <For each={hosts()}>
                  {(host: HostLedgerEntry) => (
                    <TableRow>
                      <TableCell>
                        <span class="text-sm font-medium text-base-content">{host.name}</span>
                      </TableCell>
                      <TableCell>
                        <TypeBadge type={host.type} />
                      </TableCell>
                      <TableCell>
                        <span class="text-xs text-muted">{host.source || '—'}</span>
                      </TableCell>
                      <TableCell>
                        <span class="inline-flex items-center gap-1.5">
                          <StatusDot variant={statusVariant(host.status)} size="sm" />
                          <span class="text-xs text-muted capitalize">{host.status}</span>
                        </span>
                      </TableCell>
                      <TableCell>
                        <span class="text-xs text-muted">
                          {host.first_seen
                            ? formatRelativeTime(host.first_seen, { compact: true })
                            : '—'}
                        </span>
                      </TableCell>
                      <TableCell>
                        <span class="text-xs text-muted">
                          {host.last_seen
                            ? formatRelativeTime(host.last_seen, { compact: true })
                            : '—'}
                        </span>
                      </TableCell>
                    </TableRow>
                  )}
                </For>
              </TableBody>
            </Table>
          </Show>
        </Show>
      </div>
    </Card>
  );
}
