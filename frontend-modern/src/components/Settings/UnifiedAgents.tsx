import { Component, createSignal, Show, For, onMount, createEffect, createMemo, untrack } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import Server from 'lucide-solid/icons/server';
import Users from 'lucide-solid/icons/users';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { MonitoringAPI } from '@/api/monitoring';
import { AgentProfilesAPI, type AgentProfile, type AgentProfileAssignment } from '@/api/agentProfiles';
import { HostMetadataAPI } from '@/api/hostMetadata';
import { DockerMetadataAPI } from '@/api/dockerMetadata';
import { SecurityAPI } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import type { HostLookupResponse } from '@/types/api';
import type { APITokenRecord } from '@/api/security';
import { HOST_AGENT_SCOPE, HOST_AGENT_CONFIG_READ_SCOPE, DOCKER_REPORT_SCOPE, KUBERNETES_REPORT_SCOPE, AGENT_EXEC_SCOPE } from '@/constants/apiScopes';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { logger } from '@/utils/logger';

const TOKEN_PLACEHOLDER = '<api-token>';

const buildDefaultTokenName = () => {
    const now = new Date();
    const iso = now.toISOString().slice(0, 16); // YYYY-MM-DDTHH:MM
    const stamp = iso.replace('T', ' ').replace(/:/g, '-');
    return `Agent ${stamp}`;
};

type AgentPlatform = 'linux' | 'macos' | 'freebsd' | 'windows';
type UnifiedAgentType = 'host' | 'docker' | 'kubernetes';
type UnifiedAgentStatus = 'active' | 'removed';
type ScopeCategory = 'default' | 'profile' | 'ai-managed' | 'na';

type UnifiedAgentRow = {
    rowKey: string;
    id: string;
    name: string;
    hostname?: string;
    displayName?: string;
    types: UnifiedAgentType[];
    status: UnifiedAgentStatus;
    healthStatus?: string;
    lastSeen?: number;
    removedAt?: number;
    version?: string;
    isLegacy?: boolean;
    linkedNodeId?: string;
    commandsEnabled?: boolean;
    agentId?: string;
    scope: {
        label: string;
        detail?: string;
        category: ScopeCategory;
    };
    searchText: string;
    kubernetesInfo?: {
        server?: string;
        context?: string;
        tokenName?: string;
    };
};

// Generate platform-specific commands with the appropriate Pulse URL
// Uses agentUrl from API (PULSE_PUBLIC_URL) if configured, otherwise falls back to window.location
const buildCommandsByPlatform = (url: string): Record<
    AgentPlatform,
    {
        title: string;
        description: string;
        snippets: { label: string; command: string; note?: string | any }[];
    }
> => ({
    linux: {
        title: 'Install on Linux',
        description:
            'The unified installer downloads the agent binary and configures the appropriate service for your system.',
        snippets: [
            {
                label: 'Install',
                command: `curl -fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
                note: (
                    <span>
                        Run as root (use <code>sudo</code> or <code>su -</code> if not already root). Auto-detects your init system and works on Debian, Ubuntu, Proxmox, Fedora, Alpine, Unraid, Synology, and more.
                    </span>
                ),
            },
        ],
    },
    macos: {
        title: 'Install on macOS',
        description:
            'The unified installer downloads the universal binary and sets up a launchd service for background monitoring.',
        snippets: [
            {
                label: 'Install with launchd',
                command: `curl -fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
                note: (
                    <span>
                        Run as root (use <code>sudo</code> if not already root). Creates <code>/Library/LaunchDaemons/com.pulse.agent.plist</code> and starts the agent automatically.
                    </span>
                ),
            },
        ],
    },
    freebsd: {
        title: 'Install on FreeBSD / pfSense / OPNsense',
        description:
            'The unified installer downloads the FreeBSD binary and sets up an rc.d service for background monitoring.',
        snippets: [
            {
                label: 'Install with rc.d',
                command: `curl -fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
                note: (
                    <span>
                        Run as root. <strong>Note:</strong> pfSense/OPNsense don't include bash by default. Install it first: <code>pkg install bash</code>. Creates <code>/usr/local/etc/rc.d/pulse-agent</code> and starts the agent automatically.
                    </span>
                ),
            },
        ],
    },
    windows: {
        title: 'Install on Windows',
        description:
            'Run the PowerShell script to install and configure the unified agent as a Windows service with automatic startup.',
        snippets: [
            {
                label: 'Install as Windows Service (PowerShell)',
                command: `irm ${url}/install.ps1 | iex`,
                note: (
                    <span>
                        Run in PowerShell as Administrator. The script will prompt for the Pulse URL and API token, download the agent binary, and install it as a Windows service with automatic startup.
                    </span>
                ),
            },
            {
                label: 'Install with parameters (PowerShell)',
                command: `$env:PULSE_URL="${url}"; $env:PULSE_TOKEN="${TOKEN_PLACEHOLDER}"; irm ${url}/install.ps1 | iex`,
                note: (
                    <span>
                        Non-interactive installation. Set environment variables before running to skip prompts.
                    </span>
                ),
            },
        ],
    },
});

export const UnifiedAgents: Component = () => {
    const { state } = useWebSocket();
    const navigate = useNavigate();

    let hasLoggedSecurityStatusError = false;

    const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
    const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
    const [tokenName, setTokenName] = createSignal('');
    const [confirmedNoToken, setConfirmedNoToken] = createSignal(false);
    const [currentToken, setCurrentToken] = createSignal<string | null>(null);
    const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
    const [lookupValue, setLookupValue] = createSignal('');
    const [lookupResult, setLookupResult] = createSignal<HostLookupResponse | null>(null);
    const [lookupError, setLookupError] = createSignal<string | null>(null);
    const [lookupLoading, setLookupLoading] = createSignal(false);
    const [insecureMode, setInsecureMode] = createSignal(false); // For self-signed certificates (issue #806)
    const [enableCommands, setEnableCommands] = createSignal(false); // Enable Pulse command execution (issue #903)
    const [customAgentUrl, setCustomAgentUrl] = createSignal('');
    const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
    const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
    // Track pending command config changes: hostId -> { desired value, timestamp }
    const [pendingCommandConfig, setPendingCommandConfig] = createSignal<Record<string, { enabled: boolean; timestamp: number }>>({});
    const [pendingScopeUpdates, setPendingScopeUpdates] = createSignal<Record<string, boolean>>({});
    const [expandedRowKey, setExpandedRowKey] = createSignal<string | null>(null);
    const [hostCustomUrl, setHostCustomUrl] = createSignal('');
    const [hostCustomUrlSaving, setHostCustomUrlSaving] = createSignal(false);
    const [containerUrls, setContainerUrls] = createSignal<Record<string, string>>({});
    const [containerUrlSaving, setContainerUrlSaving] = createSignal<Record<string, boolean>>({});
    const [filterType, setFilterType] = createSignal<'all' | UnifiedAgentType>('all');
    const [filterStatus, setFilterStatus] = createSignal<'all' | UnifiedAgentStatus>('all');
    const [filterScope, setFilterScope] = createSignal<'all' | Exclude<ScopeCategory, 'na'>>('all');
    const [filterSearch, setFilterSearch] = createSignal('');

    createEffect(() => {
        if (requiresToken()) {
            setConfirmedNoToken(false);
        } else {
            setCurrentToken(null);
            setLatestRecord(null);
        }
    });

    // Use agentUrl from API (PULSE_PUBLIC_URL) if configured, otherwise fall back to window.location
    const agentUrl = () => securityStatus()?.agentUrl || getPulseBaseUrl();

    const commandSections = createMemo(() => {
        const url = customAgentUrl() || agentUrl();
        const commands = buildCommandsByPlatform(url);
        return Object.entries(commands).map(([platform, meta]) => ({
            platform: platform as AgentPlatform,
            ...meta,
        }));
    });

    const connectedFromStatus = (status: string | undefined | null) => {
        if (!status) return false;
        const value = status.toLowerCase();
        return value === 'online' || value === 'running' || value === 'healthy';
    };

    onMount(() => {
        if (typeof window === 'undefined') {
            return;
        }

        const fetchSecurityStatus = async () => {
            try {
                const data = await SecurityAPI.getStatus();
                setSecurityStatus(data);
            } catch (err) {
                if (!hasLoggedSecurityStatusError) {
                    hasLoggedSecurityStatusError = true;
                    logger.error('Failed to load security status', err);
                }
            }
        };
        fetchSecurityStatus();

        const fetchAgentProfiles = async () => {
            try {
                const [profilesData, assignmentsData] = await Promise.all([
                    AgentProfilesAPI.listProfiles(),
                    AgentProfilesAPI.listAssignments(),
                ]);
                setProfiles(profilesData);
                setAssignments(assignmentsData);
            } catch (err) {
                logger.debug('Failed to load agent profiles', err);
                setProfiles([]);
                setAssignments([]);
            }
        };
        fetchAgentProfiles();
    });

    const requiresToken = () => {
        const status = securityStatus();
        if (status) {
            return status.requiresAuth || status.apiTokenConfigured;
        }
        return true;
    };

    const hasToken = () => Boolean(currentToken());
    const commandsUnlocked = () => (requiresToken() ? hasToken() : hasToken() || confirmedNoToken());

    const acknowledgeNoToken = () => {
        if (requiresToken()) {
            notificationStore.info('Generate or select a token before continuing.', 4000);
            return;
        }
        setCurrentToken(null);
        setLatestRecord(null);
        setConfirmedNoToken(true);
        notificationStore.success('Confirmed install commands without an API token.', 3500);
    };

    const handleGenerateToken = async () => {
        if (isGeneratingToken()) return;

        setIsGeneratingToken(true);
        try {
            const desiredName = tokenName().trim() || buildDefaultTokenName();
            // Generate token with unified agent reporting scopes
            const scopes = [HOST_AGENT_SCOPE, HOST_AGENT_CONFIG_READ_SCOPE, DOCKER_REPORT_SCOPE, KUBERNETES_REPORT_SCOPE, AGENT_EXEC_SCOPE];
            const { token, record } = await SecurityAPI.createToken(desiredName, scopes);

            setCurrentToken(token);
            setLatestRecord(record);
            setTokenName('');
            setConfirmedNoToken(false);
            notificationStore.success('Token generated with Host config + reporting, Docker, and Kubernetes permissions.', 4000);
        } catch (err) {
            logger.error('Failed to generate agent token', err);
            notificationStore.error('Failed to generate agent token. Confirm you are signed in as an administrator.', 6000);
        } finally {
            setIsGeneratingToken(false);
        }
    };

    const resolvedToken = () => {
        if (requiresToken()) {
            return currentToken() || TOKEN_PLACEHOLDER;
        }
        return currentToken() || 'disabled';
    };

    const handleLookup = async () => {
        const query = lookupValue().trim();
        setLookupError(null);

        if (!query) {
            setLookupResult(null);
            setLookupError('Enter a hostname or host ID to check.');
            return;
        }

        setLookupLoading(true);
        try {
            const result = await MonitoringAPI.lookupHost({ id: query, hostname: query });
            if (!result) {
                setLookupResult(null);
                setLookupError(`No host has reported with "${query}" yet. Try again in a few seconds.`);
            } else {
                setLookupResult(result);
                setLookupError(null);
            }
        } catch (err) {
            const message = err instanceof Error ? err.message : 'Host lookup failed.';
            setLookupResult(null);
            setLookupError(message);
        } finally {
            setLookupLoading(false);
        }
    };

    const getInsecureFlag = () => insecureMode() ? ' --insecure' : '';
    const getEnableCommandsFlag = () => enableCommands() ? ' --enable-commands' : '';
    const getCurlInsecureFlag = () => insecureMode() ? '-k' : '';

    const getUninstallCommand = () => {
        const url = customAgentUrl() || agentUrl();
        const token = currentToken() || latestRecord()?.id;
        const insecure = insecureMode() ? ' --insecure' : '';
        // Only include token if we have a real one - the uninstall script works without it
        // Avoid including <api-token> placeholder which causes shell syntax errors
        if (token) {
            return `curl ${getCurlInsecureFlag()}-fsSL ${url}/install.sh | bash -s -- --uninstall --url ${url} --token ${token}${insecure}`;
        }
        return `curl ${getCurlInsecureFlag()}-fsSL ${url}/install.sh | bash -s -- --uninstall --url ${url}${insecure}`;
    };

    const getWindowsUninstallCommand = () => {
        const url = customAgentUrl() || agentUrl();
        const token = currentToken() || latestRecord()?.id;
        // Include URL and token for server notification (removes agent from dashboard)
        if (token) {
            return `$env:PULSE_URL="${url}"; $env:PULSE_TOKEN="${token}"; $env:PULSE_UNINSTALL="true"; irm $env:PULSE_URL/install.ps1 | iex`;
        }
        return `$env:PULSE_UNINSTALL="true"; irm ${url}/install.ps1 | iex`;
    };

    // Track previously seen host types to prevent flapping when one source temporarily has no data
    // This preserves types we've seen before even if one array briefly becomes empty
    let previousHostTypes = new Map<string, Set<'host' | 'docker'>>();

    const allHosts = createMemo(() => {
        const hosts = state.hosts || [];
        const dockerHosts = state.dockerHosts || [];

        // Create a unified list
        const unified = new Map<string, {
            id: string;
            hostname: string;
            displayName?: string;
            types: ('host' | 'docker')[];
            status: string;
            version?: string;
            lastSeen?: number;
            isLegacy?: boolean;
            linkedNodeId?: string;
            commandsEnabled?: boolean;
            agentId?: string;
        }>();

        // Process Host Agents (include linked ones with a badge)
        hosts.forEach(h => {
            // Use id as key (not hostname) to avoid overwriting when different machines share the same hostname
            const key = h.id;
            unified.set(key, {
                id: h.id,
                agentId: h.id,
                hostname: h.hostname || 'Unknown',
                displayName: h.displayName,
                types: ['host'],
                status: h.status || 'unknown',
                version: h.agentVersion,
                lastSeen: h.lastSeen,
                isLegacy: h.isLegacy,
                linkedNodeId: h.linkedNodeId,
                commandsEnabled: h.commandsEnabled
            });
        });

        // Process Docker Agents (merge if same id - indicates same physical machine)
        dockerHosts.forEach(d => {
            // Use id as key (not hostname) to avoid overwriting 
            const key = d.id;
            const existing = unified.get(key);
            if (existing) {
                if (!existing.types.includes('docker')) {
                    existing.types.push('docker');
                }
                if (!existing.agentId && d.agentId) {
                    existing.agentId = d.agentId;
                }
                // Update version/status if newer
                if (!existing.version && d.agentVersion) existing.version = d.agentVersion;
                if (d.isLegacy) existing.isLegacy = true;
            } else {
                unified.set(key, {
                    id: d.id,
                    agentId: d.agentId || d.id,
                    hostname: d.hostname || 'Unknown',
                    displayName: d.displayName,
                    types: ['docker'],
                    status: d.status || 'unknown',
                    version: d.agentVersion || d.dockerVersion,
                    lastSeen: d.lastSeen,
                    isLegacy: d.isLegacy,
                });
            }
        });

        // Preserve previously seen types to prevent flapping
        // If we previously saw both 'host' and 'docker' for a hostname, keep both
        // unless BOTH sources are now empty (indicating intentional removal)
        const newHostTypes = new Map<string, Set<'host' | 'docker'>>();

        // Helper to ensure consistent type order: 'host' always before 'docker'
        const sortTypes = (types: ('host' | 'docker')[]): ('host' | 'docker')[] => {
            const result: ('host' | 'docker')[] = [];
            if (types.includes('host')) result.push('host');
            if (types.includes('docker')) result.push('docker');
            return result;
        };

        unified.forEach((entry, key) => {
            const currentTypes = new Set(entry.types);
            const prevTypes = previousHostTypes.get(key);

            if (prevTypes && prevTypes.size > currentTypes.size) {
                // We previously had more types - check if source data exists
                // Only add back types if the corresponding source has ANY data
                // (prevents permanent stickiness if a host is truly removed)
                if (prevTypes.has('host') && !currentTypes.has('host') && hosts.length > 0) {
                    // Host type disappeared but we still have host data overall
                    // This is likely a transient state - preserve the host type
                    currentTypes.add('host');
                }
                if (prevTypes.has('docker') && !currentTypes.has('docker') && dockerHosts.length > 0) {
                    // Docker type disappeared but we still have docker data overall
                    currentTypes.add('docker');
                }
            }

            // Always ensure consistent order: 'host' before 'docker'
            entry.types = sortTypes(Array.from(currentTypes));
            newHostTypes.set(key, new Set(entry.types));
        });
        previousHostTypes = newHostTypes;

        return Array.from(unified.values()).sort((a, b) => a.hostname.localeCompare(b.hostname));
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
            return { label: 'Profile assigned', detail: assignment.profile_id, category: 'profile' as const };
        }
        const name = profile.name || assignment.profile_id;
        const isAIManaged =
            profile.description?.toLowerCase().includes('pulse ai') ||
            name.toLowerCase().startsWith('ai scope');
        return isAIManaged
            ? { label: 'Patrol-managed', detail: name, category: 'ai-managed' as const }
            : { label: name, detail: 'Assigned profile', category: 'profile' as const };
    };

    const updateScopeAssignment = async (agentId: string, profileId: string | null, agentName: string) => {
        if (!agentId) {
            return;
        }
        if (pendingScopeUpdates()[agentId]) {
            return;
        }

        setPendingScopeUpdates(prev => ({ ...prev, [agentId]: true }));
        try {
            if (profileId) {
                await AgentProfilesAPI.assignProfile(agentId, profileId);
                setAssignments(prev => {
                    const updatedAt = new Date().toISOString();
                    const next = prev.filter(a => a.agent_id !== agentId);
                    next.push({ agent_id: agentId, profile_id: profileId, updated_at: updatedAt });
                    return next;
                });
                notificationStore.success(`Scope updated for ${agentName}. Restart the agent to apply changes.`);
            } else {
                await AgentProfilesAPI.unassignProfile(agentId);
                setAssignments(prev => prev.filter(a => a.agent_id !== agentId));
                notificationStore.success(`Scope reset for ${agentName}. Restart the agent to apply changes.`);
            }
        } catch (err) {
            logger.error('Failed to update agent scope', err);
            notificationStore.error('Failed to update agent scope');
        } finally {
            setPendingScopeUpdates(prev => {
                const next = { ...prev };
                delete next[agentId];
                return next;
            });
        }
    };

    const handleResetScope = async (agentId: string, agentName: string) => {
        if (!confirm(`Reset scope for ${agentName}? This removes any assigned profile and reverts to auto-detect.`)) return;
        await updateScopeAssignment(agentId, null, agentName);
    };

    const toggleAgentDetails = (rowKey: string) => {
        setExpandedRowKey(prev => (prev === rowKey ? null : rowKey));
    };

    const saveHostUrl = async (hostId: string) => {
        if (hostCustomUrlSaving()) return;
        setHostCustomUrlSaving(true);
        const trimmed = hostCustomUrl().trim();

        try {
            await HostMetadataAPI.updateMetadata(hostId, { customUrl: trimmed });
            setHostCustomUrl(trimmed);
            notificationStore.success('Custom URL saved');
        } catch (err) {
            logger.error('Failed to save host metadata', err);
            notificationStore.error('Failed to save custom URL');
        } finally {
            setHostCustomUrlSaving(false);
        }
    };

    const saveContainerUrl = async (hostId: string, containerId: string) => {
        if (containerUrlSaving()[containerId]) return;
        setContainerUrlSaving(prev => ({ ...prev, [containerId]: true }));
        const trimmed = (containerUrls()[containerId] || '').trim();
        const metaKey = `${hostId}:container:${containerId}`;
        try {
            await DockerMetadataAPI.updateMetadata(metaKey, { customUrl: trimmed });
            notificationStore.success('Container URL saved');
        } catch (err) {
            logger.error('Failed to save container URL', err);
            notificationStore.error('Failed to save container URL');
        } finally {
            setContainerUrlSaving(prev => ({ ...prev, [containerId]: false }));
        }
    };

    const legacyAgents = createMemo(() => allHosts().filter(h => h.isLegacy));
    const hasLegacyAgents = createMemo(() => legacyAgents().length > 0);

    const removedDockerHosts = createMemo(() => {
        const removed = state.removedDockerHosts || [];
        return removed.sort((a, b) => b.removedAt - a.removedAt);
    });

    const kubernetesClusters = createMemo(() => {
        const clusters = state.kubernetesClusters || [];
        return clusters.slice().sort((a, b) => (a.displayName || a.name || a.id).localeCompare(b.displayName || b.name || b.id));
    });

    const removedKubernetesClusters = createMemo(() => {
        const removed = state.removedKubernetesClusters || [];
        return removed.sort((a, b) => b.removedAt - a.removedAt);
    });

    // Host agents linked to PVE nodes (shown separately with unlink option)
    const linkedHostAgents = createMemo(() => {
        const hosts = state.hosts || [];
        return hosts.filter(h => h.linkedNodeId).map(h => ({
            id: h.id,
            hostname: h.hostname || 'Unknown',
            displayName: h.displayName,
            linkedNodeId: h.linkedNodeId,
            status: h.status,
            version: h.agentVersion,
            lastSeen: h.lastSeen ? new Date(h.lastSeen).getTime() : undefined,
        }));
    });
    const hasLinkedAgents = createMemo(() => linkedHostAgents().length > 0);

    const unifiedRows = createMemo<UnifiedAgentRow[]>(() => {
        const rows: UnifiedAgentRow[] = [];

        allHosts().forEach(agent => {
            const resolvedAgentId = agent.agentId || agent.id;
            const scopeInfo = getScopeInfo(resolvedAgentId);
            const name = agent.displayName || agent.hostname;
            const searchText = [name, agent.hostname, agent.id, resolvedAgentId]
                .filter(Boolean)
                .join(' ')
                .toLowerCase();

            rows.push({
                rowKey: `agent-${agent.id}`,
                id: agent.id,
                name,
                hostname: agent.hostname,
                displayName: agent.displayName,
                types: agent.types,
                status: 'active',
                healthStatus: agent.status,
                lastSeen: agent.lastSeen,
                version: agent.version,
                isLegacy: agent.isLegacy,
                linkedNodeId: agent.linkedNodeId,
                commandsEnabled: agent.commandsEnabled,
                agentId: resolvedAgentId,
                scope: scopeInfo,
                searchText,
            });
        });

        kubernetesClusters().forEach(cluster => {
            const name = cluster.customDisplayName || cluster.displayName || cluster.name || cluster.id;
            rows.push({
                rowKey: `k8s-${cluster.id}`,
                id: cluster.id,
                name,
                types: ['kubernetes'],
                status: 'active',
                healthStatus: cluster.status,
                lastSeen: cluster.lastSeen,
                version: cluster.version || cluster.agentVersion,
                agentId: cluster.agentId,
                scope: getScopeInfo(undefined),
                searchText: [name, cluster.name, cluster.displayName, cluster.id, cluster.server, cluster.context]
                    .filter(Boolean)
                    .join(' ')
                    .toLowerCase(),
                kubernetesInfo: {
                    server: cluster.server,
                    context: cluster.context,
                    tokenName: cluster.tokenName,
                },
            });
        });

        removedDockerHosts().forEach(host => {
            const name = host.displayName || host.hostname || host.id;
            rows.push({
                rowKey: `removed-docker-${host.id}`,
                id: host.id,
                name,
                hostname: host.hostname,
                displayName: host.displayName,
                types: ['docker'],
                status: 'removed',
                removedAt: host.removedAt,
                scope: getScopeInfo(undefined),
                searchText: [name, host.hostname, host.id].filter(Boolean).join(' ').toLowerCase(),
            });
        });

        removedKubernetesClusters().forEach(cluster => {
            const name = cluster.displayName || cluster.name || cluster.id;
            rows.push({
                rowKey: `removed-k8s-${cluster.id}`,
                id: cluster.id,
                name,
                types: ['kubernetes'],
                status: 'removed',
                removedAt: cluster.removedAt,
                scope: getScopeInfo(undefined),
                searchText: [name, cluster.name, cluster.id].filter(Boolean).join(' ').toLowerCase(),
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

    createEffect(() => {
        const key = expandedRowKey();
        if (!key) {
            setHostCustomUrl('');
            return;
        }

        const row = untrack(() => unifiedRows().find(r => r.rowKey === key));
        if (!row || row.status !== 'active' || !row.types.includes('host')) {
            setHostCustomUrl('');
            return;
        }

        const currentKey = key;
        void (async () => {
            try {
                const metadata = await HostMetadataAPI.getMetadata(row.id);
                if (expandedRowKey() !== currentKey) return;
                setHostCustomUrl(metadata.customUrl || '');
            } catch (err) {
                logger.error('Failed to load host metadata', err);
                if (expandedRowKey() !== currentKey) return;
                setHostCustomUrl('');
            }
        })();
    });

    createEffect(() => {
        const key = expandedRowKey();
        if (!key) {
            setContainerUrls({});
            return;
        }

        const row = untrack(() => unifiedRows().find(r => r.rowKey === key));
        if (!row || row.status !== 'active' || !row.types.includes('docker')) {
            setContainerUrls({});
            return;
        }

        const dockerHost = untrack(() => (state.dockerHosts || []).find(h => h.id === row.id));
        if (!dockerHost?.containers?.length) {
            setContainerUrls({});
            return;
        }

        const hostId = row.id;
        const containers = dockerHost.containers;
        const currentKey = key;
        void (async () => {
            try {
                const results: Record<string, string> = {};
                await Promise.all(containers.map(async (c) => {
                    const metaKey = `${hostId}:container:${c.id}`;
                    try {
                        const meta = await DockerMetadataAPI.getMetadata(metaKey);
                        results[c.id] = meta.customUrl || '';
                    } catch {
                        results[c.id] = '';
                    }
                }));
                if (expandedRowKey() !== currentKey) return;
                setContainerUrls(results);
            } catch (err) {
                logger.error('Failed to load container metadata', err);
            }
        })();
    });

    const filteredRows = createMemo(() => {
        const query = filterSearch().trim().toLowerCase();
        return unifiedRows().filter(row => {
            if (filterType() !== 'all' && !row.types.includes(filterType() as UnifiedAgentType)) {
                return false;
            }
            if (filterStatus() !== 'all' && row.status !== filterStatus()) {
                return false;
            }
            if (filterScope() !== 'all' && row.scope.category !== filterScope()) {
                return false;
            }
            if (query && !row.searchText.includes(query)) {
                return false;
            }
            return true;
        });
    });

    const hasFilters = createMemo(() => {
        return (
            filterType() !== 'all' ||
            filterStatus() !== 'all' ||
            filterScope() !== 'all' ||
            filterSearch().trim().length > 0
        );
    });

    const resetFilters = () => {
        setFilterType('all');
        setFilterStatus('all');
        setFilterScope('all');
        setFilterSearch('');
    };

    const getUpgradeCommand = (_hostname: string) => {
        const token = resolvedToken();
        const url = customAgentUrl() || agentUrl();
        return `curl ${getCurlInsecureFlag()}-fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${token}${getInsecureFlag()}`;
    };

    const handleRemoveAgent = async (id: string, types: ('host' | 'docker')[]) => {
        if (!confirm('Are you sure you want to remove this agent? This will stop monitoring but will not uninstall the agent from the remote machine.')) return;

        try {
            // Delete all types associated with this agent
            for (const type of types) {
                if (type === 'host') {
                    await MonitoringAPI.deleteHostAgent(id);
                } else if (type === 'docker') {
                    await MonitoringAPI.deleteDockerHost(id);
                }
            }
            notificationStore.success('Agent removed from Pulse');
        } catch (err) {
            logger.error('Failed to remove agent', err);
            notificationStore.error('Failed to remove agent');
        }
    };

    const handleAllowReenroll = async (hostId: string, hostname?: string) => {
        try {
            await MonitoringAPI.allowDockerHostReenroll(hostId);
            notificationStore.success(`Re-enrollment allowed for ${hostname || hostId}. Restart the agent to reconnect.`);
        } catch (err) {
            logger.error('Failed to allow re-enrollment', err);
            notificationStore.error('Failed to allow re-enrollment');
        }
    };

    const handleRemoveKubernetesCluster = async (clusterId: string) => {
        if (!confirm('Are you sure you want to remove this Kubernetes cluster? This will stop monitoring but will not uninstall the agent from the cluster.')) return;

        try {
            await MonitoringAPI.deleteKubernetesCluster(clusterId);
            notificationStore.success('Kubernetes cluster removed from Pulse');
        } catch (err) {
            logger.error('Failed to remove Kubernetes cluster', err);
            notificationStore.error('Failed to remove Kubernetes cluster');
        }
    };

    const handleAllowKubernetesReenroll = async (clusterId: string, name?: string) => {
        try {
            await MonitoringAPI.allowKubernetesClusterReenroll(clusterId);
            notificationStore.success(`Re-enrollment allowed for ${name || clusterId}. Restart the agent to reconnect.`);
        } catch (err) {
            logger.error('Failed to allow kubernetes re-enrollment', err);
            notificationStore.error('Failed to allow kubernetes re-enrollment');
        }
    };

    const handleToggleCommands = async (hostId: string, enabled: boolean) => {
        // Set optimistic/pending state immediately with timestamp
        setPendingCommandConfig(prev => ({
            ...prev,
            [hostId]: { enabled, timestamp: Date.now() }
        }));

        try {
            await MonitoringAPI.updateHostAgentConfig(hostId, { commandsEnabled: enabled });
            notificationStore.success(`Pulse command execution ${enabled ? 'enabled' : 'disabled'}. Syncing with agent...`);
        } catch (err) {
            // On error, clear the pending state so toggle reverts
            setPendingCommandConfig(prev => {
                const next = { ...prev };
                delete next[hostId];
                return next;
            });
            logger.error('Failed to toggle AI commands', err);
            notificationStore.error('Failed to update agent configuration');
        }
    };


    // Clear pending state when agent reports matching the expected value, or after timeout
    createEffect(() => {
        const pending = pendingCommandConfig();
        const hosts = state.hosts || [];
        const now = Date.now();
        const TIMEOUT_MS = 2 * 60 * 1000; // 2 minutes

        // Check if any pending config now matches the reported state or has timed out
        let updated = false;
        const newPending = { ...pending };
        const timedOut: string[] = [];

        for (const hostId of Object.keys(pending)) {
            const entry = pending[hostId];
            const host = hosts.find(h => h.id === hostId);

            if (host && host.commandsEnabled === entry.enabled) {
                // Agent confirmed the change
                delete newPending[hostId];
                updated = true;
            } else if (now - entry.timestamp > TIMEOUT_MS) {
                // Timed out waiting for agent
                delete newPending[hostId];
                timedOut.push(host?.hostname || hostId);
                updated = true;
            }
        }

        if (updated) {
            setPendingCommandConfig(newPending);
            if (timedOut.length > 0) {
                notificationStore.warning(`Config sync timed out for ${timedOut.join(', ')}. Agent may be offline.`);
            }
        }
    });

    return (
        <div class="space-y-6">
            <SettingsPanel
                title="Add a unified agent"
                description="Monitor server metrics (CPU, RAM, Disk) and Docker containers with a single agent."
                icon={<Server class="w-5 h-5" strokeWidth={2} />}
                bodyClass="space-y-5"
            >

                <div class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-100">
                    <div class="flex items-start gap-3">
                        <ProxmoxIcon class="w-5 h-5 text-orange-500 mt-0.5 shrink-0" />
                        <div class="flex-1">
                            <p class="text-sm">
                                Proxmox nodes can be added here with the unified agent for extra capabilities like temperature monitoring and Pulse Patrol automation (auto-creates the API token and links the node).
                            </p>
                            <button
                                type="button"
                                onClick={() => navigate('/settings/proxmox')}
                                class="mt-2 text-xs font-medium text-emerald-800 hover:text-emerald-900 dark:text-emerald-200 dark:hover:text-emerald-100 underline"
                            >
                                Prefer API-only? Use manual setup →
                            </button>
                        </div>
                    </div>
                </div>

                <div class="space-y-5">
                    <Show when={requiresToken()}>
                        <div class="space-y-3">
                            <div class="space-y-1">
                                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                                    <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-blue-600 text-white text-xs font-bold">1</span>
                                    Generate API token
                                </p>
                                <p class="text-sm text-gray-600 dark:text-gray-400 ml-6">
                                    Create a fresh token scoped for Host, Docker, and Kubernetes monitoring.
                                </p>
                            </div>

                            <div class="flex gap-2">
                                <input
                                    type="text"
                                    value={tokenName()}
                                    onInput={(event) => setTokenName(event.currentTarget.value)}
                                    onKeyDown={(event) => {
                                        if (event.key === 'Enter' && !isGeneratingToken()) {
                                            handleGenerateToken();
                                        }
                                    }}
                                    placeholder="Token name (optional)"
                                    class="flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/60"
                                />
                                <button
                                    type="button"
                                    onClick={handleGenerateToken}
                                    disabled={isGeneratingToken()}
                                    class="inline-flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                                >
                                    {isGeneratingToken() ? 'Generating…' : hasToken() ? 'Generate another' : 'Generate token'}
                                </button>
                            </div>

                            <Show when={latestRecord()}>
                                <div class="flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                                    </svg>
                                    <span>
                                        Token <strong>{latestRecord()?.name}</strong> created. Commands below now include this credential.
                                    </span>
                                </div>
                            </Show>

                        </div>
                    </Show>

                    <Show when={!requiresToken()}>
                        <div class="space-y-3">
                            <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-200">
                                Tokens are optional on this Pulse instance. Confirm to generate commands without embedding a token.
                            </div>
                            <button
                                type="button"
                                onClick={acknowledgeNoToken}
                                disabled={confirmedNoToken()}
                                class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${confirmedNoToken()
                                    ? 'bg-green-600 text-white cursor-default'
                                    : 'bg-gray-900 text-white hover:bg-black dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-white'
                                    }`}
                            >
                                {confirmedNoToken() ? 'No token confirmed' : 'Confirm without token'}
                            </button>
                        </div>
                    </Show>

                    {/* Show locked step 2 preview when token required but not yet generated */}
                    <Show when={requiresToken() && !commandsUnlocked()}>
                        <div class="space-y-3 opacity-60 pointer-events-none select-none">
                            <div class="flex items-center justify-between">
                                <div>
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                                        <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-gray-400 text-white text-xs font-bold">2</span>
                                        Installation commands
                                    </h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 ml-6">Generate a token above to unlock installation commands.</p>
                                </div>
                            </div>
                            <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-6 dark:border-gray-700 dark:bg-gray-800/50 text-center">
                                <svg class="w-8 h-8 mx-auto text-gray-400 dark:text-gray-500 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
                                </svg>
                                <p class="text-sm text-gray-500 dark:text-gray-400">Click "Generate token" above to see installation commands</p>
                            </div>
                        </div>
                    </Show>

                    <Show when={commandsUnlocked()}>
                        <div class="space-y-3">
                            <div class="space-y-3">
                                <div class="flex items-center justify-between">
                                    <div>
                                        <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                                            <Show when={requiresToken()}>
                                                <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-green-600 text-white text-xs font-bold">2</span>
                                            </Show>
                                            Installation commands
                                        </h4>
                                        <p class={`text-xs text-gray-500 dark:text-gray-400 mt-0.5 ${requiresToken() ? 'ml-6' : ''}`}>The installer auto-detects Docker, Kubernetes, and Proxmox on the target machine.</p>
                                    </div>
                                </div>

                                <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50">
                                    <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                                        Connection URL (Agent → Pulse)
                                    </label>
                                    <div class="flex gap-2">
                                        <input
                                            type="text"
                                            value={customAgentUrl()}
                                            onInput={(e) => setCustomAgentUrl(e.currentTarget.value)}
                                            placeholder={agentUrl()}
                                            class="flex-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                                        />
                                    </div>
                                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                                        Override the address agents use to connect to this server (e.g., use IP address <code>http://192.168.1.50:7655</code> if DNS fails).
                                        <Show when={!customAgentUrl()}>
                                            <span class="ml-1 opacity-75">Currently using auto-detected: {agentUrl()}</span>
                                        </Show>
                                    </p>
                                </div>
                                <Show when={insecureMode()}>
                                    <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-200">
                                        <span class="font-medium">TLS verification disabled</span> — skip cert checks for self-signed setups. Not recommended for production.
                                    </div>
                                </Show>
                                <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer" title="Skip TLS certificate verification (for self-signed certificates)">
                                    <input
                                        type="checkbox"
                                        checked={insecureMode()}
                                        onChange={(e) => setInsecureMode(e.currentTarget.checked)}
                                        class="rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                                    />
                                    Skip TLS certificate verification (self-signed certs; not recommended)
                                </label>
                                <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer" title="Allow Pulse Patrol to execute diagnostic and fix commands on this agent (auto-fix requires Pulse Pro)">
                                    <input
                                        type="checkbox"
                                        checked={enableCommands()}
                                        onChange={(e) => setEnableCommands(e.currentTarget.checked)}
                                        class="rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                                    />
                                    Enable Pulse command execution (for Patrol auto-fix)
                                </label>
                                <Show when={enableCommands()}>
                                    <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-sm text-blue-800 dark:border-blue-700 dark:bg-blue-900/20 dark:text-blue-200">
                                        <span class="font-medium">Pulse commands enabled</span> — The agent will accept diagnostic and fix commands from Pulse Patrol features.
                                    </div>
                                </Show>
                                <div class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-100">
                                    <span class="font-medium">Config signing (optional)</span> — Require signed remote config payloads with <code>PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true</code>. Provide keys via <code>PULSE_AGENT_CONFIG_SIGNING_KEY</code> (Pulse) and <code>PULSE_AGENT_CONFIG_PUBLIC_KEYS</code> (agents).
                                </div>
                            </div>

                            <div class="space-y-4">
                                <For each={commandSections()}>
                                    {(section) => (
                                        <div class="space-y-3 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
                                            <div class="space-y-1">
                                                <h5 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{section.title}</h5>
                                                <p class="text-xs text-gray-500 dark:text-gray-400">{section.description}</p>
                                            </div>
                                            <div class="space-y-3">
                                                <For each={section.snippets}>
                                                    {(snippet) => {
                                                        const copyCommand = () => {
                                                            let cmd = snippet.command.replace(TOKEN_PLACEHOLDER, resolvedToken());
                                                            // Insert -k flag for curl if insecure mode enabled (issue #806)
                                                            if (insecureMode() && cmd.includes('curl -fsSL')) {
                                                                cmd = cmd.replace('curl -fsSL', 'curl -kfsSL');
                                                            }
                                                            // For bash scripts (not PowerShell), append insecure flag
                                                            const isBashScript = !cmd.includes('$env:') && !cmd.includes('irm');
                                                            if (insecureMode() && isBashScript) {
                                                                cmd += getInsecureFlag();
                                                            }
                                                            // Add --enable-commands flag if enabled (issue #903)
                                                            if (enableCommands() && isBashScript) {
                                                                cmd += getEnableCommandsFlag();
                                                            }
                                                            return cmd;
                                                        };

                                                        return (
                                                            <div class="space-y-2">
                                                                <h6 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                                                                    {snippet.label}
                                                                </h6>
                                                                <div class="relative">
                                                                    <button
                                                                        type="button"
                                                                        onClick={async () => {
                                                                            const success = await copyToClipboard(copyCommand());
                                                                            if (success) {
                                                                                notificationStore.success('Copied to clipboard');
                                                                            } else {
                                                                                notificationStore.error('Failed to copy');
                                                                            }
                                                                        }}
                                                                        class="absolute top-2 right-2 p-1.5 text-gray-400 hover:text-gray-200 bg-gray-700 rounded-md transition-colors"
                                                                        title="Copy command"
                                                                    >
                                                                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                                                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                                                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                                                        </svg>
                                                                    </button>
                                                                    <pre class="overflow-x-auto rounded-md bg-gray-900/90 p-3 pr-12 text-xs text-gray-100">
                                                                        <code>{copyCommand()}</code>
                                                                    </pre>
                                                                </div>
                                                                <Show when={snippet.note}>
                                                                    <p class="text-xs text-gray-500 dark:text-gray-400">{snippet.note}</p>
                                                                </Show>
                                                            </div>
                                                        );
                                                    }}
                                                </For>
                                            </div>
                                        </div>
                                    )}
                                </For>
                            </div>

                            <div class="space-y-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-100">
                                <div class="flex items-center justify-between gap-3">
                                    <h5 class="text-sm font-semibold">Check installation status</h5>
                                    <button
                                        type="button"
                                        onClick={handleLookup}
                                        disabled={lookupLoading()}
                                        class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                                    >
                                        {lookupLoading() ? 'Checking…' : 'Check status'}
                                    </button>
                                </div>
                                <p class="text-xs text-blue-800 dark:text-blue-200">
                                    Enter the hostname (or host ID) from the machine you just installed. Pulse returns the latest status instantly.
                                </p>
                                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                                    <input
                                        type="text"
                                        value={lookupValue()}
                                        onInput={(event) => {
                                            setLookupValue(event.currentTarget.value);
                                            setLookupError(null);
                                            setLookupResult(null);
                                        }}
                                        onKeyDown={(event) => {
                                            if (event.key === 'Enter') {
                                                event.preventDefault();
                                                void handleLookup();
                                            }
                                        }}
                                        placeholder="Hostname or host ID"
                                        class="flex-1 rounded-lg border border-blue-200 bg-white px-3 py-2 text-sm text-blue-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100 dark:focus:border-blue-300 dark:focus:ring-blue-800/60"
                                    />
                                </div>
                                <Show when={lookupError()}>
                                    <p class="text-xs font-medium text-red-600 dark:text-red-300">{lookupError()}</p>
                                </Show>
                                <Show when={lookupResult()}>
                                    {(result) => {
                                        const host = () => result().host;
                                        const statusBadgeClasses = () =>
                                            host().connected
                                                ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                                : 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200';
                                        return (
                                            <div class="space-y-1 rounded-lg border border-blue-200 bg-white px-3 py-2 text-xs text-blue-900 dark:border-blue-700 dark:bg-blue-900/40 dark:text-blue-100">
                                                <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                                                    <div class="text-sm font-semibold">
                                                        {host().displayName || host().hostname}
                                                    </div>
                                                    <div class="flex items-center gap-2">
                                                        <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold ${statusBadgeClasses()}`}>
                                                            {host().connected ? 'Connected' : 'Not reporting yet'}
                                                        </span>
                                                        <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900/60 dark:text-blue-200">
                                                            {host().status || 'unknown'}
                                                        </span>
                                                    </div>
                                                </div>
                                                <div>
                                                    Last seen {formatRelativeTime(host().lastSeen)} ({formatAbsoluteTime(host().lastSeen)})
                                                </div>
                                                <Show when={host().agentVersion}>
                                                    <div class="text-xs text-blue-700 dark:text-blue-200">
                                                        Agent version {host().agentVersion}
                                                    </div>
                                                </Show>
                                            </div>
                                        );
                                    }}
                                </Show>
                            </div>
                            <details class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-700 dark:border-gray-700 dark:bg-gray-800/50 dark:text-gray-300">
                                <summary class="cursor-pointer text-sm font-medium text-gray-900 dark:text-gray-100">
                                    Troubleshooting
                                </summary>
                                <div class="mt-3 space-y-4">
                                    <div>
                                        <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Auto-detection not working?</p>
                                        <p class="mt-1 text-xs text-gray-600 dark:text-gray-400">
                                            If Docker, Kubernetes, or Proxmox isn't detected automatically, add these flags to the install command:
                                        </p>
                                        <ul class="mt-2 text-xs text-gray-600 dark:text-gray-400 list-disc list-inside space-y-1">
                                            <li><code class="bg-gray-200 dark:bg-gray-700 px-1 rounded">--enable-docker</code> — Force enable Docker/Podman monitoring</li>
                                            <li><code class="bg-gray-200 dark:bg-gray-700 px-1 rounded">--enable-kubernetes</code> — Force enable Kubernetes monitoring</li>
                                            <li><code class="bg-gray-200 dark:bg-gray-700 px-1 rounded">--enable-proxmox</code> — Force enable Proxmox integration (creates API token)</li>
                                            <li><code class="bg-gray-200 dark:bg-gray-700 px-1 rounded">--disable-docker</code> — Skip Docker even if detected</li>
                                        </ul>
                                    </div>
                                </div>
                            </details>
                        </div>
                    </Show>

                    {/* Uninstall section - always visible */}
                    <div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-4">
                        <div class="space-y-3">
                            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Uninstall agent</h4>
                            <p class="text-xs text-gray-600 dark:text-gray-400">
                                Run the appropriate command on your host to remove the Pulse agent:
                            </p>
                            {/* Linux/macOS uninstall */}
                            <div class="space-y-1">
                                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Linux / macOS / FreeBSD</span>
                                <div class="relative">
                                    <button
                                        type="button"
                                        onClick={async () => {
                                            const success = await copyToClipboard(getUninstallCommand());
                                            if (success) {
                                                notificationStore.success('Copied to clipboard');
                                            } else {
                                                notificationStore.error('Failed to copy');
                                            }
                                        }}
                                        class="absolute top-2 right-2 p-1.5 text-gray-400 hover:text-gray-200 bg-gray-700/50 hover:bg-gray-700 rounded-md transition-colors"
                                        title="Copy command"
                                    >
                                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                    </button>
                                    <pre class="overflow-x-auto rounded-md bg-gray-950 p-3 pr-12 font-mono text-xs text-red-400">
                                        <code>{getUninstallCommand()}</code>
                                    </pre>
                                </div>
                            </div>
                            {/* Windows uninstall */}
                            <div class="space-y-1">
                                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Windows (PowerShell as Administrator)</span>
                                <div class="relative">
                                    <button
                                        type="button"
                                        onClick={async () => {
                                            const success = await copyToClipboard(getWindowsUninstallCommand());
                                            if (success) {
                                                notificationStore.success('Copied to clipboard');
                                            } else {
                                                notificationStore.error('Failed to copy');
                                            }
                                        }}
                                        class="absolute top-2 right-2 p-1.5 text-gray-400 hover:text-gray-200 bg-gray-700/50 hover:bg-gray-700 rounded-md transition-colors"
                                        title="Copy command"
                                    >
                                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                    </button>
                                    <pre class="overflow-x-auto rounded-md bg-gray-950 p-3 pr-12 font-mono text-xs text-red-400">
                                        <code>{getWindowsUninstallCommand()}</code>
                                    </pre>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </SettingsPanel>

            <SettingsPanel
                title="Managed Agents"
                description="All active and removed agents, including Kubernetes clusters."
                icon={<Users class="w-5 h-5" strokeWidth={2} />}
                bodyClass="space-y-4"
            >

                <Show when={hasLinkedAgents()}>
                    <div class="flex items-start gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-900/20">
                        <svg class="h-4 w-4 mt-0.5 flex-shrink-0 text-blue-500 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <p class="text-xs text-blue-700 dark:text-blue-300">
                            <span class="font-medium">{linkedHostAgents().length}</span> host agent{linkedHostAgents().length > 1 ? 's are' : ' is'} linked to Proxmox node{linkedHostAgents().length > 1 ? 's' : ''} and flagged with a <span class="font-medium text-indigo-700 dark:text-indigo-300">Linked</span> badge.
                        </p>
                    </div>
                </Show>

                <Show when={hasLegacyAgents()}>
                    <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-700 dark:bg-amber-900/20">
                        <div class="flex items-start gap-3">
                            <svg class="h-5 w-5 flex-shrink-0 text-amber-500 dark:text-amber-400 mt-0.5" viewBox="0 0 20 20" fill="currentColor">
                                <path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
                            </svg>
                            <div class="flex-1 space-y-1">
                                <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                                    {legacyAgents().length} legacy agent{legacyAgents().length > 1 ? 's' : ''} detected
                                </p>
                                <p class="text-sm text-amber-700 dark:text-amber-300">
                                    Legacy agents (pulse-host-agent, pulse-docker-agent) are deprecated. Expand a row to copy the upgrade command.
                                </p>
                            </div>
                        </div>
                    </div>
                </Show>

                <div class="flex flex-wrap items-end gap-3">
                    <div class="space-y-1">
                        <label for="agent-filter-type" class="text-xs font-medium text-gray-600 dark:text-gray-400">Type</label>
                        <select
                            id="agent-filter-type"
                            value={filterType()}
                            onChange={(event) => setFilterType(event.currentTarget.value as 'all' | UnifiedAgentType)}
                            class="rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                        >
                            <option value="all">All types</option>
                            <option value="host">Host</option>
                            <option value="docker">Docker</option>
                            <option value="kubernetes">Kubernetes</option>
                        </select>
                    </div>
                    <div class="space-y-1">
                        <label for="agent-filter-status" class="text-xs font-medium text-gray-600 dark:text-gray-400">Status</label>
                        <select
                            id="agent-filter-status"
                            value={filterStatus()}
                            onChange={(event) => setFilterStatus(event.currentTarget.value as 'all' | UnifiedAgentStatus)}
                            class="rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                        >
                            <option value="all">All statuses</option>
                            <option value="active">Active</option>
                            <option value="removed">Removed/Blocked</option>
                        </select>
                    </div>
                    <div class="space-y-1">
                        <label for="agent-filter-scope" class="text-xs font-medium text-gray-600 dark:text-gray-400">Scope</label>
                        <select
                            id="agent-filter-scope"
                            value={filterScope()}
                            onChange={(event) => setFilterScope(event.currentTarget.value as 'all' | Exclude<ScopeCategory, 'na'>)}
                            class="rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                        >
                            <option value="all">All scopes</option>
                            <option value="default">Default</option>
                            <option value="profile">Profile assigned</option>
                            <option value="ai-managed">Patrol-managed</option>
                        </select>
                    </div>
                    <div class="min-w-[220px] flex-1 space-y-1">
                        <label for="agent-filter-search" class="text-xs font-medium text-gray-600 dark:text-gray-400">Search</label>
                        <input
                            id="agent-filter-search"
                            type="text"
                            value={filterSearch()}
                            onInput={(event) => setFilterSearch(event.currentTarget.value)}
                            placeholder="Search name, hostname, or ID"
                            class="w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                        />
                    </div>
                    <button
                        type="button"
                        onClick={resetFilters}
                        disabled={!hasFilters()}
                        class={`rounded-md px-3 py-2 text-xs font-medium transition-colors ${hasFilters()
                            ? 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700'
                            : 'bg-gray-50 text-gray-400 cursor-not-allowed dark:bg-gray-900 dark:text-gray-500'
                            }`}
                    >
                        Clear
                    </button>
                </div>

                <div class="text-xs text-gray-500 dark:text-gray-400">
                    Showing {filteredRows().length} of {unifiedRows().length} records.
                </div>

                <Card padding="none" tone="glass" class="overflow-x-auto rounded-lg">
                    <table class="w-full min-w-[960px] divide-y divide-gray-200 dark:divide-gray-700">
                        <thead class="bg-gray-50 dark:bg-gray-800">
                            <tr>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Name</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Type</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Status</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Scope</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Pulse Cmds</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Last Seen</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Version</th>
                                <th scope="col" class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Actions</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
                            <For each={filteredRows()} fallback={
                                <tr>
                                    <td colspan="8" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                                        <Show when={hasFilters()} fallback="No agents installed yet.">
                                            No agents match the current filters.
                                        </Show>
                                    </td>
                                </tr>
                            }>
                                {(row) => {
                                    const expanded = () => expandedRowKey() === row.rowKey;
                                    const isActive = () => row.status === 'active';
                                    const isRemoved = () => row.status === 'removed';
                                    const isKubernetes = () => row.types.includes('kubernetes');
                                    const resolvedAgentId = row.agentId || '';
                                    const assignment = () => resolvedAgentId ? assignmentByAgent().get(resolvedAgentId) : undefined;
                                    const isScopeUpdating = () => resolvedAgentId ? Boolean(pendingScopeUpdates()[resolvedAgentId]) : false;
                                    const agentName = row.displayName || row.hostname || row.name;
                                    const typeBadgeClass = (type: UnifiedAgentType) => {
                                        if (type === 'host') {
                                            return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300';
                                        }
                                        if (type === 'docker') {
                                            return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300';
                                        }
                                        return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300';
                                    };
                                    const statusBadgeClass = () => {
                                        if (isRemoved()) {
                                            return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200';
                                        }
                                        if (connectedFromStatus(row.healthStatus)) {
                                            return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300';
                                        }
                                        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300';
                                    };
                                    const lastSeenLabel = () => {
                                        if (isRemoved()) {
                                            return row.removedAt ? `Removed ${formatRelativeTime(row.removedAt)}` : 'Removed';
                                        }
                                        return row.lastSeen ? formatRelativeTime(row.lastSeen) : '—';
                                    };

                                    return (
                                        <>
                                            <tr>
                                                <td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">
                                                    <div class="flex items-start justify-between gap-3">
                                                        <div>
                                                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                                                {row.name}
                                                            </div>
                                                            <Show when={row.displayName && row.hostname && row.displayName !== row.hostname}>
                                                                <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                    {row.hostname}
                                                                </div>
                                                            </Show>
                                                        </div>
                                                        <button
                                                            type="button"
                                                            onClick={() => toggleAgentDetails(row.rowKey)}
                                                            class="inline-flex items-center justify-center rounded-md p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                                                            aria-label={`${expanded() ? 'Hide' : 'Show'} details for ${agentName}`}
                                                            aria-expanded={expanded()}
                                                            aria-controls={`agent-details-${row.rowKey}`}
                                                        >
                                                            <svg
                                                                class={`h-4 w-4 transition-transform ${expanded() ? 'rotate-180' : ''}`}
                                                                viewBox="0 0 20 20"
                                                                fill="currentColor"
                                                            >
                                                                <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 10.94l3.71-3.7a.75.75 0 111.06 1.06l-4.24 4.24a.75.75 0 01-1.06 0L5.21 8.29a.75.75 0 01.02-1.08z" clip-rule="evenodd" />
                                                            </svg>
                                                        </button>
                                                    </div>
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-sm">
                                                    <div class="flex flex-wrap items-center gap-2 text-xs">
                                                        <For each={row.types}>
                                                            {(type) => (
                                                                <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${typeBadgeClass(type)}`}>
                                                                    {type === 'host' ? 'Host' : type === 'docker' ? 'Docker' : 'Kubernetes'}
                                                                </span>
                                                            )}
                                                        </For>
                                                    </div>
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-sm">
                                                    <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${statusBadgeClass()}`}>
                                                        {isRemoved() ? 'Removed' : row.healthStatus || 'unknown'}
                                                    </span>
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                                    <Show when={isActive() && !isKubernetes() && resolvedAgentId} fallback={
                                                        <span class="text-xs text-gray-400 dark:text-gray-500">N/A</span>
                                                    }>
                                                        <Show when={profiles().length > 0} fallback={
                                                            <span class="text-gray-700 dark:text-gray-300" title={row.scope.detail}>
                                                                {row.scope.label}
                                                            </span>
                                                        }>
                                                            <div class="flex items-center gap-2">
                                                                <select
                                                                    value={assignment()?.profile_id || ''}
                                                                    onChange={(event) => {
                                                                        const nextValue = event.currentTarget.value;
                                                                        const currentValue = assignment()?.profile_id || '';
                                                                        if (nextValue === currentValue) {
                                                                            return;
                                                                        }
                                                                        void updateScopeAssignment(resolvedAgentId, nextValue || null, agentName);
                                                                    }}
                                                                    disabled={isScopeUpdating()}
                                                                    class="rounded-md border border-gray-300 bg-white px-2 py-1 text-xs text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                                                                >
                                                                    <option value="">Default (Auto-detect)</option>
                                                                    <For each={profiles()}>
                                                                        {(profile) => (
                                                                            <option value={profile.id}>{profile.name || profile.id}</option>
                                                                        )}
                                                                    </For>
                                                                </select>
                                                                <Show when={isScopeUpdating()}>
                                                                    <span class="text-[10px] text-gray-400 dark:text-gray-500">Updating…</span>
                                                                </Show>
                                                            </div>
                                                        </Show>
                                                    </Show>
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                                    <Show when={isActive() && row.types.includes('host')} fallback={
                                                        <span class="text-xs text-gray-400 dark:text-gray-500">N/A</span>
                                                    }>
                                                        {(() => {
                                                            const pending = pendingCommandConfig();
                                                            const isPending = row.id in pending;
                                                            const effectiveEnabled = isPending ? pending[row.id].enabled : Boolean(row.commandsEnabled);

                                                            return (
                                                                <div class="flex items-center gap-2">
                                                                    <button
                                                                        onClick={() => handleToggleCommands(row.id, !effectiveEnabled)}
                                                                        disabled={isPending}
                                                                        class={`relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${isPending ? 'opacity-60 cursor-wait' : ''
                                                                            } ${effectiveEnabled
                                                                                ? 'bg-blue-600'
                                                                                : 'bg-gray-200 dark:bg-gray-700'
                                                                            }`}
                                                                        title={isPending
                                                                            ? 'Syncing with agent...'
                                                                            : effectiveEnabled
                                                                                ? 'Pulse command execution enabled'
                                                                                : 'Pulse command execution disabled'
                                                                        }
                                                                    >
                                                                        <span
                                                                            class={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${effectiveEnabled ? 'translate-x-4' : 'translate-x-0'
                                                                                }`}
                                                                        />
                                                                    </button>
                                                                    <Show when={isPending}>
                                                                        <svg class="animate-spin h-4 w-4 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                                                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                                                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                                                        </svg>
                                                                    </Show>
                                                                </div>
                                                            );
                                                        })()}
                                                    </Show>
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                                    {lastSeenLabel()}
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                                    {row.version || '—'}
                                                </td>
                                                <td class="whitespace-nowrap px-4 py-3 text-right text-sm font-medium">
                                                    <Show when={isRemoved()} fallback={
                                                        <Show when={isKubernetes()} fallback={
                                                            <button
                                                                onClick={() => handleRemoveAgent(row.id, row.types.filter(type => type !== 'kubernetes') as ('host' | 'docker')[])}
                                                                class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                                            >
                                                                Remove
                                                            </button>
                                                        }>
                                                            <button
                                                                onClick={() => handleRemoveKubernetesCluster(row.id)}
                                                                class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                                            >
                                                                Remove
                                                            </button>
                                                        </Show>
                                                    }>
                                                        <Show when={row.types.includes('docker')} fallback={
                                                            <button
                                                                onClick={() => handleAllowKubernetesReenroll(row.id, row.name)}
                                                                class="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                                                            >
                                                                Allow re-enroll
                                                            </button>
                                                        }>
                                                            <button
                                                                onClick={() => handleAllowReenroll(row.id, row.hostname)}
                                                                class="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                                                            >
                                                                Allow re-enroll
                                                            </button>
                                                        </Show>
                                                    </Show>
                                                </td>
                                            </tr>
                                            <Show when={expanded()}>
                                                <tr id={`agent-details-${row.rowKey}`} class="bg-gray-50 dark:bg-gray-800/40">
                                                    <td colspan="8" class="px-4 py-4 text-sm text-gray-600 dark:text-gray-300">
                                                        <div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto]">
                                                            <div class="space-y-3">
                                                                <div class="flex flex-wrap items-center gap-2 text-xs">
                                                                    <For each={row.types}>
                                                                        {(type) => (
                                                                            <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${typeBadgeClass(type)}`}>
                                                                                {type === 'host' ? 'Host' : type === 'docker' ? 'Docker' : 'Kubernetes'}
                                                                            </span>
                                                                        )}
                                                                    </For>
                                                                    <Show when={row.isLegacy}>
                                                                        <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
                                                                            Legacy
                                                                        </span>
                                                                    </Show>
                                                                    <Show when={row.linkedNodeId}>
                                                                        <span class="inline-flex items-center rounded-full bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-300">
                                                                            Linked
                                                                        </span>
                                                                    </Show>
                                                                </div>
                                                                <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                    ID: <span class="font-mono text-gray-700 dark:text-gray-200">{row.id}</span>
                                                                </div>
                                                                <Show when={row.agentId && row.agentId !== row.id}>
                                                                    <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                        Agent ID: <span class="font-mono text-gray-700 dark:text-gray-200">{row.agentId}</span>
                                                                    </div>
                                                                </Show>
                                                                <Show when={row.linkedNodeId}>
                                                                    <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                        Linked node ID: <span class="font-mono text-gray-700 dark:text-gray-200">{row.linkedNodeId}</span>
                                                                    </div>
                                                                </Show>
                                                                <Show when={row.status === 'active' && row.types.includes('host')}>
                                                                    <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-900/30">
                                                                        <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                                                                            Custom URL
                                                                        </label>
                                                                        <div class="flex gap-2">
                                                                            <input
                                                                                type="text"
                                                                                value={hostCustomUrl()}
                                                                                onInput={(event) => setHostCustomUrl(event.currentTarget.value)}
                                                                                onKeyDown={(event) => {
                                                                                    if (event.key === 'Enter' && !hostCustomUrlSaving()) {
                                                                                        void saveHostUrl(row.id);
                                                                                    }
                                                                                }}
                                                                                placeholder="https://host.example.com"
                                                                                class="flex-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                                                                            />
                                                                            <button
                                                                                type="button"
                                                                                onClick={() => void saveHostUrl(row.id)}
                                                                                disabled={hostCustomUrlSaving()}
                                                                                class="inline-flex items-center justify-center rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                                                                            >
                                                                                {hostCustomUrlSaving() ? 'Saving…' : 'Save'}
                                                                            </button>
                                                                        </div>
                                                                        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                                                                            Optional override used when Pulse links to this host (leave blank to use the default).
                                                                        </p>
                                                                    </div>
                                                                </Show>
                                                                <Show when={row.status === 'active' && row.lastSeen}>
                                                                    <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                        Last seen {formatRelativeTime(row.lastSeen)} ({formatAbsoluteTime(row.lastSeen)})
                                                                    </div>
                                                                </Show>
                                                                <Show when={row.status === 'removed' && row.removedAt}>
                                                                    <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                        Removed {formatRelativeTime(row.removedAt)} ({formatAbsoluteTime(row.removedAt)})
                                                                    </div>
                                                                </Show>
                                                                <Show when={row.kubernetesInfo && (row.kubernetesInfo.server || row.kubernetesInfo.context || row.kubernetesInfo.tokenName)}>
                                                                    <div class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
                                                                        <Show when={row.kubernetesInfo?.server}>
                                                                            <div>Server: <span class="text-gray-700 dark:text-gray-200">{row.kubernetesInfo?.server}</span></div>
                                                                        </Show>
                                                                        <Show when={row.kubernetesInfo?.context}>
                                                                            <div>Context: <span class="text-gray-700 dark:text-gray-200">{row.kubernetesInfo?.context}</span></div>
                                                                        </Show>
                                                                        <Show when={row.kubernetesInfo?.tokenName}>
                                                                            <div>Token: <span class="text-gray-700 dark:text-gray-200">{row.kubernetesInfo?.tokenName}</span></div>
                                                                        </Show>
                                                                    </div>
                                                                </Show>
                                                                <Show when={row.scope.category !== 'na'}>
                                                                    <div class="text-xs text-gray-500 dark:text-gray-400">
                                                                        Scope profile: <span class="text-gray-700 dark:text-gray-200">{row.scope.label}</span>
                                                                        <Show when={row.scope.detail}>
                                                                            <span class="ml-1 text-gray-400 dark:text-gray-500">{row.scope.detail}</span>
                                                                        </Show>
                                                                    </div>
                                                                    <Show when={assignment()}>
                                                                        <div class="text-[11px] text-amber-600 dark:text-amber-400">
                                                                            Restart required to apply scope changes.
                                                                        </div>
                                                                        <button
                                                                            type="button"
                                                                            onClick={() => handleResetScope(resolvedAgentId, agentName || resolvedAgentId)}
                                                                            class="text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 text-left"
                                                                        >
                                                                            Reset to default
                                                                        </button>
                                                                    </Show>
                                                                </Show>
                                                            </div>
                                                            <div class="space-y-2 md:justify-self-end">
                                                                <div class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">
                                                                    Utilities
                                                                </div>
                                                                <div class="flex flex-col gap-2">
                                                                    <Show when={row.status === 'active' && !isKubernetes()}>
                                                                        <button
                                                                            type="button"
                                                                            onClick={async () => {
                                                                                const cmd = getUninstallCommand();
                                                                                const success = await copyToClipboard(cmd);
                                                                                if (success) {
                                                                                    notificationStore.success('Uninstall command copied');
                                                                                } else {
                                                                                    notificationStore.error('Failed to copy');
                                                                                }
                                                                            }}
                                                                            class="text-xs text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-gray-100 text-left"
                                                                        >
                                                                            Copy uninstall command
                                                                        </button>
                                                                    </Show>
                                                                    <Show when={row.isLegacy}>
                                                                        <button
                                                                            type="button"
                                                                            onClick={async () => {
                                                                                const success = await copyToClipboard(getUpgradeCommand(row.hostname || ''));
                                                                                if (success) {
                                                                                    notificationStore.success('Upgrade command copied');
                                                                                } else {
                                                                                    notificationStore.error('Failed to copy');
                                                                                }
                                                                            }}
                                                                            class="text-xs text-amber-700 hover:text-amber-900 dark:text-amber-300 dark:hover:text-amber-200 text-left"
                                                                        >
                                                                            Copy upgrade command
                                                                        </button>
                                                                    </Show>
                                                                </div>
                                                            </div>
                                                        </div>
                                                        <Show when={row.status === 'active' && row.types.includes('docker')}>
                                                            {(() => {
                                                                const containers = () => (state.dockerHosts || []).find(h => h.id === row.id)?.containers || [];
                                                                return (
                                                                    <Show when={containers().length > 0}>
                                                                        <div class="mt-4 rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-900/30">
                                                                            <div class="mb-2 text-xs font-medium text-gray-700 dark:text-gray-300">
                                                                                Container Custom URLs
                                                                            </div>
                                                                            <div class="space-y-3">
                                                                                <For each={containers()}>
                                                                                    {(container) => {
                                                                                        const cid = container.id;
                                                                                        return (
                                                                                            <div>
                                                                                                <div class="mb-1 text-xs text-gray-500 dark:text-gray-400">
                                                                                                    <span class="font-medium text-gray-700 dark:text-gray-300">{container.name || cid}</span>
                                                                                                    <span class="ml-1.5 inline-flex items-center rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-400">
                                                                                                        {container.state}
                                                                                                    </span>
                                                                                                </div>
                                                                                                <div class="flex gap-2">
                                                                                                    <input
                                                                                                        type="text"
                                                                                                        value={containerUrls()[cid] || ''}
                                                                                                        onInput={(e) => setContainerUrls(prev => ({ ...prev, [cid]: e.currentTarget.value }))}
                                                                                                        onKeyDown={(e) => {
                                                                                                            if (e.key === 'Enter' && !containerUrlSaving()[cid]) {
                                                                                                                void saveContainerUrl(row.id, cid);
                                                                                                            }
                                                                                                        }}
                                                                                                        placeholder="https://container.example.com"
                                                                                                        class="flex-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                                                                                                    />
                                                                                                    <button
                                                                                                        type="button"
                                                                                                        onClick={() => void saveContainerUrl(row.id, cid)}
                                                                                                        disabled={Boolean(containerUrlSaving()[cid])}
                                                                                                        class="inline-flex items-center justify-center rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                                                                                                    >
                                                                                                        {containerUrlSaving()[cid] ? 'Saving…' : 'Save'}
                                                                                                    </button>
                                                                                                </div>
                                                                                            </div>
                                                                                        );
                                                                                    }}
                                                                                </For>
                                                                            </div>
                                                                            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                                                                                Optional URL override used when Pulse links to this container (leave blank to use the default).
                                                                            </p>
                                                                        </div>
                                                                    </Show>
                                                                );
                                                            })()}
                                                        </Show>
                                                    </td>
                                                </tr>
                                            </Show>
                                        </>
                                    );
                                }}
                            </For>
                        </tbody>
                    </table>
                </Card>
            </SettingsPanel>
        </div >
    );
};
