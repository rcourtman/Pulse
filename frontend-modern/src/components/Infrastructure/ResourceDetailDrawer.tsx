import { Show, For } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import { getDrawerHeaderActionButtonClass } from '@/components/shared/buttonModel';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import { WebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import type { StatusIndicator } from '@/utils/status';
import { ReportMergeModal } from './ReportMergeModal';
import { ProxmoxMailGatewayMailFlow } from '@/features/proxmox/ProxmoxMailGatewayDrawer';
import { K8sNamespacesDrawer } from '@/components/Kubernetes/K8sNamespacesDrawer';
import { ResourceDetailDrawerDebugTab } from './ResourceDetailDrawerDebugTab';
import {
  ResourceAccessDisclosure,
  ResourceDetailDrawerOverviewTab,
} from './ResourceDetailDrawerOverviewTab';
import { useResourceDetailDrawerState } from './useResourceDetailDrawerState';
import {
  DEFAULT_RESOURCE_DETAIL_DRAWER_PRESENTATION,
  type ResourceDetailDrawerPresentation,
} from './resourceDetailDrawerPresentation';
import { DockerContainerLifecycleControls } from '@/features/docker/DockerContainerLifecycleControls';
import { isDockerContainerLifecycleResource } from '@/features/docker/dockerContainerLifecycleActions';
import { ResourceOperatorStateSection } from './ResourceOperatorStateSection';
import { MaintenanceVerificationSection } from './MaintenanceVerificationSection';
import { ResourceActionHistory } from './ResourceActionHistory';
import { CONTAINER_CPU_CAPACITY_DESCRIPTION } from '@/features/docker/dockerCpuPresentation';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { GUEST_DRAWER_BACKUP_PRECAUTION } from '@/components/Workloads/guestDrawerModel';
import { getShippedDocUrl } from '@/utils/docsLinks';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
  /**
   * The status the table row's own dot shows. Rows judge staleness, impaired
   * sources and platform states (a succeeded pod, a scaled-to-zero
   * deployment) that the raw status does not carry, so the drawer takes the
   * row's verdict rather than recomputing a second, disagreeing one.
   */
  statusIndicator?: StatusIndicator;
  presentation?: ResourceDetailDrawerPresentation;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
  initialShowAccessContext?: boolean;
  initialShowHostDetails?: boolean;
  onResourceActionSettled?: () => void | Promise<void>;
}

const TabAvailabilityNotice: Component<{ message: string }> = (props) => (
  <InlineNotice tone="info" role="status">
    {props.message}
  </InlineNotice>
);

type SpecializedDrawerTab = 'mail' | 'namespaces';

export const getSpecializedTabAvailabilityMessage = (tab: SpecializedDrawerTab): string => {
  switch (tab) {
    case 'mail':
      return 'PMG resources only.';
    case 'namespaces':
      return 'Kubernetes clusters only.';
  }
};

const DrawerContent: Component<ResourceDetailDrawerProps> = (props) => {
  const presentation = () => props.presentation ?? DEFAULT_RESOURCE_DETAIL_DRAWER_PRESENTATION;
  const drawer = useResourceDetailDrawerState({
    get resource() {
      return props.resource;
    },
    presentation: presentation(),
    resolveResourceLabel: props.resolveResourceLabel,
    initialShowAccessContext: props.initialShowAccessContext,
    initialShowHostDetails: props.initialShowHostDetails,
  });
  const headingId = () => `resource-detail-drawer-heading-${props.resource.id}`;
  const statusIndicator = () => props.statusIndicator ?? drawer.statusIndicator();
  const customUrl = () => props.resource.customUrl?.trim() ?? '';
  // The Kubernetes page scopes its rows by cluster id, or by name when no id
  // is reported, the same value its cluster table writes into the URL.
  const kubernetesScopeCluster = () =>
    drawer.kubernetesMeta()?.clusterId?.trim() || drawer.kubernetesClusterName();
  // The health issue is the first "Worth checking" item on Overview, so the
  // header names the subject only; source and identity badges say what the
  // row cannot (an agent merged with PVE, an Unraid host).
  const HeaderIdentity = () => (
    <DrawerSubjectHeading
      headingId={headingId()}
      title={drawer.displayName()}
      statusVariant={statusIndicator().variant}
      statusLabel={statusIndicator().label}
      trailing={
        <Show when={drawer.headerBadges().length > 0}>
          <div class="flex flex-wrap gap-1.5" data-testid="resource-header-badges">
            <For each={drawer.headerBadges()}>
              {(badge) => (
                <span class={badge.classes} title={badge.title}>
                  {badge.label}
                </span>
              )}
            </For>
          </div>
        </Show>
      }
    />
  );
  const HeaderActions = () => (
    <>
      <Show when={customUrl()}>
        <WebInterfaceLink
          url={customUrl()}
          ariaLabel={`Open web interface for ${drawer.displayName()}`}
          class={getDrawerHeaderActionButtonClass()}
        >
          <ExternalLinkIcon class="h-3.5 w-3.5" aria-hidden="true" />
          <span>Open</span>
        </WebInterfaceLink>
      </Show>
      <Show when={isDockerContainerLifecycleResource(props.resource)}>
        <DockerContainerLifecycleControls
          resource={props.resource}
          surface="resource-detail"
          onActionSettled={props.onResourceActionSettled}
        />
      </Show>
    </>
  );

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <Show
        when={props.onClose}
        fallback={
          <div class="flex items-start justify-between gap-4">
            <HeaderIdentity />
            <div class="flex shrink-0 items-center gap-1.5">
              <HeaderActions />
            </div>
          </div>
        }
      >
        {(onClose) => (
          <ObjectDrawerHeader
            collapseLabel={`Collapse ${drawer.displayName()} details`}
            onCollapse={onClose()}
            actions={<HeaderActions />}
          >
            <HeaderIdentity />
          </ObjectDrawerHeader>
        )}
      </Show>

      <Show
        when={props.resource.type === 'app-container' && props.resource.platformType === 'docker'}
      >
        <p class="text-xs leading-relaxed text-muted" data-testid="container-drawer-cpu-scale">
          {CONTAINER_CPU_CAPACITY_DESCRIPTION}
        </p>
      </Show>

      <Show when={drawer.guestReadPrecaution()}>
        {(message) => (
          <InlineNotice
            tone="warning"
            role="status"
            data-testid="resource-guest-read-precaution"
            actionHref={getShippedDocUrl('VM_DISK_MONITORING.md')}
            actionLabel="Backup safety guidance"
          >
            <p>{message()}</p>
            <p>{GUEST_DRAWER_BACKUP_PRECAUTION}</p>
          </InlineNotice>
        )}
      </Show>

      <Subtabs
        class="mb-1"
        ariaLabel="Resource detail sections"
        value={drawer.activeTab()}
        onChange={(value) =>
          drawer.setActiveTab(value as Parameters<typeof drawer.setActiveTab>[0])
        }
        tabs={drawer.tabs().map((tab) => ({ value: tab.id, label: tab.label }))}
      />

      <div class={`[overflow-anchor:none] ${drawer.activeTab() === 'overview' ? '' : 'hidden'}`}>
        <ResourceDetailDrawerOverviewTab
          resource={props.resource}
          drawer={drawer}
          presentation={presentation()}
        />
      </div>

      <div class={`[overflow-anchor:none] ${drawer.activeTab() === 'manage' ? '' : 'hidden'}`}>
        <Show when={drawer.activeTab() === 'manage'}>
          <div class="space-y-3" data-testid="resource-manage-tab">
            <ResourceOperatorStateSection
              resourceId={props.resource.id}
              resourceType={props.resource.type}
              platformType={props.resource.platformType}
              capabilities={props.resource.capabilities}
            />
            <Show when={drawer.hasAccessContext()}>
              <ResourceAccessDisclosure resource={props.resource} drawer={drawer} />
            </Show>
            <Show when={presentation() !== 'table-row'}>
              <MaintenanceVerificationSection resourceId={props.resource.id} />
            </Show>
            <Show when={drawer.actionAuditAvailable() || drawer.actionAuditError()}>
              <ResourceActionHistory
                audits={drawer.sortedActionAudits()}
                count={drawer.actionAuditCount()}
                loadingLabel={drawer.actionAuditLoadingLabel()}
                error={drawer.actionAuditError()}
                onRetry={drawer.refetchActionAudits}
              />
            </Show>
          </div>
        </Show>
      </div>

      {/* PMG Mail Tab */}
      <div class={`[overflow-anchor:none] ${drawer.activeTab() === 'mail' ? '' : 'hidden'}`}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={drawer.activeTab() === 'mail'}>
          <Show
            when={props.resource.type === 'pmg'}
            fallback={
              <TabAvailabilityNotice message={getSpecializedTabAvailabilityMessage('mail')} />
            }
          >
            <ProxmoxMailGatewayMailFlow resource={props.resource} />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Namespaces Tab */}
      <div class={`[overflow-anchor:none] ${drawer.activeTab() === 'namespaces' ? '' : 'hidden'}`}>
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
              scopeCluster={kubernetesScopeCluster()}
            />
          </Show>
        </Show>
      </div>

      {/* Debug Tab */}
      <Show when={drawer.debugEnabled()}>
        <div class={`[overflow-anchor:none] ${drawer.activeTab() === 'debug' ? '' : 'hidden'}`}>
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
    // Local tabs, disclosures and asynchronous reads belong to one canonical
    // resource. Same-ID snapshots stay mounted; a different ID starts fresh.
    <Show when={props.resource.id} keyed>
      {(_resourceId) => (
        <DrawerContent
          resource={props.resource}
          onClose={props.onClose}
          statusIndicator={props.statusIndicator}
          presentation={props.presentation}
          resolveResourceLabel={props.resolveResourceLabel}
          initialShowAccessContext={props.initialShowAccessContext}
          initialShowHostDetails={props.initialShowHostDetails}
          onResourceActionSettled={props.onResourceActionSettled}
        />
      )}
    </Show>
  );
};

export default ResourceDetailDrawer;
