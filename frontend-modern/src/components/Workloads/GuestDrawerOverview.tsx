import { For, Show } from 'solid-js';
import { TerminalSquare } from 'lucide-solid';
import BoxIcon from 'lucide-solid/icons/box';

import { formatDiscoveryAge } from '@/api/discovery';
import { buildInfrastructureOnboardingPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { DiscoveryProvenanceMarker } from '@/components/shared/DiscoveryProvenanceMarker';
import { DiscoveryReadinessBadge } from '@/components/shared/DiscoveryReadinessBadge';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import type { DiscoveryIdentifiedSummary } from '@/utils/discoveryPresentation';
import type { DiscoveryReadinessPresentation } from '@/utils/resourceDiscoveryReadiness';
import { formatBytes, formatUptime } from '@/utils/format';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';

import { DiskList } from './DiskList';
import { getGuestDrawerMemoryRows, isGuestDrawerVM } from './guestDrawerModel';
import type { NestedWorkloadContext } from './nestedWorkloadContext';
import {
  IN_GUEST_AGENT_INSTALL_ACTION_LABEL,
  IN_GUEST_AGENT_INSTALL_TITLE,
  WORKLOAD_ACTION_AGENT_LABEL,
} from './workloadAgentReadiness';

import type { GuestDrawerProps } from './guestDrawerModel';

interface GuestDrawerOverviewProps {
  guest: GuestDrawerProps['guest'];
  guestId: string;
  guestOsSummary: string;
  agentLabel: string;
  agentTitle: string;
  hasAgentInfo: boolean;
  hasFilesystemDetails: boolean;
  hasNetworkInterfaces: boolean;
  hasOsInfo: boolean;
  hasWorkloadActionAgent: boolean;
  showInGuestAgentInstallCue: boolean;
  ipAddresses: string[];
  networkInterfaces: NonNullable<GuestDrawerProps['guest']['networkInterfaces']>;
  nestedWorkloadContext?: NestedWorkloadContext;
  normalizedTags: string[];
  onCustomUrlChange?: GuestDrawerProps['onCustomUrlChange'];
  customUrl?: GuestDrawerProps['customUrl'];
  backupPresentation: {
    ageClass: string;
    ageLabel: string;
    dateLabel: string;
  } | null;
  diskThresholds?: MetricDisplayThresholds | null;
  discoveryIdentifiedSummary?: DiscoveryIdentifiedSummary | null;
  discoveryReadinessPresentation?: DiscoveryReadinessPresentation | null;
  webInterfaceTargetLabel: string;
  workloadActionAgentTitle: string;
}

export function GuestDrawerOverview(props: GuestDrawerOverviewProps) {
  return (
    <div>
      <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
        <Show when={props.discoveryIdentifiedSummary}>
          {(summary) => (
            <InfoCardFrame>
              <div class="flex items-center justify-between gap-2 mb-2">
                <div class="flex min-w-0 items-center gap-1.5">
                  <h3 class="truncate text-[11px] font-medium uppercase tracking-wide text-base-content">
                    Identified Service
                  </h3>
                  <DiscoveryProvenanceMarker />
                </div>
                <span class="shrink-0 text-[10px] font-medium text-muted">
                  {summary().confidencePercent}
                </span>
              </div>
              <div class="space-y-1.5 text-[11px]">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-muted">Service</span>
                  <span
                    class="font-medium text-base-content truncate ml-2"
                    title={summary().serviceName}
                  >
                    {summary().serviceName}
                  </span>
                </div>
                <Show when={summary().category}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Category</span>
                    <span class="font-medium text-base-content truncate ml-2">
                      {summary().category}
                    </span>
                  </div>
                </Show>
                <Show when={summary().serviceVersion}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Version</span>
                    <span
                      class="font-medium text-base-content truncate ml-2"
                      title={summary().serviceVersion}
                    >
                      {summary().serviceVersion}
                    </span>
                  </div>
                </Show>
                <Show when={summary().suggestedUrl}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Endpoint</span>
                    <span
                      class="font-medium text-base-content truncate ml-2"
                      title={summary().suggestedUrl}
                    >
                      {summary().suggestedUrl}
                    </span>
                  </div>
                </Show>
                <Show when={summary().portCount > 0}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Ports</span>
                    <span class="font-medium text-base-content">{summary().portCount}</span>
                  </div>
                </Show>
                <Show when={summary().cliAccess}>
                  <div
                    class="text-[10px] text-muted truncate font-mono"
                    title={summary().cliAccess}
                  >
                    {summary().cliAccess}
                  </div>
                </Show>
                <div class="flex flex-wrap gap-1 pt-1 text-[10px] text-muted">
                  <span>{summary().sourceLabel}</span>
                  <Show when={summary().observedAt}>
                    <span>· {formatDiscoveryAge(summary().observedAt!)}</span>
                  </Show>
                </div>
              </div>
            </InfoCardFrame>
          )}
        </Show>
        <Show when={props.discoveryReadinessPresentation}>
          {(presentation) => (
            <InfoCardFrame>
              <div class="flex items-center justify-between gap-2 mb-2">
                <h3 class="truncate text-[11px] font-medium uppercase tracking-wide text-base-content">
                  AI Context
                </h3>
                <DiscoveryReadinessBadge presentation={presentation()} compact />
              </div>
              <div class="space-y-1.5 text-[11px]">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-muted">Discovery</span>
                  <span class="font-medium text-base-content truncate ml-2">
                    {presentation().shortLabel}
                  </span>
                </div>
                <Show when={presentation().detail}>
                  <p class="text-[10px] leading-4 text-muted">{presentation().detail}</p>
                </Show>
              </div>
            </InfoCardFrame>
          )}
        </Show>
        <InfoCardFrame>
          <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
            System
          </h3>
          <div class="space-y-1.5 text-[11px]">
            <Show when={props.guest.cpus}>
              <div class="flex items-center justify-between">
                <span class="text-muted">CPUs</span>
                <span class="font-medium text-base-content">{props.guest.cpus}</span>
              </div>
            </Show>
            <Show when={props.guest.uptime > 0}>
              <div class="flex items-center justify-between">
                <span class="text-muted">Uptime</span>
                <span class="font-medium text-base-content">
                  {formatUptime(props.guest.uptime)}
                </span>
              </div>
            </Show>
            <Show when={props.guest.node}>
              <div class="flex items-center justify-between">
                <span class="text-muted">Node</span>
                <span class="font-medium text-base-content">{props.guest.node}</span>
              </div>
            </Show>
            <Show when={props.hasAgentInfo}>
              <div class="flex items-center justify-between">
                <span class="text-muted">Guest agent</span>
                <span class="font-medium text-base-content truncate ml-2" title={props.agentTitle}>
                  {props.agentLabel}
                </span>
              </div>
            </Show>
            <Show when={props.hasWorkloadActionAgent}>
              <div class="flex items-center justify-between">
                <span class="text-muted">Actions</span>
                <span
                  class="font-medium text-base-content truncate ml-2"
                  title={props.workloadActionAgentTitle}
                >
                  {WORKLOAD_ACTION_AGENT_LABEL}
                </span>
              </div>
            </Show>
            <Show when={props.showInGuestAgentInstallCue}>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">Actions</span>
                <a
                  href={buildInfrastructureOnboardingPath('agent')}
                  class="inline-flex items-center gap-1 text-right text-[11px] font-semibold text-amber-700 underline-offset-2 hover:underline dark:text-amber-300"
                  title={IN_GUEST_AGENT_INSTALL_TITLE}
                >
                  <TerminalSquare class="h-3.5 w-3.5" aria-hidden="true" />
                  <span>{IN_GUEST_AGENT_INSTALL_ACTION_LABEL}</span>
                </a>
              </div>
            </Show>
          </div>
        </InfoCardFrame>

        <Show when={props.nestedWorkloadContext}>
          {(context) => {
            const visibleItems = () => context().items.slice(0, 4);
            const hiddenCount = () => Math.max(0, context().count - visibleItems().length);
            return (
              <InfoCardFrame data-testid="nested-workload-context-card">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <h3 class="truncate text-[11px] font-medium uppercase tracking-wide text-base-content">
                    {context().title}
                  </h3>
                  <a
                    href={context().href}
                    class="inline-flex shrink-0 items-center gap-1 text-[10px] font-semibold text-blue-600 underline-offset-2 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-blue-300"
                    aria-label={`Open ${context().label} page for ${props.guest.name}`}
                  >
                    <BoxIcon class="h-3 w-3" aria-hidden="true" />
                    <span>Open Docker</span>
                  </a>
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted">Containers</span>
                    <span class="font-medium text-base-content">{context().count}</span>
                  </div>
                  <For each={visibleItems()}>
                    {(item) => (
                      <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                        <span class="truncate font-medium text-base-content" title={item.name}>
                          {item.name}
                        </span>
                        <span class="rounded bg-surface-alt px-1.5 py-0.5 text-[10px] font-medium text-muted">
                          {item.status}
                        </span>
                      </div>
                    )}
                  </For>
                  <Show when={hiddenCount() > 0}>
                    <div class="text-[10px] font-medium text-muted">+{hiddenCount()} more</div>
                  </Show>
                </div>
              </InfoCardFrame>
            );
          }}
        </Show>

        {/* vSphere placement card: vCenter / Datacenter / Cluster live on
              WorkloadGuest.vmware and aren't surfaced by System (Node already
              shows the runtime host). Render only when the workload is a
              vSphere VM and at least one of these fields is populated. */}
        <Show
          when={
            (props.guest.platformScopes?.includes('vmware-vsphere') ?? false) &&
            (props.guest.vmware?.connectionName ||
              props.guest.vmware?.vcenterHost ||
              props.guest.vmware?.datacenterName ||
              props.guest.vmware?.clusterName)
          }
        >
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              vSphere
            </h3>
            <div class="space-y-1.5 text-[11px]">
              <Show when={props.guest.vmware?.connectionName || props.guest.vmware?.vcenterHost}>
                <div class="flex items-center justify-between gap-2 min-w-0">
                  <span class="text-muted shrink-0">vCenter</span>
                  <span
                    class="font-medium text-base-content truncate"
                    title={
                      props.guest.vmware?.vcenterHost || props.guest.vmware?.connectionName || ''
                    }
                  >
                    {props.guest.vmware?.connectionName || props.guest.vmware?.vcenterHost}
                  </span>
                </div>
              </Show>
              <Show when={props.guest.vmware?.datacenterName}>
                <div class="flex items-center justify-between gap-2 min-w-0">
                  <span class="text-muted shrink-0">Datacenter</span>
                  <span
                    class="font-medium text-base-content truncate"
                    title={props.guest.vmware?.datacenterName || ''}
                  >
                    {props.guest.vmware?.datacenterName}
                  </span>
                </div>
              </Show>
              <Show when={props.guest.vmware?.clusterName}>
                <div class="flex items-center justify-between gap-2 min-w-0">
                  <span class="text-muted shrink-0">Cluster</span>
                  <span
                    class="font-medium text-base-content truncate"
                    title={props.guest.vmware?.clusterName || ''}
                  >
                    {props.guest.vmware?.clusterName}
                  </span>
                </div>
              </Show>
            </div>
          </InfoCardFrame>
        </Show>

        <Show when={props.hasOsInfo || props.ipAddresses.length > 0}>
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              Guest Info
            </h3>
            <div class="space-y-2">
              <Show when={props.hasOsInfo}>
                <div class="text-[11px] text-muted truncate" title={props.guestOsSummary}>
                  <Show when={(props.guest.osName?.length ?? 0) > 0}>
                    <span class="font-medium">{props.guest.osName}</span>
                  </Show>
                  <Show
                    when={
                      (props.guest.osName?.length ?? 0) > 0 &&
                      (props.guest.osVersion?.length ?? 0) > 0
                    }
                  >
                    <span class="text-muted mx-1">•</span>
                  </Show>
                  <Show when={(props.guest.osVersion?.length ?? 0) > 0}>
                    <span>{props.guest.osVersion}</span>
                  </Show>
                </div>
              </Show>
              <Show when={props.ipAddresses.length > 0}>
                <div class="flex flex-wrap gap-1">
                  <For each={props.ipAddresses}>
                    {(ip) => (
                      <span
                        class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200 max-w-full truncate"
                        title={ip}
                      >
                        {ip}
                      </span>
                    )}
                  </For>
                </div>
              </Show>
            </div>
          </InfoCardFrame>
        </Show>

        <Show when={getGuestDrawerMemoryRows(props.guest).length > 0}>
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              Memory
            </h3>
            <div class="space-y-1.5 text-[11px]">
              <For each={getGuestDrawerMemoryRows(props.guest)}>
                {(row) => (
                  <div class="flex items-center justify-between gap-2 min-w-0">
                    <span class="shrink-0 text-muted">{row.label}</span>
                    <span
                      class="truncate text-right font-medium text-base-content"
                      title={row.value}
                    >
                      {row.value}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </InfoCardFrame>
        </Show>

        <Show when={props.guest.lastBackup}>
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              Backup
            </h3>
            <div class="space-y-1 text-[11px]">
              <Show when={props.backupPresentation}>
                {(presentation) => (
                  <>
                    <div class="flex items-center justify-between">
                      <span class="text-muted">Last Backup</span>
                      <span class={`font-medium ${presentation().ageClass}`}>
                        {presentation().ageLabel}
                      </span>
                    </div>
                    <div class="text-[10px] text-muted">{presentation().dateLabel}</div>
                  </>
                )}
              </Show>
            </div>
          </InfoCardFrame>
        </Show>

        <Show when={props.normalizedTags.length > 0}>
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              Tags
            </h3>
            <div class="flex flex-wrap gap-1">
              <For each={props.normalizedTags}>
                {(tag) => (
                  <span class="inline-block rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                    {tag}
                  </span>
                )}
              </For>
            </div>
          </InfoCardFrame>
        </Show>

        <Show
          when={props.hasFilesystemDetails && props.guest.disks && props.guest.disks.length > 0}
        >
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              Filesystems
            </h3>
            <div class="text-[11px] text-muted">
              <DiskList
                disks={props.guest.disks || []}
                diskStatusReason={
                  isGuestDrawerVM(props.guest) ? (props.guest as any).diskStatusReason : undefined
                }
                thresholds={props.diskThresholds}
              />
            </div>
          </InfoCardFrame>
        </Show>

        <Show when={props.hasNetworkInterfaces}>
          <InfoCardFrame>
            <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              Network
            </h3>
            <div class="space-y-2">
              <For each={props.networkInterfaces.slice(0, 4)}>
                {(iface) => {
                  const addresses = iface.addresses ?? [];
                  const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                  return (
                    <div class="rounded border border-dashed border-border p-2 overflow-hidden">
                      <div class="flex items-center gap-2 text-[11px] font-medium text-base-content min-w-0">
                        <span class="truncate min-w-0">{iface.name || 'interface'}</span>
                        <Show when={iface.mac}>
                          <span
                            class="text-[9px] text-muted font-normal truncate shrink-0 max-w-[100px]"
                            title={iface.mac}
                          >
                            {iface.mac}
                          </span>
                        </Show>
                      </div>
                      <Show when={addresses.length > 0}>
                        <div class="flex flex-wrap gap-1 mt-1">
                          <For each={addresses}>
                            {(ip) => (
                              <span
                                class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200 max-w-full truncate"
                                title={ip}
                              >
                                {ip}
                              </span>
                            )}
                          </For>
                        </div>
                      </Show>
                      <Show when={hasTraffic}>
                        <div class="flex gap-3 mt-1 text-[10px] text-muted">
                          <span>RX {formatBytes(iface.rxBytes ?? 0)}</span>
                          <span>TX {formatBytes(iface.txBytes ?? 0)}</span>
                        </div>
                      </Show>
                    </div>
                  );
                }}
              </For>
            </div>
          </InfoCardFrame>
        </Show>
      </div>

      <div class="mt-3">
        <WebInterfaceUrlField
          metadataKind="guest"
          metadataId={props.guestId}
          targetLabel={props.webInterfaceTargetLabel}
          customUrl={props.customUrl}
          onCustomUrlChange={(url) => props.onCustomUrlChange?.(props.guestId, url)}
          suggestedUrl={props.discoveryIdentifiedSummary?.suggestedUrl}
          suggestedUrlReasonText={props.discoveryIdentifiedSummary?.suggestedUrlReasonText}
          suggestedUrlReasonTitle={props.discoveryIdentifiedSummary?.suggestedUrlReasonTitle}
          suggestedUrlDiagnostic={props.discoveryIdentifiedSummary?.suggestedUrlDiagnostic}
        />
      </div>
    </div>
  );
}
