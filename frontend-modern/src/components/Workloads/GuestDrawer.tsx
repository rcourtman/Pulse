import { Component, Show, createMemo } from 'solid-js';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import { getDrawerHeaderActionButtonClass } from '@/components/shared/buttonModel';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import { WebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { getGuestHealthIndicator } from '@/utils/status';
import {
  GUEST_DRAWER_BACKUP_PRECAUTION,
  getGuestDrawerGuestReadPrecaution,
  getGuestDrawerCurrentMetrics,
  getGuestDrawerDeferredMetrics,
  type GuestDrawerHistoryDeferredMetric,
  type GuestDrawerProps,
} from './guestDrawerModel';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { getShippedDocUrl } from '@/utils/docsLinks';
import { useGuestDrawerState } from './useGuestDrawerState';
import { GuestDrawerHistoryRangeSelect } from './GuestDrawerHistoryRangeSelect';
import { GuestDrawerOverview } from './GuestDrawerOverview';
import { GuestDrawerManage } from './GuestDrawerManage';

const GuestDrawerContent: Component<GuestDrawerProps> = (props) => {
  const {
    activeTab,
    agentHeading,
    agentLabel,
    agentTitle,
    backupPresentation,
    diskThresholds,
    guestId,
    hasAgentInfo,
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
  const baseCurrentMetrics = createMemo(() => getGuestDrawerCurrentMetrics(props.guest));
  const historyCurrentMetrics = createMemo(() => {
    const current = baseCurrentMetrics();
    if (!props.guest.telemetryStale) return current;
    return props.guest.disksFromAgent && current.disk !== undefined ? { disk: current.disk } : {};
  });
  const historyDeferredMetrics = createMemo(() => {
    const deferred = getGuestDrawerDeferredMetrics(props.guest);
    const stale = props.guest.telemetryStale;
    if (!stale) return deferred;
    const staleMessage = stale.age
      ? `Last value received ${stale.age} ago.`
      : 'Last value received before polling stopped.';
    const next: Record<string, GuestDrawerHistoryDeferredMetric> = { ...deferred };
    for (const [metric, lastKnownValue] of Object.entries(baseCurrentMetrics())) {
      if (metric === 'disk' && props.guest.disksFromAgent) continue;
      if (!next[metric] && lastKnownValue !== undefined) {
        next[metric] = { lastKnownValue, message: staleMessage };
      }
    }
    return next;
  });

  const guestReadPrecaution = createMemo(() => getGuestDrawerGuestReadPrecaution(props.guest));

  const headerIndicator = createMemo(() =>
    getGuestHealthIndicator(props.guest, props.parentNodeOnline !== false),
  );

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
      <Show when={props.guest.telemetryStale}>
        {(stale) => (
          <InlineNotice tone="warning" role="status" data-testid="guest-telemetry-stale-notice">
            <p class="font-medium">Readings are stale</p>
            <p>{stale().detail}</p>
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
          { value: 'manage', label: 'Manage' },
        ]}
        trailing={
          <Show when={activeTab() === 'overview' && hasHistorySupport() && historyTarget()}>
            <GuestDrawerHistoryRangeSelect
              range={historyRange()}
              onRangeChange={setHistoryRange}
              class="mb-1 sm:mb-0"
            />
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
          hasWorkloadActionAgent={hasWorkloadActionAgent()}
          showInGuestAgentInstallCue={showInGuestAgentInstallCue()}
          workloadActionAgentTitle={workloadActionAgentTitle()}
          parentMemoryTotal={props.parentMemoryTotal}
          memoryDisplayBasis={props.memoryDisplayBasis}
          alerts={props.alerts}
          historyTarget={hasHistorySupport() ? historyTarget() : null}
          historyRange={historyRange()}
          historyEnabled={activeTab() === 'overview'}
          currentMetrics={historyCurrentMetrics()}
          deferredMetrics={historyDeferredMetrics()}
        />
      </div>

      <div class={`[overflow-anchor:none] ${activeTab() === 'manage' ? '' : 'hidden'}`}>
        <Show when={activeTab() === 'manage'}>
          <GuestDrawerManage
            guest={props.guest}
            resourceId={guestId()}
            metadataId={webInterfaceMetadataId()}
            targetLabel={webInterfaceTargetLabel()}
            customUrl={props.customUrl}
            onCustomUrlChange={props.onCustomUrlChange}
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
