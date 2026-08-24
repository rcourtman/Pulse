import { Component, Show, Suspense, createMemo } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { DrawerHeaderActionGroup, DrawerHeaderIconButton } from '@/components/shared/Button';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { DiscoveryReadinessBadge } from '@/components/shared/DiscoveryReadinessBadge';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { getSimpleStatusIndicator } from '@/utils/status';
import { getGuestDrawerCurrentMetrics, type GuestDrawerProps } from './guestDrawerModel';
import { useGuestDrawerState } from './useGuestDrawerState';
import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from './GuestDrawerHistory';
import { GuestDrawerOverview } from './GuestDrawerOverview';
import { GuestDrawerManage } from './GuestDrawerManage';

export const GuestDrawer: Component<GuestDrawerProps> = (props) => {
  const {
    activeTab,
    agentHeading,
    agentLabel,
    agentTitle,
    backupPresentation,
    discoveryAgentId,
    discoveryIdentifiedSummary,
    discoveryLoadingState,
    discoveryReadinessPresentation,
    discoveryResourceId,
    discoveryResourceType,
    diskThresholds,
    guestId,
    hasAgentInfo,
    hasDiscoverySupport,
    hasFilesystemDetails,
    hasHistorySupport,
    hasNetworkInterfaces,
    hasOsInfo,
    hasWorkloadActionAgent,
    historyRange,
    historyTarget,
    ipAddresses,
    guestOsSummary,
    networkInterfaces,
    normalizedTags,
    setHistoryRange,
    showInGuestAgentInstallCue,
    switchTab,
    webInterfaceMetadataId,
    webInterfaceTargetLabel,
    workloadActionAgentTitle,
  } = useGuestDrawerState(props);
  const headingId = () => `guest-drawer-heading-${guestId()}`;
  const historyCurrentMetrics = createMemo(() => getGuestDrawerCurrentMetrics(props.guest));

  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.guest.status));

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <div class="flex items-start justify-between gap-3">
        <DrawerSubjectHeading
          headingId={headingId()}
          title={props.guest.name}
          statusVariant={headerIndicator().variant}
          statusLabel={headerIndicator().label}
        />
        <DrawerHeaderActionGroup>
          <DrawerHeaderIconButton onClick={() => props.onClose()} aria-label="Close guest drawer">
            <XIcon class="h-4 w-4" aria-hidden="true" />
          </DrawerHeaderIconButton>
        </DrawerHeaderActionGroup>
      </div>
      <Show when={discoveryReadinessPresentation()}>
        {(presentation) => (
          <div class="flex items-center gap-2 text-xs text-muted">
            <DiscoveryReadinessBadge presentation={presentation()} />
            <span class="truncate" title={presentation().detail || presentation().title}>
              {presentation().detail || presentation().statusLabel}
            </span>
          </div>
        )}
      </Show>
      <Subtabs
        class="mb-1"
        ariaLabel="Guest drawer sections"
        value={activeTab()}
        onChange={(value) => switchTab(value as Parameters<typeof switchTab>[0])}
        tabs={[
          { value: 'overview', label: 'Overview' },
          ...(hasHistorySupport()
            ? [{ value: 'history', label: 'History' } satisfies SubtabOption]
            : []),
          { value: 'manage', label: 'Manage' },
          ...(hasDiscoverySupport()
            ? [{ value: 'discovery', label: 'Discovery' } satisfies SubtabOption]
            : []),
        ]}
        trailing={
          <Show when={hasHistorySupport() && activeTab() === 'history'}>
            <GuestDrawerHistoryRangeSelect range={historyRange()} onRangeChange={setHistoryRange} />
          </Show>
        }
      />

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
                 overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <GuestDrawerOverview
          guest={props.guest}
          guestOsSummary={guestOsSummary()}
          agentHeading={agentHeading()}
          agentLabel={agentLabel()}
          agentTitle={agentTitle()}
          hasAgentInfo={hasAgentInfo()}
          hasFilesystemDetails={hasFilesystemDetails()}
          hasNetworkInterfaces={hasNetworkInterfaces()}
          hasOsInfo={hasOsInfo()}
          ipAddresses={ipAddresses()}
          networkInterfaces={networkInterfaces()}
          nestedWorkloadContext={props.nestedWorkloadContext}
          normalizedTags={normalizedTags()}
          backupPresentation={backupPresentation()}
          diskThresholds={diskThresholds()}
          discoveryIdentifiedSummary={discoveryIdentifiedSummary()}
          hasWorkloadActionAgent={hasWorkloadActionAgent()}
          showInGuestAgentInstallCue={showInGuestAgentInstallCue()}
          workloadActionAgentTitle={workloadActionAgentTitle()}
          alerts={props.alerts}
        />
      </div>

      {hasHistorySupport() && activeTab() === 'history' && (
        <div style={{ 'overflow-anchor': 'none' }}>
          <GuestDrawerHistory
            target={historyTarget()}
            range={historyRange()}
            currentMetrics={historyCurrentMetrics()}
          />
        </div>
      )}

      {/* Always rendered, hidden via CSS. Wrapped in a local Suspense
                     so DiscoveryTab's createResource loading state doesn't bubble
                     up to the app-level Suspense and replace the entire page. */}
      {hasDiscoverySupport() && (
        <div
          class={activeTab() === 'discovery' ? '' : 'hidden'}
          style={{ 'overflow-anchor': 'none' }}
        >
          <Suspense fallback={<DiscoveryLoadingFallback text={discoveryLoadingState.text} />}>
            <DiscoveryTab
              resourceType={discoveryResourceType()!}
              agentId={discoveryAgentId()}
              resourceId={discoveryResourceId()}
              hostname={props.guest.name}
              showManualRunAction
            />
          </Suspense>
        </div>
      )}

      <div class={activeTab() === 'manage' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <Show when={activeTab() === 'manage'}>
          <GuestDrawerManage
            guest={props.guest}
            resourceId={guestId()}
            metadataId={webInterfaceMetadataId()}
            targetLabel={webInterfaceTargetLabel()}
            customUrl={props.customUrl}
            onCustomUrlChange={props.onCustomUrlChange}
            suggestion={discoveryIdentifiedSummary() ?? undefined}
          />
        </Show>
      </div>
    </section>
  );
};
