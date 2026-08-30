import { For, Show, Suspense } from 'solid-js';
import type { Component } from 'solid-js';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import type {
  Resource,
  ResourceChangeKind,
  ResourceChangeSourceAdapter,
  ResourceChangeSourceType,
} from '@/types/resource';
import { formatUptime, formatRelativeTime } from '@/utils/format';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { TemperaturesCard } from '@/components/shared/cards/TemperaturesCard';
import { RaidCard } from '@/components/shared/cards/RaidCard';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { FormSelect } from '@/components/shared/FormSelect';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import {
  WEB_INTERFACE_LINK_COLOR_CLASS,
  WebInterfaceLink,
} from '@/components/shared/WebInterfaceLink';
import { getAllFilterOptionLabel } from '@/components/shared/filterOptionPresentation';
import { getServiceHealthPresentation } from '@/utils/serviceHealthPresentation';
import { ResourceCorrelationSummary } from './ResourceCorrelationSummary';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import { InlineResourceSummaryTables } from './ResourceDetailSummary';
import { ResourceInvestigationContextTables } from './ResourceInvestigationContextTables';
import { DetailSectionTable } from '@/components/shared/DetailSectionTable';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
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
import { isPulseAgentPlatformResource } from '@/utils/agentResources';
import { formatInteger } from './resourceDetailMappers';
import { buildPbsJobHealthEvidenceModel } from './resourceDetailDrawerServiceModel';
import { AvailabilityProbeStatusCards } from './AvailabilityProbeStatusCard';
import { ResourceDetailDrawerSupportDisclosure as SupportDisclosure } from './ResourceDetailDrawerSupportDisclosure';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';
import type { ResourceDetailDrawerPresentation } from './resourceDetailDrawerPresentation';

interface ResourceDetailDrawerOverviewTabProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
  presentation?: ResourceDetailDrawerPresentation;
}

const hasMetadataEntries = (value?: Record<string, unknown> | null): boolean =>
  Boolean(value && Object.keys(value).length > 0);

const TrueNASDetailsDisclosure: Component<{
  drawer: UseResourceDetailDrawerStateResult;
  class?: string;
  contentClass?: string;
}> = (props) => (
  <SupportDisclosure
    title="TrueNAS"
    summary={props.drawer.trueNASDetailsSummary()}
    expanded={props.drawer.showTrueNASDetails()}
    onToggle={() => props.drawer.setShowTrueNASDetails((value) => !value)}
    showLabel="Show TrueNAS"
    hideLabel="Hide TrueNAS"
    class={props.class}
    contentClass={props.contentClass ?? 'mt-3 space-y-3'}
    dataTestId="resource-truenas-details-section"
  >
    <DetailSectionTable sections={props.drawer.trueNASDetailSections()} />
  </SupportDisclosure>
);

const KubernetesDetailsDisclosure: Component<{
  drawer: UseResourceDetailDrawerStateResult;
  class?: string;
  contentClass?: string;
}> = (props) => (
  <SupportDisclosure
    title="Kubernetes"
    summary={props.drawer.kubernetesDetailsSummary()}
    expanded={props.drawer.showKubernetesDetails()}
    onToggle={() => props.drawer.setShowKubernetesDetails((value) => !value)}
    showLabel="Show Kubernetes"
    hideLabel="Hide Kubernetes"
    class={props.class}
    contentClass={props.contentClass ?? 'mt-3 space-y-3'}
    dataTestId="resource-kubernetes-details-section"
  >
    <DetailSectionTable sections={props.drawer.kubernetesDetailSections()} />
  </SupportDisclosure>
);

const VMwareDetailsDisclosure: Component<{
  drawer: UseResourceDetailDrawerStateResult;
  class?: string;
  contentClass?: string;
}> = (props) => (
  <SupportDisclosure
    title="vSphere"
    summary={props.drawer.vmwareDetailsSummary()}
    expanded={props.drawer.showVMwareDetails()}
    onToggle={() => props.drawer.setShowVMwareDetails((value) => !value)}
    showLabel="Show vSphere"
    hideLabel="Hide vSphere"
    class={props.class}
    contentClass={props.contentClass ?? 'mt-3 space-y-3'}
    dataTestId="resource-vmware-details-section"
  >
    <DetailSectionTable sections={props.drawer.vmwareDetailSections()} />
  </SupportDisclosure>
);

const machineHostDetailsTitle = (resource: Resource): string =>
  isPulseAgentPlatformResource(resource) ? 'Machine' : 'Host';

const machineHostDetailsNoun = (resource: Resource): string =>
  isPulseAgentPlatformResource(resource) ? 'machine' : 'host';

const HostDetailsDisclosure: Component<{
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
  class?: string;
  contentClass?: string;
}> = (props) => {
  const noun = () => machineHostDetailsNoun(props.resource);

  return (
    <SupportDisclosure
      title={machineHostDetailsTitle(props.resource)}
      summary={props.drawer.hostDetailSummary()}
      expanded={props.drawer.showHostDetails()}
      onToggle={() => props.drawer.setShowHostDetails((value) => !value)}
      showLabel={isPulseAgentPlatformResource(props.resource) ? 'Show details' : `Show ${noun()}`}
      hideLabel={isPulseAgentPlatformResource(props.resource) ? 'Hide details' : `Hide ${noun()}`}
      class={props.class}
      contentClass={
        props.contentClass ??
        'mt-3 flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.375rem)] [&>*]:min-w-[220px] [&>*]:max-w-full [&>*]:overflow-hidden'
      }
      dataTestId="resource-host-details-section"
    >
      <Show when={props.drawer.proxmoxNode()}>
        {(node) => (
          <>
            <SystemInfoCard variant="node" node={node()} />
            <HardwareCard variant="node" node={node()} />
            <RootDiskCard node={node()} />
            <Show when={!props.drawer.agentInfo()?.networkInterfaces?.length}>
              <NetworkInterfacesCard interfaces={props.drawer.proxmoxNetworkInterfaces()} />
            </Show>
          </>
        )}
      </Show>
      <Show when={props.drawer.agentInfo()}>
        {(agent) => (
          <>
            <SystemInfoCard variant="agent" agent={agent()} />
            <HardwareCard variant="agent" agent={agent()} />
            <NetworkInterfacesCard interfaces={agent().networkInterfaces} />
            <DisksCard disks={agent().disks} />
            <RaidCard arrays={props.drawer.agentMeta()?.raid} />
            <TemperaturesCard rows={props.drawer.temperatureRows()} title="Thermals" />
            <TemperaturesCard rows={props.drawer.customSensorRows()} title="Custom Metrics" />
          </>
        )}
      </Show>
    </SupportDisclosure>
  );
};

const pbsEvidenceBadgeClass = (tone: string): string => {
  switch (tone) {
    case 'danger':
      return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-700 dark:bg-rose-950/40 dark:text-rose-300';
    case 'warning':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-300';
    case 'success':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300';
    case 'info':
      return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-700 dark:bg-sky-950/40 dark:text-sky-300';
    default:
      return 'border-border bg-surface-hover text-muted';
  }
};

const timelineKindOptions: Array<{ label: string; value: ResourceChangeKind | '' }> = [
  { label: getAllFilterOptionLabel('kinds'), value: '' },
  ...RESOURCE_CHANGE_KIND_ORDER.map((kind) => ({
    label: getResourceChangeKindPresentation(kind).label,
    value: kind,
  })),
];

const timelineSourceTypeOptions: Array<{ label: string; value: ResourceChangeSourceType | '' }> = [
  { label: getAllFilterOptionLabel('sources'), value: '' },
  ...RESOURCE_CHANGE_SOURCE_TYPE_ORDER.map((sourceType) => ({
    label: getResourceChangeSourceTypePresentation(sourceType).label,
    value: sourceType,
  })),
];

const timelineSourceAdapterOptions: Array<{
  label: string;
  value: ResourceChangeSourceAdapter | '';
}> = [
  { label: getAllFilterOptionLabel('adapters'), value: '' },
  ...RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER.map((sourceAdapter) => ({
    label: getResourceChangeSourceAdapterPresentation(sourceAdapter).label,
    value: sourceAdapter,
  })),
];

export const ResourceAccessDisclosure: Component<{
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
  class?: string;
}> = (props) => {
  const customUrl = () => (props.resource.customUrl ?? '').trim();

  return (
    <SupportDisclosure
      title="Access"
      summary={props.drawer.accessSummary()}
      expanded={props.drawer.showAccessContext()}
      onToggle={() => props.drawer.setShowAccessContext((value) => !value)}
      showLabel="Show access"
      hideLabel="Hide access"
      class={props.class}
      contentClass="mt-3 space-y-3"
      dataTestId="resource-access-section"
      headerExtra={
        <Show when={customUrl()}>
          {(href) => (
            <WebInterfaceLink
              url={href()}
              ariaLabel={`Open web interface for ${props.resource.name}`}
              invalidAriaLabel={`Web interface URL for ${props.resource.name} is invalid`}
              class={`inline-flex min-w-0 max-w-[260px] items-center gap-1 text-[11px] font-medium ${WEB_INTERFACE_LINK_COLOR_CLASS}`}
              title={`Open ${href()}`}
            >
              <span class="truncate" data-testid="resource-access-open-url">
                {href()}
              </span>
              <ExternalLinkIcon class="h-3 w-3 shrink-0" aria-hidden="true" />
            </WebInterfaceLink>
          )}
        </Show>
      }
    >
      <Show when={props.drawer.relatedLinks().length > 0}>
        <div class="space-y-1">
          <div class="text-[10px] font-medium uppercase tracking-wide text-base-content">Links</div>
          <div class="flex flex-wrap gap-2">
            <For each={props.drawer.relatedLinks()}>
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

      <Show when={props.drawer.discoveryConfig()}>
        {(config) => (
          <div class="space-y-3">
            <WebInterfaceUrlField
              metadataKind={config().metadataKind}
              metadataId={config().metadataId}
              targetLabel={config().targetLabel}
              title="Web interface"
              customUrl={props.resource.customUrl}
              discoveryLoading={props.drawer.discoveryLoading()}
              suggestedUrl={props.drawer.discoveryIdentifiedSummary()?.suggestedUrl}
              suggestedUrlReasonText={
                props.drawer.discoveryIdentifiedSummary()?.suggestedUrlReasonText
              }
              suggestedUrlReasonTitle={
                props.drawer.discoveryIdentifiedSummary()?.suggestedUrlReasonTitle
              }
              suggestedUrlDiagnostic={
                props.drawer.discoveryIdentifiedSummary()?.suggestedUrlDiagnostic
              }
              embedded
            />

            <Show when={props.drawer.discoveryFeatureEnabled() && !props.drawer.hasDiscoveryTab()}>
              <div
                class="space-y-2 border-t border-border pt-3"
                data-testid="resource-access-analysis"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div class="text-[10px] font-medium uppercase tracking-wide text-base-content">
                      Analysis
                    </div>
                    <Show when={props.drawer.discoveryContextSummary()}>
                      <div class="mt-1 text-[10px] text-base-content">
                        {props.drawer.discoveryContextSummary()}
                      </div>
                    </Show>
                  </div>
                  <button
                    type="button"
                    onClick={() => props.drawer.setShowDiscoveryContext((value) => !value)}
                    class="inline-flex items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-base"
                  >
                    {props.drawer.showDiscoveryContext() ? 'Hide analysis' : 'Open analysis'}
                  </button>
                </div>

                <Show when={props.drawer.showDiscoveryContext()}>
                  <Suspense fallback={<DiscoveryLoadingFallback />}>
                    <DiscoveryTab
                      resourceType={config().resourceType}
                      agentId={config().agentId}
                      resourceId={config().resourceId}
                      hostname={config().hostname}
                      canonicalResourceId={props.resource.id}
                      commandsEnabled={props.drawer.agentMeta()?.commandsEnabled}
                    />
                  </Suspense>
                </Show>
              </div>
            </Show>
          </div>
        )}
      </Show>
    </SupportDisclosure>
  );
};

export const ResourceDetailDrawerOverviewTab: Component<ResourceDetailDrawerOverviewTabProps> = (
  props,
) => {
  const { resource, drawer } = props;
  const showPlatformId = shouldShowResourcePlatformId(resource);
  const pbsJobHealthEvidence = () => buildPbsJobHealthEvidenceModel(drawer.pbsData());
  const compactTableRow = () => props.presentation === 'table-row';
  const shouldRenderChangeHistorySection = () =>
    !compactTableRow() ||
    drawer.hasTimelineFilters() ||
    drawer.sortedResourceTimeline().length > 0 ||
    drawer.resourceTimelineCount() > 0 ||
    Boolean(drawer.facetBundleError());
  const attentionItems = () => {
    const items = (resource.alerts ?? []).map((alert) => ({
      id: alert.id,
      message: alert.message,
      severity: alert.level,
    }));
    const healthIssue = drawer.healthIssue();
    if (healthIssue && !items.some((item) => item.message === healthIssue.primary)) {
      const severity = /critical|failed|faulted|error/i.test(resource.status)
        ? 'critical'
        : 'warning';
      items.unshift(
        ...[healthIssue.primary, ...healthIssue.details].map((message, index) => ({
          id: `resource-health-issue-${index}`,
          message,
          severity,
        })),
      );
    }
    return items;
  };

  return (
    <div class="space-y-3">
      <DrawerAttentionSection items={attentionItems()} />
      <TechnicalDetailsSection dataTestId="resource-technical-details">
        <InlineResourceSummaryTables
          resource={resource}
          drawer={drawer}
          showPlatformId={showPlatformId}
          content="all"
          dataTestId="resource-technical-summary-section"
        />
      </TechnicalDetailsSection>

      <Show when={resource.availability || (resource.availabilityChecks?.length ?? 0) > 0}>
        <div class="flex flex-wrap gap-3 [&>*]:min-w-[240px] [&>*]:flex-1">
          <AvailabilityProbeStatusCards
            availability={resource.availability}
            checks={resource.availabilityChecks}
          />
        </div>
      </Show>

      <div data-testid="resource-secondary-sections" class="space-y-3">
        <Show when={shouldRenderChangeHistorySection()}>
          <InfoCardFrame data-testid="resource-change-history-section" class="w-full">
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
                <div class="mt-1 flex flex-wrap justify-end gap-2">
                  <Show when={drawer.hasTimelineFilters()}>
                    <div class="text-blue-700 dark:text-blue-300">Change filters active</div>
                  </Show>
                  <Show
                    when={!drawer.showHistoryFilters() && !drawer.hasTimelineFilters()}
                    fallback={
                      <Show when={drawer.showHistoryFilters() && !drawer.hasTimelineFilters()}>
                        <button
                          type="button"
                          class="rounded-md border border-border bg-surface-hover px-2.5 py-1 text-[10px] font-semibold text-base-content hover:bg-surface"
                          onClick={() => drawer.setShowHistoryFilters(false)}
                        >
                          Hide filters
                        </button>
                      </Show>
                    }
                  >
                    <button
                      type="button"
                      class="rounded-md border border-border bg-surface-hover px-2.5 py-1 text-[10px] font-semibold text-base-content hover:bg-surface"
                      onClick={() => drawer.setShowHistoryFilters(true)}
                    >
                      Filter history
                    </button>
                  </Show>
                </div>
              </div>
            </div>
            <Show when={drawer.showHistoryFilters() || drawer.hasTimelineFilters()}>
              <div class="mt-3 space-y-2">
                <FormSelect
                  label="Change kind"
                  fieldBaseClass="space-y-1 text-[10px]"
                  labelClass="text-muted"
                  selectBaseClass="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
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
                </FormSelect>
                <FormSelect
                  label="Source type"
                  fieldBaseClass="space-y-1 text-[10px]"
                  labelClass="text-muted"
                  selectBaseClass="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
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
                </FormSelect>
                <FormSelect
                  label="Source adapter"
                  fieldBaseClass="space-y-1 text-[10px]"
                  labelClass="text-muted"
                  selectBaseClass="w-full rounded border border-border bg-base px-2 py-1 text-[11px] text-base-content"
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
                </FormSelect>

                <Show when={drawer.hasTimelineFilters()}>
                  <div class="flex justify-end">
                    <button
                      type="button"
                      class="rounded-md border border-border bg-surface-hover px-2.5 py-1 text-[10px] font-semibold text-base-content hover:bg-surface"
                      onClick={() => {
                        drawer.setTimelineKindFilter('');
                        drawer.setTimelineSourceTypeFilter('');
                        drawer.setTimelineSourceAdapterFilter('');
                        drawer.setShowHistoryFilters(false);
                      }}
                    >
                      Clear filters
                    </button>
                  </div>
                </Show>
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
                          <InfoCardKeyValueRow
                            label="Confidence"
                            value={formatConfidenceLabel(change.confidence)}
                          />
                          <InfoCardKeyValueRow
                            label="Adapter"
                            value={sourceAdapterPresentation?.label || '—'}
                          />
                          <Show when={change.actor}>
                            <InfoCardKeyValueRow
                              label="Actor"
                              value={change.actor}
                              valueClass="break-words"
                            />
                          </Show>
                          <Show when={change.from || change.to}>
                            <InfoCardKeyValueRow
                              label="Transition"
                              value={`${change.from || '—'} → ${change.to || '—'}`}
                              valueClass="break-words"
                            />
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
                                return (
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
          </InfoCardFrame>
        </Show>

        <Show when={drawer.hasCorrelationContext()}>
          <ResourceCorrelationSummary
            dataTestId="resource-relationship-map-section"
            title="Relationship map"
            relationships={drawer.resourceRelationships()}
            dependencies={drawer.resourceDependencies()}
            dependents={drawer.resourceDependents()}
            correlations={drawer.resourceCorrelations()}
            resolveResourceLabel={drawer.resolveResourceLabel}
            showLastSeen
          />
        </Show>

        <Show
          when={
            drawer.hasServiceDetails() ||
            drawer.hasVMwareDetails() ||
            drawer.hasTrueNASDetails() ||
            drawer.hasKubernetesDetails() ||
            drawer.hasHostDetails() ||
            drawer.hasInvestigationContext()
          }
        >
          <TechnicalDetailsDisclosure
            title="Platform details"
            subtitle="Provider, service, and host context"
            dataTestId="resource-platform-details"
          >
            <div
              data-testid="resource-support-sections"
              class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.375rem)] [&>*]:min-w-[260px] [&>*]:max-w-full [&>*]:overflow-hidden"
            >
              <Show when={drawer.hasTrueNASDetails()}>
                <TrueNASDetailsDisclosure drawer={drawer} class="h-full" />
              </Show>

              <Show when={drawer.hasKubernetesDetails()}>
                <KubernetesDetailsDisclosure drawer={drawer} class="h-full" />
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
                  <ResourceInvestigationContextTables resource={resource} drawer={drawer} />
                </SupportDisclosure>
              </Show>

              <Show when={drawer.hasServiceDetails()}>
                <SupportDisclosure
                  title="Service"
                  summary={drawer.serviceDetailsSummary()}
                  expanded={drawer.showServiceDetails()}
                  onToggle={() => drawer.setShowServiceDetails((value) => !value)}
                  showLabel="Show service"
                  hideLabel="Hide service"
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
                        <InfoCardKeyValueRow
                          label="Containers"
                          value={formatInteger(drawer.dockerContainerCount())}
                        />
                        <InfoCardKeyValueRow
                          label="Updates"
                          value={formatInteger(drawer.dockerUpdatesAvailable())}
                          valueClass={
                            drawer.dockerUpdatesAvailable() > 0
                              ? 'text-sky-700 dark:text-sky-300'
                              : ''
                          }
                        />
                        <Show when={drawer.dockerUpdatesCheckedRelative()}>
                          <InfoCardKeyValueRow
                            label="Checked"
                            value={drawer.dockerUpdatesCheckedRelative()}
                          />
                        </Show>

                        <Show when={drawer.showDockerUpdateControls()}>
                          <div class="space-y-1.5 border-t border-sky-200 pt-2 dark:border-sky-700">
                            <Show
                              when={
                                drawer.dockerHostCommand()?.type ||
                                drawer.dockerHostCommand()?.status
                              }
                            >
                              <div class="rounded border border-sky-200 bg-surface px-2 py-1.5 text-[10px] dark:border-sky-700">
                                <InfoCardKeyValueRow
                                  label="Action"
                                  value={formatIdentifierLabel(drawer.dockerHostCommand()?.type, {
                                    fallback: 'command',
                                  })}
                                />
                                <InfoCardKeyValueRow
                                  class="mt-1"
                                  label="State"
                                  value={formatIdentifierLabel(drawer.dockerHostCommand()?.status, {
                                    fallback: 'unknown',
                                  })}
                                  valueClass={
                                    drawer.dockerHostCommandActive()
                                      ? 'text-sky-700 dark:text-sky-300'
                                      : ''
                                  }
                                />
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
                                  drawer.dockerUpdateActionsLoading()
                                    ? 'Loading settings...'
                                    : undefined
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
                      const connection = getServiceHealthPresentation(
                        resource.status,
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
                            <InfoCardKeyValueRow
                              label="State"
                              value={connection.label}
                              valueClass={connection.text}
                            />
                            <Show when={pbs().version}>
                              <InfoCardKeyValueRow label="Version" value={pbs().version} />
                            </Show>
                            <Show when={pbs().uptimeSeconds || resource.uptime}>
                              <InfoCardKeyValueRow
                                label="Uptime"
                                value={formatUptime(pbs().uptimeSeconds ?? resource.uptime ?? 0)}
                              />
                            </Show>
                            <Show when={drawer.pbsActiveTaskCount() > 0}>
                              <InfoCardKeyValueRow
                                label="Active tasks"
                                value={formatInteger(drawer.pbsActiveTaskCount())}
                                valueClass="text-emerald-700 dark:text-emerald-300"
                              />
                            </Show>
                            <Show when={drawer.showPbsJobDetail()}>
                              <div class="space-y-1.5 border-t border-indigo-200 pt-2 dark:border-indigo-700">
                                <div class="grid grid-cols-2 gap-2 md:grid-cols-3">
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
                                  <Show when={drawer.pbsActiveTaskCount() > 0}>
                                    <div class="rounded border border-emerald-200 bg-emerald-50 px-2 py-1.5 dark:border-emerald-700 dark:bg-emerald-950/40">
                                      <div class="text-[10px] text-muted">Active</div>
                                      <div class="text-sm font-semibold text-emerald-700 dark:text-emerald-300">
                                        {formatInteger(drawer.pbsActiveTaskCount())}
                                      </div>
                                    </div>
                                  </Show>
                                </div>
                                <Show when={drawer.pbsActiveTaskCount() > 0}>
                                  <div
                                    data-testid="pbs-active-tasks"
                                    class="rounded border border-emerald-200 bg-surface px-2 py-1.5 dark:border-emerald-700"
                                  >
                                    <div class="flex items-center justify-between gap-2">
                                      <span class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                        Active tasks
                                      </span>
                                      <span class="text-[10px] font-semibold text-emerald-700 dark:text-emerald-300">
                                        {formatInteger(drawer.pbsActiveTaskCount())}
                                      </span>
                                    </div>
                                    <div class="mt-2 space-y-2 border-t border-emerald-200 pt-2 dark:border-emerald-700">
                                      <For each={drawer.pbsActiveTasks()}>
                                        {(task) => (
                                          <div class="flex items-start justify-between gap-2 text-[10px]">
                                            <div class="min-w-0">
                                              <div
                                                class="truncate font-medium text-base-content"
                                                title={task.label}
                                              >
                                                {task.label}
                                              </div>
                                              <Show when={task.context}>
                                                <div
                                                  class="truncate text-muted"
                                                  title={task.context ?? undefined}
                                                >
                                                  {task.context}
                                                </div>
                                              </Show>
                                              <Show when={task.error}>
                                                <div
                                                  class="truncate text-rose-700 dark:text-rose-300"
                                                  title={task.error}
                                                >
                                                  {task.error}
                                                </div>
                                              </Show>
                                            </div>
                                            <span class="shrink-0 rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 font-medium text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">
                                              {task.statusLabel}
                                            </span>
                                          </div>
                                        )}
                                      </For>
                                    </div>
                                  </div>
                                </Show>
                                <Show when={pbsJobHealthEvidence().evidenceCount > 0}>
                                  <div
                                    data-testid="pbs-job-health-evidence"
                                    class="rounded border border-indigo-200 bg-surface px-2 py-1.5 dark:border-indigo-700"
                                  >
                                    <div class="flex flex-wrap items-center justify-between gap-2">
                                      <span class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                        Job health evidence
                                      </span>
                                      <span class="text-[10px] font-semibold text-indigo-700 dark:text-indigo-300">
                                        {pbsJobHealthEvidence().countLabel}
                                      </span>
                                    </div>
                                    <div class="mt-2 space-y-2 border-t border-indigo-200 pt-2 dark:border-indigo-700">
                                      <For each={pbsJobHealthEvidence().entries}>
                                        {(entry) => (
                                          <div class="min-w-0 rounded border border-border bg-surface-hover px-2 py-1.5 text-[10px]">
                                            <div class="flex flex-wrap items-start justify-between gap-2">
                                              <div class="min-w-0">
                                                <div
                                                  class="truncate font-medium text-base-content"
                                                  title={entry.label}
                                                >
                                                  {entry.label}
                                                </div>
                                                <div
                                                  class="truncate text-muted"
                                                  title={entry.sourceLabel}
                                                >
                                                  {entry.sourceLabel}
                                                </div>
                                              </div>
                                              <div class="flex max-w-full flex-wrap justify-end gap-1">
                                                <For each={entry.badges}>
                                                  {(badge) => (
                                                    <span
                                                      class={`shrink-0 rounded-full border px-2 py-0.5 font-medium ${pbsEvidenceBadgeClass(
                                                        badge.tone,
                                                      )}`}
                                                    >
                                                      {badge.label}
                                                    </span>
                                                  )}
                                                </For>
                                              </div>
                                            </div>
                                            <div class="mt-1 grid gap-1 text-muted sm:grid-cols-2">
                                              <Show when={entry.context}>
                                                <span
                                                  class="truncate"
                                                  title={entry.context ?? undefined}
                                                >
                                                  {entry.context}
                                                </span>
                                              </Show>
                                              <Show when={entry.stateLabel}>
                                                <span
                                                  class="truncate"
                                                  title={entry.stateLabel ?? undefined}
                                                >
                                                  State:{' '}
                                                  <span class="font-medium text-base-content">
                                                    {entry.stateLabel}
                                                  </span>
                                                </span>
                                              </Show>
                                              <Show when={entry.freshnessLabel}>
                                                <span
                                                  class="truncate"
                                                  title={entry.freshnessLabel ?? undefined}
                                                >
                                                  {entry.freshnessLabel}
                                                </span>
                                              </Show>
                                              <Show when={entry.postureReason}>
                                                <span
                                                  class="truncate text-amber-700 dark:text-amber-300"
                                                  title={entry.postureReason ?? undefined}
                                                >
                                                  {entry.postureReason}
                                                </span>
                                              </Show>
                                              <Show when={entry.error}>
                                                <span
                                                  class="truncate text-rose-700 dark:text-rose-300"
                                                  title={entry.error ?? undefined}
                                                >
                                                  {entry.error}
                                                </span>
                                              </Show>
                                            </div>
                                          </div>
                                        )}
                                      </For>
                                    </div>
                                  </div>
                                </Show>
                                <details class="rounded border border-indigo-200 bg-surface px-2 py-1.5 dark:border-indigo-700">
                                  <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-muted">
                                    <span>Types</span>
                                    <span class="text-muted">
                                      {drawer.pbsVisibleJobBreakdown().length}
                                    </span>
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
                      const connection = getServiceHealthPresentation(
                        resource.status,
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
                            <InfoCardKeyValueRow
                              label="State"
                              value={connection.label}
                              valueClass={connection.text}
                            />
                            <Show when={pmg().version}>
                              <InfoCardKeyValueRow label="Version" value={pmg().version} />
                            </Show>
                            <Show when={pmg().uptimeSeconds || resource.uptime}>
                              <InfoCardKeyValueRow
                                label="Uptime"
                                value={formatUptime(pmg().uptimeSeconds ?? resource.uptime ?? 0)}
                              />
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
                                      <InfoCardKeyValueRow
                                        label="Nodes"
                                        value={formatInteger(pmg().nodeCount)}
                                      />
                                    </Show>
                                    <Show when={drawer.pmgUpdatedRelative()}>
                                      <InfoCardKeyValueRow
                                        class={
                                          pmg().nodeCount
                                            ? 'border-t border-rose-200 pt-1.5 dark:border-rose-700'
                                            : ''
                                        }
                                        label="Updated"
                                        value={drawer.pmgUpdatedRelative()}
                                      />
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
                                        <InfoCardKeyValueRow
                                          label={entry.label}
                                          value={formatInteger(entry.value)}
                                          valueClass={
                                            entry.warn ? 'text-amber-600 dark:text-amber-400' : ''
                                          }
                                        />
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
                                        <InfoCardKeyValueRow
                                          label={entry.label}
                                          value={formatInteger(entry.value)}
                                        />
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

              <Show when={drawer.hasVMwareDetails()}>
                <VMwareDetailsDisclosure drawer={drawer} class="h-full" />
              </Show>

              <Show when={drawer.hasHostDetails()}>
                <HostDetailsDisclosure resource={resource} drawer={drawer} class="h-full" />
              </Show>
            </div>
          </TechnicalDetailsDisclosure>
        </Show>
      </div>
    </div>
  );
};
