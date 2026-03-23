import { For, Show, Suspense } from 'solid-js';
import type { Component } from 'solid-js';
import type {
  Resource,
  ResourceChangeKind,
  ResourceChangeSourceAdapter,
  ResourceChangeSourceType,
} from '@/types/resource';
import { formatUptime, formatRelativeTime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { TagBadges } from '@/components/shared/TagBadges';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { TemperaturesCard } from '@/components/shared/cards/TemperaturesCard';
import { RaidCard } from '@/components/shared/cards/RaidCard';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import { getServiceHealthPresentation } from '@/utils/serviceHealthPresentation';
import {
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
} from '@/utils/resourcePolicyPresentation';
import { ResourceCorrelationSummary } from './ResourceCorrelationSummary';
import { ResourceChangeSummary } from './ResourceChangeSummary';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import {
  RESOURCE_CHANGE_KIND_ORDER,
  RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER,
  RESOURCE_CHANGE_SOURCE_TYPE_ORDER,
  getResourceChangeKindPresentation,
  getResourceChangeSourceAdapterPresentation,
  getResourceChangeSourceTypePresentation,
} from '@/utils/resourceChangePresentation';
import { formatConfidenceLabel } from '@/utils/confidencePresentation';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import { shouldShowResourcePlatformId } from '@/utils/resourceIdentity';
import { buildInfrastructureResourceHref } from '@/routing/resourceLinks';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import { formatInteger } from './resourceDetailMappers';
import {
  ResourceDetailDrawerSupportDisclosure as SupportDisclosure,
} from './ResourceDetailDrawerSupportDisclosure';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';

interface ResourceDetailDrawerOverviewTabProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
}

const hasMetadataEntries = (value?: Record<string, unknown> | null): boolean =>
  Boolean(value && Object.keys(value).length > 0);

const timelineKindOptions: Array<{ label: string; value: ResourceChangeKind | '' }> = [
  { label: 'All kinds', value: '' },
  ...RESOURCE_CHANGE_KIND_ORDER.map((kind) => ({
    label: getResourceChangeKindPresentation(kind).label,
    value: kind,
  })),
];

const timelineSourceTypeOptions: Array<{ label: string; value: ResourceChangeSourceType | '' }> = [
  { label: 'All sources', value: '' },
  ...RESOURCE_CHANGE_SOURCE_TYPE_ORDER.map((sourceType) => ({
    label: getResourceChangeSourceTypePresentation(sourceType).label,
    value: sourceType,
  })),
];

const timelineSourceAdapterOptions: Array<{
  label: string;
  value: ResourceChangeSourceAdapter | '';
}> = [
  { label: 'All adapters', value: '' },
  ...RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER.map((sourceAdapter) => ({
    label: getResourceChangeSourceAdapterPresentation(sourceAdapter).label,
    value: sourceAdapter,
  })),
];

export const ResourceDetailDrawerOverviewTab: Component<ResourceDetailDrawerOverviewTabProps> = (
  props,
) => {
  const { resource, drawer } = props;
  const showPlatformId = shouldShowResourcePlatformId(resource);

  return (
    <div class="space-y-3">
      <div data-testid="resource-summary-section" class="grid gap-3 sm:grid-cols-2">
        <Card data-testid="resource-current-state-section" padding="sm" class="h-full shadow-sm">
          <div class="mb-2 text-[10px] font-medium uppercase tracking-wide text-base-content">
            Current state
          </div>
          <div class="space-y-1.5 text-[11px]">
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">State</span>
              <span class="font-medium text-base-content capitalize">
                {resource.status || 'unknown'}
              </span>
            </div>
            <Show when={resource.uptime}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Uptime</span>
                <span class="font-medium text-base-content">
                  {formatUptime(resource.uptime ?? 0)}
                </span>
              </div>
            </Show>
            <Show when={resource.lastSeen}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Last Seen</span>
                <span class="font-medium text-base-content" title={drawer.lastSeenAbsolute()}>
                  {drawer.lastSeen() || '—'}
                </span>
              </div>
            </Show>
            <Show when={drawer.sourceSummary()}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Sources</span>
                <span
                  class={`font-medium ${drawer.sourceSummary()!.className}`}
                  title={drawer.sourceSummary()!.title}
                >
                  {drawer.sourceSummary()!.label}
                </span>
              </div>
            </Show>
            <Show when={(resource.alerts?.length || 0) > 0}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Alerts</span>
                <span class="font-medium text-amber-600 dark:text-amber-400">
                  {formatInteger(resource.alerts?.length)}
                </span>
              </div>
            </Show>
            <Show when={showPlatformId && !drawer.hasRuntimeOperationalContext()}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Platform ID</span>
                <span class="font-medium text-base-content truncate" title={resource.platformId}>
                  {resource.platformId}
                </span>
              </div>
            </Show>
            <Show when={drawer.hasRuntimeOperationalContext()}>
              <div class="mt-2 space-y-1.5">
                <Show when={showPlatformId}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Platform ID</span>
                    <span class="font-medium text-base-content truncate" title={resource.platformId}>
                      {resource.platformId}
                    </span>
                  </div>
                </Show>
                <Show when={drawer.kubernetesCapabilityBadges().length > 0}>
                  <div class="flex flex-col gap-1">
                    <span class="text-muted">Platform signals</span>
                    <div class="flex flex-wrap gap-1">
                      <For each={drawer.kubernetesCapabilityBadges()}>
                        {(badge) => (
                          <span class={badge.classes} title={badge.title}>
                            {badge.label}
                          </span>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
                <Show when={drawer.relatedLinks().length > 0}>
                  <div class="flex flex-col gap-1">
                    <span class="text-muted">Quick links</span>
                    <div class="flex flex-wrap gap-2">
                      <For each={drawer.relatedLinks()}>
                        {(link) => (
                          <a
                            href={link.href}
                            aria-label={link.ariaLabel}
                            class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-2.5 py-1 text-[11px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-900"
                          >
                            {link.compactLabel}
                          </a>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
              </div>
            </Show>
          </div>
        </Card>

        <Card data-testid="resource-identity-section" padding="sm" class="h-full shadow-sm">
          <div class="mb-2 text-[10px] font-medium uppercase tracking-wide text-base-content">
            Identity
          </div>
          <div class="space-y-1.5 text-[11px]">
            <For each={drawer.primaryIdentityRows()}>
              {(row) => (
                <div class="flex items-center justify-between gap-2">
                  <span class="text-muted">{row.label}</span>
                  <span class="font-medium text-base-content truncate" title={row.value}>
                    {row.value}
                  </span>
                </div>
              )}
            </For>
            <Show when={drawer.identityIpValues().length > 0}>
              <div class="flex flex-col gap-1">
                <span class="text-muted">IP Addresses</span>
                <div class="flex flex-wrap gap-1">
                  <For each={drawer.identityIpValues()}>
                    {(ip) => (
                      <span
                        class="inline-flex items-center rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200"
                        title={ip}
                      >
                        {ip}
                      </span>
                    )}
                  </For>
                </div>
              </div>
            </Show>
            <Show when={resource.tags && resource.tags.length > 0}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Tags</span>
                <TagBadges tags={resource.tags} maxVisible={6} />
              </div>
            </Show>
            <Show when={drawer.identityAliasValues().length > 0}>
              <Show
                when={drawer.hasAliasOverflow()}
                fallback={
                  <div class="flex flex-col gap-1">
                    <span class="text-muted">Aliases</span>
                    <div class="flex flex-wrap gap-1">
                      <For each={drawer.aliasPreviewValues()}>
                        {(value) => (
                          <span
                            class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]"
                            title={value}
                          >
                            {value}
                          </span>
                        )}
                      </For>
                    </div>
                  </div>
                }
              >
                <details class="rounded border border-border bg-surface px-2 py-1.5">
                  <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-muted">
                    <span>Aliases</span>
                    <span class="text-muted">{drawer.identityAliasValues().length}</span>
                  </summary>
                  <div class="mt-2 flex flex-wrap gap-1 border-t border-border pt-2">
                    <For each={drawer.identityAliasValues()}>
                      {(value) => (
                        <span
                          class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]"
                          title={value}
                        >
                          {value}
                        </span>
                      )}
                    </For>
                  </div>
                </details>
              </Show>
            </Show>
            <Show when={!drawer.identityCardHasRichData()}>
              <div class="rounded border border-dashed bg-surface-hover px-2 py-1.5 text-[10px] ">
                No identity metadata yet.
              </div>
            </Show>
          </div>
        </Card>
      </div>

      <div
        data-testid="resource-secondary-sections"
        class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.375rem)] [&>*]:min-w-[260px] [&>*]:max-w-full [&>*]:overflow-hidden"
      >
        <div
          data-testid="resource-change-history-section"
          class="h-full rounded border border-border bg-surface p-3 shadow-sm"
        >
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
                Change history
              </div>
              <Show when={drawer.resourceTimelineCount() > 0}>
                <div class="mt-1">
                  <ResourceFacetSummary
                    recentChanges={drawer.historyRecentChanges()}
                    counts={drawer.historyFacetCounts()}
                  />
                </div>
              </Show>
            </div>
            <div class="text-right text-[10px] text-muted">
              <div>{drawer.historyLoadingLabel()}</div>
              <Show when={drawer.hasTimelineFilters()}>
                <div class="mt-0.5 text-blue-700 dark:text-blue-300">Change filters active</div>
              </Show>
            </div>
          </div>
          <div class="mt-3 space-y-2">
            <label class="space-y-1 text-[10px]">
              <span class="text-muted">Change kind</span>
              <select
                class="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
                value={drawer.timelineKindFilter()}
                onChange={(event) =>
                  drawer.setTimelineKindFilter(
                    (event.currentTarget.value || '') as ResourceChangeKind | '',
                  )
                }
              >
                <For each={timelineKindOptions}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </select>
            </label>
            <label class="space-y-1 text-[10px]">
              <span class="text-muted">Source type</span>
              <select
                class="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
                value={drawer.timelineSourceTypeFilter()}
                onChange={(event) =>
                  drawer.setTimelineSourceTypeFilter(
                    (event.currentTarget.value || '') as ResourceChangeSourceType | '',
                  )
                }
              >
                <For each={timelineSourceTypeOptions}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </select>
            </label>
            <label class="space-y-1 text-[10px]">
              <span class="text-muted">Source adapter</span>
              <select
                class="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
                value={drawer.timelineSourceAdapterFilter()}
                onChange={(event) =>
                  drawer.setTimelineSourceAdapterFilter(
                    (event.currentTarget.value || '') as ResourceChangeSourceAdapter | '',
                  )
                }
              >
                <For each={timelineSourceAdapterOptions}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </select>
            </label>
          </div>

          <Show when={drawer.hasTimelineFilters()}>
            <div class="mt-2 flex justify-end">
              <button
                type="button"
                class="rounded-md border border-border bg-surface-hover px-2.5 py-1 text-[10px] font-semibold text-base-content hover:bg-surface"
                onClick={() => {
                  drawer.setTimelineKindFilter('');
                  drawer.setTimelineSourceTypeFilter('');
                  drawer.setTimelineSourceAdapterFilter('');
                }}
              >
                Clear filters
              </button>
            </div>
          </Show>

          <Show when={drawer.facetBundleError()}>
            <div class="mt-2 rounded border border-amber-200 bg-amber-50 px-2 py-1.5 text-[10px] text-amber-700 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
              <div class="flex items-start justify-between gap-2">
                <span>{drawer.facetBundleError()}</span>
                <button
                  type="button"
                  class="shrink-0 font-medium text-amber-700 underline dark:text-amber-200"
                  onClick={() => drawer.refetchHistoryFacets()}
                >
                  Retry
                </button>
              </div>
            </div>
          </Show>

          <Show
            when={drawer.sortedResourceTimeline().length > 0}
            fallback={
              <div class="mt-3 rounded border border-dashed border-border bg-surface-hover px-2 py-2 text-[10px] text-muted">
                No events yet.
              </div>
            }
          >
            <div class="mt-3 space-y-2">
              <For each={drawer.sortedResourceTimeline()}>
                {(change) => {
                  const kindPresentation = getResourceChangeKindPresentation(change.kind);
                  const sourceTypePresentation = getResourceChangeSourceTypePresentation(
                    change.sourceType,
                  );
                  const sourceAdapterPresentation = change.sourceAdapter
                    ? getResourceChangeSourceAdapterPresentation(change.sourceAdapter)
                    : null;

                  return (
                    <div class="rounded border border-border bg-surface-hover px-2 py-1.5 text-[10px]">
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <div class="font-medium text-base-content">{kindPresentation.label}</div>
                          <div class="mt-0.5 text-muted">
                            {formatRelativeTime(change.observedAt)}
                            <Show when={change.occurredAt}>
                              <span class="mx-1">•</span>
                              <span>Occurred {formatRelativeTime(change.occurredAt)}</span>
                            </Show>
                          </div>
                        </div>
                        <span class="text-muted">{sourceTypePresentation.label}</span>
                      </div>
                      <div class="mt-1 space-y-1">
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Confidence</span>
                          <span class="font-medium text-base-content">
                            {formatConfidenceLabel(change.confidence)}
                          </span>
                        </div>
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Adapter</span>
                          <span class="font-medium text-base-content">
                            {sourceAdapterPresentation?.label || '—'}
                          </span>
                        </div>
                        <Show when={change.actor}>
                          <div class="flex items-center justify-between gap-2">
                            <span class="text-muted">Actor</span>
                            <span class="font-medium text-base-content">{change.actor}</span>
                          </div>
                        </Show>
                        <Show when={change.from || change.to}>
                          <div class="flex items-center justify-between gap-2">
                            <span class="text-muted">Transition</span>
                            <span class="font-medium text-base-content">
                              {change.from || '—'} → {change.to || '—'}
                            </span>
                          </div>
                        </Show>
                      </div>
                      <Show when={change.reason}>
                        <div class="mt-1 rounded border border-border bg-base px-2 py-1 text-[10px] text-base-content">
                          {change.reason}
                        </div>
                      </Show>
                      <Show when={hasMetadataEntries(change.metadata)}>
                        <details class="mt-1 rounded border border-border bg-base px-2 py-1">
                          <summary class="cursor-pointer list-none text-[10px] font-medium text-muted">
                            Metadata
                          </summary>
                          <pre class="mt-2 overflow-auto whitespace-pre-wrap break-words text-[10px] text-base-content">
                            {JSON.stringify(change.metadata ?? {}, null, 2)}
                          </pre>
                        </details>
                      </Show>
                      <Show when={change.relatedResources && change.relatedResources.length > 0}>
                        <div class="mt-1 flex flex-wrap items-center gap-1 text-muted">
                          <span>Related:</span>
                          <For each={change.relatedResources ?? []}>
                            {(relatedResource) => {
                              const label = drawer.resolveResourceLabel(relatedResource);
                              const href = buildInfrastructureResourceHref(relatedResource);
                              return href ? (
                                <a
                                  class="inline-flex rounded bg-surface px-1.5 py-0.5 text-[10px] text-blue-700 hover:underline dark:text-blue-300"
                                  href={href}
                                  aria-label={`Open related resource ${label} in Infrastructure`}
                                >
                                  {label}
                                </a>
                              ) : (
                                <span class="inline-flex rounded bg-surface px-1.5 py-0.5 text-[10px] text-base-content">
                                  {label}
                                </span>
                              );
                            }}
                          </For>
                        </div>
                      </Show>
                    </div>
                  );
                }}
              </For>
            </div>
          </Show>
        </div>

        <Show when={drawer.hasServiceDetails()}>
          <SupportDisclosure
            title="Service details"
            summary={drawer.serviceDetailsSummary()}
            expanded={drawer.showServiceDetails()}
            onToggle={() => drawer.setShowServiceDetails((value) => !value)}
            showLabel="Show service details"
            hideLabel="Hide service details"
            class="h-full"
            contentClass="mt-3 space-y-3"
            dataTestId="resource-service-details-section"
          >
            <Show when={resource.type === 'docker-host'}>
              <div class="rounded border border-sky-200 bg-sky-50 p-3 dark:border-sky-700 dark:bg-sky-900">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-sky-700 dark:text-sky-300">
                    Docker runtime
                  </div>
                  <Show when={drawer.dockerHostData()?.runtime}>
                    <span
                      class="max-w-[55%] truncate text-[10px] text-sky-700 dark:text-sky-300"
                      title={drawer.dockerHostData()?.runtime}
                    >
                      {drawer.dockerHostData()?.runtime}
                    </span>
                  </Show>
                </div>

                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Containers</span>
                    <span class="font-medium text-base-content">
                      {formatInteger(drawer.dockerContainerCount())}
                    </span>
                  </div>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Updates</span>
                    <span
                      class={`font-medium ${drawer.dockerUpdatesAvailable() > 0 ? 'text-sky-700 dark:text-sky-300' : 'text-base-content'}`}
                    >
                      {formatInteger(drawer.dockerUpdatesAvailable())}
                    </span>
                  </div>
                  <Show when={drawer.dockerUpdatesCheckedRelative()}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted">Checked</span>
                      <span class="font-medium text-base-content">
                        {drawer.dockerUpdatesCheckedRelative()}
                      </span>
                    </div>
                  </Show>

                  <Show when={drawer.showDockerUpdateControls()}>
                    <div class="space-y-1.5 border-t border-sky-200 pt-2 dark:border-sky-700">
                      <Show when={drawer.dockerHostCommand()?.type || drawer.dockerHostCommand()?.status}>
                        <div class="rounded border border-sky-200 bg-surface px-2 py-1.5 text-[10px] dark:border-sky-700">
                          <div class="flex items-center justify-between gap-2">
                            <span class="text-muted">Action</span>
                            <span class="font-medium text-base-content">
                              {formatIdentifierLabel(drawer.dockerHostCommand()?.type, {
                                fallback: 'command',
                              })}
                            </span>
                          </div>
                          <div class="mt-1 flex items-center justify-between gap-2">
                            <span class="text-muted">State</span>
                            <span
                              class={`font-medium ${drawer.dockerHostCommandActive() ? 'text-sky-700 dark:text-sky-300' : 'text-base-content'}`}
                            >
                              {formatIdentifierLabel(drawer.dockerHostCommand()?.status, {
                                fallback: 'unknown',
                              })}
                            </span>
                          </div>
                          <Show when={drawer.dockerHostCommand()?.message}>
                            <div
                              class="mt-1 text-muted truncate"
                              title={drawer.dockerHostCommand()?.message}
                            >
                              {drawer.dockerHostCommand()?.message}
                            </div>
                          </Show>
                          <Show when={drawer.dockerHostCommand()?.failureReason}>
                            <div
                              class="mt-1 text-red-700 dark:text-red-300 truncate"
                              title={drawer.dockerHostCommand()?.failureReason}
                            >
                              {drawer.dockerHostCommand()?.failureReason}
                            </div>
                          </Show>
                        </div>
                      </Show>

                      <Show when={drawer.dockerActionError()}>
                        <div class="rounded border border-red-200 bg-red-50 px-2 py-1.5 text-[10px] text-red-700 dark:border-red-700 dark:bg-red-900 dark:text-red-200">
                          {drawer.dockerActionError()}
                        </div>
                      </Show>
                      <Show when={drawer.dockerActionNote()}>
                        <div class="rounded border border-sky-200 bg-surface px-2 py-1.5 text-[10px] text-base-content dark:border-sky-700">
                          {drawer.dockerActionNote()}
                        </div>
                      </Show>

                      <div class="flex flex-wrap items-center gap-2">
                        <button
                          type="button"
                          disabled={
                            drawer.dockerActionBusy() ||
                            drawer.dockerUpdateActionsLoading() ||
                            drawer.dockerHostCommandActive() ||
                            drawer.dockerHostSourceId() === null
                          }
                          onClick={drawer.queueDockerUpdateCheck}
                          class="rounded-md border border-border bg-surface px-2.5 py-1 text-[11px] font-semibold text-base-content hover:bg-surface-hover disabled:opacity-60"
                          title={
                            drawer.dockerUpdateActionsLoading() ? 'Loading settings...' : undefined
                          }
                        >
                          Check now
                        </button>

                        <button
                          type="button"
                          disabled={
                            drawer.dockerActionBusy() ||
                            drawer.dockerUpdateActionsLoading() ||
                            drawer.dockerUpdateActionsDisabled() ||
                            drawer.dockerHostCommandActive() ||
                            drawer.dockerHostSourceId() === null ||
                            drawer.dockerUpdatesAvailable() <= 0
                          }
                          onClick={drawer.queueDockerUpdateAll}
                          class="rounded-md border border-sky-200 bg-sky-600 px-2.5 py-1 text-[11px] font-semibold text-white hover:bg-sky-700 disabled:opacity-60 disabled:hover:bg-sky-600 dark:border-sky-700 dark:bg-sky-600 dark:hover:bg-sky-500 dark:disabled:hover:bg-sky-600"
                          title={
                            drawer.dockerUpdateActionsDisabled()
                              ? 'Updates disabled by server settings.'
                              : undefined
                          }
                        >
                          {drawer.confirmUpdateAll()
                            ? 'Confirm update'
                            : `Update all${drawer.dockerUpdatesAvailable() > 0 ? ` (${drawer.dockerUpdatesAvailable()})` : ''}`}
                        </button>
                      </div>
                    </div>
                  </Show>

                  <button
                    type="button"
                    onClick={drawer.toggleDockerUpdateControls}
                    class="inline-flex items-center rounded-md border border-sky-200 bg-surface px-2.5 py-1 text-[10px] font-medium text-sky-700 transition-colors hover:bg-base dark:border-sky-700 dark:text-sky-300"
                  >
                    {drawer.showDockerUpdateControls() ? 'Hide actions' : 'Show actions'}
                  </button>
                </div>
              </div>
            </Show>

            <Show when={drawer.pbsData()}>
              {(pbs) => {
                const connection = getServiceHealthPresentation(resource.status, pbs().connectionHealth);
                return (
                  <div class="rounded border border-indigo-200 bg-indigo-50 p-3 dark:border-indigo-700 dark:bg-indigo-900">
                    <div class="mb-2 flex items-center justify-between gap-2">
                      <div class="text-[11px] font-medium uppercase tracking-wide text-indigo-700 dark:text-indigo-300">
                        PBS
                      </div>
                      <Show when={pbs().hostname}>
                        <span
                          class="max-w-[55%] truncate text-[10px] text-indigo-700 dark:text-indigo-300"
                          title={pbs().hostname}
                        >
                          {pbs().hostname}
                        </span>
                      </Show>
                    </div>
                    <div class="space-y-1.5 text-[11px]">
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">State</span>
                        <span class={`font-medium ${connection.text}`}>{connection.label}</span>
                      </div>
                      <Show when={pbs().version}>
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Version</span>
                          <span class="font-medium text-base-content">{pbs().version}</span>
                        </div>
                      </Show>
                      <Show when={pbs().uptimeSeconds || resource.uptime}>
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Uptime</span>
                          <span class="font-medium text-base-content">
                            {formatUptime(pbs().uptimeSeconds ?? resource.uptime ?? 0)}
                          </span>
                        </div>
                      </Show>
                      <Show when={drawer.showPbsJobDetail()}>
                        <div class="space-y-1.5 border-t border-indigo-200 pt-2 dark:border-indigo-700">
                          <div class="grid grid-cols-2 gap-2">
                            <div class="rounded border border-indigo-200 bg-surface px-2 py-1.5 dark:border-indigo-700">
                              <div class="text-[10px] text-muted">Datastores</div>
                              <div class="text-sm font-semibold text-base-content">
                                {formatInteger(pbs().datastoreCount)}
                              </div>
                            </div>
                            <div class="rounded border border-indigo-200 bg-surface px-2 py-1.5 dark:border-indigo-700">
                              <div class="text-[10px] text-muted">Jobs</div>
                              <div class="text-sm font-semibold text-base-content">
                                {formatInteger(drawer.pbsJobTotal())}
                              </div>
                            </div>
                          </div>
                          <details class="rounded border border-indigo-200 bg-surface px-2 py-1.5 dark:border-indigo-700">
                            <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-muted">
                              <span>Types</span>
                              <span class="text-muted">{drawer.pbsVisibleJobBreakdown().length}</span>
                            </summary>
                            <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-indigo-200 pt-2 text-[10px] dark:border-indigo-700">
                              <For each={drawer.pbsVisibleJobBreakdown()}>
                                {(entry) => (
                                  <span class="text-muted">
                                    {entry.label}:{' '}
                                    <span class="font-medium text-base-content">
                                      {formatInteger(entry.value)}
                                    </span>
                                  </span>
                                )}
                              </For>
                            </div>
                          </details>
                        </div>
                      </Show>
                      <button
                        type="button"
                        onClick={() => drawer.setShowPbsJobDetail((value) => !value)}
                        class="inline-flex items-center rounded-md border border-indigo-200 bg-surface px-2.5 py-1 text-[10px] font-medium text-indigo-700 transition-colors hover:bg-base dark:border-indigo-700 dark:text-indigo-300"
                      >
                        {drawer.showPbsJobDetail() ? 'Hide jobs' : 'Show jobs'}
                      </button>
                    </div>
                  </div>
                );
              }}
            </Show>

            <Show when={drawer.pmgData()}>
              {(pmg) => {
                const connection = getServiceHealthPresentation(resource.status, pmg().connectionHealth);
                return (
                  <div class="rounded border border-rose-200 bg-rose-50 p-3 dark:border-rose-700 dark:bg-rose-900">
                    <div class="mb-2 flex items-center justify-between gap-2">
                      <div class="text-[11px] font-medium uppercase tracking-wide text-rose-700 dark:text-rose-300">
                        PMG
                      </div>
                      <Show when={pmg().hostname}>
                        <span
                          class="max-w-[55%] truncate text-[10px] text-rose-700 dark:text-rose-300"
                          title={pmg().hostname}
                        >
                          {pmg().hostname}
                        </span>
                      </Show>
                    </div>
                    <div class="space-y-1.5 text-[11px]">
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">State</span>
                        <span class={`font-medium ${connection.text}`}>{connection.label}</span>
                      </div>
                      <Show when={pmg().version}>
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Version</span>
                          <span class="font-medium text-base-content">{pmg().version}</span>
                        </div>
                      </Show>
                      <Show when={pmg().uptimeSeconds || resource.uptime}>
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Uptime</span>
                          <span class="font-medium text-base-content">
                            {formatUptime(pmg().uptimeSeconds ?? resource.uptime ?? 0)}
                          </span>
                        </div>
                      </Show>
                      <Show when={drawer.showPmgMailFlowDetail()}>
                        <div class="space-y-1.5 border-t border-rose-200 pt-2 dark:border-rose-700">
                          <div class="grid grid-cols-2 gap-2">
                            <div class="rounded border border-rose-200 bg-surface px-2 py-1.5 dark:border-rose-700">
                              <div class="text-[10px] text-muted">Queue</div>
                              <div
                                class={`text-sm font-semibold ${drawer.pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-base-content'}`}
                              >
                                {formatInteger(pmg().queueTotal)}
                              </div>
                            </div>
                            <div class="rounded border border-rose-200 bg-surface px-2 py-1.5 dark:border-rose-700">
                              <div class="text-[10px] text-muted">Backlog</div>
                              <div
                                class={`text-sm font-semibold ${drawer.pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-base-content'}`}
                              >
                                {formatInteger(drawer.pmgQueueBacklog())}
                              </div>
                            </div>
                          </div>
                          <Show when={pmg().nodeCount || drawer.pmgUpdatedRelative()}>
                            <div
                              data-testid="pmg-support-context"
                              class="space-y-1.5 rounded border border-dashed border-rose-200 bg-surface px-2 py-1.5 text-[10px] dark:border-rose-700"
                            >
                              <Show when={pmg().nodeCount}>
                                <div class="flex items-center justify-between gap-2">
                                  <span class="text-muted">Nodes</span>
                                  <span class="font-medium text-base-content">
                                    {formatInteger(pmg().nodeCount)}
                                  </span>
                                </div>
                              </Show>
                              <Show when={drawer.pmgUpdatedRelative()}>
                                <div
                                  class={`flex items-center justify-between gap-2 ${pmg().nodeCount ? 'border-t border-rose-200 pt-1.5 dark:border-rose-700' : ''}`}
                                >
                                  <span class="text-muted">Updated</span>
                                  <span class="font-medium text-base-content">
                                    {drawer.pmgUpdatedRelative()}
                                  </span>
                                </div>
                              </Show>
                            </div>
                          </Show>
                          <details class="rounded border border-rose-200 bg-surface px-2 py-1.5 dark:border-rose-700">
                            <summary class="cursor-pointer list-none text-[10px] font-medium text-muted">
                              Queue detail
                            </summary>
                            <div class="mt-2 space-y-1.5 border-t border-rose-200 pt-2 text-[10px] dark:border-rose-700">
                              <For each={drawer.pmgVisibleQueueBreakdown()}>
                                {(entry) => (
                                  <div class="flex items-center justify-between gap-2 text-muted">
                                    <span>{entry.label}</span>
                                    <span
                                      class={`font-medium ${entry.warn ? 'text-amber-600 dark:text-amber-400' : 'text-base-content'}`}
                                    >
                                      {formatInteger(entry.value)}
                                    </span>
                                  </div>
                                )}
                              </For>
                            </div>
                          </details>
                          <details class="rounded border border-rose-200 bg-surface px-2 py-1.5 dark:border-rose-700">
                            <summary class="cursor-pointer list-none text-[10px] font-medium text-muted">
                              Mail detail
                            </summary>
                            <div class="mt-2 space-y-1.5 border-t border-rose-200 pt-2 text-[10px] dark:border-rose-700">
                              <For each={drawer.pmgVisibleMailBreakdown()}>
                                {(entry) => (
                                  <div class="flex items-center justify-between gap-2 text-muted">
                                    <span>{entry.label}</span>
                                    <span class="font-medium text-base-content">
                                      {formatInteger(entry.value)}
                                    </span>
                                  </div>
                                )}
                              </For>
                            </div>
                          </details>
                        </div>
                      </Show>
                      <button
                        type="button"
                        onClick={() => drawer.setShowPmgMailFlowDetail((value) => !value)}
                        class="inline-flex items-center rounded-md border border-rose-200 bg-surface px-2.5 py-1 text-[10px] font-medium text-rose-700 transition-colors hover:bg-base dark:border-rose-700 dark:text-rose-300"
                      >
                        {drawer.showPmgMailFlowDetail() ? 'Hide mail flow' : 'Show mail flow'}
                      </button>
                    </div>
                  </div>
                );
              }}
            </Show>
          </SupportDisclosure>
        </Show>

        <Show when={drawer.hasHostDetails()}>
          <SupportDisclosure
            title="Host details"
            summary={drawer.hostDetailSummary()}
            expanded={drawer.showHostDetails()}
            onToggle={() => drawer.setShowHostDetails((value) => !value)}
            showLabel="Show host details"
            hideLabel="Hide host details"
            class="h-full"
            contentClass="mt-3 flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.375rem)] [&>*]:min-w-[220px] [&>*]:max-w-full [&>*]:overflow-hidden"
            dataTestId="resource-host-details-section"
          >
            <Show when={drawer.proxmoxNode()}>
              {(node) => (
                <>
                  <SystemInfoCard variant="node" node={node()} />
                  <HardwareCard variant="node" node={node()} />
                  <RootDiskCard node={node()} />
                </>
              )}
            </Show>
            <Show when={drawer.agentInfo()}>
              {(agent) => (
                <>
                  <SystemInfoCard variant="agent" agent={agent()} />
                  <HardwareCard variant="agent" agent={agent()} />
                  <NetworkInterfacesCard interfaces={agent().networkInterfaces} />
                  <DisksCard disks={agent().disks} />
                  <RaidCard arrays={drawer.agentMeta()?.raid} />
                  <TemperaturesCard rows={drawer.temperatureRows()} />
                </>
              )}
            </Show>
          </SupportDisclosure>
        </Show>

        <Show when={drawer.hasInvestigationContext()}>
          <SupportDisclosure
            title="Context"
            summary={drawer.investigationContextSummary()}
            expanded={drawer.showInvestigationContext()}
            onToggle={() => drawer.setShowInvestigationContext((value) => !value)}
            showLabel="Show context"
            hideLabel="Hide context"
            class="h-full"
            contentClass="mt-3 space-y-3"
            dataTestId="resource-investigation-context"
          >
            <Show when={drawer.resourceIntelligence()}>
              {(intel) => (
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted uppercase tracking-wide">AI Intelligence</span>
                  </div>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Health</span>
                    <span class="font-semibold text-base-content">
                      {intel().health.grade} · {Math.round(intel().health.score)}/100
                    </span>
                  </div>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Trend</span>
                    <span class="font-semibold capitalize text-base-content">
                      {intel().health.trend}
                    </span>
                  </div>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Notes</span>
                    <span class="font-semibold text-base-content">{intel().note_count}</span>
                  </div>
                  <ResourceChangeSummary
                    class="space-y-0"
                    title="Latest canonical change"
                    changes={intel().recent_changes}
                    resolveResourceLabel={drawer.resolveResourceLabel}
                    maxChanges={1}
                    compact
                  />
                  <Show when={drawer.hasCorrelationContext()}>
                    <div data-testid="resource-correlation-context" class="space-y-1.5">
                      <div class="flex flex-wrap items-center justify-between gap-2">
                        <span class="text-[10px] font-medium uppercase tracking-wide text-base-content">
                          Correlations
                        </span>
                        <button
                          type="button"
                          onClick={() => drawer.setShowCorrelationContext((value) => !value)}
                          class="inline-flex items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-surface-hover"
                        >
                          {drawer.showCorrelationContext() ? 'Hide correlations' : 'Show correlations'}
                        </button>
                      </div>

                      <Show when={drawer.showCorrelationContext()}>
                        <div class="pt-1">
                          <ResourceCorrelationSummary
                            title="Correlations"
                            dependencies={drawer.resourceDependencies()}
                            dependents={drawer.resourceDependents()}
                            correlations={drawer.resourceCorrelations()}
                            resolveResourceLabel={drawer.resolveResourceLabel}
                            showLastSeen
                          />
                        </div>
                      </Show>
                    </div>
                  </Show>
                </div>
              )}
            </Show>

            <Show when={drawer.hasGovernanceData()}>
              <div class="space-y-1.5 text-[11px]">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-muted uppercase tracking-wide">Data Governance</span>
                </div>
                <Show when={resource.policy}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Sensitivity</span>
                    <span class="font-semibold text-base-content">
                      {getResourceSensitivityLabel(resource.policy?.sensitivity)}
                    </span>
                  </div>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Routing</span>
                    <span class="font-semibold text-base-content">
                      {getResourceRoutingScopeLabel(resource.policy?.routing.scope)}
                    </span>
                  </div>
                </Show>
                <Show when={drawer.policyRedactions().length > 0 || drawer.governanceSummary()}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Redactions</span>
                    <span class="font-semibold text-base-content">
                      {drawer.policyRedactions().length}
                    </span>
                  </div>
                </Show>
                <Show when={drawer.policyRedactions().length > 0}>
                  <div class="flex flex-col gap-1">
                    <span class="text-muted">Redaction labels</span>
                    <div class="flex flex-wrap gap-1">
                      <For each={drawer.policyRedactions()}>
                        {(label) => (
                          <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                            {label}
                          </span>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
                <Show when={drawer.governanceSummary()}>
                  <div class="flex flex-col gap-1">
                    <span class="text-muted">AI-Safe Summary</span>
                    <div class="rounded border border-border bg-surface-hover px-2 py-1.5 text-[10px] text-base-content">
                      {drawer.governanceSummary()}
                    </div>
                  </div>
                </Show>
              </div>
            </Show>
          </SupportDisclosure>
        </Show>
      </div>

      <Show when={drawer.discoveryConfig()}>
        {(config) => (
          <div class="space-y-2">
            <WebInterfaceUrlField
              metadataKind={config().metadataKind}
              metadataId={config().metadataId}
              targetLabel={config().targetLabel}
            />

            <SupportDisclosure
              title="Analysis"
              summary={drawer.discoveryContextSummary()}
              expanded={drawer.showDiscoveryContext()}
              onToggle={() => drawer.setShowDiscoveryContext((value) => !value)}
              showLabel="Open analysis"
              hideLabel="Hide analysis"
              class="h-full"
              dataTestId="resource-discovery-context"
            >
              <Suspense
                fallback={
                  <div class="flex items-center justify-center py-8">
                    <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                    <span class="ml-2 text-sm text-muted">{getDiscoveryLoadingState().text}</span>
                  </div>
                }
              >
                <DiscoveryTab
                  resourceType={config().resourceType}
                  agentId={config().agentId}
                  resourceId={config().resourceId}
                  hostname={config().hostname}
                />
              </Suspense>
            </SupportDisclosure>
          </div>
        )}
      </Show>
    </div>
  );
};
