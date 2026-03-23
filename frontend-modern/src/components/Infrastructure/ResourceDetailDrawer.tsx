import { Show, For } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { StatusDot } from '@/components/shared/StatusDot';
import { ReportMergeModal } from './ReportMergeModal';
import { PMGInstanceDrawer } from '@/components/PMG/PMGInstanceDrawer';
import { K8sDeploymentsDrawer } from '@/components/Kubernetes/K8sDeploymentsDrawer';
import { K8sNamespacesDrawer } from '@/components/Kubernetes/K8sNamespacesDrawer';
import { SwarmServicesDrawer } from '@/components/Docker/SwarmServicesDrawer';
import { ResourceDetailDrawerDebugTab } from './ResourceDetailDrawerDebugTab';
import { ResourceDetailDrawerOverviewTab } from './ResourceDetailDrawerOverviewTab';
import { useResourceDetailDrawerState } from './useResourceDetailDrawerState';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
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
  const drawer = useResourceDetailDrawerState({
    resource: props.resource,
    resolveResourceLabel: props.resolveResourceLabel,
  });

  return (
    <div class="space-y-3">
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1 min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              variant={drawer.statusIndicator().variant}
              title={drawer.statusIndicator().label}
              ariaLabel={drawer.statusIndicator().label}
              size="sm"
            />
            <div
              class="text-sm font-semibold text-base-content truncate"
              title={drawer.displayName()}
            >
              {drawer.displayName()}
            </div>
          </div>
          <div class="text-[11px] text-muted truncate" title={drawer.headerIdentity()}>
            {drawer.headerIdentity()}
          </div>
          <div class="flex flex-wrap gap-1.5" data-testid="resource-header-badges">
            <For each={drawer.headerBadges()}>
              {(badge) => (
                <span class={badge.classes} title={badge.title}>
                  {badge.label}
                </span>
              )}
            </For>
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
        <ResourceDetailDrawerOverviewTab resource={props.resource} drawer={drawer} />
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
            <PMGInstanceDrawer
              resourceId={props.resource.id}
              resourceName={drawer.displayName() || props.resource.id}
            />
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
