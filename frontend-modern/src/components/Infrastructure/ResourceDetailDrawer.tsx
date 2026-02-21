import { Show, Suspense, createMemo, For, createSignal, createEffect } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { getDisplayName } from '@/types/resource';
import { formatUptime, formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/Dashboard/TagBadges';
import { getHostStatusIndicator } from '@/utils/status';
import { getPlatformBadge, getSourceBadge, getTypeBadge, getUnifiedSourceBadges } from './resourceBadges';
import { buildWorkloadsHref } from './workloadsLink';
import { buildServiceDetailLinks } from './serviceDetailLinks';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { TemperaturesCard } from '@/components/shared/cards/TemperaturesCard';
import { RaidCard } from '@/components/shared/cards/RaidCard';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { ReportMergeModal } from './ReportMergeModal';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { PMGInstanceDrawer } from '@/components/PMG/PMGInstanceDrawer';
import { K8sDeploymentsDrawer } from '@/components/Kubernetes/K8sDeploymentsDrawer';
import { K8sNamespacesDrawer } from '@/components/Kubernetes/K8sNamespacesDrawer';
import { MonitoringAPI } from '@/api/monitoring';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import { SwarmServicesDrawer } from '@/components/Docker/SwarmServicesDrawer';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';

interface ResourceDetailDrawerProps {
  resource: Resource;
  onClose?: () => void;
}

import {
  type AgentPlatformData,
  type KubernetesPlatformData,
  type PlatformData,
  type DockerPlatformData,
  toDiscoveryConfig,
  toNodeFromProxmox,
  toHostFromAgent,
  buildTemperatureRows,
  normalizeHealthLabel,
  healthToneClass,
  formatInteger,
  ALIAS_COLLAPSE_THRESHOLD,
  formatSourceType
} from './resourceDetailMappers';

const DrawerContent: Component<ResourceDetailDrawerProps> = (props) => {
  type DrawerTab = 'overview' | 'mail' | 'namespaces' | 'deployments' | 'swarm' | 'discovery' | 'debug';
  const [activeTab, setActiveTab] = createSignal<DrawerTab>('overview');
  const [debugEnabled] = createLocalStorageBooleanSignal(STORAGE_KEYS.DEBUG_MODE, false);
  const [copied, setCopied] = createSignal(false);
  const [showReportModal, setShowReportModal] = createSignal(false);

  const displayName = createMemo(() => getDisplayName(props.resource));
  const statusIndicator = createMemo(() => getHostStatusIndicator({ status: props.resource.status }));
  const lastSeen = createMemo(() => formatRelativeTime(props.resource.lastSeen));
  const lastSeenAbsolute = createMemo(() => formatAbsoluteTime(props.resource.lastSeen));

  const platformBadge = createMemo(() => getPlatformBadge(props.resource.platformType));
  const sourceBadge = createMemo(() => getSourceBadge(props.resource.sourceType));
  const typeBadge = createMemo(() => getTypeBadge(props.resource.type));
  const unifiedSourceBadges = createMemo(() => {
    const platformData = props.resource.platformData as PlatformData | undefined;
    return getUnifiedSourceBadges(platformData?.sources ?? []);
  });
  const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
  const platformData = createMemo(() => props.resource.platformData as PlatformData | undefined);
  const agentMeta = createMemo(() => props.resource.agent ?? (platformData()?.agent as AgentPlatformData | undefined));
  const kubernetesMeta = createMemo(
    () => props.resource.kubernetes ?? (platformData()?.kubernetes as KubernetesPlatformData | undefined),
  );
  const kubernetesCapabilityBadges = createMemo(() => {
    const capabilities = kubernetesMeta()?.metricCapabilities;
    if (!capabilities) return [];

    const supportedBadge = 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400';
    const unsupportedBadge = 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300';
    const badges: { label: string; classes: string; title: string }[] = [];

    if (capabilities.nodeCpuMemory) {
      badges.push({
        label: 'K8s Node CPU/Memory',
        classes: supportedBadge,
        title: 'Node CPU and memory metrics are available.',
      });
    }
    if (capabilities.nodeTelemetry) {
      badges.push({
        label: 'Node Telemetry (Agent)',
        classes: supportedBadge,
        title: 'Linked Pulse host agent provides node uptime, temperature, disk, network, and disk I/O.',
      });
    }
    if (capabilities.podCpuMemory) {
      badges.push({
        label: 'Pod CPU/Memory',
        classes: supportedBadge,
        title: 'Pod CPU and memory metrics are available.',
      });
    }
    if (capabilities.podNetwork) {
      badges.push({
        label: 'Pod Network',
        classes: supportedBadge,
        title: 'Pod network throughput is available.',
      });
    }
    if (capabilities.podEphemeralDisk) {
      badges.push({
        label: 'Pod Ephemeral Disk',
        classes: supportedBadge,
        title: 'Pod ephemeral storage usage is available.',
      });
    }
    if (!capabilities.podDiskIo) {
      badges.push({
        label: 'Pod Disk I/O Unsupported',
        classes: unsupportedBadge,
        title: 'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
      });
    }

    return badges;
  });

  const proxmoxNode = createMemo(() => toNodeFromProxmox(props.resource));
  const agentHost = createMemo(() => toHostFromAgent(props.resource, agentMeta()));
  const temperatureRows = createMemo(() => buildTemperatureRows(agentHost()?.sensors));

  const dockerHostData = createMemo(() => platformData()?.docker as DockerPlatformData | undefined);
  const dockerHostSourceId = createMemo(() => (dockerHostData()?.hostSourceId || '').trim() || null);
  const dockerUpdatesAvailable = createMemo(() => dockerHostData()?.updatesAvailableCount ?? 0);
  const dockerContainerCount = createMemo(() => dockerHostData()?.containerCount ?? 0);
  const dockerUpdatesCheckedRelative = createMemo(() => {
    const raw = dockerHostData()?.updatesLastCheckedAt;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });
  const dockerHostCommand = createMemo(() => dockerHostData()?.command);
  const dockerHostCommandActive = createMemo(() => {
    const status = (dockerHostCommand()?.status || '').trim().toLowerCase();
    return ['queued', 'dispatched', 'acknowledged', 'in_progress'].includes(status);
  });
  const dockerUpdateActionsDisabled = createMemo(() => shouldHideDockerUpdateActions());
  const dockerUpdateActionsLoading = createMemo(() => !areSystemSettingsLoaded());

  const [dockerActionError, setDockerActionError] = createSignal<string>('');
  const [dockerActionNote, setDockerActionNote] = createSignal<string>('');
  const [confirmUpdateAll, setConfirmUpdateAll] = createSignal(false);
  const [dockerActionBusy, setDockerActionBusy] = createSignal(false);
  const dockerSwarmInfo = createMemo(() => dockerHostData()?.swarm);
  const dockerSwarmClusterKey = createMemo(() => {
    const swarm = dockerSwarmInfo();
    return (swarm?.clusterName || swarm?.clusterId || '').trim();
  });

  const [k8sDeploymentsPrefillNamespace, setK8sDeploymentsPrefillNamespace] = createSignal('');

  const pbsData = createMemo(() => platformData()?.pbs);
  const pmgData = createMemo(() => platformData()?.pmg);
  const pbsJobTotal = createMemo(() => {
    const pbs = pbsData();
    if (!pbs) return 0;
    return (
      (pbs.backupJobCount || 0) +
      (pbs.syncJobCount || 0) +
      (pbs.verifyJobCount || 0) +
      (pbs.pruneJobCount || 0) +
      (pbs.garbageJobCount || 0)
    );
  });
  const pmgQueueBacklog = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return 0;
    return (pmg.queueDeferred || 0) + (pmg.queueHold || 0);
  });
  const pmgUpdatedRelative = createMemo(() => {
    const raw = pmgData()?.lastUpdated;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });
  const pbsJobBreakdown = createMemo(() => {
    const pbs = pbsData();
    if (!pbs) return [] as Array<{ label: string; value: number }>;
    return [
      { label: 'Backup', value: pbs.backupJobCount || 0 },
      { label: 'Sync', value: pbs.syncJobCount || 0 },
      { label: 'Verify', value: pbs.verifyJobCount || 0 },
      { label: 'Prune', value: pbs.pruneJobCount || 0 },
      { label: 'Garbage', value: pbs.garbageJobCount || 0 },
    ];
  });
  const pbsVisibleJobBreakdown = createMemo(() => {
    const all = pbsJobBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const pmgQueueBreakdown = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return [] as Array<{ label: string; value: number; warn?: boolean }>;
    return [
      { label: 'Active', value: pmg.queueActive || 0 },
      { label: 'Deferred', value: pmg.queueDeferred || 0, warn: (pmg.queueDeferred || 0) > 0 },
      { label: 'Hold', value: pmg.queueHold || 0, warn: (pmg.queueHold || 0) > 0 },
      { label: 'Incoming', value: pmg.queueIncoming || 0 },
    ];
  });
  const pmgVisibleQueueBreakdown = createMemo(() => {
    const all = pmgQueueBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const pmgMailBreakdown = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return [] as Array<{ label: string; value: number }>;
    return [
      { label: 'Mail', value: pmg.mailCountTotal || 0 },
      { label: 'Spam', value: pmg.spamIn || 0 },
      { label: 'Virus', value: pmg.virusIn || 0 },
    ];
  });
  const pmgVisibleMailBreakdown = createMemo(() => {
    const all = pmgMailBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const sourceStatus = createMemo(() => platformData()?.sourceStatus ?? {});
  const sourceHealthSummary = createMemo(() => {
    const entries = Object.entries(sourceStatus());
    if (entries.length === 0) return null;

    let healthy = 0;
    let warning = 0;
    let unhealthy = 0;
    const parts: string[] = [];

    for (const [source, status] of entries) {
      const normalized = (status?.status || '').trim().toLowerCase();
      parts.push(`${source}:${normalized || 'unknown'}`);
      if (['online', 'running', 'healthy', 'connected', 'ok'].includes(normalized)) {
        healthy += 1;
      } else if (['degraded', 'warning', 'stale'].includes(normalized)) {
        warning += 1;
      } else {
        unhealthy += 1;
      }
    }

    const total = entries.length;
    if (unhealthy > 0) {
      return {
        label: `${unhealthy}/${total} unhealthy`,
        className: 'text-red-600 dark:text-red-400',
        title: parts.join(' • '),
      };
    }
    if (warning > 0) {
      return {
        label: `${warning}/${total} degraded`,
        className: 'text-amber-600 dark:text-amber-400',
        title: parts.join(' • '),
      };
    }
    return {
      label: `${healthy}/${total} healthy`,
      className: 'text-emerald-600 dark:text-emerald-400',
      title: parts.join(' • '),
    };
  });
  const sourceSummary = createMemo(() => {
    const health = sourceHealthSummary();
    if (health) return health;
    const sources = mergedSources();
    if (sources.length === 0) return null;
    return {
      label: sources.length === 1 ? sources[0].toUpperCase() : `${sources.length} sources`,
      className: 'text-slate-700 dark:text-slate-200',
      title: sources.join(' • '),
    };
  });
  const identityAliasValues = createMemo(() => {
    const data = platformData();
    const pbs = data?.pbs;
    const pmg = data?.pmg;
    const proxmox = data?.proxmox;
    const agent = data?.agent;
    const raw = [
      props.resource.discoveryTarget?.hostId,
      props.resource.discoveryTarget?.resourceId,
      proxmox?.nodeName,
      agent?.agentId,
      agent?.hostname,
      pbs?.instanceId,
      pbs?.hostname,
      pmg?.instanceId,
      pmg?.hostname,
      props.resource.identity?.hostname,
      props.resource.identity?.machineId,
    ];
    const seen = new Set<string>();
    const deduped: string[] = [];
    for (const value of raw) {
      if (!value) continue;
      const trimmed = value.trim();
      if (!trimmed) continue;
      const normalized = trimmed.toLowerCase();
      if (seen.has(normalized)) continue;
      seen.add(normalized);
      deduped.push(trimmed);
    }
    return deduped;
  });
  const primaryIdentityRows = createMemo(() => {
    const rows: Array<{ label: string; value: string }> = [];
    if (props.resource.identity?.hostname) {
      rows.push({ label: 'Hostname', value: props.resource.identity.hostname });
    }
    if (props.resource.identity?.machineId) {
      rows.push({ label: 'Machine ID', value: props.resource.identity.machineId });
    }
    if (props.resource.clusterId) {
      rows.push({ label: 'Cluster', value: props.resource.clusterId });
    }
    if (props.resource.parentId) {
      rows.push({ label: 'Parent', value: props.resource.parentId });
    }
    if (props.resource.discoveryTarget?.resourceType) {
      rows.push({
        label: 'Discovery',
        value: `${props.resource.discoveryTarget.resourceType}:${props.resource.discoveryTarget.resourceId}`,
      });
    }
    return rows;
  });
  const identityCardHasRichData = createMemo(() =>
    primaryIdentityRows().length > 0 ||
    (props.resource.identity?.ips?.length || 0) > 0 ||
    (props.resource.tags?.length || 0) > 0 ||
    identityAliasValues().length > 0,
  );
  const aliasPreviewValues = createMemo(() => identityAliasValues().slice(0, ALIAS_COLLAPSE_THRESHOLD));
  const hasAliasOverflow = createMemo(() => identityAliasValues().length > ALIAS_COLLAPSE_THRESHOLD);
  const hasMergedSources = createMemo(() => mergedSources().length > 1);
  const discoveryConfig = createMemo(() => toDiscoveryConfig(props.resource));
  const workloadsHref = createMemo(() => buildWorkloadsHref(props.resource));
  const relatedLinks = createMemo(() => {
    const links: Array<{ href: string; label: string; ariaLabel: string }> = [];
    const workloads = workloadsHref();
    if (workloads) {
      links.push({
        href: workloads,
        label: 'Open in Workloads',
        ariaLabel: `Open related workloads for ${displayName()}`,
      });
    }
    links.push(...buildServiceDetailLinks(props.resource));
    const seen = new Set<string>();
    return links.filter((link) => {
      if (seen.has(link.href)) return false;
      seen.add(link.href);
      return true;
    });
  });
  const sourceSections = createMemo(() => {
    const data = platformData();
    if (!data) return [];
    const sections = [
      { id: 'proxmox', label: 'Proxmox', payload: data.proxmox },
      { id: 'agent', label: 'Agent', payload: data.agent },
      { id: 'docker', label: 'Containers', payload: data.docker },
      { id: 'pbs', label: 'PBS', payload: data.pbs },
      { id: 'pmg', label: 'PMG', payload: data.pmg },
      { id: 'kubernetes', label: 'Kubernetes', payload: data.kubernetes },
      { id: 'metrics', label: 'Metrics', payload: data.metrics },
    ];
    return sections.filter((section) => section.payload !== undefined);
  });
  const identityMatchInfo = createMemo(() => {
    const data = platformData();
    return (
      data?.identityMatch ??
      data?.matchResults ??
      data?.matchCandidates ??
      data?.matches ??
      undefined
    );
  });
  const debugBundle = createMemo(() => ({
    resource: props.resource,
    identity: {
      resourceIdentity: props.resource.identity,
      matchInfo: identityMatchInfo(),
    },
    sources: {
      sourceStatus: sourceStatus(),
      proxmox: platformData()?.proxmox,
      agent: platformData()?.agent,
      docker: platformData()?.docker,
      pbs: platformData()?.pbs,
      pmg: platformData()?.pmg,
      kubernetes: platformData()?.kubernetes,
      metrics: platformData()?.metrics,
    },
  }));
  const debugJson = createMemo(() => JSON.stringify(debugBundle(), null, 2));

  createEffect(() => {
    if (!debugEnabled() && activeTab() === 'debug') {
      setActiveTab('overview');
    }
  });

  const tabs = createMemo(() => {
    const base = [
      { id: 'overview' as DrawerTab, label: 'Overview' },
      ...(props.resource.type === 'pmg' ? [{ id: 'mail' as DrawerTab, label: 'Mail' }] : []),
      ...(props.resource.type === 'k8s-cluster' ? [{ id: 'namespaces' as DrawerTab, label: 'Namespaces' }] : []),
      ...(props.resource.type === 'k8s-cluster' ? [{ id: 'deployments' as DrawerTab, label: 'Deployments' }] : []),
      ...(props.resource.type === 'docker-host' && dockerSwarmClusterKey()
        ? [{ id: 'swarm' as DrawerTab, label: 'Swarm' }]
        : []),
      { id: 'discovery' as DrawerTab, label: 'Discovery' },
    ];
    if (debugEnabled()) {
      base.push({ id: 'debug' as DrawerTab, label: 'Debug' });
    }
    return base;
  });

  createEffect(() => {
    const current = activeTab();
    const available = new Set(tabs().map((tab) => tab.id));
    if (!available.has(current)) {
      setActiveTab('overview');
    }
  });

  const formatSourceTime = (value?: string | number) => {
    if (!value) return '';
    const timestamp = typeof value === 'number' ? value : Date.parse(value);
    if (!Number.isFinite(timestamp)) return '';
    return formatRelativeTime(timestamp);
  };

  const handleCopyJson = async () => {
    const payload = debugJson();
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(payload);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = payload;
        textarea.setAttribute('readonly', 'true');
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

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
            <div class="text-sm font-semibold text-slate-900 dark:text-slate-100 truncate" title={displayName()}>
              {displayName()}
            </div>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 truncate" title={props.resource.id}>
            {props.resource.id}
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
            <For each={kubernetesCapabilityBadges()}>
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
            class="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-700 dark:hover:text-slate-300"
            aria-label="Close"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </Show>
      </div>

      <Show when={relatedLinks().length > 0}>
        <div class="flex items-center justify-end gap-2">
          <For each={relatedLinks()}>
            {(link) => (
              <a
                href={link.href}
                aria-label={link.ariaLabel}
                class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-900"
              >
                {link.label}
              </a>
            )}
          </For>
        </div>
      </Show>

      <div class="flex items-center gap-6 border-b border-slate-200 dark:border-slate-700 px-1 mb-1">
        <For each={tabs()}>
          {(tab) => (
            <button
              onClick={() => setActiveTab(tab.id)}
              class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === tab.id
                ? 'text-blue-600 dark:text-blue-400'
                : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
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
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        <Show when={proxmoxNode() || agentHost()}>
          <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
            <Show when={proxmoxNode()}>
              {(node) => (
                <>
                  <SystemInfoCard variant="node" node={node()} />
                  <HardwareCard variant="node" node={node()} />
                  <RootDiskCard node={node()} />
                </>
              )}
            </Show>
            <Show when={agentHost()}>
              {(host) => (
                <>
                  <SystemInfoCard variant="host" host={host()} />
                  <HardwareCard variant="host" host={host()} />
                  <NetworkInterfacesCard interfaces={host().networkInterfaces} />
                  <DisksCard disks={host().disks} />
                  <RaidCard arrays={agentMeta()?.raid} />
                  <TemperaturesCard rows={temperatureRows()} />
                </>
              )}
            </Show>
          </div>
        </Show>

        <div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3 mt-3">
          <div class="rounded border border-slate-200 bg-white p-3 dark:border-slate-600 dark:bg-slate-800">
            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Runtime</div>
            <div class="space-y-1.5 text-[11px]">
              <div class="flex items-center justify-between gap-2">
                <span class="text-slate-500 dark:text-slate-400">State</span>
                <span class="font-medium text-slate-700 dark:text-slate-200 capitalize">{props.resource.status || 'unknown'}</span>
              </div>
              <Show when={props.resource.uptime}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Uptime</span>
                  <span class="font-medium text-slate-700 dark:text-slate-200">{formatUptime(props.resource.uptime ?? 0)}</span>
                </div>
              </Show>
              <Show when={props.resource.lastSeen}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Last Seen</span>
                  <span
                    class="font-medium text-slate-700 dark:text-slate-200"
                    title={lastSeenAbsolute()}
                  >
                    {lastSeen() || '—'}
                  </span>
                </div>
              </Show>
              <Show when={sourceSummary()}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Sources</span>
                  <span class={`font-medium ${sourceSummary()!.className}`} title={sourceSummary()!.title}>
                    {sourceSummary()!.label}
                  </span>
                </div>
              </Show>
              <div class="flex items-center justify-between gap-2">
                <span class="text-slate-500 dark:text-slate-400">Mode</span>
                <span class="font-medium text-slate-700 dark:text-slate-200">{formatSourceType(props.resource.sourceType)}</span>
              </div>
              <Show when={(props.resource.alerts?.length || 0) > 0}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Alerts</span>
                  <span class="font-medium text-amber-600 dark:text-amber-400">
                    {formatInteger(props.resource.alerts?.length)}
                  </span>
                </div>
              </Show>
              <Show when={props.resource.platformId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Platform ID</span>
                  <span class="font-medium text-slate-700 dark:text-slate-200 truncate" title={props.resource.platformId}>
                    {props.resource.platformId}
                  </span>
                </div>
              </Show>
            </div>
          </div>

          <div class="rounded border border-slate-200 bg-white p-3 dark:border-slate-600 dark:bg-slate-800">
            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Identity</div>
            <div class="space-y-1.5 text-[11px]">
              <For each={primaryIdentityRows()}>
                {(row) => (
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-slate-500 dark:text-slate-400">{row.label}</span>
                    <span class="font-medium text-slate-700 dark:text-slate-200 truncate" title={row.value}>
                      {row.value}
                    </span>
                  </div>
                )}
              </For>
              <Show when={props.resource.identity?.ips && props.resource.identity.ips.length > 0}>
                <div class="flex flex-col gap-1">
                  <span class="text-slate-500 dark:text-slate-400">IP Addresses</span>
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
                  <span class="text-slate-500 dark:text-slate-400">Tags</span>
                  <TagBadges tags={props.resource.tags} maxVisible={6} />
                </div>
              </Show>
              <Show when={identityAliasValues().length > 0}>
                <Show
                  when={hasAliasOverflow()}
                  fallback={
                    <div class="flex flex-col gap-1">
                      <span class="text-slate-500 dark:text-slate-400">Aliases</span>
                      <div class="flex flex-wrap gap-1">
                        <For each={aliasPreviewValues()}>
                          {(value) => (
                            <span class="inline-flex items-center rounded bg-slate-100 px-1.5 py-0.5 text-[10px] text-slate-700 dark:bg-slate-800 dark:text-slate-200" title={value}>
                              {value}
                            </span>
                          )}
                        </For>
                      </div>
                    </div>
                  }
                >
                  <details class="rounded border border-slate-200 bg-white px-2 py-1.5 dark:border-slate-600 dark:bg-slate-800">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-slate-600 dark:text-slate-300">
                      <span>Aliases</span>
                      <span class="text-slate-500 dark:text-slate-400">{identityAliasValues().length}</span>
                    </summary>
                    <div class="mt-2 flex flex-wrap gap-1 border-t border-slate-200 pt-2 dark:border-slate-600">
                      <For each={identityAliasValues()}>
                        {(value) => (
                          <span class="inline-flex items-center rounded bg-slate-100 px-1.5 py-0.5 text-[10px] text-slate-700 dark:bg-slate-800 dark:text-slate-200" title={value}>
                            {value}
                          </span>
                        )}
                      </For>
                    </div>
                  </details>
                </Show>
              </Show>
              <Show when={!identityCardHasRichData()}>
                <div class="rounded border border-dashed border-slate-300 bg-slate-50 px-2 py-1.5 text-[10px] text-slate-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-400">
                  No enriched identity metadata yet.
                </div>
              </Show>
            </div>
          </div>

          <Show when={props.resource.type === 'docker-host'}>
            <div class="rounded border border-sky-200 bg-sky-50 p-3 dark:border-sky-700 dark:bg-sky-900">
              <div class="mb-2 flex items-center justify-between gap-2">
                <div class="text-[11px] font-medium uppercase tracking-wide text-sky-700 dark:text-sky-300">Container Updates</div>
                <Show when={dockerHostData()?.runtime}>
                  <span class="max-w-[55%] truncate text-[10px] text-sky-700 dark:text-sky-300" title={dockerHostData()?.runtime}>
                    {dockerHostData()?.runtime}
                  </span>
                </Show>
              </div>

              <div class="space-y-1.5 text-[11px]">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Containers</span>
                  <span class="font-medium text-slate-700 dark:text-slate-200">{formatInteger(dockerContainerCount())}</span>
                </div>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-slate-500 dark:text-slate-400">Updates Available</span>
                  <span class={`font-medium ${dockerUpdatesAvailable() > 0 ? 'text-sky-700 dark:text-sky-300' : 'text-slate-700 dark:text-slate-200'}`}>
                    {formatInteger(dockerUpdatesAvailable())}
                  </span>
                </div>
                <Show when={dockerUpdatesCheckedRelative()}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-slate-500 dark:text-slate-400">Last Check</span>
                    <span class="font-medium text-slate-700 dark:text-slate-200">{dockerUpdatesCheckedRelative()}</span>
                  </div>
                </Show>

                <Show when={dockerHostCommand()?.type || dockerHostCommand()?.status}>
                  <div class="rounded border border-sky-200 bg-white px-2 py-1.5 text-[10px] dark:border-sky-700 dark:bg-slate-800">
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-slate-500 dark:text-slate-400">Command</span>
                      <span class="font-medium text-slate-700 dark:text-slate-200">
                        {(dockerHostCommand()?.type || 'command').replace(/_/g, ' ')}
                      </span>
                    </div>
                    <div class="mt-1 flex items-center justify-between gap-2">
                      <span class="text-slate-500 dark:text-slate-400">Status</span>
                      <span class={`font-medium ${dockerHostCommandActive() ? 'text-sky-700 dark:text-sky-300' : 'text-slate-700 dark:text-slate-200'}`}>
                        {(dockerHostCommand()?.status || 'unknown').replace(/_/g, ' ')}
                      </span>
                    </div>
                    <Show when={dockerHostCommand()?.message}>
                      <div class="mt-1 text-slate-600 dark:text-slate-300 truncate" title={dockerHostCommand()?.message}>
                        {dockerHostCommand()?.message}
                      </div>
                    </Show>
                    <Show when={dockerHostCommand()?.failureReason}>
                      <div class="mt-1 text-red-700 dark:text-red-300 truncate" title={dockerHostCommand()?.failureReason}>
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
                  <div class="rounded border border-sky-200 bg-white px-2 py-1.5 text-[10px] text-slate-700 dark:border-sky-700 dark:bg-slate-800 dark:text-slate-200">
                    {dockerActionNote()}
                  </div>
                </Show>

                <div class="flex flex-wrap items-center gap-2 pt-1">
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
                        setDockerActionNote('Update check queued. Results will refresh on the next agent report.');
                      } catch (err) {
                        setDockerActionError((err as Error)?.message || 'Failed to queue update check');
                      } finally {
                        setDockerActionBusy(false);
                      }
                    }}
                    class="rounded-md border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60 disabled:hover:bg-white dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-800 dark:disabled:hover:bg-slate-900"
                    title={dockerUpdateActionsLoading() ? 'Loading server settings...' : undefined}
                  >
                    Check Updates
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
                        setDockerActionNote(`Click again to confirm updating ${dockerUpdatesAvailable()} container(s).`);
                        return;
                      }

                      try {
                        setDockerActionBusy(true);
                        await MonitoringAPI.updateAllDockerContainers(hostId);
                        setDockerActionNote('Batch update queued. Progress will appear as the agent reports back.');
                      } catch (err) {
                        setDockerActionError((err as Error)?.message || 'Failed to queue batch update');
                      } finally {
                        setDockerActionBusy(false);
                        setConfirmUpdateAll(false);
                      }
                    }}
                    class="rounded-md border border-sky-200 bg-sky-600 px-2.5 py-1 text-[11px] font-semibold text-white hover:bg-sky-700 disabled:opacity-60 disabled:hover:bg-sky-600 dark:border-sky-700 dark:bg-sky-600 dark:hover:bg-sky-500 dark:disabled:hover:bg-sky-600"
                    title={dockerUpdateActionsDisabled() ? 'Docker updates are disabled by server configuration.' : undefined}
                  >
                    {confirmUpdateAll() ? 'Confirm Update All' : `Update All${dockerUpdatesAvailable() > 0 ? ` (${dockerUpdatesAvailable()})` : ''}`}
                  </button>
                </div>
              </div>
            </div>
          </Show>

          <Show when={pbsData()}>
            {(pbs) => (
              <div class="rounded border border-indigo-200 bg-indigo-50 p-3 dark:border-indigo-700 dark:bg-indigo-900">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-indigo-700 dark:text-indigo-300">PBS Service</div>
                  <Show when={pbs().hostname}>
                    <span class="max-w-[55%] truncate text-[10px] text-indigo-700 dark:text-indigo-300" title={pbs().hostname}>
                      {pbs().hostname}
                    </span>
                  </Show>
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-slate-500 dark:text-slate-400">Connection</span>
                    <span class={`font-medium ${healthToneClass(pbs().connectionHealth)}`}>
                      {normalizeHealthLabel(pbs().connectionHealth)}
                    </span>
                  </div>
                  <Show when={pbs().version}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-slate-500 dark:text-slate-400">Version</span>
                      <span class="font-medium text-slate-700 dark:text-slate-200">{pbs().version}</span>
                    </div>
                  </Show>
                  <Show when={pbs().uptimeSeconds || props.resource.uptime}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-slate-500 dark:text-slate-400">Uptime</span>
                      <span class="font-medium text-slate-700 dark:text-slate-200">
                        {formatUptime(pbs().uptimeSeconds ?? props.resource.uptime ?? 0)}
                      </span>
                    </div>
                  </Show>
                  <div class="grid grid-cols-2 gap-2 pt-1">
                    <div class="rounded border border-indigo-200 bg-white px-2 py-1.5 dark:border-indigo-700 dark:bg-slate-800">
                      <div class="text-[10px] text-slate-500 dark:text-slate-400">Datastores</div>
                      <div class="text-sm font-semibold text-slate-700 dark:text-slate-200">{formatInteger(pbs().datastoreCount)}</div>
                    </div>
                    <div class="rounded border border-indigo-200 bg-white px-2 py-1.5 dark:border-indigo-700 dark:bg-slate-800">
                      <div class="text-[10px] text-slate-500 dark:text-slate-400">Total Jobs</div>
                      <div class="text-sm font-semibold text-slate-700 dark:text-slate-200">{formatInteger(pbsJobTotal())}</div>
                    </div>
                  </div>
                  <details class="rounded border border-indigo-200 bg-white px-2 py-1.5 dark:border-indigo-700 dark:bg-slate-800">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-slate-600 dark:text-slate-300">
                      <span>Job breakdown</span>
                      <span class="text-slate-500 dark:text-slate-400">{pbsVisibleJobBreakdown().length} types</span>
                    </summary>
                    <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-indigo-200 pt-2 text-[10px] dark:border-indigo-700">
                      <For each={pbsVisibleJobBreakdown()}>
                        {(entry) => (
                          <span class="text-slate-500 dark:text-slate-400">
                            {entry.label}:{' '}
                            <span class="font-medium text-slate-700 dark:text-slate-200">{formatInteger(entry.value)}</span>
                          </span>
                        )}
                      </For>
                    </div>
                  </details>
                </div>
              </div>
            )}
          </Show>

          <Show when={pmgData()}>
            {(pmg) => (
              <div class="rounded border border-rose-200 bg-rose-50 p-3 dark:border-rose-700 dark:bg-rose-900">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-rose-700 dark:text-rose-300">Mail Gateway</div>
                  <Show when={pmg().hostname}>
                    <span class="max-w-[55%] truncate text-[10px] text-rose-700 dark:text-rose-300" title={pmg().hostname}>
                      {pmg().hostname}
                    </span>
                  </Show>
                </div>
                <div class="space-y-1.5 text-[11px]">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-slate-500 dark:text-slate-400">Connection</span>
                    <span class={`font-medium ${healthToneClass(pmg().connectionHealth)}`}>
                      {normalizeHealthLabel(pmg().connectionHealth)}
                    </span>
                  </div>
                  <Show when={pmg().version}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-slate-500 dark:text-slate-400">Version</span>
                      <span class="font-medium text-slate-700 dark:text-slate-200">{pmg().version}</span>
                    </div>
                  </Show>
                  <Show when={pmg().uptimeSeconds || props.resource.uptime}>
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-slate-500 dark:text-slate-400">Uptime</span>
                      <span class="font-medium text-slate-700 dark:text-slate-200">
                        {formatUptime(pmg().uptimeSeconds ?? props.resource.uptime ?? 0)}
                      </span>
                    </div>
                  </Show>
                  <div class="grid grid-cols-3 gap-2 pt-1">
                    <div class="rounded border border-rose-200 bg-white px-2 py-1.5 dark:border-rose-700 dark:bg-slate-800">
                      <div class="text-[10px] text-slate-500 dark:text-slate-400">Nodes</div>
                      <div class="text-sm font-semibold text-slate-700 dark:text-slate-200">{formatInteger(pmg().nodeCount)}</div>
                    </div>
                    <div class="rounded border border-rose-200 bg-white px-2 py-1.5 dark:border-rose-700 dark:bg-slate-800">
                      <div class="text-[10px] text-slate-500 dark:text-slate-400">Queue Total</div>
                      <div class={`text-sm font-semibold ${pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-slate-700 dark:text-slate-200'}`}>
                        {formatInteger(pmg().queueTotal)}
                      </div>
                    </div>
                    <div class="rounded border border-rose-200 bg-white px-2 py-1.5 dark:border-rose-700 dark:bg-slate-800">
                      <div class="text-[10px] text-slate-500 dark:text-slate-400">Backlog</div>
                      <div class={`text-sm font-semibold ${pmgQueueBacklog() > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-slate-700 dark:text-slate-200'}`}>
                        {formatInteger(pmgQueueBacklog())}
                      </div>
                    </div>
                  </div>
                  <details class="rounded border border-rose-200 bg-white px-2 py-1.5 dark:border-rose-700 dark:bg-slate-800">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-slate-600 dark:text-slate-300">
                      <span>Queue breakdown</span>
                      <span class="text-slate-500 dark:text-slate-400">{pmgVisibleQueueBreakdown().length} signals</span>
                    </summary>
                    <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-rose-200 pt-2 text-[10px] dark:border-rose-700">
                      <For each={pmgVisibleQueueBreakdown()}>
                        {(entry) => (
                          <span class="text-slate-500 dark:text-slate-400">
                            {entry.label}:{' '}
                            <span class={`font-medium ${entry.warn ? 'text-amber-600 dark:text-amber-400' : 'text-slate-700 dark:text-slate-200'}`}>
                              {formatInteger(entry.value)}
                            </span>
                          </span>
                        )}
                      </For>
                    </div>
                  </details>
                  <details class="rounded border border-rose-200 bg-white px-2 py-1.5 dark:border-rose-700 dark:bg-slate-800">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-slate-600 dark:text-slate-300">
                      <span>Mail processing</span>
                      <span class="text-slate-500 dark:text-slate-400">{pmgVisibleMailBreakdown().length} signals</span>
                    </summary>
                    <div class="mt-2 grid grid-cols-3 gap-x-3 gap-y-1 border-t border-rose-200 pt-2 text-[10px] dark:border-rose-700">
                      <For each={pmgVisibleMailBreakdown()}>
                        {(entry) => (
                          <span class="text-slate-500 dark:text-slate-400">
                            {entry.label}:{' '}
                            <span class="font-medium text-slate-700 dark:text-slate-200">{formatInteger(entry.value)}</span>
                          </span>
                        )}
                      </For>
                    </div>
                    <Show when={pmgUpdatedRelative()}>
                      <div class="mt-2 flex items-center justify-between gap-2 border-t border-rose-200 pt-2 text-[10px] dark:border-rose-700">
                        <span class="text-slate-500 dark:text-slate-400">Updated</span>
                        <span class="font-medium text-slate-700 dark:text-slate-200">{pmgUpdatedRelative()}</span>
                      </div>
                    </Show>
                  </details>
                </div>
              </div>
            )}
          </Show>
        </div>

        <Show when={discoveryConfig()}>
          {(config) => (
            <div class="mt-3">
              <WebInterfaceUrlField
                metadataKind={config().metadataKind}
                metadataId={config().metadataId}
                targetLabel={config().targetLabel}
              />
            </div>
          )}
        </Show>
      </div>

      {/* Discovery Tab */}
      <div class={activeTab() === 'discovery' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        <Show
          when={discoveryConfig()}
          fallback={
            <div class="rounded border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300">
              Discovery is not available for this resource type yet.
            </div>
          }
        >
          {(config) => (
            <Suspense
              fallback={
                <div class="flex items-center justify-center py-8">
                  <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                  <span class="ml-2 text-sm text-slate-500 dark:text-slate-400">Loading discovery...</span>
                </div>
              }
            >
              <DiscoveryTab
                resourceType={config().resourceType}
                hostId={config().hostId}
                resourceId={config().resourceId}
                hostname={config().hostname}
              />
            </Suspense>
          )}
        </Show>
      </div>

      {/* PMG Mail Tab */}
      <div class={activeTab() === 'mail' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'mail'}>
          <Show
            when={props.resource.type === 'pmg'}
            fallback={
              <div class="rounded border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300">
                Mail details are only available for PMG resources.
              </div>
            }
          >
            <PMGInstanceDrawer
              resourceId={props.resource.id}
              resourceName={props.resource.name || props.resource.displayName || props.resource.id}
            />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Namespaces Tab */}
      <div class={activeTab() === 'namespaces' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'namespaces'}>
          <Show
            when={props.resource.type === 'k8s-cluster'}
            fallback={
              <div class="rounded border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300">
                Namespaces are only available for Kubernetes cluster resources.
              </div>
            }
          >
            <K8sNamespacesDrawer
              cluster={props.resource.name || props.resource.displayName || ''}
              onOpenDeployments={(ns) => {
                setK8sDeploymentsPrefillNamespace((ns || '').trim());
                setActiveTab('deployments');
              }}
            />
          </Show>
        </Show>
      </div>

      {/* Kubernetes Deployments Tab */}
      <div class={activeTab() === 'deployments' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'deployments'}>
          <Show
            when={props.resource.type === 'k8s-cluster'}
            fallback={
              <div class="rounded border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300">
                Deployments are only available for Kubernetes cluster resources.
              </div>
            }
          >
            <K8sDeploymentsDrawer
              cluster={props.resource.name || props.resource.displayName || ''}
              initialNamespace={k8sDeploymentsPrefillNamespace() || null}
            />
          </Show>
        </Show>
      </div>

      {/* Docker Swarm Tab */}
      <div class={activeTab() === 'swarm' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
        {/* Mount on-demand to avoid background fetching when the tab isn't open. */}
        <Show when={activeTab() === 'swarm'}>
          <Show
            when={props.resource.type === 'docker-host' && dockerSwarmClusterKey()}
            fallback={
              <div class="rounded border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300">
                Swarm details are only available for Docker hosts reporting Swarm metadata.
              </div>
            }
          >
            <SwarmServicesDrawer cluster={dockerSwarmClusterKey()} swarm={dockerSwarmInfo()} />
          </Show>
        </Show>
      </div>

      {/* Debug Tab */}
      <Show when={debugEnabled()}>
        <div class={activeTab() === 'debug' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
          <div class="flex items-center justify-between gap-3">
            <div class="text-xs text-slate-500 dark:text-slate-400">
              Debug mode is enabled via localStorage (<code>pulse_debug_mode</code>).
            </div>
            <button
              type="button"
              onClick={handleCopyJson}
              class="rounded-md border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-800"
            >
              {copied() ? 'Copied' : 'Copy JSON'}
            </button>
          </div>

          <div class="mt-3 space-y-4">
            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Unified Resource</div>
              <pre class="max-h-[280px] overflow-auto rounded-md bg-slate-900 p-3 text-[11px] text-slate-100">
                {JSON.stringify(props.resource, null, 2)}
              </pre>
            </div>

            <div>
              <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Identity Matching</div>
              <pre class="max-h-[220px] overflow-auto rounded-md bg-slate-900 p-3 text-[11px] text-slate-100">
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
              <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Sources</div>
              <div class="space-y-2">
                <For each={sourceSections()}>
                  {(section) => {
                    const status = sourceStatus()[section.id];
                    const lastSeenText = formatSourceTime(status?.lastSeen);
                    return (
                      <details class="rounded-md border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-800">
                        <summary class="flex cursor-pointer list-none items-center justify-between text-sm font-medium text-slate-700 dark:text-slate-200">
                          <span>{section.label}</span>
                          <span class="text-[11px] text-slate-500 dark:text-slate-400">
                            {status?.status ?? 'unknown'}
                            {lastSeenText ? ` • ${lastSeenText}` : ''}
                          </span>
                        </summary>
                        <Show when={status?.error}>
                          <div class="mt-2 text-[11px] text-amber-600 dark:text-amber-300">
                            {status?.error}
                          </div>
                        </Show>
                        <pre class="mt-3 max-h-[220px] overflow-auto rounded-md bg-slate-900 p-3 text-[11px] text-slate-100">
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
            class="text-xs font-medium text-slate-500 transition-colors hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
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
  return <DrawerContent resource={props.resource} onClose={props.onClose} />;
};

export default ResourceDetailDrawer;
