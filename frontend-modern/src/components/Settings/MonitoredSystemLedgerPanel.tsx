import { createResource, For, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
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
import { MonitoredSystemLedgerAPI } from '@/api/monitoredSystemLedger';
import type { MonitoredSystemLedgerEntry } from '@/api/monitoredSystemLedger';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  getMonitoredSystemLedgerErrorState,
  getMonitoredSystemLedgerLoadingState,
} from '@/utils/unifiedAgentInventoryPresentation';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';

interface MonitoredSystemLedgerPanelProps {
  embedded?: boolean;
}

function usagePercent(total: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((total / limit) * 100));
}

export function MonitoredSystemLedgerPanel(props: MonitoredSystemLedgerPanelProps = {}) {
  const [ledger, { refetch }] = createResource(() => MonitoredSystemLedgerAPI.getLedger());

  const total = () => ledger()?.total ?? 0;
  const limit = () => ledger()?.limit ?? 0;
  const systems = () => ledger()?.systems ?? [];
  const hasLimit = () => limit() > 0;
  const overLimit = () => hasLimit() && total() > limit();
  const pct = () => usagePercent(total(), limit());

  const content = (
    <>
      {/* Summary */}
      <div class="space-y-1">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-base-content">Monitored Systems</h3>
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
        <p class="text-xs text-muted">
          Pulse sells monitored systems, not everything underneath them. VMs, containers, pods,
          disks, backups, and other child resources do not count separately.
        </p>
      </div>

      {/* Loading state */}
      <Show when={ledger.loading}>
        <p class="text-sm text-muted py-4 text-center">{getMonitoredSystemLedgerLoadingState().text}</p>
      </Show>

      {/* Error state */}
      <Show when={ledger.error}>
        <div class="text-sm text-red-600 dark:text-red-400 py-4 text-center">
          <p>{getMonitoredSystemLedgerErrorState().title}</p>
          <button
            type="button"
            class="mt-2 text-xs text-primary hover:underline disabled:opacity-50"
            disabled={ledger.loading}
            onClick={() => refetch()}
          >
            {ledger.loading
              ? getMonitoredSystemLedgerErrorState().retryingLabel
              : getMonitoredSystemLedgerErrorState().retryLabel}
          </button>
        </div>
      </Show>

      {/* Loaded content */}
      <Show when={!ledger.loading && !ledger.error && ledger()}>
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

        <Show
          when={systems().length > 0}
          fallback={<p class="text-sm text-muted py-4 text-center">No monitored systems counted.</p>}
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
              <For each={systems()}>
                {(system: MonitoredSystemLedgerEntry) => {
                  const indicator = getSimpleStatusIndicator(system.status);
                  return (
                    <TableRow>
                      <TableCell>
                        <span class="text-sm font-medium text-base-content">{system.name}</span>
                      </TableCell>
                      <TableCell>
                        <span class="inline-flex items-center gap-1.5">
                          <StatusDot variant={indicator.variant} size="sm" />
                          <span class="text-xs text-muted">{indicator.label}</span>
                        </span>
                      </TableCell>
                      <TableCell>
                        <span class="text-xs text-muted">
                          {system.last_seen
                            ? formatRelativeTime(system.last_seen, { compact: true })
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
    </>
  );

  if (props.embedded) {
    return <div class="space-y-4">{content}</div>;
  }

  return (
    <SettingsPanel
      title="Monitored System Ledger"
      description="Review the monitored systems currently counting toward your Pulse Pro allocation."
      icon={<PulseLogoIcon class="w-5 h-5" />}
      bodyClass="space-y-4"
    >
      {content}
    </SettingsPanel>
  );
}
