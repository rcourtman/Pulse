import { createMemo, createResource, createSignal, For, Show } from 'solid-js';
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
import type {
  MonitoredSystemLedgerEntry,
  MonitoredSystemLedgerExplanationSurface,
} from '@/api/monitoredSystemLedger';
import type { EntitlementLimitStatus, MonitoredSystemContinuityStatus } from '@/api/license';
import { apiErrorCode, apiErrorDetailField } from '@/api/responseUtils';
import { getSimpleStatusIndicator } from '@/utils/status';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';
import {
  formatMonitoredSystemGroupedSourcesLabel,
  formatMonitoredSystemLedgerUnavailableMessage,
  formatMonitoredSystemLatestIncludedSignalSentence,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemLedgerErrorState,
  getMonitoredSystemLedgerDescription,
  getMonitoredSystemLedgerLoadingState,
  getMonitoredSystemLedgerUnavailableState,
  getMonitoredSystemCountingDetailsToggleLabel,
  getMonitoredSystemLedgerPresentation,
} from '@/utils/monitoredSystemPresentation';
import { MonitoredSystemDefinitionDisclosure } from '@/components/Commercial/MonitoredSystemDefinitionDisclosure';

interface MonitoredSystemLedgerPanelProps {
  embedded?: boolean;
  monitoredSystemContinuity?: MonitoredSystemContinuityStatus | null;
  monitoredSystemLimit?: EntitlementLimitStatus | null;
  showCountingRulesByDefault?: boolean;
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

function includedSurfaces(
  system: MonitoredSystemLedgerEntry,
): MonitoredSystemLedgerExplanationSurface[] {
  if (system.explanation.surfaces.length > 0) {
    return system.explanation.surfaces;
  }

  return [
    {
      name: system.name,
      type: system.type,
      source: system.source,
    },
  ];
}

function formatLimitValue(value: number | undefined): string {
  if (typeof value !== 'number' || value <= 0) {
    return getMonitoredSystemLedgerPresentation().unlimitedLimitLabel;
  }
  return String(value);
}

function formatCapturedAt(value: number | undefined): string | null {
  if (typeof value !== 'number' || value <= 0) {
    return null;
  }
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return date.toLocaleDateString();
}

export function MonitoredSystemLedgerPanel(props: MonitoredSystemLedgerPanelProps = {}) {
  const [explanation, { refetch }] = createResource(() => MonitoredSystemLedgerAPI.explain());
  const [expandedSystemKey, setExpandedSystemKey] = createSignal<string | null>(null);
  const presentation = getMonitoredSystemLedgerPresentation();

  const ledger = createMemo(() =>
    explanation.state === 'errored' ? undefined : explanation()?.ledger,
  );
  const total = () => ledger()?.total ?? 0;
  const limit = () => ledger()?.limit ?? props.monitoredSystemLimit?.limit ?? 0;
  const systems = () => ledger()?.systems ?? [];
  const hasLimit = () => limit() > 0;
  const overLimit = () => hasLimit() && total() > limit();
  const pct = () => usagePercent(total(), limit());
  const usageUnavailableReason = () => {
    if (apiErrorCode(explanation.error) === 'monitored_system_usage_unavailable') {
      return apiErrorDetailField(explanation.error, 'reason') ?? undefined;
    }
    if (!ledger() && props.monitoredSystemLimit?.current_available === false) {
      return props.monitoredSystemLimit.current_unavailable_reason;
    }
    return undefined;
  };
  const usageUnavailable = () =>
    !ledger() &&
    (apiErrorCode(explanation.error) === 'monitored_system_usage_unavailable' ||
      props.monitoredSystemLimit?.current_available === false);
  const genericError = () => Boolean(explanation.error) && !usageUnavailable();
  const continuity = () => props.monitoredSystemContinuity ?? null;
  const hasContinuityContext = () => {
    const current = continuity();
    if (!current) {
      return false;
    }
    return (
      current.capture_pending ||
      current.effective_limit !== current.plan_limit ||
      (typeof current.grandfathered_floor === 'number' && current.grandfathered_floor > 0)
    );
  };
  const continuityForDisplay = createMemo(() => (hasContinuityContext() ? continuity() : null));
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
              defaultOpen={props.showCountingRulesByDefault}
              buttonClass="text-xs font-medium text-muted underline-offset-2 transition-colors hover:text-base-content hover:underline"
              detailClass="max-w-xl text-xs text-muted"
            />
          </div>
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
          <Show when={!ledger() && usageUnavailable()}>
            <span class="text-sm font-medium text-amber-700 dark:text-amber-300">
              {presentation.usageVerifyingLabel}
            </span>
          </Show>
        </div>
      </div>

      <Show when={continuityForDisplay()}>
        {(current) => (
          <div
            class="rounded-lg border px-3 py-3 text-xs"
            classList={{
              'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100':
                current().capture_pending,
              'border-green-200 bg-green-50 text-green-900 dark:border-green-900 dark:bg-green-950/40 dark:text-green-100':
                !current().capture_pending,
            }}
          >
            <p class="font-semibold">{presentation.continuityHeading}</p>
            <dl class="mt-2 grid gap-2 sm:grid-cols-4">
              <div>
                <dt class="text-[11px] opacity-75">{presentation.continuityPlanLimitLabel}</dt>
                <dd class="font-medium">{formatLimitValue(current().plan_limit)}</dd>
              </div>
              <div>
                <dt class="text-[11px] opacity-75">{presentation.continuityEffectiveLimitLabel}</dt>
                <dd class="font-medium">{formatLimitValue(current().effective_limit)}</dd>
              </div>
              <Show when={current().grandfathered_floor}>
                {(floor) => (
                  <div>
                    <dt class="text-[11px] opacity-75">
                      {presentation.continuityGrandfatheredFloorLabel}
                    </dt>
                    <dd class="font-medium">{floor()}</dd>
                  </div>
                )}
              </Show>
              <div>
                <dt class="text-[11px] opacity-75">{presentation.continuityCaptureLabel}</dt>
                <dd class="font-medium">
                  {current().capture_pending
                    ? presentation.continuityCapturePendingLabel
                    : (formatCapturedAt(current().captured_at) ??
                      presentation.continuityCaptureCapturedLabel)}
                </dd>
              </div>
            </dl>
          </div>
        )}
      </Show>

      {/* Loading state */}
      <Show when={explanation.loading && !usageUnavailable()}>
        <p class="text-sm text-muted py-4 text-center">
          {getMonitoredSystemLedgerLoadingState().text}
        </p>
      </Show>

      {/* Usage-unavailable state */}
      <Show when={usageUnavailable()}>
        <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100">
          <p class="font-semibold">{getMonitoredSystemLedgerUnavailableState().title}</p>
          <p class="mt-1">
            {formatMonitoredSystemLedgerUnavailableMessage(usageUnavailableReason())}
          </p>
          <button
            type="button"
            class="mt-3 text-xs font-medium underline-offset-2 transition-colors hover:underline disabled:opacity-50"
            disabled={explanation.loading}
            onClick={() => refetch()}
          >
            {explanation.loading
              ? getMonitoredSystemLedgerErrorState().retryingLabel
              : getMonitoredSystemLedgerErrorState().retryLabel}
          </button>
        </div>
      </Show>

      {/* Error state */}
      <Show when={genericError()}>
        <div class="text-sm text-red-600 dark:text-red-400 py-4 text-center">
          <p>{getMonitoredSystemLedgerErrorState().title}</p>
          <button
            type="button"
            class="mt-2 text-xs text-primary hover:underline disabled:opacity-50"
            disabled={explanation.loading}
            onClick={() => refetch()}
          >
            {explanation.loading
              ? getMonitoredSystemLedgerErrorState().retryingLabel
              : getMonitoredSystemLedgerErrorState().retryLabel}
          </button>
        </div>
      </Show>

      {/* Loaded content */}
      <Show when={!explanation.loading && !genericError() && ledger()}>
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
                          <div class="flex flex-wrap items-center gap-1.5">
                            <span class="rounded-full border border-border bg-surface px-2 py-0.5 text-[11px] font-medium text-base-content">
                              {presentation.countedSystemBadgeLabel}
                            </span>
                            <span class="text-[11px] text-muted">
                              {formatMonitoredSystemGroupedSourcesLabel(
                                includedSurfaces(system).length,
                              )}
                            </span>
                          </div>
                          <p class="max-w-xl text-xs text-muted">{system.explanation.summary}</p>
                          <Show when={includedSurfaces(system).length > 0}>
                            <div class="flex flex-wrap gap-1">
                              <For each={includedSurfaces(system)}>
                                {(surface) => (
                                  <span class="rounded-full bg-surface-alt px-2 py-0.5 text-[11px] text-muted">
                                    {formatMonitoredSystemSurfaceAttribution(surface)}
                                  </span>
                                )}
                              </For>
                            </div>
                          </Show>
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
                                  {presentation.countingExplanationHeading}
                                </p>
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
                              </div>
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
                                      {formatMonitoredSystemLatestIncludedSignalSentence(signal())}
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
                              <Show when={system.explanation.surfaces.length > 0}>
                                <div class="space-y-1">
                                  <p class="font-medium text-base-content">
                                    {presentation.groupedSourcesHeading}
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
