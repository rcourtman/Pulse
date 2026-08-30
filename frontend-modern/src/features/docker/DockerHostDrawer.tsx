import { Show, Suspense, createEffect, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { useDiscoveryFeatureAvailability } from '@/components/Discovery/useDiscoveryFeatureAvailability';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  GuestDrawerHistory,
  GuestDrawerHistoryRangeSelect,
} from '@/components/Workloads/GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import type { Resource } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

import { DockerHostDrawerManagement, DockerHostDrawerOverview } from './DockerHostDrawerOverview';
import {
  DOCKER_HOST_DRAWER_HISTORY_GROUPS,
  getDockerHostDrawerCurrentMetrics,
  getDockerHostDrawerHistoryTarget,
} from './dockerHostDrawerModel';

interface DockerHostDrawerProps {
  host: Resource;
  customUrl?: string;
  onCustomUrlChange?: (url: string) => void;
  onClose?: () => void;
}

type DockerHostDrawerTab = 'overview' | 'history' | 'manage' | 'discovery';

export const DockerHostDrawer: Component<DockerHostDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<DockerHostDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const { discoveryFeatureEnabled } = useDiscoveryFeatureAvailability();

  const headingId = () => `docker-host-drawer-heading-${props.host.id}`;
  const displayName = createMemo(() => asTrimmedString(props.host.name) || props.host.id);
  const historyTarget = createMemo(() => getDockerHostDrawerHistoryTarget(props.host));
  const currentMetrics = createMemo(() => getDockerHostDrawerCurrentMetrics(props.host));
  const discoveryConfig = createMemo(() => toDiscoveryConfig(props.host));
  const hasDiscoverySupport = createMemo(
    () => discoveryFeatureEnabled() && Boolean(discoveryConfig()),
  );
  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.host.status));
  const metadataId = createMemo(
    () => asTrimmedString(props.host.docker?.hostSourceId) || asTrimmedString(props.host.id),
  );

  createEffect(() => {
    if (activeTab() === 'discovery' && !hasDiscoverySupport()) {
      setActiveTab('overview');
    }
  });

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="docker-host-drawer">
      <Show
        when={props.onClose}
        fallback={
          <DrawerSubjectHeading
            headingId={headingId()}
            title={displayName()}
            statusVariant={headerIndicator().variant}
            statusLabel={headerIndicator().label}
          />
        }
      >
        {(onClose) => (
          <ObjectDrawerHeader
            collapseLabel={`Collapse ${displayName()} details`}
            onCollapse={onClose()}
          >
            <DrawerSubjectHeading
              headingId={headingId()}
              title={displayName()}
              statusVariant={headerIndicator().variant}
              statusLabel={headerIndicator().label}
            />
          </ObjectDrawerHeader>
        )}
      </Show>

      <Subtabs
        class="mb-1"
        ariaLabel="Docker host drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as DockerHostDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'history', label: 'History' },
          { value: 'manage', label: 'Manage' },
          ...(hasDiscoverySupport()
            ? [{ value: 'discovery', label: 'Discovery' } satisfies SubtabOption]
            : []),
        ]}
        trailing={
          <Show when={activeTab() === 'history'}>
            <GuestDrawerHistoryRangeSelect range={historyRange()} onRangeChange={setHistoryRange} />
          </Show>
        }
      />

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
          overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <DockerHostDrawerOverview host={props.host} />
      </div>

      <div class={activeTab() === 'manage' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <Show when={activeTab() === 'manage'}>
          <div class="space-y-3" data-testid="docker-host-manage-tab">
            <DockerHostDrawerManagement host={props.host} />
            <ResourceOperatorStateSection
              resourceId={props.host.id}
              resourceType="docker-host"
              platformType={props.host.platformType || 'docker'}
              capabilities={props.host.capabilities}
            />
            <WebInterfaceUrlField
              metadataKind="docker-host"
              metadataId={metadataId()}
              targetLabel={displayName()}
              title="Access"
              customUrl={props.customUrl ?? props.host.customUrl ?? ''}
              onCustomUrlChange={props.onCustomUrlChange}
            />
          </div>
        </Show>
      </div>

      <div class={activeTab() === 'history' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <GuestDrawerHistory
          currentMetrics={currentMetrics()}
          groups={DOCKER_HOST_DRAWER_HISTORY_GROUPS}
          range={historyRange()}
          target={historyTarget()}
        />
      </div>

      <Show when={hasDiscoverySupport() ? discoveryConfig() : null}>
        {(config) => (
          <div
            class={activeTab() === 'discovery' ? '' : 'hidden'}
            style={{ 'overflow-anchor': 'none' }}
          >
            <Suspense fallback={<DiscoveryLoadingFallback />}>
              <DiscoveryTab
                resourceType={config().resourceType}
                agentId={config().agentId}
                resourceId={config().resourceId}
                hostname={config().hostname}
                canonicalResourceId={props.host.id}
                showManualRunAction
              />
            </Suspense>
          </div>
        )}
      </Show>
    </section>
  );
};

export default DockerHostDrawer;
