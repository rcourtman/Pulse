import { Show, For, Suspense } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { StatusDot } from '@/components/shared/StatusDot';
import { ReportMergeModal } from './ReportMergeModal';
import { ProxmoxMailGatewayDrawer } from '@/features/proxmox/ProxmoxMailGatewayDrawer';
import { K8sDeploymentsDrawer } from '@/components/Kubernetes/K8sDeploymentsDrawer';
import { K8sNamespacesDrawer } from '@/components/Kubernetes/K8sNamespacesDrawer';
import { SwarmServicesDrawer } from '@/components/Docker/SwarmServicesDrawer';
import {
  GuestDrawerHistory,
  GuestDrawerHistoryRangeSelect,
} from '@/components/Workloads/GuestDrawerHistory';
import { ResourceDetailDrawerDebugTab } from './ResourceDetailDrawerDebugTab';
import { ResourceDetailDrawerOverviewTab } from './ResourceDetailDrawerOverviewTab';
import { useResourceDetailDrawerState } from './useResourceDetailDrawerState';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import {
  DEFAULT_RESOURCE_DETAIL_DRAWER_PRESENTATION,
  type ResourceDetailDrawerPresentation,
} from './resourceDetailDrawerPresentation';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
  presentation?: ResourceDetailDrawerPresentation;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
  initialShowAccessContext?: boolean;
  initialShowTrueNASDetails?: boolean;
}

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

const DrawerContent: Component<ResourceDetailDrawerProps> = (props) => {
  const presentation = () => props.presentation ?? DEFAULT_RESOURCE_DETAIL_DRAWER_PRESENTATION;
  const drawer = useResourceDetailDrawerState({
    resource: props.resource,
    presentation: presentation(),
    resolveResourceLabel: props.resolveResourceLabel,
    initialShowAccessContext: props.initialShowAccessContext,
    initialShowTrueNASDetails: props.initialShowTrueNASDetails,
  });
  const headingId = () => `resource-detail-drawer-heading-${props.resource.id}`;

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1 min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              variant={drawer.statusIndicator().variant}
              title={drawer.statusIndicator().label}
              ariaLabel={drawer.statusIndicator().label}
              size="sm"
            />
            <h2
              id={headingId()}
              class="text-sm font-semibold text-base-content truncate m-0"
              title={drawer.displayName()}
            >
              {drawer.displayName()}
            </h2>
          </div>
          <div class="flex flex-wrap gap-1.5" data-testid="resource-header-badges">
            <For each={drawer.headerBadges()}>
              {(badge) => (
                <span class={badge.classes} title={badge.title}>
                  {badge.label}
                </span>
              )}
            </For>
            <Show when={drawer.healthIssue()}>
              {(issue) => (
                <span
                  class="inline-flex max-w-full items-center rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900 dark:text-amber-300"
                  title={issue().title}
                >
                  {issue().compactLabel}
                </span>
              )}
            </Show>
          </div>
        </div>

        <Show when={props.onClose}>
          <button
            type="button"
            onClick={() => props.onClose?.()}
            class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
            aria-label="Close resource drawer"
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
        <For each={drawer.tabs()}>
          {(tab) => (
            <button
              onClick={() => drawer.setActiveTab(tab.id)}
              class={`pb-2 text-sm font-medium transition-colors relative ${
                drawer.activeTab() === tab.id
                  ? 'text-blue-600 dark:text-blue-400'
                  : ' hover:text-muted'
              }`}
            >
              {tab.label}
              <Show when={drawer.activeTab() === tab.id}>
                <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
              </Show>
            </button>
          )}
        </For>
      </div>

      <div
        class={drawer.activeTab() === 'overview' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <ResourceDetailDrawerOverviewTab
          resource={props.resource}
          drawer={drawer}
          presentation={presentation()}
        />
      </div>

      {/* Agent Machine Metrics History Tab */}
      <div
        class={drawer.activeTab() === 'history' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <Show when={drawer.activeTab() === 'history'}>
          <Show
            when={drawer.metricsHistoryTarget()}
            fallback={<TabAvailabilityNotice message="Metrics history is unavailable." />}
          >
            {(target) => (
              <div class="space-y-3" data-testid="resource-metrics-history-tab">
                <div class="flex items-center justify-end">
                  <GuestDrawerHistoryRangeSelect
                    range={drawer.metricsHistoryRange()}
                    onRangeChange={drawer.setMetricsHistoryRange}
                  />
                </div>
                <GuestDrawerHistory
                  target={target()}
                  range={drawer.metricsHistoryRange()}
                  fallbackMetrics={drawer.metricsHistoryFallbackMetrics()}
                />
              </div>
            )}
          </Show>
        </Show>
      </div>

      {/* Discovery Tab */}
      <div
        class={drawer.activeTab() === 'discovery' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <Show when={drawer.activeTab() === 'discovery'}>
          <Show
            when={drawer.discoveryConfig()}
            fallback={<TabAvailabilityNotice message="Discovery is unavailable." />}
          >
            {(config) => (
              <Suspense
                fallback={
                  <div class="flex items-center justify-center py-8">
                    <div class="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
                    <span class="ml-2 text-sm text-muted">{getDiscoveryLoadingState().text}</span>
                  </div>
                }
              >
                <DiscoveryTab
                  resourceType={config().resourceType}
                  agentId={config().agentId}
                  resourceId={config().resourceId}
                  hostname={config().hostname}
                  commandsEnabled={drawer.agentMeta()?.commandsEnabled}
                  showManualRunAction
                />
              </Suspense>
            )}
          </Show>
        </Show>
      </div>

      {/* PMG Mail Tab */}
      <div
        class={drawer.activeTab() === 'mail' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={drawer.activeTab() === 'mail'}>
          <Show
            when={props.resource.type === 'pmg'}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('mail')} />
            }
          >
            <ProxmoxMailGatewayDrawer instanceRow={props.resource} />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Namespaces Tab */}
      <div
        class={drawer.activeTab() === 'namespaces' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={drawer.activeTab() === 'namespaces'}>
          <Show
            when={props.resource.type === 'k8s-cluster'}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('namespaces')} />
            }
          >
            <K8sNamespacesDrawer
              cluster={drawer.kubernetesClusterName()}
              onOpenDeployments={(ns) => {
                drawer.setK8sDeploymentsPrefillNamespace((ns || '').trim());
                drawer.setActiveTab('deployments');
              }}
            />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Deployments Tab */}
      <div
        class={drawer.activeTab() === 'deployments' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={drawer.activeTab() === 'deployments'}>
          <Show
            when={props.resource.type === 'k8s-cluster'}
            fallback={
              <TabAvailabilityNotice
                message={getSpecializedTabAvailabilityMessage('deployments')}
              />
            }
          >
            <K8sDeploymentsDrawer
              cluster={drawer.kubernetesClusterName()}
              initialNamespace={drawer.k8sDeploymentsPrefillNamespace() || null}
            />
          </Show>
        </Show>
      </div>

      {/* Docker Swarm Tab */}
      <div
        class={drawer.activeTab() === 'swarm' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={drawer.activeTab() === 'swarm'}>
          <Show
            when={props.resource.type === 'docker-host' && drawer.dockerSwarmClusterKey()}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('swarm')} />
            }
          >
            <SwarmServicesDrawer
              cluster={drawer.dockerSwarmClusterKey()}
              swarm={drawer.dockerSwarmInfo()}
            />
          </Show>
        </Show>
      </div>

      {/* Debug Tab */}
      <Show when={drawer.debugEnabled()}>
        <div
          class={drawer.activeTab() === 'debug' ? '' : 'hidden'}
          style={{ 'overflow-anchor': 'none' }}
        >
          <ResourceDetailDrawerDebugTab resource={props.resource} drawer={drawer} />
        </div>
      </Show>

      <Show when={drawer.hasMergedSources()}>
        <div class="flex items-center justify-end">
          <button
            type="button"
            onClick={() => drawer.setShowReportModal(true)}
            class="text-xs font-medium transition-colors hover:text-muted"
          >
            Split merged resource
          </button>
        </div>
      </Show>

      <ReportMergeModal
        isOpen={drawer.showReportModal()}
        resourceId={props.resource.id}
        resourceName={drawer.displayName()}
        sources={drawer.mergedSources()}
        onClose={() => drawer.setShowReportModal(false)}
      />
    </section>
  );
};

export const ResourceDetailDrawer: Component<ResourceDetailDrawerProps> = (props) => {
  return (
    <DrawerContent
      resource={props.resource}
      onClose={props.onClose}
      presentation={props.presentation}
      resolveResourceLabel={props.resolveResourceLabel}
      initialShowAccessContext={props.initialShowAccessContext}
      initialShowTrueNASDetails={props.initialShowTrueNASDetails}
    />
  );
};

export default ResourceDetailDrawer;
