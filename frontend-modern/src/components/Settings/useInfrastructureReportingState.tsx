import { createEffect, createMemo, createSignal, For, onMount, Show } from 'solid-js';
import { useWebSocket } from '@/App';
import {
  MonitoringAPI,
  type RemovedDockerHost,
  type RemovedHostAgent,
  type RemovedKubernetesCluster,
} from '@/api/monitoring';
import {
  AgentProfilesAPI,
  MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
  type AgentProfile,
  type AgentProfileAssignment,
} from '@/api/agentProfiles';
import { notificationStore } from '@/stores/notifications';
import { useResources } from '@/hooks/useResources';
import { logger } from '@/utils/logger';
import { getPreferredNamedEntityLabel } from '@/utils/resourceIdentity';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  getActionableKubernetesClusterIdFromResource,
  getExplicitAgentIdFromResource,
} from '@/utils/agentResources';
import {
  getAgentCapabilityBadgeClass,
  type AgentCapability,
} from '@/utils/agentCapabilityPresentation';
import {
  ALLOW_RECONNECT_LABEL,
  MONITORING_STOPPED_STATUS_LABEL,
  getUnifiedAgentStatusPresentation,
} from '@/utils/unifiedAgentStatusPresentation';
import {
  getInventorySubjectLabel,
  getUnifiedAgentAllowReconnectErrorMessage,
  getUnifiedAgentAllowReconnectSuccessMessage,
  getUnifiedAgentLastSeenLabel,
  getUnifiedAgentStopMonitoringErrorMessage,
  getUnifiedAgentStopMonitoringSuccessMessage,
  getUnifiedAgentStopMonitoringUnavailableMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import type { ConnectedInfrastructureItem } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getCapabilitySurfaceLabel,
  getRowReportingSummary,
  getStopMonitoringScopeLabel,
  joinHumanList,
  rowFromConnectedInfrastructureItem,
  type AgentPlatform,
  type InventoryActionNotice,
  type InventoryActionType,
  type ScopeCategory,
  type StopMonitoringDialogState,
  type UnifiedAgentRow,
} from './infrastructureOperationsModel';

export const useInfrastructureReportingState = () => {
  const { state } = useWebSocket();
  const { mutate: mutateResources, refetch: refetchResources } = useResources();

  const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
  const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
  const [pendingScopeUpdates, setPendingScopeUpdates] = createSignal<Record<string, boolean>>({});
  const [pendingInventoryActions, setPendingInventoryActions] = createSignal<
    Record<string, InventoryActionType>
  >({});
  const [inventoryActionNotice, setInventoryActionNotice] =
    createSignal<InventoryActionNotice | null>(null);
  const [optimisticRemovedHostAgents, setOptimisticRemovedHostAgents] = createSignal<
    RemovedHostAgent[]
  >([]);
  const [optimisticRemovedDockerHosts, setOptimisticRemovedDockerHosts] = createSignal<
    RemovedDockerHost[]
  >([]);
  const [optimisticRemovedKubernetesClusters, setOptimisticRemovedKubernetesClusters] =
    createSignal<RemovedKubernetesCluster[]>([]);
  const [stopMonitoringDialog, setStopMonitoringDialog] =
    createSignal<StopMonitoringDialogState | null>(null);
  const [expandedRowKey, setExpandedRowKey] = createSignal<string | null>(null);
  const [selectedIgnoredRowKey, setSelectedIgnoredRowKey] = createSignal<string | null>(null);
  const [filterCapability, setFilterCapability] = createSignal<'all' | AgentCapability>('all');
  const [filterScope, setFilterScope] = createSignal<'all' | Exclude<ScopeCategory, 'na'>>('all');
  const [filterSearch, setFilterSearch] = createSignal('');
  let recoveryQueueSectionRef: HTMLDivElement | undefined;

  const loadAgentProfiles = async () => {
    try {
      const [profilesData, assignmentsData] = await Promise.all([
        AgentProfilesAPI.listProfiles(),
        AgentProfilesAPI.listAssignments(),
      ]);
      setProfiles(profilesData);
      setAssignments(assignmentsData);
    } catch (err) {
      logger.debug('Failed to load agent profiles', err);
      notificationStore.error(err instanceof Error ? err.message : 'Failed to load agent profiles');
    }
  };

  onMount(() => {
    void loadAgentProfiles();
  });

  const profileById = createMemo(() => {
    const map = new Map<string, AgentProfile>();
    for (const profile of profiles()) {
      map.set(profile.id, profile);
    }
    return map;
  });

  const assignmentByAgent = createMemo(() => {
    const map = new Map<string, AgentProfileAssignment>();
    for (const assignment of assignments()) {
      map.set(assignment.agent_id, assignment);
    }
    return map;
  });

  const getAgentId = (resource: Resource) => getExplicitAgentIdFromResource(resource);
  const getAgentActionId = (resource: Resource) => getActionableAgentIdFromResource(resource);
  const getDockerActionId = (resource: Resource) =>
    getActionableDockerRuntimeIdFromResource(resource);
  const getKubernetesActionId = (resource: Resource) =>
    getActionableKubernetesClusterIdFromResource(resource);

  const getScopeInfo = (agentId: string | undefined) => {
    if (!agentId) {
      return { label: 'N/A', detail: '', category: 'na' as const };
    }
    const assignment = assignmentByAgent().get(agentId);
    if (!assignment) {
      return { label: 'Default', detail: 'Auto-detect', category: 'default' as const };
    }
    const profile = profileById().get(assignment.profile_id);
    if (!profile) {
      return {
        label: 'Profile assigned',
        detail: assignment.profile_id,
        category: 'profile' as const,
      };
    }
    const name = profile.name || assignment.profile_id;
    const isAIManaged =
      profile.description?.toLowerCase().includes('pulse ai') ||
      name.toLowerCase().startsWith('ai scope');
    return isAIManaged
      ? { label: 'Patrol-managed', detail: name, category: 'ai-managed' as const }
      : { label: name, detail: 'Assigned profile', category: 'profile' as const };
  };

  const getProfileOptionLabel = (profileId: string) => {
    const profile = profileById().get(profileId);
    if (profile) {
      return profile.name || profile.id;
    }
    return `Missing profile (${profileId})`;
  };

  const updateScopeAssignment = async (
    agentId: string,
    profileId: string | null,
    agentName: string,
  ) => {
    if (!agentId || pendingScopeUpdates()[agentId]) {
      return;
    }

    setPendingScopeUpdates((prev) => ({ ...prev, [agentId]: true }));
    try {
      if (profileId) {
        await AgentProfilesAPI.assignProfile(agentId, profileId);
        setAssignments((prev) => {
          const updatedAt = new Date().toISOString();
          const next = prev.filter((assignment) => assignment.agent_id !== agentId);
          next.push({ agent_id: agentId, profile_id: profileId, updated_at: updatedAt });
          return next;
        });
        notificationStore.success(
          `Scope updated for ${agentName}. Restart the agent to apply changes.`,
        );
      } else {
        await AgentProfilesAPI.unassignProfile(agentId);
        setAssignments((prev) => prev.filter((assignment) => assignment.agent_id !== agentId));
        notificationStore.success(
          `Scope reset for ${agentName}. Restart the agent to apply changes.`,
        );
      }
    } catch (err) {
      logger.error('Failed to update agent scope', err);
      if (err instanceof Error && err.message === MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE) {
        await loadAgentProfiles();
      }
      notificationStore.error(
        err instanceof Error && err.message ? err.message : 'Failed to update agent scope',
      );
    } finally {
      setPendingScopeUpdates((prev) => {
        const next = { ...prev };
        delete next[agentId];
        return next;
      });
    }
  };

  const handleResetScope = async (agentId: string, agentName: string) => {
    if (
      !confirm(
        `Reset scope for ${agentName}? This removes any assigned profile and reverts to auto-detect.`,
      )
    ) {
      return;
    }
    await updateScopeAssignment(agentId, null, agentName);
  };

  const toggleAgentDetails = (rowKey: string) => {
    setExpandedRowKey(rowKey);
  };

  const connectedInfrastructureItems = createMemo<ConnectedInfrastructureItem[]>(
    () => state.connectedInfrastructure,
  );

  const matchesRemovedAgent = (
    resource: Resource,
    ids: { agentId?: string; dockerId?: string },
    capabilities: AgentCapability[],
  ) => {
    if (capabilities.includes('agent') && ids.agentId) {
      const agentId = ids.agentId;
      if (
        resource.id === agentId ||
        getAgentId(resource) === agentId ||
        getAgentActionId(resource) === agentId
      ) {
        return true;
      }
    }

    if (capabilities.includes('docker') && ids.dockerId) {
      const dockerId = ids.dockerId;
      if (resource.id === dockerId || getDockerActionId(resource) === dockerId) {
        return true;
      }
    }

    return false;
  };

  const reconcileRemovedAgent = (
    ids: { agentId?: string; dockerId?: string },
    capabilities: AgentCapability[],
    row: UnifiedAgentRow,
  ) => {
    const removedAt = Date.now();
    if (capabilities.includes('agent') && ids.agentId) {
      setOptimisticRemovedHostAgents((prev) => [
        {
          id: ids.agentId,
          hostname: row.hostname,
          displayName: row.displayName || row.name,
          removedAt,
        },
        ...prev.filter((item) => item.id !== ids.agentId),
      ]);
    }
    if (capabilities.includes('docker') && ids.dockerId) {
      setOptimisticRemovedDockerHosts((prev) => [
        {
          id: ids.dockerId,
          hostname: row.hostname,
          displayName: row.displayName || row.name,
          removedAt,
        },
        ...prev.filter((item) => item.id !== ids.dockerId),
      ]);
    }
    mutateResources((prev) =>
      prev.filter((resource) => !matchesRemovedAgent(resource, ids, capabilities)),
    );
    void refetchResources().catch((err) => {
      logger.debug('Failed to refresh resources after agent removal', err);
    });
  };

  const reconcileRemovedKubernetesCluster = (clusterId: string, clusterName?: string) => {
    setOptimisticRemovedKubernetesClusters((prev) => [
      {
        id: clusterId,
        name: clusterName || clusterId,
        displayName: clusterName,
        removedAt: Date.now(),
      },
      ...prev.filter((item) => item.id !== clusterId),
    ]);
    mutateResources((prev) =>
      prev.filter(
        (resource) =>
          !(
            resource.type === 'k8s-cluster' &&
            (getKubernetesActionId(resource) === clusterId || resource.id === clusterId)
          ),
      ),
    );
    void refetchResources().catch((err) => {
      logger.debug('Failed to refresh resources after kubernetes removal', err);
    });
  };

  const unifiedRows = createMemo<UnifiedAgentRow[]>(() => {
    const rows: UnifiedAgentRow[] = [];
    const optimisticHostIDs = new Set(
      optimisticRemovedHostAgents()
        .map((item) => item.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );
    const optimisticDockerIDs = new Set(
      optimisticRemovedDockerHosts()
        .map((item) => item.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );
    const optimisticKubernetesIDs = new Set(
      optimisticRemovedKubernetesClusters()
        .map((item) => item.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );

    connectedInfrastructureItems().forEach((item) => {
      const optimisticFilteredItem: ConnectedInfrastructureItem =
        item.status === 'active'
          ? {
              ...item,
              surfaces: item.surfaces.filter((surface) => {
                const controlId = surface.controlId?.trim();
                if (!controlId) return true;
                if (surface.kind === 'agent') return !optimisticHostIDs.has(controlId);
                if (surface.kind === 'docker') return !optimisticDockerIDs.has(controlId);
                if (surface.kind === 'kubernetes') return !optimisticKubernetesIDs.has(controlId);
                return true;
              }),
            }
          : item;

      if (
        optimisticFilteredItem.status === 'active' &&
        optimisticFilteredItem.surfaces.length === 0
      ) {
        return;
      }

      rows.push(
        rowFromConnectedInfrastructureItem(
          optimisticFilteredItem,
          getScopeInfo(optimisticFilteredItem.scopeAgentId),
        ),
      );
    });

    const backendIgnoredSurfaceKeys = new Set(
      rows
        .filter((row) => row.status === 'removed')
        .flatMap((row) =>
          row.surfaces.map(
            (surface) => `${surface.kind}:${surface.controlId || surface.idValue || row.id}`,
          ),
        ),
    );

    optimisticRemovedDockerHosts().forEach((runtime) => {
      const key = `docker:${runtime.id}`;
      if (backendIgnoredSurfaceKeys.has(key)) return;
      const name = getPreferredNamedEntityLabel(runtime);
      rows.push({
        rowKey: `removed-docker-${runtime.id}`,
        id: runtime.id,
        dockerActionId: runtime.id,
        name,
        hostname: runtime.hostname,
        displayName: runtime.displayName,
        capabilities: ['docker'],
        status: 'removed',
        removedAt: runtime.removedAt,
        upgradePlatform: 'linux' as AgentPlatform,
        scope: getScopeInfo(undefined),
        installFlags: ['--enable-docker', '--disable-host'],
        searchText: [name, runtime.hostname, runtime.id].filter(Boolean).join(' ').toLowerCase(),
        surfaces: [
          {
            key: 'docker',
            kind: 'docker',
            label: 'Docker runtime data',
            detail: 'Pulse is blocking Docker runtime reports from this machine.',
            idLabel: 'Docker runtime ID',
            idValue: runtime.id,
            action: 'allow-reconnect',
            controlId: runtime.id,
          },
        ],
      });
    });

    optimisticRemovedHostAgents().forEach((host) => {
      const key = `agent:${host.id}`;
      if (backendIgnoredSurfaceKeys.has(key)) return;
      const name = getPreferredNamedEntityLabel(host);
      rows.push({
        rowKey: `removed-host-${host.id}`,
        id: host.id,
        agentActionId: host.id,
        name,
        hostname: host.hostname,
        displayName: host.displayName,
        capabilities: ['agent'],
        status: 'removed',
        removedAt: host.removedAt,
        upgradePlatform: 'linux' as AgentPlatform,
        scope: getScopeInfo(undefined),
        installFlags: [],
        searchText: [name, host.hostname, host.id].filter(Boolean).join(' ').toLowerCase(),
        surfaces: [
          {
            key: 'agent',
            kind: 'agent',
            label: 'Host telemetry',
            detail: 'Pulse is blocking host telemetry from this machine.',
            idLabel: 'Agent ID',
            idValue: host.id,
            action: 'allow-reconnect',
            controlId: host.id,
          },
        ],
      });
    });

    optimisticRemovedKubernetesClusters().forEach((cluster) => {
      const key = `kubernetes:${cluster.id}`;
      if (backendIgnoredSurfaceKeys.has(key)) return;
      const name = getPreferredNamedEntityLabel(cluster);
      rows.push({
        rowKey: `removed-k8s-${cluster.id}`,
        id: cluster.id,
        kubernetesActionId: cluster.id,
        name,
        capabilities: ['kubernetes'],
        status: 'removed',
        removedAt: cluster.removedAt,
        upgradePlatform: 'linux' as AgentPlatform,
        scope: getScopeInfo(undefined),
        installFlags: ['--enable-kubernetes'],
        searchText: [name, cluster.name, cluster.id].filter(Boolean).join(' ').toLowerCase(),
        surfaces: [
          {
            key: 'kubernetes',
            kind: 'kubernetes',
            label: 'Kubernetes cluster data',
            detail: 'Pulse is blocking Kubernetes telemetry for this cluster.',
            idLabel: 'Cluster ID',
            idValue: cluster.id,
            action: 'allow-reconnect',
            controlId: cluster.id,
          },
        ],
      });
    });

    rows.sort((a, b) => {
      if (a.status !== b.status) {
        return a.status === 'active' ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });

    return rows;
  });

  const matchesInventoryFilters = (row: UnifiedAgentRow) => {
    const query = filterSearch().trim().toLowerCase();
    if (
      filterCapability() !== 'all' &&
      !row.capabilities.includes(filterCapability() as AgentCapability)
    ) {
      return false;
    }
    if (filterScope() !== 'all' && row.scope.category !== filterScope()) {
      return false;
    }
    if (query && !row.searchText.includes(query)) {
      return false;
    }
    return true;
  };

  const activeRows = createMemo(() => unifiedRows().filter((row) => row.status === 'active'));
  const monitoringStoppedRows = createMemo(() =>
    unifiedRows().filter((row) => row.status === 'removed'),
  );
  const linkedAgents = createMemo(() =>
    activeRows()
      .filter((row) => Boolean(row.linkedNodeId))
      .map((row) => ({
        id: row.id,
        hostname: row.hostname || row.name,
        displayName: row.displayName,
        linkedNodeId: row.linkedNodeId!,
        status: row.healthStatus || 'online',
        version: row.version,
        lastSeen: row.lastSeen,
      })),
  );
  const hasLinkedAgents = createMemo(() => linkedAgents().length > 0);
  const outdatedAgents = createMemo(() => activeRows().filter((row) => row.isOutdatedBinary));
  const hasOutdatedAgents = createMemo(() => outdatedAgents().length > 0);

  const filteredActiveRows = createMemo(() => activeRows().filter(matchesInventoryFilters));
  const filteredMonitoringStoppedRows = createMemo(() =>
    monitoringStoppedRows().filter(matchesInventoryFilters),
  );
  const selectedActiveRow = createMemo(() => {
    const selectedKey = expandedRowKey();
    return filteredActiveRows().find((row) => row.rowKey === selectedKey) || null;
  });
  const selectedIgnoredRow = createMemo(() => {
    const selectedKey = selectedIgnoredRowKey();
    return filteredMonitoringStoppedRows().find((row) => row.rowKey === selectedKey) || null;
  });

  const hasFilters = createMemo(() => {
    return (
      filterCapability() !== 'all' || filterScope() !== 'all' || filterSearch().trim().length > 0
    );
  });

  const hasMonitoringStoppedRows = createMemo(() => monitoringStoppedRows().length > 0);
  const showMonitoringStoppedSection = createMemo(() => {
    return hasMonitoringStoppedRows() || hasFilters();
  });
  const coverageSummary = createMemo(() => {
    const active = activeRows();
    return {
      agent: active.filter((row) => row.capabilities.includes('agent')).length,
      docker: active.filter((row) => row.capabilities.includes('docker')).length,
      kubernetes: active.filter((row) => row.capabilities.includes('kubernetes')).length,
      proxmox: active.filter((row) => row.capabilities.includes('proxmox')).length,
      pbs: active.filter((row) => row.capabilities.includes('pbs')).length,
      pmg: active.filter((row) => row.capabilities.includes('pmg')).length,
    };
  });

  const reportingCoverageSummaryText = createMemo(() => {
    const summary = coverageSummary();
    const activeClauses = [
      summary.agent > 0 ? `${summary.agent} host${summary.agent === 1 ? '' : 's'}` : null,
      summary.docker > 0
        ? `${summary.docker} Docker runtime${summary.docker === 1 ? '' : 's'}`
        : null,
      summary.kubernetes > 0
        ? `${summary.kubernetes} Kubernetes cluster${summary.kubernetes === 1 ? '' : 's'}`
        : null,
      summary.proxmox > 0
        ? `${summary.proxmox} Proxmox node${summary.proxmox === 1 ? '' : 's'}`
        : null,
      summary.pbs > 0 ? `${summary.pbs} PBS server${summary.pbs === 1 ? '' : 's'}` : null,
      summary.pmg > 0 ? `${summary.pmg} PMG server${summary.pmg === 1 ? '' : 's'}` : null,
    ].filter((value): value is string => Boolean(value));

    if (activeClauses.length === 0) {
      return 'Pulse is not currently receiving live infrastructure reports.';
    }

    return `Pulse is currently receiving live reports from ${joinHumanList(activeClauses)}.`;
  });

  const inventoryStatusSummaryText = createMemo(() => {
    const activeCount = filteredActiveRows().length;
    const recoveryCount = filteredMonitoringStoppedRows().length;
    const recoveryClause =
      recoveryCount > 0
        ? `${recoveryCount} item${recoveryCount === 1 ? ' is' : 's are'} currently ignored by Pulse`
        : 'nothing is currently ignored by Pulse';
    return `${activeCount} item${activeCount === 1 ? ' is' : 's are'} actively reporting right now, and ${recoveryClause}. Stopping monitoring in Pulse does not uninstall software on the remote system.`;
  });

  const resetFilters = () => {
    setFilterCapability('all');
    setFilterScope('all');
    setFilterSearch('');
  };

  const setInventoryActionPending = (
    rowKey: string,
    action: InventoryActionType,
    pending: boolean,
  ) => {
    setPendingInventoryActions((prev) => {
      const next = { ...prev };
      if (pending) {
        next[rowKey] = action;
      } else {
        delete next[rowKey];
      }
      return next;
    });
  };

  const getPendingInventoryAction = (rowKey: string) => pendingInventoryActions()[rowKey];

  const scrollToRecoveryQueue = () => {
    recoveryQueueSectionRef?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  const setRecoveryQueueSectionRef = (value: HTMLDivElement | undefined) => {
    recoveryQueueSectionRef = value;
  };

  const openStopMonitoringDialog = (row: UnifiedAgentRow) => {
    setStopMonitoringDialog({
      row,
      subject: getInventorySubjectLabel(row.name, 'this item'),
      scopeLabel: getStopMonitoringScopeLabel(row),
    });
  };

  const handleRemoveAgent = async (row: UnifiedAgentRow) => {
    const subject = getInventorySubjectLabel(row.name, 'this host');

    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'stop-monitoring', true);
    try {
      let removed = false;
      if (row.capabilities.includes('agent') && row.agentActionId) {
        await MonitoringAPI.deleteAgent(row.agentActionId);
        removed = true;
      }
      if (row.capabilities.includes('docker') && row.dockerActionId) {
        await MonitoringAPI.deleteDockerRuntime(row.dockerActionId, { force: true });
        removed = true;
      }
      if (removed) {
        reconcileRemovedAgent(
          { agentId: row.agentActionId, dockerId: row.dockerActionId },
          row.capabilities,
          row,
        );
        setInventoryActionNotice({
          tone: 'success',
          title: `Monitoring stopped for ${subject}`,
          detail:
            'Pulse removed it from active reporting and will ignore new reports until you allow reconnect. You can review it in Ignored by Pulse below.',
          showRecoveryQueueLink: true,
        });
        notificationStore.success(getUnifiedAgentStopMonitoringSuccessMessage(subject));
      } else {
        notificationStore.error(getUnifiedAgentStopMonitoringUnavailableMessage());
      }
    } catch (err) {
      logger.error('Failed to stop monitoring host', err);
      notificationStore.error(getUnifiedAgentStopMonitoringErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'stop-monitoring', false);
      setStopMonitoringDialog(null);
    }
  };

  const handleAllowHostReconnect = async (row: UnifiedAgentRow) => {
    const agentId = row.agentActionId || row.id;
    const subject = getInventorySubjectLabel(row.displayName || row.hostname || row.name, agentId);
    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'allow-reconnect', true);
    try {
      await MonitoringAPI.allowHostAgentReenroll(agentId);
      setOptimisticRemovedHostAgents((prev) => prev.filter((item) => item.id !== agentId));
      setInventoryActionNotice({
        tone: 'info',
        title: `Reconnect allowed for ${subject}`,
        detail: 'Pulse will accept reports from it again the next time it checks in.',
      });
      notificationStore.success(getUnifiedAgentAllowReconnectSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to allow reconnect for host agent', err);
      notificationStore.error(getUnifiedAgentAllowReconnectErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'allow-reconnect', false);
    }
  };

  const handleAllowDockerReconnect = async (row: UnifiedAgentRow) => {
    const agentId = row.dockerActionId || row.id;
    const subject = getInventorySubjectLabel(row.displayName || row.hostname || row.name, agentId);
    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'allow-reconnect', true);
    try {
      await MonitoringAPI.allowDockerRuntimeReenroll(agentId);
      setOptimisticRemovedDockerHosts((prev) => prev.filter((item) => item.id !== agentId));
      setInventoryActionNotice({
        tone: 'info',
        title: `Reconnect allowed for ${subject}`,
        detail: 'Pulse will accept reports from it again the next time it checks in.',
      });
      notificationStore.success(getUnifiedAgentAllowReconnectSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to allow reconnect for docker runtime', err);
      notificationStore.error(getUnifiedAgentAllowReconnectErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'allow-reconnect', false);
    }
  };

  const handleRemoveKubernetesCluster = async (row: UnifiedAgentRow) => {
    const clusterId = row.kubernetesActionId || row.id;
    const subject = getInventorySubjectLabel(row.name, 'this cluster');

    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'stop-monitoring', true);
    try {
      await MonitoringAPI.deleteKubernetesCluster(clusterId);
      reconcileRemovedKubernetesCluster(clusterId, row.name);
      setInventoryActionNotice({
        tone: 'success',
        title: `Monitoring stopped for ${subject}`,
        detail:
          'Pulse removed it from active reporting and will ignore new reports until you allow reconnect. You can review it in Ignored by Pulse below.',
        showRecoveryQueueLink: true,
      });
      notificationStore.success(getUnifiedAgentStopMonitoringSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to stop monitoring kubernetes cluster', err);
      notificationStore.error(getUnifiedAgentStopMonitoringErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'stop-monitoring', false);
      setStopMonitoringDialog(null);
    }
  };

  const handleAllowKubernetesReconnect = async (row: UnifiedAgentRow) => {
    const clusterId = row.kubernetesActionId || row.id;
    const subject = getInventorySubjectLabel(row.name, clusterId);
    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'allow-reconnect', true);
    try {
      await MonitoringAPI.allowKubernetesClusterReenroll(clusterId);
      setOptimisticRemovedKubernetesClusters((prev) =>
        prev.filter((item) => item.id !== clusterId),
      );
      setInventoryActionNotice({
        tone: 'info',
        title: `Reconnect allowed for ${subject}`,
        detail: 'Pulse will accept reports from it again the next time it checks in.',
      });
      notificationStore.success(getUnifiedAgentAllowReconnectSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to allow reconnect for kubernetes cluster', err);
      notificationStore.error(getUnifiedAgentAllowReconnectErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'allow-reconnect', false);
    }
  };

  createEffect(() => {
    const rows = filteredActiveRows();
    const selectedKey = expandedRowKey();

    if (rows.length === 0) {
      if (selectedKey !== null) {
        setExpandedRowKey(null);
      }
      return;
    }

    if (selectedKey && !rows.some((row) => row.rowKey === selectedKey)) {
      setExpandedRowKey(null);
    }
  });

  createEffect(() => {
    const rows = filteredMonitoringStoppedRows();
    const selectedKey = selectedIgnoredRowKey();

    if (rows.length === 0) {
      if (selectedKey !== null) {
        setSelectedIgnoredRowKey(null);
      }
      return;
    }

    if (selectedKey && !rows.some((row) => row.rowKey === selectedKey)) {
      setSelectedIgnoredRowKey(null);
    }
  });

  const reportingColumns = [
    {
      key: 'name',
      label: 'Name',
      render: (row: UnifiedAgentRow) => {
        const selected = () => expandedRowKey() === row.rowKey;
        const agentName = row.displayName || row.hostname || row.name;
        const reportingSummary = getRowReportingSummary(row);
        return (
          <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
            <div class="min-w-0 text-left">
              <div class="truncate text-sm font-medium text-base-content">{row.name}</div>
              <Show when={row.displayName && row.hostname && row.displayName !== row.hostname}>
                <div class="truncate text-xs text-muted">{row.hostname}</div>
              </Show>
              <Show when={reportingSummary}>
                <div class="mt-1 line-clamp-2 text-xs text-muted">{reportingSummary}</div>
              </Show>
            </div>
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                toggleAgentDetails(row.rowKey);
              }}
              class={`inline-flex min-h-10 items-center justify-center rounded-md px-2.5 py-1.5 text-xs font-medium sm:min-h-9 ${
                selected()
                  ? 'bg-blue-600 text-white shadow-sm hover:bg-blue-700'
                  : 'text-muted hover:bg-surface hover:text-base-content'
              }`}
              aria-label={`View details for ${agentName}`}
              aria-pressed={selected()}
              aria-controls={`agent-details-${row.rowKey}`}
            >
              {selected() ? 'Open details' : 'View details'}
            </button>
          </div>
        );
      },
    },
    {
      key: 'capabilities',
      label: 'Reporting surfaces',
      render: (row: UnifiedAgentRow) => (
        <div class="space-y-1 text-xs">
          <For each={row.capabilities}>
            {(capability) => (
              <div class="flex items-center gap-2">
                <span
                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getAgentCapabilityBadgeClass(capability)}`}
                >
                  {getCapabilitySurfaceLabel(capability)}
                </span>
              </div>
            )}
          </For>
        </div>
      ),
    },
    {
      key: 'status',
      label: 'Status',
      render: (row: UnifiedAgentRow) => {
        const statusPresentation = () =>
          getUnifiedAgentStatusPresentation(row.status, row.healthStatus);
        return (
          <span
            class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${statusPresentation().badgeClass}`}
          >
            {statusPresentation().label}
          </span>
        );
      },
    },
    {
      key: 'lastSeen',
      label: 'Last Seen',
      render: (row: UnifiedAgentRow) => (
        <span class="text-xs text-muted">
          {getUnifiedAgentLastSeenLabel(row, MONITORING_STOPPED_STATUS_LABEL)}
        </span>
      ),
    },
    {
      key: 'actions',
      label: 'Actions',
      align: 'right' as const,
      render: (row: UnifiedAgentRow) => {
        const isRemoved = () => row.status === 'removed';
        const isKubernetes = () =>
          row.capabilities.includes('kubernetes') && !row.capabilities.includes('agent');
        const pendingAction = () => getPendingInventoryAction(row.rowKey);
        const isStopping = () => pendingAction() === 'stop-monitoring';
        const isAllowingReconnect = () => pendingAction() === 'allow-reconnect';
        const canRemove = () => {
          const needsAgent = row.capabilities.includes('agent') && !row.agentActionId;
          const needsDocker =
            row.capabilities.includes('docker') && !row.dockerActionId && !row.agentActionId;
          return !needsAgent && !needsDocker;
        };
        return (
          <Show
            when={isRemoved()}
            fallback={
              <Show
                when={isKubernetes()}
                fallback={
                  <button
                    onClick={() => openStopMonitoringDialog(row)}
                    disabled={!canRemove() || Boolean(pendingAction())}
                    title={!canRemove() ? 'Agent ID unavailable for removal' : undefined}
                    class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 hover:text-red-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                  >
                    {isStopping() ? 'Stopping…' : 'Stop monitoring'}
                  </button>
                }
              >
                <button
                  onClick={() => openStopMonitoringDialog(row)}
                  disabled={Boolean(pendingAction())}
                  class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 hover:text-red-900 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                >
                  {isStopping() ? 'Stopping…' : 'Stop monitoring'}
                </button>
              </Show>
            }
          >
            <Show
              when={row.capabilities.includes('docker')}
              fallback={
                <Show
                  when={row.capabilities.includes('kubernetes')}
                  fallback={
                    <button
                      onClick={() => void handleAllowHostReconnect(row)}
                      disabled={Boolean(pendingAction())}
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                    >
                      {isAllowingReconnect() ? 'Allowing reconnect…' : ALLOW_RECONNECT_LABEL}
                    </button>
                  }
                >
                  <button
                    onClick={() => void handleAllowKubernetesReconnect(row)}
                    disabled={Boolean(pendingAction())}
                    class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                  >
                    {isAllowingReconnect() ? 'Allowing reconnect…' : ALLOW_RECONNECT_LABEL}
                  </button>
                </Show>
              }
            >
              <button
                onClick={() => void handleAllowDockerReconnect(row)}
                disabled={Boolean(pendingAction())}
                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
              >
                {isAllowingReconnect() ? 'Allowing reconnect…' : ALLOW_RECONNECT_LABEL}
              </button>
            </Show>
          </Show>
        );
      },
    },
  ];

  return {
    activeRows,
    assignmentByAgent,
    filterCapability,
    filterScope,
    filterSearch,
    filteredActiveRows,
    filteredMonitoringStoppedRows,
    getPendingInventoryAction,
    getProfileOptionLabel,
    handleAllowDockerReconnect,
    handleAllowHostReconnect,
    handleAllowKubernetesReconnect,
    handleRemoveAgent,
    handleRemoveKubernetesCluster,
    handleResetScope,
    hasFilters,
    hasLinkedAgents,
    hasOutdatedAgents,
    inventoryActionNotice,
    inventoryStatusSummaryText,
    linkedAgents,
    openStopMonitoringDialog,
    outdatedAgents,
    pendingScopeUpdates,
    profileById,
    profiles,
    reportingColumns,
    reportingCoverageSummaryText,
    resetFilters,
    scrollToRecoveryQueue,
    selectedActiveRow,
    selectedIgnoredRow,
    selectedIgnoredRowKey,
    setExpandedRowKey,
    setFilterCapability,
    setFilterScope,
    setFilterSearch,
    setInventoryActionNotice,
    setRecoveryQueueSectionRef,
    setSelectedIgnoredRowKey,
    setStopMonitoringDialog,
    showMonitoringStoppedSection,
    stopMonitoringDialog,
    toggleAgentDetails,
    updateScopeAssignment,
  };
};

export type InfrastructureReportingState = ReturnType<typeof useInfrastructureReportingState>;
