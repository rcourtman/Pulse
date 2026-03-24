import { createResource, createSignal, For, Show } from 'solid-js';
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
import {
  formatMonitoredSystemLatestIncludedSignalSentence,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemLedgerDescription,
  getMonitoredSystemCountingDetailsToggleLabel,
  getMonitoredSystemLedgerPresentation,
} from '@/utils/monitoredSystemPresentation';
import { MonitoredSystemDefinitionDisclosure } from '@/components/Commercial/MonitoredSystemDefinitionDisclosure';

interface MonitoredSystemLedgerPanelProps {
  embedded?: boolean;
}

function usagePercent(total: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((total / limit) * 100));
}

function latestIncludedSignalSummary(system: MonitoredSystemLedgerEntry): {
  relative: string;
  attribution: string;
} | null {
  if (!system.latest_included_signal.at) {
    return null;
  }
  return {
    relative: formatRelativeTime(system.latest_included_signal.at, { compact: true }),
    attribution: formatMonitoredSystemSurfaceAttribution(system.latest_included_signal),
  };
}

export function MonitoredSystemLedgerPanel(props: MonitoredSystemLedgerPanelProps = {}) {
  const [ledger, { refetch }] = createResource(() => MonitoredSystemLedgerAPI.getLedger());
  const [expandedSystemKey, setExpandedSystemKey] = createSignal<string | null>(null);
  const presentation = getMonitoredSystemLedgerPresentation();

  const total = () => ledger()?.total ?? 0;
  const limit = () => ledger()?.limit ?? 0;
  const systems = () => ledger()?.systems ?? [];
  const hasLimit = () => limit() > 0;
  const overLimit = () => hasLimit() && total() > limit();
  const pct = () => usagePercent(total(), limit());
  const systemKey = (system: MonitoredSystemLedgerEntry, index: number) =>
    `${system.name}:${system.type}:${index}`;
  const toggleSystemExplanation = (key: string) => {
    setExpandedSystemKey((current) => (current === key ? null : key));
  };

  const content = (
    <>
      {/* Summary */}
      <div class="space-y-1">
        <div class="flex items-center justify-between">
          <div class="space-y-1">
            <h3 class="text-sm font-semibold text-base-content">{presentation.sectionTitle}</h3>
            <MonitoredSystemDefinitionDisclosure
              buttonClass="text-xs font-medium text-muted underline-offset-2 transition-colors hover:text-base-content hover:underline"
              detailClass="max-w-xl text-xs text-muted"
            />
          </div>
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
          fallback={<p class="text-sm text-muted py-4 text-center">{presentation.emptyState}</p>}
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{presentation.tableNameLabel}</TableHead>
                <TableHead>{presentation.tableStatusLabel}</TableHead>
                <TableHead>{presentation.tableLatestIncludedSignalLabel}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <For each={systems()}>
                {(system: MonitoredSystemLedgerEntry, index) => {
                  const indicator = getSimpleStatusIndicator(system.status);
                  const key = systemKey(system, index());
                  const explanationID = `monitored-system-explanation-${index()}`;
                  const expanded = () => expandedSystemKey() === key;
                  const latestSignal = latestIncludedSignalSummary(system);
                  return (
                    <TableRow>
                      <TableCell>
                        <div class="space-y-1 whitespace-normal">
                          <span class="text-sm font-medium text-base-content">{system.name}</span>
                          <button
                            type="button"
                            class="block text-[11px] font-medium text-muted underline-offset-2 transition-colors hover:text-base-content hover:underline"
                            aria-expanded={expanded()}
                            aria-controls={explanationID}
                            onClick={() => toggleSystemExplanation(key)}
                          >
                            {getMonitoredSystemCountingDetailsToggleLabel(expanded())}
                          </button>
                          <Show when={expanded()}>
                            <div
                              id={explanationID}
                              class="space-y-2 rounded-md border border-border bg-surface px-3 py-2 text-xs text-muted"
                            >
                              <div class="space-y-1">
                                <p class="font-medium text-base-content">
                                  {presentation.currentStatusHeading}
                                </p>
                                <p class="whitespace-normal text-base-content">
                                  {system.status_explanation.summary}
                                </p>
                                <Show when={latestSignal}>
                                  {(signal) => (
                                    <p class="whitespace-normal text-base-content">
                                      {formatMonitoredSystemLatestIncludedSignalSentence(
                                        signal(),
                                      )}
                                    </p>
                                  )}
                                </Show>
                                <Show when={system.status_explanation.reasons.length > 0}>
                                  <ul class="space-y-1 whitespace-normal text-base-content">
                                    <For each={system.status_explanation.reasons}>
                                      {(reason) => <li>{reason.summary}</li>}
                                    </For>
                                  </ul>
                                </Show>
                              </div>
                              <p class="whitespace-normal text-base-content">
                                {system.explanation.summary}
                              </p>
                              <Show when={system.explanation.reasons.length > 0}>
                                <ul class="space-y-1 whitespace-normal">
                                  <For each={system.explanation.reasons}>
                                    {(reason) => <li>{reason.summary}</li>}
                                  </For>
                                </ul>
                              </Show>
                              <Show when={system.explanation.surfaces.length > 0}>
                                <div class="space-y-1">
                                  <p class="font-medium text-base-content">
                                    {presentation.includedCollectionPathsHeading}
                                  </p>
                                  <ul class="space-y-1 whitespace-normal">
                                    <For each={system.explanation.surfaces}>
                                      {(surface) => (
                                        <li>{formatMonitoredSystemSurfaceAttribution(surface)}</li>
                                      )}
                                    </For>
                                  </ul>
                                </div>
                              </Show>
                            </div>
                          </Show>
                        </div>
                      </TableCell>
                      <TableCell>
                        <span class="inline-flex items-center gap-1.5">
                          <StatusDot variant={indicator.variant} size="sm" />
                          <span class="text-xs text-muted">{indicator.label}</span>
                        </span>
                      </TableCell>
                      <TableCell>
                        <Show
                          when={latestSignal}
                          fallback={
                            <span class="text-xs text-muted">
                              {presentation.noIncludedSignalLabel}
                            </span>
                          }
                        >
                          {(signal) => (
                            <div class="space-y-0.5">
                              <span class="block text-xs text-muted">{signal().relative}</span>
                              <span class="block text-[11px] text-muted whitespace-normal">
                                {signal().attribution}
                              </span>
                            </div>
                          )}
                        </Show>
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
      title={presentation.panelTitle}
      description={getMonitoredSystemLedgerDescription()}
      icon={<PulseLogoIcon class="w-5 h-5" />}
      bodyClass="space-y-4"
    >
      {content}
    </SettingsPanel>
  );
}
