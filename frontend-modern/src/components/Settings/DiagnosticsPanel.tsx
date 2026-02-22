import { Component, Show, For, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { showSuccess, showError } from '@/utils/toast';
import { formatRelativeTime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import Activity from 'lucide-solid/icons/activity';
import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Database from 'lucide-solid/icons/database';
import Network from 'lucide-solid/icons/network';
import Shield from 'lucide-solid/icons/shield';
import Cpu from 'lucide-solid/icons/cpu';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Download from 'lucide-solid/icons/download';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import Sparkles from 'lucide-solid/icons/sparkles';

// Type definitions
interface DiagnosticsNode {
    id: string;
    name: string;
    host: string;
    type: string;
    authMethod: string;
    connected: boolean;
    error?: string;
    details?: Record<string, unknown>;
    lastPoll?: string;
    clusterInfo?: Record<string, unknown>;
}

interface DiagnosticsPBS {
    id: string;
    name: string;
    host: string;
    connected: boolean;
    error?: string;
    details?: Record<string, unknown>;
}

interface SystemDiagnostic {
    os: string;
    arch: string;
    goVersion: string;
    numCPU: number;
    numGoroutine: number;
    memoryMB: number;
}

interface DiscoveryDiagnostic {
    enabled: boolean;
    configuredSubnet?: string;
    activeSubnet?: string;
    environmentOverride?: string;
    subnetAllowlist?: string[];
    subnetBlocklist?: string[];
    scanning?: boolean;
    scanInterval?: string;
    lastScanStartedAt?: string;
    lastResultTimestamp?: string;
    lastResultServers?: number;
    lastResultErrors?: number;
}

interface APITokenDiagnostic {
    enabled: boolean;
    tokenCount: number;
    hasEnvTokens: boolean;
    hasLegacyToken: boolean;
    recommendTokenSetup: boolean;
    recommendTokenRotation: boolean;
    legacyDockerHostCount?: number;
    unusedTokenCount?: number;
    notes?: string[];
}

interface DockerAgentDiagnostic {
    hostsTotal: number;
    hostsOnline: number;
    hostsReportingVersion: number;
    hostsWithTokenBinding: number;
    hostsWithoutTokenBinding: number;
    hostsWithoutVersion?: number;
    hostsOutdatedVersion?: number;
    hostsWithStaleCommand?: number;
    hostsPendingUninstall?: number;
    hostsNeedingAttention: number;
    recommendedAgentVersion?: string;
    notes?: string[];
}

interface AlertsDiagnostic {
    legacyThresholdsDetected: boolean;
    legacyThresholdSources?: string[];
    legacyScheduleSettings?: string[];
    missingCooldown: boolean;
    missingGroupingWindow: boolean;
    notes?: string[];
}

interface MetricsStoreDiagnostic {
    enabled: boolean;
    status: 'healthy' | 'buffering' | 'empty' | 'unavailable';
    dbSize?: number;
    rawCount?: number;
    minuteCount?: number;
    hourlyCount?: number;
    dailyCount?: number;
    totalPoints?: number;
    bufferSize?: number;
    notes?: string[];
    error?: string;
}

interface AIChatDiagnostic {
    enabled: boolean;
    running: boolean;
    healthy: boolean;
    port?: number;
    url?: string;
    model?: string;
    mcpConnected: boolean;
    mcpToolCount?: number;
    notes?: string[];
}

interface DiagnosticsData {
    version: string;
    runtime: string;
    uptime: number;
    nodes: DiagnosticsNode[];
    pbs: DiagnosticsPBS[];
    system: SystemDiagnostic;
    metricsStore?: MetricsStoreDiagnostic | null;
    apiTokens?: APITokenDiagnostic | null;
    dockerAgents?: DockerAgentDiagnostic | null;
    alerts?: AlertsDiagnostic | null;
    aiChat?: AIChatDiagnostic | null;
    discovery?: DiscoveryDiagnostic | null;
    errors: string[];
}

// Utility functions
function formatUptime(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours < 24) return `${hours}h ${minutes}m`;
    const days = Math.floor(hours / 24);
    return `${days}d ${hours % 24}h`;
}


import type { JSX } from 'solid-js';

// ...

// DiagnosticCard - a mini card component for individual diagnostic items
const DiagnosticCard: Component<{
    title: string;
    icon: Component<{ class?: string }>;
    status?: 'success' | 'warning' | 'error' | 'info';
    children: JSX.Element;
}> = (props) => {
    const statusColors = {
        success: 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900',
        warning: 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900',
        error: 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900',
        info: 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900',
    };

    const iconColors = {
        success: 'text-green-600 dark:text-green-400',
        warning: 'text-amber-600 dark:text-amber-400',
        error: 'text-red-600 dark:text-red-400',
        info: 'text-blue-600 dark:text-blue-400',
    };

    return (
        <div class={`rounded-md border p-4 transition-all hover:shadow-sm ${statusColors[props.status || 'info']}`}>
            <div class="flex items-center gap-3 mb-3">
                <div class={`p-2 rounded-md bg-surface ${iconColors[props.status || 'info']}`}>
                    <props.icon class="w-4 h-4" />
                </div>
                <h4 class="text-sm font-semibold text-base-content">{props.title}</h4>
            </div>
            <div class="text-xs text-muted space-y-1.5">
                {props.children}
            </div>
        </div>
    );
};

// StatusBadge component
const StatusBadge: Component<{
    status: 'online' | 'offline' | 'warning' | 'unknown';
    label?: string;
}> = (props) => {
    const colors = {
        online: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
        offline: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
        warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
        unknown: 'bg-surface-alt text-base-content',
    };

    return (
        <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium uppercase tracking-wide ${colors[props.status]}`}>
            <span class={`w-1.5 h-1.5 rounded-full ${props.status === 'online' ? 'bg-emerald-400' : props.status === 'offline' ? 'bg-rose-400' : props.status === 'warning' ? 'bg-amber-400' : 'bg-slate-400'}`} />
            {props.label || props.status}
        </span>
    );
};

// MetricRow component for displaying key-value pairs
const MetricRow: Component<{
    label: string;
    value: string | number | undefined;
    mono?: boolean;
}> = (props) => (
    <div class="flex items-center justify-between py-1.5 border-b border-border-subtle last:border-0">
        <span class="text-muted">{props.label}</span>
        <span class={`text-base-content ${props.mono ? 'font-mono text-[11px]' : 'font-medium'}`}>
            {props.value ?? 'Unknown'}
        </span>
    </div>
);

export const DiagnosticsPanel: Component = () => {
    const [loading, setLoading] = createSignal(false);
    const [diagnosticsData, setDiagnosticsData] = createSignal<DiagnosticsData | null>(null);
    const [exportLoading, setExportLoading] = createSignal(false);

    const runDiagnostics = async () => {
        setLoading(true);
        try {
            const data = await apiFetchJSON('/api/diagnostics') as DiagnosticsData;
            setDiagnosticsData(data);
            showSuccess('Diagnostics completed');
        } catch (error) {
            showError(error instanceof Error ? error.message : 'Failed to run diagnostics');
        } finally {
            setLoading(false);
        }
    };

    // sanitizeDiagnostics redacts IPs, hostnames, subnets, and token hints
    // so the export is safe to attach to a public GitHub issue.
    const sanitizeDiagnostics = (raw: DiagnosticsData): DiagnosticsData => {
        const data: DiagnosticsData = JSON.parse(JSON.stringify(raw));

        // Regex matching IPv4 addresses, IPv6 addresses, hostnames with dots, and CIDR notation
        const ipv4Re = /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(\/\d{1,2})?\b/g;

        const redactString = (s: string): string => s.replace(ipv4Re, '[REDACTED_IP]');

        // Redact node hosts
        if (Array.isArray(data.nodes)) {
            data.nodes = data.nodes.map((node, i) => ({
                ...node,
                host: `node-${i + 1}`,
                name: `node-${i + 1}`,
                id: `node-${i + 1}`,
                error: node.error ? redactString(node.error) : undefined,
            }));
        }

        // Redact PBS hosts
        if (Array.isArray(data.pbs)) {
            data.pbs = data.pbs.map((p, i) => ({
                ...p,
                host: `pbs-${i + 1}`,
                name: `pbs-${i + 1}`,
                id: `pbs-${i + 1}`,
                error: p.error ? redactString(p.error) : undefined,
            }));
        }

        // Redact discovery subnets
        if (data.discovery) {
            data.discovery = {
                ...data.discovery,
                configuredSubnet: data.discovery.configuredSubnet ? '[REDACTED_SUBNET]' : undefined,
                activeSubnet: data.discovery.activeSubnet ? '[REDACTED_SUBNET]' : undefined,
                environmentOverride: data.discovery.environmentOverride ? '[REDACTED]' : undefined,
                subnetAllowlist: data.discovery.subnetAllowlist?.map(() => '[REDACTED_SUBNET]'),
                subnetBlocklist: data.discovery.subnetBlocklist?.map(() => '[REDACTED_SUBNET]'),
            };
            // Redact IPs in discovery history (field exists in backend but not in TS interface)
            const disc = data.discovery as any;
            if (Array.isArray(disc.history)) {
                disc.history = disc.history.map((h: Record<string, unknown>) => ({
                    ...h,
                    subnet: '[REDACTED_SUBNET]',
                }));
            }
        }

        // Redact API token details (tokens/usage arrays exist in backend but not in TS interface)
        if (data.apiTokens) {
            const tokens = data.apiTokens as any;
            if (Array.isArray(tokens.tokens)) {
                tokens.tokens = tokens.tokens.map((t: Record<string, unknown>, i: number) => ({
                    ...t,
                    hint: '[REDACTED]',
                    id: `token-${i + 1}`,
                    name: `token-${i + 1}`,
                }));
            }
            if (Array.isArray(tokens.usage)) {
                tokens.usage = tokens.usage.map((u: Record<string, unknown>) => ({
                    ...u,
                    hosts: undefined,
                }));
            }
        }

        // Redact Docker agent identifiers
        if (data.dockerAgents) {
            const agents = data.dockerAgents as any;
            if (Array.isArray(agents.attention)) {
                agents.attention = agents.attention.map((a: Record<string, unknown>, i: number) => ({
                    ...a,
                    hostId: `docker-host-${i + 1}`,
                    name: `docker-host-${i + 1}`,
                    tokenHint: a.tokenHint ? '[REDACTED]' : undefined,
                }));
            }
        }

        // Redact AI chat URL
        if (data.aiChat?.url) {
            data.aiChat.url = '[REDACTED]';
        }

        // Redact IPs in error messages
        if (Array.isArray(data.errors)) {
            data.errors = data.errors.map(redactString);
        }

        // Redact IPs from any raw snapshot data that may be present
        const raw2 = data as any;
        if (Array.isArray(raw2.nodeSnapshots)) {
            raw2.nodeSnapshots = raw2.nodeSnapshots.map((s: Record<string, unknown>, i: number) => ({
                ...s,
                instance: `node-${i + 1}`,
            }));
        }
        if (Array.isArray(raw2.guestSnapshots)) {
            raw2.guestSnapshots = raw2.guestSnapshots.map((s: Record<string, unknown>, i: number) => ({
                ...s,
                instance: `node-${i + 1}`,
            }));
        }
        if (Array.isArray(raw2.memorySources)) {
            raw2.memorySources = raw2.memorySources.map((s: Record<string, unknown>, i: number) => ({
                ...s,
                instance: `node-${i + 1}`,
            }));
        }

        return data;
    };

    const exportDiagnostics = async (sanitize: boolean) => {
        setExportLoading(true);
        try {
            const data = diagnosticsData();
            if (!data) {
                showError('Run diagnostics first');
                return;
            }

            const exportData = sanitize ? sanitizeDiagnostics(data) : data;
            const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            const type = sanitize ? 'sanitized' : 'full';
            a.download = `pulse-diagnostics-${type}-${new Date().toISOString().split('T')[0]}.json`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            showSuccess(`Diagnostics exported (${type})`);
        } finally {
            setExportLoading(false);
        }
    };

    // Calculate overall system health
    const systemHealth = () => {
        const data = diagnosticsData();
        if (!data) return 'unknown';

        const issues: string[] = [];

        // Check node connectivity
        const disconnectedNodes = data.nodes?.filter(n => !n.connected).length || 0;
        if (disconnectedNodes > 0) issues.push('nodes');

        // Check PBS connectivity  
        const disconnectedPbs = data.pbs?.filter(p => !p.connected).length || 0;
        if (disconnectedPbs > 0) issues.push('pbs');

        // Check for errors
        if (data.errors?.length > 0) issues.push('errors');

        // Check metrics store health
        if (data.metricsStore && data.metricsStore.status === 'unavailable') issues.push('metrics');

        // Check alerts config
        if (data.alerts?.legacyThresholdsDetected || data.alerts?.missingCooldown) issues.push('alerts');

        if (issues.length === 0) return 'healthy';
        if (issues.length <= 2) return 'warning';
        return 'critical';
    };

    const healthTone = () => {
        const health = systemHealth();
        if (health === 'healthy') {
            return {
                headerBg: 'bg-emerald-50 dark:bg-emerald-950',
                headerBorder: 'border-b border-emerald-200 dark:border-emerald-800',
                iconWrap: 'bg-emerald-100 dark:bg-emerald-900',
                icon: 'text-emerald-700 dark:text-emerald-300',
                subtitle: 'text-emerald-700 dark:text-emerald-300',
                meta: 'text-emerald-700 dark:text-emerald-300',
                button:
                    'border border-emerald-300 dark:border-emerald-700 bg-emerald-100 dark:bg-emerald-900 text-emerald-800 dark:text-emerald-100 hover:bg-emerald-200 dark:hover:bg-emerald-900',
            };
        }
        if (health === 'warning') {
            return {
                headerBg: 'bg-amber-50 dark:bg-amber-950',
                headerBorder: 'border-b border-amber-200 dark:border-amber-800',
                iconWrap: 'bg-amber-100 dark:bg-amber-900',
                icon: 'text-amber-700 dark:text-amber-300',
                subtitle: 'text-amber-700 dark:text-amber-300',
                meta: 'text-amber-700 dark:text-amber-300',
                button:
                    'border border-amber-300 dark:border-amber-700 bg-amber-100 dark:bg-amber-900 text-amber-800 dark:text-amber-100 hover:bg-amber-200 dark:hover:bg-amber-900',
            };
        }
        if (health === 'critical') {
            return {
                headerBg: 'bg-rose-50 dark:bg-rose-950',
                headerBorder: 'border-b border-rose-200 dark:border-rose-800',
                iconWrap: 'bg-rose-100 dark:bg-rose-900',
                icon: 'text-rose-700 dark:text-rose-300',
                subtitle: 'text-rose-700 dark:text-rose-300',
                meta: 'text-rose-700 dark:text-rose-300',
                button:
                    'border border-rose-300 dark:border-rose-700 bg-rose-100 dark:bg-rose-900 text-rose-800 dark:text-rose-100 hover:bg-rose-200 dark:hover:bg-rose-900',
            };
        }
        return {
            headerBg: 'bg-surface-alt',
            headerBorder: 'border-b border-border',
            iconWrap: 'bg-surface-hover',
            icon: 'text-base-content',
            subtitle: 'text-muted',
            meta: 'text-muted',
            button:
                'border border-border bg-surface-hover text-base-content hover:bg-surface-hover',
        };
    };

    return (
        <div class="space-y-6">
            {/* Header Card */}
            <Card
                padding="none"
                class="overflow-hidden border border-border"
                border={false}
            >
                <div class={`px-4 sm:px-6 py-4 sm:py-5 ${healthTone().headerBg} ${healthTone().headerBorder}`}>
                    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                        <div class="flex items-center gap-3 sm:gap-4">
                            <div class={`p-2 sm:p-3 rounded-md flex-shrink-0 ${healthTone().iconWrap}`}>
                                <Activity class={`w-5 h-5 sm:w-6 sm:h-6 ${healthTone().icon}`} />
                            </div>
                            <div class="min-w-0">
                                <h2 class="text-base sm:text-lg font-semibold text-base-content">System Diagnostics</h2>
                                <p class={`text-xs sm:text-sm hidden sm:block ${healthTone().subtitle}`}>
                                    Connection health, configuration status, and troubleshooting tools
                                </p>
                            </div>
                        </div>
                        <div class="flex items-center justify-between sm:justify-end gap-3 flex-wrap">
                            <Show when={diagnosticsData()}>
                                <div class={`text-left sm:text-right text-xs ${healthTone().meta}`}>
                                    <div>Version {diagnosticsData()?.version}</div>
                                    <div>Uptime: {formatUptime(diagnosticsData()?.uptime || 0)}</div>
                                </div>
                            </Show>
                            <button
                                type="button"
                                onClick={runDiagnostics}
                                disabled={loading()}
                                class={`flex min-h-10 sm:min-h-9 min-w-10 items-center gap-2 px-3 sm:px-4 py-2.5 rounded-md font-medium text-sm transition-colors disabled:opacity-50 whitespace-nowrap ${healthTone().button}`}
                            >
                                <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
                                <span class="sm:hidden">{loading() ? '...' : 'Run'}</span>
                                <span class="hidden sm:inline">{loading() ? 'Running...' : 'Run Diagnostics'}</span>
                            </button>
                        </div>
                    </div>
                </div>

                {/* Quick Actions */}
                <div class="px-4 sm:px-6 py-3 sm:py-4 bg-surface-alt border-t border-border flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                    <p class="text-xs text-muted">
                        Test all connections and inspect runtime configuration
                    </p>
                    <Show when={diagnosticsData()}>
                        <div class="flex items-center gap-2 flex-wrap">
                            <button
                                type="button"
                                onClick={() => exportDiagnostics(false)}
                                disabled={exportLoading()}
                                class="flex min-h-10 sm:min-h-9 items-center gap-1.5 px-3 py-2 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover transition-colors"
                            >
                                <Download class="w-3.5 h-3.5" />
                                Full
                            </button>
                            <button
                                type="button"
                                onClick={() => exportDiagnostics(true)}
                                disabled={exportLoading()}
                                class="flex min-h-10 sm:min-h-9 items-center gap-1.5 px-3 py-2 text-sm font-medium text-green-700 dark:text-green-300 bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded-md hover:bg-green-100 dark:hover:bg-green-900 transition-colors"
                            >
                                <Download class="w-3.5 h-3.5" />
                                GitHub
                            </button>
                        </div>
                    </Show>
                </div>
            </Card>

            {/* Diagnostics Content */}
            <Show when={diagnosticsData()} fallback={
                <Card padding="lg" class="text-center">
                    <div class="py-12">
                        <Activity class="w-12 h-12 mx-auto text-muted mb-4" />
                        <h3 class="text-lg font-medium text-base-content mb-2">
                            No diagnostics data yet
                        </h3>
                        <p class="text-sm text-muted mb-6">
                            Click "Run Diagnostics" above to test connections and inspect system status
                        </p>
                        <button
                            type="button"
                            onClick={runDiagnostics}
                            disabled={loading()}
                            class="inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-5 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-md font-medium text-sm transition-colors disabled:opacity-50"
                        >
                            <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
                            Run Diagnostics
                        </button>
                    </div>
                </Card>
            }>
                {/* Summary Grid */}
                <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
                    {/* System Info Card */}
                    <DiagnosticCard
                        title="System Runtime"
                        icon={Cpu}
                        status="info"
                    >
                        <MetricRow label="OS / Arch" value={`${diagnosticsData()?.system?.os || '?'} / ${diagnosticsData()?.system?.arch || '?'}`} />
                        <MetricRow label="Go Runtime" value={diagnosticsData()?.system?.goVersion} mono />
                        <MetricRow label="CPU Cores" value={diagnosticsData()?.system?.numCPU} />
                        <MetricRow label="Goroutines" value={diagnosticsData()?.system?.numGoroutine} />
                        <MetricRow label="Memory" value={`${diagnosticsData()?.system?.memoryMB || 0} MB`} />
                    </DiagnosticCard>

                    {/* Nodes Status Card */}
                    <DiagnosticCard
                        title="PVE Nodes"
                        icon={Server}
                        status={diagnosticsData()?.nodes?.every(n => n.connected) ? 'success' : 'warning'}
                    >
                        <div class="flex items-center justify-between mb-2">
                            <span>Total Nodes</span>
                            <span class="font-bold text-lg text-base-content">
                                {diagnosticsData()?.nodes?.length || 0}
                            </span>
                        </div>
                        <div class="space-y-1">
                            <For each={diagnosticsData()?.nodes || []}>
                                {(node) => (
                                    <div class="flex items-center justify-between py-1 border-b border-border-subtle last:border-0">
                                        <span class="truncate max-w-[120px]" title={node.host}>{node.name}</span>
                                        <StatusBadge status={node.connected ? 'online' : 'offline'} />
                                    </div>
                                )}
                            </For>
                        </div>
                    </DiagnosticCard>

                    {/* PBS Status Card */}
                    <DiagnosticCard
                        title="PBS Instances"
                        icon={HardDrive}
                        status={diagnosticsData()?.pbs?.every(p => p.connected) ? 'success' : (diagnosticsData()?.pbs?.length ? 'warning' : 'info')}
                    >
                        <Show when={(diagnosticsData()?.pbs?.length || 0) > 0} fallback={
                            <div class="text-center py-4 text-muted">
                                No PBS configured
                            </div>
                        }>
                            <div class="flex items-center justify-between mb-2">
                                <span>Total Instances</span>
                                <span class="font-bold text-lg text-base-content">
                                    {diagnosticsData()?.pbs?.length || 0}
                                </span>
                            </div>
                            <div class="space-y-1">
                                <For each={diagnosticsData()?.pbs || []}>
                                    {(pbs) => (
                                        <div class="flex items-center justify-between py-1 border-b border-border-subtle last:border-0">
                                            <span class="truncate max-w-[120px]" title={pbs.host}>{pbs.name}</span>
                                            <StatusBadge status={pbs.connected ? 'online' : 'offline'} />
                                        </div>
                                    )}
                                </For>
                            </div>
                        </Show>
                    </DiagnosticCard>

                    {/* Discovery Status Card */}
                    <DiagnosticCard
                        title="Network Discovery"
                        icon={Network}
                        status={diagnosticsData()?.discovery?.enabled ? 'success' : 'info'}
                    >
                        <MetricRow
                            label="Status"
                            value={diagnosticsData()?.discovery?.enabled ? 'Enabled' : 'Disabled'}
                        />
                        <MetricRow
                            label="Subnet"
                            value={diagnosticsData()?.discovery?.configuredSubnet || 'auto'}
                            mono
                        />
                        <MetricRow
                            label="Scan Interval"
                            value={diagnosticsData()?.discovery?.scanInterval || 'default'}
                        />
                        <MetricRow
                            label="Last Scan"
                            value={formatRelativeTime(diagnosticsData()?.discovery?.lastScanStartedAt, { compact: true, emptyText: 'Never' })}
                        />
                        <MetricRow
                            label="Servers Found"
                            value={diagnosticsData()?.discovery?.lastResultServers ?? 0}
                        />
                    </DiagnosticCard>
                </div>

                {/* Detailed Status Cards */}
                <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    {/* Metrics Store */}
                    <Show when={diagnosticsData()?.metricsStore}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-border">
                                <div class="p-2 rounded-md bg-blue-100 dark:bg-blue-900">
                                    <Database class="w-4 h-4 text-blue-600 dark:text-blue-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-base-content">Metrics Store</h4>
                                    <p class="text-xs text-muted">History persistence health</p>
                                </div>
                                <div class="ml-auto">
                                    <StatusBadge
                                        status={
                                            diagnosticsData()?.metricsStore?.status === 'healthy'
                                                ? 'online'
                                                : diagnosticsData()?.metricsStore?.status === 'buffering'
                                                    ? 'warning'
                                                    : diagnosticsData()?.metricsStore?.status === 'empty'
                                                        ? 'warning'
                                                        : 'offline'
                                        }
                                        label={diagnosticsData()?.metricsStore?.status || 'unknown'}
                                    />
                                </div>
                            </div>
                            <div class="space-y-2 text-xs">
                                <MetricRow label="Enabled" value={diagnosticsData()?.metricsStore?.enabled ? 'Yes' : 'No'} />
                                <MetricRow label="DB Size" value={`${Math.round((diagnosticsData()?.metricsStore?.dbSize ?? 0) / (1024 * 1024))} MB`} />
                                <MetricRow label="Total Points" value={diagnosticsData()?.metricsStore?.totalPoints ?? 0} />
                                <MetricRow label="Raw Points" value={diagnosticsData()?.metricsStore?.rawCount ?? 0} />
                                <MetricRow label="Minute Points" value={diagnosticsData()?.metricsStore?.minuteCount ?? 0} />
                                <MetricRow label="Hourly Points" value={diagnosticsData()?.metricsStore?.hourlyCount ?? 0} />
                                <MetricRow label="Daily Points" value={diagnosticsData()?.metricsStore?.dailyCount ?? 0} />
                                <MetricRow label="Buffer Size" value={diagnosticsData()?.metricsStore?.bufferSize ?? 0} />
                            </div>
                            <Show when={(diagnosticsData()?.metricsStore?.notes?.length || 0) > 0}>
                                <div class="mt-3 rounded-md bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 p-2">
                                    <div class="flex items-start gap-2 text-xs text-amber-700 dark:text-amber-300">
                                        <AlertTriangle class="w-4 h-4 flex-shrink-0 mt-0.5" />
                                        <div class="space-y-1">
                                            <For each={diagnosticsData()?.metricsStore?.notes || []}>
                                                {(note) => <div>{note}</div>}
                                            </For>
                                        </div>
                                    </div>
                                </div>
                            </Show>
                            <Show when={diagnosticsData()?.metricsStore?.error}>
                                <div class="mt-3 rounded-md bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 p-2 text-xs text-red-700 dark:text-red-300">
                                    {diagnosticsData()?.metricsStore?.error}
                                </div>
                            </Show>
                        </Card>
                    </Show>

                    {/* API Tokens */}
                    <Show when={diagnosticsData()?.apiTokens}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-border">
                                <div class="p-2 rounded-md bg-blue-100 dark:bg-blue-900">
                                    <Shield class="w-4 h-4 text-blue-600 dark:text-blue-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-base-content">API Tokens</h4>
                                    <p class="text-xs text-muted">Authentication status</p>
                                </div>
                                <div class="ml-auto">
                                    <StatusBadge
                                        status={diagnosticsData()?.apiTokens?.enabled ? 'online' : 'warning'}
                                        label={diagnosticsData()?.apiTokens?.enabled ? 'Enabled' : 'Disabled'}
 />
 </div>
 </div>
 <div class="space-y-2 text-xs">
 <MetricRow label="Configured Tokens" value={diagnosticsData()?.apiTokens?.tokenCount} />
 <MetricRow label="Unused Tokens" value={diagnosticsData()?.apiTokens?.unusedTokenCount ?? 0} />
 <MetricRow label="Legacy Docker Hosts" value={diagnosticsData()?.apiTokens?.legacyDockerHostCount ?? 0} />
 </div>
 <Show when={diagnosticsData()?.apiTokens?.hasLegacyToken}>
 <div class="mt-3 p-2 bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded text-xs text-amber-700 dark:text-amber-300">
 Legacy token detected. Consider migrating to scoped tokens.
 </div>
 </Show>
 </Card>
 </Show>

 {/* Docker Agents */}
 <Show when={diagnosticsData()?.dockerAgents}>
 <Card padding="md">
 <div class="flex items-center gap-3 mb-4 pb-3 border-b border-border">
 <div class="p-2 rounded-md bg-blue-100 dark:bg-blue-900">
 <Database class="w-4 h-4 text-blue-600 dark:text-blue-400" />
 </div>
 <div>
 <h4 class="text-sm font-semibold text-base-content">Docker Agents</h4>
 <p class="text-xs text-muted">Container monitoring</p>
 </div>
 <div class="ml-auto text-right">
 <div class="text-lg font-bold text-base-content">
 {diagnosticsData()?.dockerAgents?.hostsOnline}/{diagnosticsData()?.dockerAgents?.hostsTotal}
 </div>
 <div class="text-[10px] text-muted">online</div>
 </div>
 </div>
 <div class="space-y-2 text-xs">
 <MetricRow label="With Token Binding" value={diagnosticsData()?.dockerAgents?.hostsWithTokenBinding} />
 <MetricRow label="Need Attention" value={diagnosticsData()?.dockerAgents?.hostsNeedingAttention} />
 <MetricRow label="Outdated Version" value={diagnosticsData()?.dockerAgents?.hostsOutdatedVersion ?? 0} />
 </div>
 <Show when={diagnosticsData()?.dockerAgents?.recommendedAgentVersion}>
 <div class="mt-3 pt-2 border-t border-border-subtle text-xs text-muted">
 Recommended version: {diagnosticsData()?.dockerAgents?.recommendedAgentVersion}
 </div>
 </Show>
 </Card>
 </Show>

 {/* Alerts Configuration */}
 <Show when={diagnosticsData()?.alerts}>
 <Card padding="md">
 <div class="flex items-center gap-3 mb-4 pb-3 border-b border-border">
 <div class="p-2 rounded-md bg-rose-100 dark:bg-rose-900">
 <AlertTriangle class="w-4 h-4 text-rose-600 dark:text-rose-400" />
 </div>
 <div>
 <h4 class="text-sm font-semibold text-base-content">Alerts Configuration</h4>
 <p class="text-xs text-muted">Alert system status</p>
 </div>
 </div>
 <div class="flex flex-wrap gap-2">
 <span class={`px-2 py-1 rounded text-xs font-medium ${diagnosticsData()?.alerts?.legacyThresholdsDetected
 ?'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
 : 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
 }`}>
                                    Legacy thresholds: {diagnosticsData()?.alerts?.legacyThresholdsDetected ? 'Detected' : 'Migrated'}
                                </span>
                                <span class={`px-2 py-1 rounded text-xs font-medium ${diagnosticsData()?.alerts?.missingCooldown
 ? 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
 : 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
 }`}>
                                    Cooldown: {diagnosticsData()?.alerts?.missingCooldown ? 'Missing' : 'Configured'}
                                </span>
                                <span class={`px-2 py-1 rounded text-xs font-medium ${diagnosticsData()?.alerts?.missingGroupingWindow
 ? 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
 : 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
 }`}>
                                    Grouping: {diagnosticsData()?.alerts?.missingGroupingWindow ? 'Disabled' : 'Enabled'}
 </span>
 </div>
 <Show when={(diagnosticsData()?.alerts?.notes?.length || 0) > 0}>
 <ul class="mt-3 pt-2 border-t border-border-subtle space-y-1 text-xs text-muted list-disc pl-4">
 <For each={diagnosticsData()?.alerts?.notes || []}>
 {(note) => <li>{note}</li>}
 </For>
 </ul>
 </Show>
 </Card>
 </Show>

 {/* AI Chat Status */}
 <Show when={diagnosticsData()?.aiChat}>
 <Card padding="md">
 <div class="flex items-center gap-3 mb-4 pb-3 border-b border-border">
 <div class="p-2 rounded-md bg-blue-100 dark:bg-blue-900">
 <Sparkles class="w-4 h-4 text-blue-600 dark:text-blue-400" />
 </div>
 <div>
 <h4 class="text-sm font-semibold text-base-content">Pulse Assistant</h4>
 <p class="text-xs text-muted">Pulse Assistant Service</p>
 </div>
 <div class="ml-auto">
 <StatusBadge
 status={diagnosticsData()?.aiChat?.running ?'online' : (diagnosticsData()?.aiChat?.enabled ? 'offline' : 'unknown')}
                                        label={diagnosticsData()?.aiChat?.running ? 'Running' : (diagnosticsData()?.aiChat?.enabled ? 'Stopped' : 'Disabled')}
                                    />
                                </div>
                            </div>
                            <div class="space-y-2 text-xs">
                                <MetricRow label="Model" value={diagnosticsData()?.aiChat?.model} />
                                <MetricRow label="Port" value={diagnosticsData()?.aiChat?.port} mono />
                                <MetricRow label="Status" value={diagnosticsData()?.aiChat?.healthy ? 'Healthy' : 'Unhealthy'} />
                            </div>
                            <div class="mt-3 pt-3 border-t border-border-subtle flex items-center justify-between text-xs">
                                <span class="text-muted">MCP Connection</span>
                                <div class="flex items-center gap-1.5">
                                    {diagnosticsData()?.aiChat?.mcpConnected ?
                                        <CheckCircle class="w-3.5 h-3.5 text-emerald-400" /> :
                                        <XCircle class="w-3.5 h-3.5 text-rose-400" />
                                    }
                                    <span class={diagnosticsData()?.aiChat?.mcpConnected ? 'text-green-700 dark:text-green-300' : 'text-slate-500'}>
                                        {diagnosticsData()?.aiChat?.mcpConnected ? 'Connected' : 'Disconnected'}
                                    </span>
                                </div>
                            </div>
                            <Show when={(diagnosticsData()?.aiChat?.notes?.length || 0) > 0}>
                                <ul class="mt-3 bg-amber-50 dark:bg-amber-900 p-2 rounded text-xs text-amber-700 dark:text-amber-400 list-disc pl-4">
                                    <For each={diagnosticsData()?.aiChat?.notes || []}>
                                        {(note) => <li>{note}</li>}
                                    </For>
                                </ul>
                            </Show>
                        </Card>
                    </Show>
                </div>

                {/* Errors Section */}
                <Show when={(diagnosticsData()?.errors?.length || 0) > 0}>
                    <Card padding="md" class="border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900">
                        <div class="flex items-center gap-3 mb-3">
                            <XCircle class="w-5 h-5 text-red-600 dark:text-red-400" />
                            <h4 class="text-sm font-semibold text-red-900 dark:text-red-100">Errors Detected</h4>
                        </div>
                        <ul class="space-y-2 text-xs text-red-700 dark:text-red-300">
                            <For each={diagnosticsData()?.errors || []}>
                                {(error) => (
                                    <li class="flex items-start gap-2 p-2 bg-red-100 dark:bg-red-900 rounded">
                                        <span class="text-rose-400">•</span>
                                        <span>{error}</span>
                                    </li>
                                )}
                            </For>
                        </ul>
                    </Card>
                </Show>
            </Show >
        </div >
    );
};

export default DiagnosticsPanel;
