import { Show, Suspense, For } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import type {
  Resource,
  ResourceChangeKind,
  ResourceChangeSourceAdapter,
  ResourceChangeSourceType,
} from '@/types/resource';
import { formatUptime, formatRelativeTime } from '@/utils/format';
import { StatusDot } from '@/components/shared/StatusDot';
import { Card } from '@/components/shared/Card';
import { TagBadges } from '@/components/shared/TagBadges';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { TemperaturesCard } from '@/components/shared/cards/TemperaturesCard';
import { RaidCard } from '@/components/shared/cards/RaidCard';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import { ReportMergeModal } from './ReportMergeModal';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { PMGInstanceDrawer } from '@/components/PMG/PMGInstanceDrawer';
import { K8sDeploymentsDrawer } from '@/components/Kubernetes/K8sDeploymentsDrawer';
import { K8sNamespacesDrawer } from '@/components/Kubernetes/K8sNamespacesDrawer';
import { MonitoringAPI } from '@/api/monitoring';
import { SwarmServicesDrawer } from '@/components/Docker/SwarmServicesDrawer';
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
import { buildInfrastructureResourceHref } from '@/routing/resourceLinks';
import { formatInteger, formatSourceType } from './resourceDetailMappers';
import { useResourceDetailDrawerState } from './useResourceDetailDrawerState';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
}

const hasMetadataEntries = (value?: Record<string, unknown> | null): boolean =>
  Boolean(value && Object.keys(value).length > 0);

interface SupportDisclosureProps {
  title: string;
  summary?: string | null;
  expanded: boolean;
  onToggle: () => void;
  showLabel: string;
  hideLabel: string;
  children: JSX.Element;
  class?: string;
  buttonClass?: string;
  contentClass?: string;
  dataTestId?: string;
}

const SupportDisclosure: Component<SupportDisclosureProps> = (props) => {
  const summary = () => props.summary?.trim() ?? '';

  return (
    <div
      data-testid={props.dataTestId}
      class={props.class ?? 'rounded border border-dashed border-border bg-surface-hover p-3'}
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
            {props.title}
          </div>
          <Show when={summary()}>
            <div class="mt-1 text-[10px] text-base-content">{summary()}</div>
          </Show>
        </div>

        <button
          type="button"
          onClick={props.onToggle}
          class={
            props.buttonClass ??
            'inline-flex items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-base'
          }
        >
          {props.expanded ? props.hideLabel : props.showLabel}
        </button>
      </div>

      <Show when={props.expanded}>
        <div class={props.contentClass ?? 'mt-3'}>{props.children}</div>
      </Show>
    </div>
  );
};

const TabAvailabilityNotice: Component<{ message: string }> = (props) => (
  <div class="rounded border border-dashed border-border bg-surface-hover p-4 text-sm text-muted">
    {props.message}
  </div>
);

type SpecializedDrawerTab = 'mail' | 'namespaces' | 'deployments' | 'swarm';

export const getSpecializedTabAvailabilityMessage = (tab: SpecializedDrawerTab): string => {
  switch (tab) {
    case 'mail':
      return 'PMG resources only.';
    case 'namespaces':
    case 'deployments':
      return 'Kubernetes clusters only.';
    case 'swarm':
      return 'Docker runtimes with Swarm only.';
  }
};

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

const DrawerContent: Component<ResourceDetailDrawerProps> = (props) => {
  const {
    activeTab,
    setActiveTab,
    debugEnabled,
    copied,
    showReportModal,
    setShowReportModal,
    showInvestigationContext,
    setShowInvestigationContext,
    showCorrelationContext,
    setShowCorrelationContext,
    showDiscoveryContext,
    setShowDiscoveryContext,
    showHostDetails,
    setShowHostDetails,
    showServiceDetails,
    setShowServiceDetails,
    showDockerUpdateControls,
    setShowDockerUpdateControls,
    showPbsJobDetail,
    setShowPbsJobDetail,
    showPmgMailFlowDetail,
    setShowPmgMailFlowDetail,
    displayName,
    kubernetesClusterName,
    resolveResourceLabel,
    statusIndicator,
    lastSeen,
    lastSeenAbsolute,
    platformBadge,
    sourceBadge,
    typeBadge,
    unifiedSourceBadges,
    hasUnifiedSources,
    policyRedactions,
    governanceSummary,
    hasGovernanceData,
    agentMeta,
    kubernetesCapabilityBadges,
    proxmoxNode,
    agentInfo,
    temperatureRows,
    dockerHostData,
    dockerHostSourceId,
    dockerUpdatesAvailable,
    dockerContainerCount,
    dockerUpdatesCheckedRelative,
    dockerHostCommand,
    dockerHostCommandActive,
    dockerUpdateActionsDisabled,
    dockerUpdateActionsLoading,
    dockerActionError,
    setDockerActionError,
    dockerActionNote,
    setDockerActionNote,
    confirmUpdateAll,
    setConfirmUpdateAll,
    dockerActionBusy,
    setDockerActionBusy,
    dockerSwarmInfo,
    dockerSwarmClusterKey,
    k8sDeploymentsPrefillNamespace,
    setK8sDeploymentsPrefillNamespace,
    timelineKindFilter,
    setTimelineKindFilter,
    timelineSourceTypeFilter,
    setTimelineSourceTypeFilter,
    timelineSourceAdapterFilter,
    setTimelineSourceAdapterFilter,
    resourceIntelligence,
    resourceDependencies,
    resourceDependents,
    resourceCorrelations,
    hasCorrelationContext,
    hasInvestigationContext,
    investigationContextSummary,
    pbsData,
    pmgData,
    pbsJobTotal,
    pmgQueueBacklog,
    pmgUpdatedRelative,
    pbsVisibleJobBreakdown,
    pmgVisibleQueueBreakdown,
    pmgVisibleMailBreakdown,
    historyFacetCounts,
    historyRecentChanges,
    hasTimelineFilters,
    historyLoadingLabel,
    resourceTimelineCount,
    sortedResourceTimeline,
    facetBundleError,
    refetchHistoryFacets,
    mergedSources,
    sourceSummary,
    identityAliasValues,
    primaryIdentityRows,
    identityCardHasRichData,
    aliasPreviewValues,
    hasAliasOverflow,
    hasIdentitySupportContext,
    hasMergedSources,
    discoveryConfig,
    discoveryContextSummary,
    hasHostDetails,
    hostDetailSummary,
    hasServiceDetails,
    serviceDetailsSummary,
    headerIdentity,
    relatedLinks,
    hasRuntimeOperationalContext,
    sourceSections,
    identityMatchInfo,
    tabs,
    handleCopyJson,
  } = useResourceDetailDrawerState({
    resource: props.resource,
    resolveResourceLabel: props.resolveResourceLabel,
  });

  return (
    <div class="space-y-3">
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1 min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              variant={statusIndicator().variant}
              title={statusIndicator().label}
              ariaLabel={statusIndicator().label}
              size="sm"
            />
            <div class="text-sm font-semibold text-base-content truncate" title={displayName()}>
              {displayName()}
            </div>
          </div>
          <div class="text-[11px] text-muted truncate" title={headerIdentity()}>
            {headerIdentity()}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <Show when={typeBadge()}>
              {(badge) => (
                <span class={badge().classes} title={badge().title}>
                  {badge().label}
                </span>
              )}
            </Show>
            <Show
              when={hasUnifiedSources()}
              fallback={
                <>
                  <Show when={platformBadge()}>
                    {(badge) => (
                      <span class={badge().classes} title={badge().title}>
                        {badge().label}
                      </span>
                    )}
                  </Show>
                  <Show when={sourceBadge()}>
                    {(badge) => (
                      <span class={badge().classes} title={badge().title}>
                        {badge().label}
                      </span>
                    )}
                  </Show>
                </>
              }
            >
              <For each={unifiedSourceBadges()}>
                {(badge) => (
                  <span class={badge.classes} title={badge.title}>
                    {badge.label}
                  </span>
                )}
              </For>
            </Show>
          </div>
        </div>

        <Show when={props.onClose}>
          <button
            type="button"
            onClick={() => props.onClose?.()}
            class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
            aria-label="Close"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </Show>
      </div>

      <div class="flex items-center gap-6 border-b border-border px-1 mb-1">
        <For each={tabs()}>
          {(tab) => (
            <button
              onClick={() => setActiveTab(tab.id)}
              class={`pb-2 text-sm font-medium transition-colors relative ${
                activeTab() === tab.id ? 'text-blue-600 dark:text-blue-400' : ' hover:text-muted'
              }`}
            >
              {tab.label}
              <Show when={activeTab() === tab.id}>
                <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
              </Show>
            </button>
          )}
        </For>
      </div>

      {/* Overview Tab */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <div class="space-y-3">
          <div data-testid="resource-summary-section">
            <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
              Summary
            </div>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
              <Card
                data-testid="resource-current-state-section"
                padding="sm"
                class="h-full shadow-sm"
              >
                <div class="mb-2 text-[10px] font-medium uppercase tracking-wide text-base-content">
                  Current state
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">State</span>
                    <span class="font-medium text-base-content capitalize">
                      {props.resource.status || 'unknown'}
                    </span>
                  </div>
                  <Show when={props.resource.uptime}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted">Uptime</span>
                      <span class="font-medium text-base-content">
                        {formatUptime(props.resource.uptime ?? 0)}
                      </span>
                    </div>
                  </Show>
                  <Show when={props.resource.lastSeen}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted">Last Seen</span>
                      <span class="font-medium text-base-content" title={lastSeenAbsolute()}>
                        {lastSeen() || '—'}
                      </span>
                    </div>
                  </Show>
                  <Show when={sourceSummary()}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted">Sources</span>
                      <span
                        class={`font-medium ${sourceSummary()!.className}`}
                        title={sourceSummary()!.title}
                      >
                        {sourceSummary()!.label}
                      </span>
                    </div>
                  </Show>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Mode</span>
                    <span class="font-medium text-base-content">
                      {formatSourceType(props.resource.sourceType)}
                    </span>
                  </div>
                  <Show when={(props.resource.alerts?.length || 0) > 0}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted">Alerts</span>
                      <span class="font-medium text-amber-600 dark:text-amber-400">
                        {formatInteger(props.resource.alerts?.length)}
                      </span>
                    </div>
                  </Show>
                  <Show when={props.resource.platformId && !hasRuntimeOperationalContext()}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted">Platform ID</span>
                      <span
                        class="font-medium text-base-content truncate"
                        title={props.resource.platformId}
                      >
                        {props.resource.platformId}
                      </span>
                    </div>
                  </Show>
                  <Show when={hasRuntimeOperationalContext() || hasIdentitySupportContext()}>
                    <div class="mt-2 rounded border border-dashed border-border bg-surface-hover p-3">
                      <div class="space-y-2.5">
                        <Show when={hasRuntimeOperationalContext()}>
                          <div class="space-y-1.5">
                            <Show when={props.resource.platformId}>
                              <div class="flex items-center justify-between gap-2">
                                <span class="text-muted">Platform ID</span>
                                <span
                                  class="font-medium text-base-content truncate"
                                  title={props.resource.platformId}
                                >
                                  {props.resource.platformId}
                                </span>
                              </div>
                            </Show>
                            <Show when={kubernetesCapabilityBadges().length > 0}>
                              <div class="flex flex-col gap-1">
                                <span class="text-muted">Platform signals</span>
                                <div class="flex flex-wrap gap-1">
                                  <For each={kubernetesCapabilityBadges()}>
                                    {(badge) => (
                                      <span class={badge.classes} title={badge.title}>
                                        {badge.label}
                                      </span>
                                    )}
                                  </For>
                                </div>
                              </div>
                            </Show>
                            <Show when={relatedLinks().length > 0}>
                              <div class="flex flex-col gap-1">
                                <span class="text-muted">Quick links</span>
                                <div class="flex flex-wrap gap-2">
                                  <For each={relatedLinks()}>
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
                        <Show when={hasIdentitySupportContext()}>
                          <div class="space-y-1.5">
                            <Show
                              when={
                                props.resource.identity?.ips &&
                                props.resource.identity.ips.length > 0
                              }
                            >
                              <div class="flex flex-col gap-1">
                                <span class="text-muted">IP Addresses</span>
                                <div class="flex flex-wrap gap-1">
                                  <For each={props.resource.identity?.ips ?? []}>
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
                            <Show when={props.resource.tags && props.resource.tags.length > 0}>
                              <div class="flex items-center justify-between gap-2">
                                <span class="text-muted">Tags</span>
                                <TagBadges tags={props.resource.tags} maxVisible={6} />
                              </div>
                            </Show>
                            <Show when={identityAliasValues().length > 0}>
                              <Show
                                when={hasAliasOverflow()}
                                fallback={
                                  <div class="flex flex-col gap-1">
                                    <span class="text-muted">Aliases</span>
                                    <div class="flex flex-wrap gap-1">
                                      <For each={aliasPreviewValues()}>
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
                                    <span class="text-muted">{identityAliasValues().length}</span>
                                  </summary>
                                  <div class="mt-2 flex flex-wrap gap-1 border-t border-border pt-2">
                                    <For each={identityAliasValues()}>
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
                          </div>
                        </Show>
                      </div>
                    </div>
                  </Show>
                </div>
              </Card>

              <Card data-testid="resource-identity-section" padding="sm" class="h-full shadow-sm">
                <div class="mb-2 text-[10px] font-medium uppercase tracking-wide text-base-content">
                  Identity
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <For each={primaryIdentityRows()}>
                    {(row) => (
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">{row.label}</span>
                        <span class="font-medium text-base-content truncate" title={row.value}>
                          {row.value}
                        </span>
                      </div>
                    )}
                  </For>
                  <Show when={!identityCardHasRichData()}>
                    <div class="rounded border border-dashed bg-surface-hover px-2 py-1.5 text-[10px] ">
                      No identity metadata yet.
                    </div>
                  </Show>
                </div>
              </Card>
            </div>
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
                  <Show when={resourceTimelineCount() > 0}>
                    <div class="mt-1">
                      <ResourceFacetSummary
                        recentChanges={historyRecentChanges()}
                        counts={historyFacetCounts()}
                      />
                    </div>
                  </Show>
                </div>
                <div class="text-right text-[10px] text-muted">
                  <div>{historyLoadingLabel()}</div>
                  <Show when={hasTimelineFilters()}>
                    <div class="mt-0.5 text-blue-700 dark:text-blue-300">Change filters active</div>
                  </Show>
                </div>
              </div>
              <div class="mt-3 space-y-2">
                <label class="space-y-1 text-[10px]">
                  <span class="text-muted">Change kind</span>
                  <select
                    class="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
                    value={timelineKindFilter()}
                    onChange={(event) =>
                      setTimelineKindFilter(
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
                    value={timelineSourceTypeFilter()}
                    onChange={(event) =>
                      setTimelineSourceTypeFilter(
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
                    value={timelineSourceAdapterFilter()}
                    onChange={(event) =>
                      setTimelineSourceAdapterFilter(
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

              <Show when={hasTimelineFilters()}>
                <div class="mt-2 flex justify-end">
                  <button
                    type="button"
                    class="rounded-md border border-border bg-surface-hover px-2.5 py-1 text-[10px] font-semibold text-base-content hover:bg-surface"
                    onClick={() => {
                      setTimelineKindFilter('');
                      setTimelineSourceTypeFilter('');
                      setTimelineSourceAdapterFilter('');
                    }}
                  >
                    Clear filters
                  </button>
                </div>
              </Show>

              <Show when={facetBundleError()}>
                <div class="mt-2 rounded border border-amber-200 bg-amber-50 px-2 py-1.5 text-[10px] text-amber-700 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                  <div class="flex items-start justify-between gap-2">
                    <span>{facetBundleError()}</span>
                    <button
                      type="button"
                      class="shrink-0 font-medium text-amber-700 underline dark:text-amber-200"
                      onClick={() => refetchHistoryFacets()}
                    >
                      Retry
                    </button>
                  </div>
                </div>
              </Show>

              <div class="mt-3 rounded border border-border bg-surface p-3">
                <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                  Event log
                </div>
                <Show
                  when={sortedResourceTimeline().length > 0}
                  fallback={
                    <div class="rounded border border-dashed border-border bg-surface-hover px-2 py-2 text-[10px] text-muted">
                      No events yet.
                    </div>
                  }
                >
                  <div class="space-y-2">
                    <For each={sortedResourceTimeline()}>
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
                                <div class="font-medium text-base-content">
                                  {kindPresentation.label}
                                </div>
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
                            <Show
                              when={change.relatedResources && change.relatedResources.length > 0}
                            >
                              <div class="mt-1 flex flex-wrap items-center gap-1 text-muted">
                                <span>Related:</span>
                                <For each={change.relatedResources ?? []}>
                                  {(relatedResource) => {
                                    const label = resolveResourceLabel(relatedResource);
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
            </div>

            <Show when={hasServiceDetails()}>
              <SupportDisclosure
                title="Service details"
                summary={serviceDetailsSummary()}
                expanded={showServiceDetails()}
                onToggle={() => setShowServiceDetails((value) => !value)}
                showLabel="Show service details"
                hideLabel="Hide service details"
                class="h-full"
                contentClass="mt-3 space-y-3"
                dataTestId="resource-service-details-section"
              >
                <Show when={props.resource.type === 'docker-host'}>
                  <div class="rounded border border-sky-200 bg-sky-50 p-3 dark:border-sky-700 dark:bg-sky-900">
                    <div class="mb-2 flex items-center justify-between gap-2">
                      <div class="text-[11px] font-medium uppercase tracking-wide text-sky-700 dark:text-sky-300">
                        Docker runtime
                      </div>
                      <Show when={dockerHostData()?.runtime}>
                        <span
                          class="max-w-[55%] truncate text-[10px] text-sky-700 dark:text-sky-300"
                          title={dockerHostData()?.runtime}
                        >
                          {dockerHostData()?.runtime}
                        </span>
                      </Show>
                    </div>

                    <div class="space-y-1.5 text-[11px]">
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">Containers</span>
                        <span class="font-medium text-base-content">
                          {formatInteger(dockerContainerCount())}
                        </span>
                      </div>
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">Updates</span>
                        <span
                          class={`font-medium ${dockerUpdatesAvailable() > 0 ? 'text-sky-700 dark:text-sky-300' : 'text-base-content'}`}
                        >
                          {formatInteger(dockerUpdatesAvailable())}
                        </span>
                      </div>
                      <Show when={dockerUpdatesCheckedRelative()}>
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-muted">Checked</span>
                          <span class="font-medium text-base-content">
                            {dockerUpdatesCheckedRelative()}
                          </span>
                        </div>
                      </Show>

                      <Show when={showDockerUpdateControls()}>
                        <div class="space-y-1.5 border-t border-sky-200 pt-2 dark:border-sky-700">
                          <Show when={dockerHostCommand()?.type || dockerHostCommand()?.status}>
                            <div class="rounded border border-sky-200 bg-surface px-2 py-1.5 text-[10px] dark:border-sky-700">
                              <div class="flex items-center justify-between gap-2">
                                <span class="text-muted">Action</span>
                                <span class="font-medium text-base-content">
                                  {formatIdentifierLabel(dockerHostCommand()?.type, {
                                    fallback: 'command',
                                  })}
                                </span>
                              </div>
                              <div class="mt-1 flex items-center justify-between gap-2">
                                <span class="text-muted">State</span>
                                <span
                                  class={`font-medium ${dockerHostCommandActive() ? 'text-sky-700 dark:text-sky-300' : 'text-base-content'}`}
                                >
                                  {formatIdentifierLabel(dockerHostCommand()?.status, {
                                    fallback: 'unknown',
                                  })}
                                </span>
                              </div>
                              <Show when={dockerHostCommand()?.message}>
                                <div
                                  class="mt-1 text-muted truncate"
                                  title={dockerHostCommand()?.message}
                                >
                                  {dockerHostCommand()?.message}
                                </div>
                              </Show>
                              <Show when={dockerHostCommand()?.failureReason}>
                                <div
                                  class="mt-1 text-red-700 dark:text-red-300 truncate"
                                  title={dockerHostCommand()?.failureReason}
                                >
                                  {dockerHostCommand()?.failureReason}
                                </div>
                              </Show>
                            </div>
                          </Show>

                          <Show when={dockerActionError()}>
                            <div class="rounded border border-red-200 bg-red-50 px-2 py-1.5 text-[10px] text-red-700 dark:border-red-700 dark:bg-red-900 dark:text-red-200">
                              {dockerActionError()}
                            </div>
                          </Show>
                          <Show when={dockerActionNote()}>
                            <div class="rounded border border-sky-200 bg-surface px-2 py-1.5 text-[10px] text-base-content dark:border-sky-700">
                              {dockerActionNote()}
                            </div>
                          </Show>

                          <div class="flex flex-wrap items-center gap-2">
                            <button
                              type="button"
                              disabled={
                                dockerActionBusy() ||
                                dockerUpdateActionsLoading() ||
                                dockerHostCommandActive() ||
                                dockerHostSourceId() === null
                              }
                              onClick={async () => {
                                setDockerActionError('');
                                setDockerActionNote('');
                                setConfirmUpdateAll(false);
                                const hostId = dockerHostSourceId();
                                if (!hostId) return;
                                try {
                                  setDockerActionBusy(true);
                                  await MonitoringAPI.checkDockerUpdates(hostId);
                                  setDockerActionNote('Check queued.');
                                } catch (err) {
                                  setDockerActionError(
                                    (err as Error)?.message || 'Failed to queue check',
                                  );
                                } finally {
                                  setDockerActionBusy(false);
                                }
                              }}
                              class="rounded-md border border-border bg-surface px-2.5 py-1 text-[11px] font-semibold text-base-content hover:bg-surface-hover disabled:opacity-60"
                              title={
                                dockerUpdateActionsLoading() ? 'Loading settings...' : undefined
                              }
                            >
                              Check now
                            </button>

                            <button
                              type="button"
                              disabled={
                                dockerActionBusy() ||
                                dockerUpdateActionsLoading() ||
                                dockerUpdateActionsDisabled() ||
                                dockerHostCommandActive() ||
                                dockerHostSourceId() === null ||
                                dockerUpdatesAvailable() <= 0
                              }
                              onClick={async () => {
                                setDockerActionError('');
                                setDockerActionNote('');
                                const hostId = dockerHostSourceId();
                                if (!hostId) return;

                                if (!confirmUpdateAll()) {
                                  setConfirmUpdateAll(true);
                                  setDockerActionNote(
                                    `Click again to update ${dockerUpdatesAvailable()} containers.`,
                                  );
                                  return;
                                }

                                try {
                                  setDockerActionBusy(true);
                                  await MonitoringAPI.updateAllDockerContainers(hostId);
                                  setDockerActionNote('Update queued.');
                                } catch (err) {
                                  setDockerActionError(
                                    (err as Error)?.message || 'Failed to queue update',
                                  );
                                } finally {
                                  setDockerActionBusy(false);
                                  setConfirmUpdateAll(false);
                                }
                              }}
                              class="rounded-md border border-sky-200 bg-sky-600 px-2.5 py-1 text-[11px] font-semibold text-white hover:bg-sky-700 disabled:opacity-60 disabled:hover:bg-sky-600 dark:border-sky-700 dark:bg-sky-600 dark:hover:bg-sky-500 dark:disabled:hover:bg-sky-600"
                              title={
                                dockerUpdateActionsDisabled()
                                  ? 'Updates disabled by server settings.'
                                  : undefined
                              }
                            >
                              {confirmUpdateAll()
                                ? 'Confirm update'
                                : `Update all${dockerUpdatesAvailable() > 0 ? ` (${dockerUpdatesAvailable()})` : ''}`}
                            </button>
                          </div>
                        </div>
                      </Show>

                      <button
                        type="button"
                        onClick={() => setShowDockerUpdateControls((value) => !value)}
                        class="inline-flex items-center rounded-md border border-sky-200 bg-surface px-2.5 py-1 text-[10px] font-medium text-sky-700 transition-colors hover:bg-base dark:border-sky-700 dark:text-sky-300"
                      >
                        {showDockerUpdateControls() ? 'Hide actions' : 'Show actions'}
                      </button>
                    </div>
                  </div>
                </Show>

                <Show when={pbsData()}>
                  {(pbs) => {
                    const connection = getServiceHealthPresentation(
                      props.resource.status,
                      pbs().connectionHealth,
                    );
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
                          <Show when={pbs().uptimeSeconds || props.resource.uptime}>
                            <div class="flex items-center justify-between gap-2">
                              <span class="text-muted">Uptime</span>
                              <span class="font-medium text-base-content">
                                {formatUptime(pbs().uptimeSeconds ?? props.resource.uptime ?? 0)}
                              </span>
                            </div>
                          </Show>
                          <Show when={showPbsJobDetail()}>
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
                                    {formatInteger(pbsJobTotal())}
                                  </div>
                                </div>
                              </div>
                              <details class="rounded border border-indigo-200 bg-surface px-2 py-1.5 dark:border-indigo-700">
                                <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-muted">
                                  <span>Types</span>
                                  <span class="text-muted">{pbsVisibleJobBreakdown().length}</span>
                                </summary>
                                <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-indigo-200 pt-2 text-[10px] dark:border-indigo-700">
                                  <For each={pbsVisibleJobBreakdown()}>
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
                            onClick={() => setShowPbsJobDetail((value) => !value)}
                            class="inline-flex items-center rounded-md border border-indigo-200 bg-surface px-2.5 py-1 text-[10px] font-medium text-indigo-700 transition-colors hover:bg-base dark:border-indigo-700 dark:text-indigo-300"
                          >
                            {showPbsJobDetail() ? 'Hide jobs' : 'Show jobs'}
                          </button>
                        </div>
                      </div>
                    );
                  }}
                </Show>

                <Show when={pmgData()}>
                  {(pmg) => {
                    const connection = getServiceHealthPresentation(
                      props.resource.status,
                      pmg().connectionHealth,
                    );
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
                          <Show when={pmg().uptimeSeconds || props.resource.uptime}>
                            <div class="flex items-center justify-between gap-2">
                              <span class="text-muted">Uptime</span>
                              <span class="font-medium text-base-content">
                                {formatUptime(pmg().uptimeSeconds ?? props.resource.uptime ?? 0)}
                              </span>
                            </div>
                          </Show>
                          <Show when={showPmgMailFlowDetail()}>
                            <div class="space-y-1.5 border-t border-rose-200 pt-2 dark:border-rose-700">
                              <div class="grid grid-cols-2 gap-2">
                                <div class="rounded border border-rose-200 bg-surface px-2 py-1.5 dark:border-rose-700">
                                  <div class="text-[10px] text-muted">Queue</div>
                                  <div
                                    class={`text-sm font-semibold ${pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-base-content'}`}
                                  >
                                    {formatInteger(pmg().queueTotal)}
                                  </div>
                                </div>
                                <div class="rounded border border-rose-200 bg-surface px-2 py-1.5 dark:border-rose-700">
                                  <div class="text-[10px] text-muted">Backlog</div>
                                  <div
                                    class={`text-sm font-semibold ${pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-base-content'}`}
                                  >
                                    {formatInteger(pmgQueueBacklog())}
                                  </div>
                                </div>
                              </div>
                              <Show when={pmg().nodeCount || pmgUpdatedRelative()}>
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
                                  <Show when={pmgUpdatedRelative()}>
                                    <div
                                      class={`flex items-center justify-between gap-2 ${pmg().nodeCount ? 'border-t border-rose-200 pt-1.5 dark:border-rose-700' : ''}`}
                                    >
                                      <span class="text-muted">Updated</span>
                                      <span class="font-medium text-base-content">
                                        {pmgUpdatedRelative()}
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
                                  <For each={pmgVisibleQueueBreakdown()}>
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
                                  <For each={pmgVisibleMailBreakdown()}>
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
                            onClick={() => setShowPmgMailFlowDetail((value) => !value)}
                            class="inline-flex items-center rounded-md border border-rose-200 bg-surface px-2.5 py-1 text-[10px] font-medium text-rose-700 transition-colors hover:bg-base dark:border-rose-700 dark:text-rose-300"
                          >
                            {showPmgMailFlowDetail() ? 'Hide mail flow' : 'Show mail flow'}
                          </button>
                        </div>
                      </div>
                    );
                  }}
                </Show>
              </SupportDisclosure>
            </Show>

            <Show when={hasHostDetails()}>
              <SupportDisclosure
                title="Host details"
                summary={hostDetailSummary()}
                expanded={showHostDetails()}
                onToggle={() => setShowHostDetails((value) => !value)}
                showLabel="Show host details"
                hideLabel="Hide host details"
                class="h-full"
                contentClass="mt-3 flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.375rem)] [&>*]:min-w-[220px] [&>*]:max-w-full [&>*]:overflow-hidden"
                dataTestId="resource-host-details-section"
              >
                <Show when={proxmoxNode()}>
                  {(node) => (
                    <>
                      <SystemInfoCard variant="node" node={node()} />
                      <HardwareCard variant="node" node={node()} />
                      <RootDiskCard node={node()} />
                    </>
                  )}
                </Show>
                <Show when={agentInfo()}>
                  {(agent) => (
                    <>
                      <SystemInfoCard variant="agent" agent={agent()} />
                      <HardwareCard variant="agent" agent={agent()} />
                      <NetworkInterfacesCard interfaces={agent().networkInterfaces} />
                      <DisksCard disks={agent().disks} />
                      <RaidCard arrays={agentMeta()?.raid} />
                      <TemperaturesCard rows={temperatureRows()} />
                    </>
                  )}
                </Show>
              </SupportDisclosure>
            </Show>

            <Show when={hasInvestigationContext()}>
              <SupportDisclosure
                title="Investigation context"
                summary={investigationContextSummary()}
                expanded={showInvestigationContext()}
                onToggle={() => setShowInvestigationContext((value) => !value)}
                showLabel="Show context"
                hideLabel="Hide context"
                class="h-full"
                contentClass="mt-3 space-y-3"
                dataTestId="resource-investigation-context"
              >
                <Show when={resourceIntelligence()}>
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
                        resolveResourceLabel={resolveResourceLabel}
                        maxChanges={1}
                        compact
                      />
                      <Show when={hasCorrelationContext()}>
                        <div data-testid="resource-correlation-context" class="space-y-1.5">
                          <div class="flex flex-wrap items-center justify-between gap-2">
                            <span class="text-[10px] font-medium uppercase tracking-wide text-base-content">
                              Correlation context
                            </span>
                            <button
                              type="button"
                              onClick={() => setShowCorrelationContext((value) => !value)}
                              class="inline-flex items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-surface-hover"
                            >
                              {showCorrelationContext() ? 'Hide correlations' : 'Show correlations'}
                            </button>
                          </div>

                          <Show when={showCorrelationContext()}>
                            <div class="pt-1">
                              <ResourceCorrelationSummary
                                title="Correlation context"
                                dependencies={resourceDependencies()}
                                dependents={resourceDependents()}
                                correlations={resourceCorrelations()}
                                resolveResourceLabel={resolveResourceLabel}
                                showLastSeen
                              />
                            </div>
                          </Show>
                        </div>
                      </Show>
                    </div>
                  )}
                </Show>

                <Show when={hasGovernanceData()}>
                  <div class="space-y-1.5 text-[11px]">
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted uppercase tracking-wide">Data Governance</span>
                    </div>
                    <Show when={props.resource.policy}>
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">Sensitivity</span>
                        <span class="font-semibold text-base-content">
                          {getResourceSensitivityLabel(props.resource.policy?.sensitivity)}
                        </span>
                      </div>
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">Routing</span>
                        <span class="font-semibold text-base-content">
                          {getResourceRoutingScopeLabel(props.resource.policy?.routing.scope)}
                        </span>
                      </div>
                    </Show>
                    <Show when={policyRedactions().length > 0 || governanceSummary()}>
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-muted">Redactions</span>
                        <span class="font-semibold text-base-content">
                          {policyRedactions().length}
                        </span>
                      </div>
                    </Show>
                    <Show when={policyRedactions().length > 0}>
                      <div class="flex flex-col gap-1">
                        <span class="text-muted">Redaction labels</span>
                        <div class="flex flex-wrap gap-1">
                          <For each={policyRedactions()}>
                            {(label) => (
                              <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                                {label}
                              </span>
                            )}
                          </For>
                        </div>
                      </div>
                    </Show>
                    <Show when={governanceSummary()}>
                      <div class="flex flex-col gap-1">
                        <span class="text-muted">AI-Safe Summary</span>
                        <div class="rounded border border-border bg-surface-hover px-2 py-1.5 text-[10px] text-base-content">
                          {governanceSummary()}
                        </div>
                      </div>
                    </Show>
                  </div>
                </Show>
              </SupportDisclosure>
            </Show>
          </div>

          <Show when={discoveryConfig()}>
            {(config) => (
              <div class="space-y-2">
                <WebInterfaceUrlField
                  metadataKind={config().metadataKind}
                  metadataId={config().metadataId}
                  targetLabel={config().targetLabel}
                />

                <SupportDisclosure
                  title="Discovery context"
                  summary={discoveryContextSummary()}
                  expanded={showDiscoveryContext()}
                  onToggle={() => setShowDiscoveryContext((value) => !value)}
                  showLabel="Show metadata"
                  hideLabel="Hide metadata"
                  class="h-full"
                  dataTestId="resource-discovery-context"
                >
                  <Suspense
                    fallback={
                      <div class="flex items-center justify-center py-8">
                        <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                        <span class="ml-2 text-sm text-muted">
                          {getDiscoveryLoadingState().text}
                        </span>
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
      </div>

      {/* PMG Mail Tab */}
      <div class={activeTab() === 'mail' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'mail'}>
          <Show
            when={props.resource.type === 'pmg'}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('mail')} />
            }
          >
            <PMGInstanceDrawer
              resourceId={props.resource.id}
              resourceName={displayName() || props.resource.id}
            />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Namespaces Tab */}
      <div
        class={activeTab() === 'namespaces' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'namespaces'}>
          <Show
            when={props.resource.type === 'k8s-cluster'}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('namespaces')} />
            }
          >
            <K8sNamespacesDrawer
              cluster={kubernetesClusterName()}
              onOpenDeployments={(ns) => {
                setK8sDeploymentsPrefillNamespace((ns || '').trim());
                setActiveTab('deployments');
              }}
            />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Deployments Tab */}
      <div
        class={activeTab() === 'deployments' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'deployments'}>
          <Show
            when={props.resource.type === 'k8s-cluster'}
            fallback={
              <TabAvailabilityNotice
                message={getSpecializedTabAvailabilityMessage('deployments')}
              />
            }
          >
            <K8sDeploymentsDrawer
              cluster={kubernetesClusterName()}
              initialNamespace={k8sDeploymentsPrefillNamespace() || null}
            />
          </Show>
        </Show>
      </div>

      {/* Docker Swarm Tab */}
      <div class={activeTab() === 'swarm' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'swarm'}>
          <Show
            when={props.resource.type === 'docker-host' && dockerSwarmClusterKey()}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('swarm')} />
            }
          >
            <SwarmServicesDrawer cluster={dockerSwarmClusterKey()} swarm={dockerSwarmInfo()} />
          </Show>
        </Show>
      </div>

      {/* Debug Tab */}
      <Show when={debugEnabled()}>
        <div class={activeTab() === 'debug' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
          <div class="flex items-center justify-between gap-3">
            <div class="text-xs text-muted">
              Debug mode is enabled via localStorage (<code>pulse_debug_mode</code>).
            </div>
            <button
              type="button"
              onClick={handleCopyJson}
              class="rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              {copied() ? 'Copied' : 'Copy JSON'}
            </button>
          </div>

          <div class="mt-3 space-y-4">
            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Unified Resource
              </div>
              <pre class="max-h-[280px] overflow-auto rounded-md bg-base p-3 text-[11px] text-base-content">
                {JSON.stringify(props.resource, null, 2)}
              </pre>
            </div>

            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Identity Matching
              </div>
              <pre class="max-h-[220px] overflow-auto rounded-md bg-base p-3 text-[11px] text-base-content">
                {JSON.stringify(
                  {
                    identity: props.resource.identity,
                    matchInfo: identityMatchInfo(),
                  },
                  null,
                  2,
                )}
              </pre>
            </div>

            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Sources
              </div>
              <div class="space-y-2">
                <For each={sourceSections()}>
                  {(section) => {
                    const status = sourceStatus()[section.id];
                    const lastSeenText = formatRelativeTime(status?.lastSeen);
                    return (
                      <details class="rounded-md border border-border bg-surface p-3">
                        <summary class="flex cursor-pointer list-none items-center justify-between text-sm font-medium text-base-content">
                          <span>{section.label}</span>
                          <span class="text-[11px] text-muted">
                            {status?.status ?? 'unknown'}
                            {lastSeenText ? ` • ${lastSeenText}` : ''}
                          </span>
                        </summary>
                        <Show when={status?.error}>
                          <div class="mt-2 text-[11px] text-amber-600 dark:text-amber-300">
                            {status?.error}
                          </div>
                        </Show>
                        <pre class="mt-3 max-h-[220px] overflow-auto rounded-md bg-base p-3 text-[11px] text-base-content">
                          {JSON.stringify(section.payload ?? {}, null, 2)}
                        </pre>
                      </details>
                    );
                  }}
                </For>
              </div>
            </div>
          </div>
        </div>
      </Show>

      <Show when={hasMergedSources()}>
        <div class="flex items-center justify-end">
          <button
            type="button"
            onClick={() => setShowReportModal(true)}
            class="text-xs font-medium transition-colors hover:text-muted"
          >
            Split merged resource
          </button>
        </div>
      </Show>

      <ReportMergeModal
        isOpen={showReportModal()}
        resourceId={props.resource.id}
        resourceName={displayName()}
        sources={mergedSources()}
        onClose={() => setShowReportModal(false)}
      />
    </div>
  );
};

export const ResourceDetailDrawer: Component<ResourceDetailDrawerProps> = (props) => {
  return (
    <DrawerContent
      resource={props.resource}
      onClose={props.onClose}
      resolveResourceLabel={props.resolveResourceLabel}
    />
  );
};

export default ResourceDetailDrawer;
