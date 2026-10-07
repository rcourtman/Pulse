import { Component, Show, Suspense, createMemo } from 'solid-js';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { getDrawerHeaderActionButtonClass } from '@/components/shared/buttonModel';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { DiscoveryReadinessBadge } from '@/components/shared/DiscoveryReadinessBadge';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { WebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  GUEST_DRAWER_BACKUP_PRECAUTION,
  getGuestDrawerGuestReadPrecaution,
  getGuestDrawerCurrentMetrics,
  getGuestDrawerDeferredMetrics,
  type GuestDrawerProps,
} from './guestDrawerModel';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { getShippedDocUrl } from '@/utils/docsLinks';
import { useGuestDrawerState } from './useGuestDrawerState';
import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from './GuestDrawerHistory';
import { GuestDrawerOverview } from './GuestDrawerOverview';
import { GuestDrawerManage } from './GuestDrawerManage';

const GuestDrawerContent: Component<GuestDrawerProps> = (props) => {
  const {
    activeTab,
    agentHeading,
    agentLabel,
    agentTitle,
    backupPresentation,
    discoveryAgentId,
    discoveryIdentifiedSummary,
    discoveryPanelKey,
    discoveryReadError,
    discoveryReadLoading,
    retryDiscoveryRead,
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
  const historyDeferredMetrics = createMemo(() => getGuestDrawerDeferredMetrics(props.guest));

  const guestReadPrecaution = createMemo(() => getGuestDrawerGuestReadPrecaution(props.guest));

  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.guest.status));

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      {/* Phone rows hide the adjacent web link so names stay distinguishable;
          the header keeps the saved service one tap from the expanded row. */}
      <ObjectDrawerHeader
        collapseLabel={`Collapse ${props.guest.name} details`}
        onCollapse={props.onClose}
        actions={
          <Show when={props.customUrl?.trim()}>
            <WebInterfaceLink
              url={props.customUrl}
              ariaLabel={`Open web interface for ${props.guest.name}`}
              class={getDrawerHeaderActionButtonClass()}
            >
              <ExternalLinkIcon class="h-3.5 w-3.5" aria-hidden="true" />
              <span>Open</span>
            </WebInterfaceLink>
          </Show>
        }
      >
        <DrawerSubjectHeading
          headingId={headingId()}
          title={props.guest.name}
          statusVariant={headerIndicator().variant}
          statusLabel={headerIndicator().label}
        />
      </ObjectDrawerHeader>
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
      <Show when={discoveryReadError()}>
        {(message) => (
          <InlineNotice
            tone="warning"
            role="status"
            actionLabel={
              discoveryReadLoading() ? 'Retrying service details...' : 'Retry service details'
            }
            actionOnClick={retryDiscoveryRead}
          >
            {message()}
          </InlineNotice>
        )}
      </Show>
      <Show when={guestReadPrecaution()}>
        {(message) => (
          <InlineNotice
            tone="warning"
            role="status"
            data-testid="guest-read-precaution"
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
      <div class={`[overflow-anchor:none] ${activeTab() === 'overview' ? '' : 'hidden'}`}>
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
          parentMemoryTotal={props.parentMemoryTotal}
          memoryDisplayBasis={props.memoryDisplayBasis}
          alerts={props.alerts}
        />
      </div>

      {hasHistorySupport() && activeTab() === 'history' && (
        <div class="[overflow-anchor:none]">
          <GuestDrawerHistory
            target={historyTarget()}
            range={historyRange()}
            currentMetrics={historyCurrentMetrics()}
            deferredMetrics={historyDeferredMetrics()}
          />
        </div>
      )}

      {/* Always rendered, hidden via CSS. Wrapped in a local Suspense
                     so DiscoveryTab's createResource loading state doesn't bubble
                     up to the app-level Suspense and replace the entire page. */}
      <Show when={discoveryPanelKey()} keyed>
        {(_targetKey) => (
          <div class={`[overflow-anchor:none] ${activeTab() === 'discovery' ? '' : 'hidden'}`}>
            <Suspense fallback={<DiscoveryLoadingFallback text={discoveryLoadingState.text} />}>
              <DiscoveryTab
                resourceType={discoveryResourceType()!}
                agentId={discoveryAgentId()}
                resourceId={discoveryResourceId()}
                hostname={props.guest.name}
                canonicalResourceId={props.guest.id}
                showManualRunAction
                runBlockReason={
                  guestReadPrecaution()
                    ? 'Discovery is paused during a backup or guest-read deferral. Saved results remain available. Clearing this pause does not prove thaw.'
                    : null
                }
              />
            </Suspense>
          </div>
        )}
      </Show>

      <div class={`[overflow-anchor:none] ${activeTab() === 'manage' ? '' : 'hidden'}`}>
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

// A new canonical guest must not inherit another guest's tabs, editable forms
// or outstanding requests. Ordinary same-ID snapshots remain reactive/mounted.
export const GuestDrawer: Component<GuestDrawerProps> = (props) => (
  <Show when={getCanonicalWorkloadId(props.guest)} keyed>
    {(_guestId) => <GuestDrawerContent {...props} />}
  </Show>
);
