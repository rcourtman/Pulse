import { Component, Show, For, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { showSuccess, showError } from '@/utils/toast';
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

interface TemperatureProxyDiagnostic {
    legacySSHDetected: boolean;
    recommendProxyUpgrade: boolean;
    socketFound: boolean;
    socketPath?: string;
    socketPermissions?: string;
    socketOwner?: string;
    socketGroup?: string;
    proxyReachable?: boolean;
    proxyVersion?: string;
    notes?: string[];
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

interface OpenCodeDiagnostic {
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
    temperatureProxy?: TemperatureProxyDiagnostic | null;
    apiTokens?: APITokenDiagnostic | null;
    dockerAgents?: DockerAgentDiagnostic | null;
    alerts?: AlertsDiagnostic | null;
    openCode?: OpenCodeDiagnostic | null;
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

function formatRelativeTime(timestamp?: string | number): string {
    if (!timestamp) return 'Never';
    const date = typeof timestamp === 'string' ? new Date(timestamp) : new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSec = Math.floor(diffMs / 1000);

    if (diffSec < 60) return 'just now';
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
    return `${Math.floor(diffSec / 86400)}d ago`;
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
        success: 'border-green-200 dark:border-green-800 bg-green-50/50 dark:bg-green-900/20',
        warning: 'border-amber-200 dark:border-amber-800 bg-amber-50/50 dark:bg-amber-900/20',
        error: 'border-red-200 dark:border-red-800 bg-red-50/50 dark:bg-red-900/20',
        info: 'border-blue-200 dark:border-blue-800 bg-blue-50/50 dark:bg-blue-900/20',
    };

    const iconColors = {
        success: 'text-green-600 dark:text-green-400',
        warning: 'text-amber-600 dark:text-amber-400',
        error: 'text-red-600 dark:text-red-400',
        info: 'text-blue-600 dark:text-blue-400',
    };

    return (
        <div class={`rounded-xl border p-4 transition-all hover:shadow-md ${statusColors[props.status || 'info']}`}>
            <div class="flex items-center gap-3 mb-3">
                <div class={`p-2 rounded-lg bg-white/60 dark:bg-gray-800/60 ${iconColors[props.status || 'info']}`}>
                    <props.icon class="w-4 h-4" />
                </div>
                <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{props.title}</h4>
            </div>
            <div class="text-xs text-gray-600 dark:text-gray-400 space-y-1.5">
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
        online: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
        offline: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
        warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
        unknown: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300',
    };

    return (
        <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium uppercase tracking-wide ${colors[props.status]}`}>
            <span class={`w-1.5 h-1.5 rounded-full ${props.status === 'online' ? 'bg-green-500' : props.status === 'offline' ? 'bg-red-500' : props.status === 'warning' ? 'bg-amber-500' : 'bg-gray-400'}`} />
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
    <div class="flex items-center justify-between py-1.5 border-b border-gray-100 dark:border-gray-700/50 last:border-0">
        <span class="text-gray-500 dark:text-gray-400">{props.label}</span>
        <span class={`text-gray-900 dark:text-gray-100 ${props.mono ? 'font-mono text-[11px]' : 'font-medium'}`}>
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

    const exportDiagnostics = async (sanitize: boolean) => {
        setExportLoading(true);
        try {
            const data = diagnosticsData();
            if (!data) {
                showError('Run diagnostics first');
                return;
            }

            const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
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

        // Check temperature proxy
        if (data.temperatureProxy?.legacySSHDetected) issues.push('temperature');

        // Check alerts config
        if (data.alerts?.legacyThresholdsDetected || data.alerts?.missingCooldown) issues.push('alerts');

        if (issues.length === 0) return 'healthy';
        if (issues.length <= 2) return 'warning';
        return 'critical';
    };

    const healthColor = () => {
        const health = systemHealth();
        if (health === 'healthy') return 'from-green-500 to-emerald-600';
        if (health === 'warning') return 'from-amber-500 to-orange-600';
        if (health === 'critical') return 'from-red-500 to-rose-600';
        return 'from-gray-400 to-gray-500';
    };

    return (
        <div class="space-y-6">
            {/* Header Card */}
            <Card
                padding="none"
                class="overflow-hidden border border-gray-200 dark:border-gray-700"
                border={false}
            >
                <div class={`bg-gradient-to-r ${healthColor()} px-4 sm:px-6 py-4 sm:py-5`}>
                    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                        <div class="flex items-center gap-3 sm:gap-4">
                            <div class="p-2 sm:p-3 bg-white/20 rounded-xl backdrop-blur-sm flex-shrink-0">
                                <Activity class="w-5 h-5 sm:w-6 sm:h-6 text-white" />
                            </div>
                            <div class="min-w-0">
                                <h2 class="text-base sm:text-lg font-bold text-white">System Diagnostics</h2>
                                <p class="text-xs sm:text-sm text-white/80 hidden sm:block">
                                    Connection health, configuration status, and troubleshooting tools
                                </p>
                            </div>
                        </div>
                        <div class="flex items-center justify-between sm:justify-end gap-3 flex-wrap">
                            <Show when={diagnosticsData()}>
                                <div class="text-left sm:text-right text-white/80 text-xs">
                                    <div>Version {diagnosticsData()?.version}</div>
                                    <div>Uptime: {formatUptime(diagnosticsData()?.uptime || 0)}</div>
                                </div>
                            </Show>
                            <button
                                type="button"
                                onClick={runDiagnostics}
                                disabled={loading()}
                                class="flex items-center gap-2 px-3 sm:px-4 py-2 sm:py-2.5 bg-white/20 hover:bg-white/30 text-white rounded-lg font-medium text-xs sm:text-sm transition-all disabled:opacity-50 backdrop-blur-sm whitespace-nowrap"
                            >
                                <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
                                <span class="sm:hidden">{loading() ? '...' : 'Run'}</span>
                                <span class="hidden sm:inline">{loading() ? 'Running...' : 'Run Diagnostics'}</span>
                            </button>
                        </div>
                    </div>
                </div>

                {/* Quick Actions */}
                <div class="px-4 sm:px-6 py-3 sm:py-4 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                        Test all connections and inspect runtime configuration
                    </p>
                    <Show when={diagnosticsData()}>
                        <div class="flex items-center gap-2 flex-wrap">
                            <button
                                type="button"
                                onClick={() => exportDiagnostics(false)}
                                disabled={exportLoading()}
                                class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                            >
                                <Download class="w-3.5 h-3.5" />
                                Full
                            </button>
                            <button
                                type="button"
                                onClick={() => exportDiagnostics(true)}
                                disabled={exportLoading()}
                                class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-green-700 dark:text-green-300 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-800 rounded-lg hover:bg-green-100 dark:hover:bg-green-900/50 transition-colors"
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
                        <Activity class="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-4" />
                        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
                            No diagnostics data yet
                        </h3>
                        <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">
                            Click "Run Diagnostics" above to test connections and inspect system status
                        </p>
                        <button
                            type="button"
                            onClick={runDiagnostics}
                            disabled={loading()}
                            class="inline-flex items-center gap-2 px-5 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium text-sm transition-colors disabled:opacity-50"
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
                            <span class="font-bold text-lg text-gray-900 dark:text-gray-100">
                                {diagnosticsData()?.nodes?.length || 0}
                            </span>
                        </div>
                        <div class="space-y-1">
                            <For each={diagnosticsData()?.nodes || []}>
                                {(node) => (
                                    <div class="flex items-center justify-between py-1 border-b border-gray-100 dark:border-gray-700/50 last:border-0">
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
                            <div class="text-center py-4 text-gray-400 dark:text-gray-500">
                                No PBS configured
                            </div>
                        }>
                            <div class="flex items-center justify-between mb-2">
                                <span>Total Instances</span>
                                <span class="font-bold text-lg text-gray-900 dark:text-gray-100">
                                    {diagnosticsData()?.pbs?.length || 0}
                                </span>
                            </div>
                            <div class="space-y-1">
                                <For each={diagnosticsData()?.pbs || []}>
                                    {(pbs) => (
                                        <div class="flex items-center justify-between py-1 border-b border-gray-100 dark:border-gray-700/50 last:border-0">
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
                            value={formatRelativeTime(diagnosticsData()?.discovery?.lastScanStartedAt)}
                        />
                        <MetricRow
                            label="Servers Found"
                            value={diagnosticsData()?.discovery?.lastResultServers ?? 0}
                        />
                    </DiagnosticCard>
                </div>

                {/* Detailed Status Cards */}
                <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    {/* Temperature Proxy */}
                    <Show when={diagnosticsData()?.temperatureProxy}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-gray-200 dark:border-gray-700">
                                <div class="p-2 rounded-lg bg-orange-100 dark:bg-orange-900/30">
                                    <Activity class="w-4 h-4 text-orange-600 dark:text-orange-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Temperature Proxy</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Hardware temperature monitoring</p>
                                </div>
                                <div class="ml-auto">
                                    <StatusBadge
                                        status={diagnosticsData()?.temperatureProxy?.socketFound ? 'online' : 'warning'}
                                        label={diagnosticsData()?.temperatureProxy?.socketFound ? 'Connected' : 'Not Found'}
                                    />
                                </div>
                            </div>
                            <div class="grid grid-cols-2 gap-3 text-xs">
                                <div class="flex items-center gap-2">
                                    {diagnosticsData()?.temperatureProxy?.socketFound ?
                                        <CheckCircle class="w-4 h-4 text-green-500" /> :
                                        <XCircle class="w-4 h-4 text-red-500" />
                                    }
                                    <span>Proxy Socket</span>
                                </div>
                                <div class="flex items-center gap-2">
                                    {diagnosticsData()?.temperatureProxy?.proxyReachable ?
                                        <CheckCircle class="w-4 h-4 text-green-500" /> :
                                        <AlertTriangle class="w-4 h-4 text-amber-500" />
                                    }
                                    <span>Daemon</span>
                                </div>
                            </div>
                            <Show when={diagnosticsData()?.temperatureProxy?.proxyVersion}>
                                <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700/50 text-xs text-gray-500 dark:text-gray-400">
                                    Version: {diagnosticsData()?.temperatureProxy?.proxyVersion}
                                </div>
                            </Show>
                            <Show when={diagnosticsData()?.temperatureProxy?.legacySSHDetected}>
                                <div class="mt-3 p-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded text-xs text-amber-700 dark:text-amber-300">
                                    ⚠️ Legacy SSH temperature collection detected - consider upgrading
                                </div>
                            </Show>
                        </Card>
                    </Show>

                    {/* API Tokens */}
                    <Show when={diagnosticsData()?.apiTokens}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-gray-200 dark:border-gray-700">
                                <div class="p-2 rounded-lg bg-blue-100 dark:bg-blue-900/30">
                                    <Shield class="w-4 h-4 text-blue-600 dark:text-blue-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">API Tokens</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Authentication status</p>
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
                                <div class="mt-3 p-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded text-xs text-amber-700 dark:text-amber-300">
                                    ⚠️ Legacy token detected - consider migrating to scoped tokens
                                </div>
                            </Show>
                        </Card>
                    </Show>

                    {/* Docker Agents */}
                    <Show when={diagnosticsData()?.dockerAgents}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-gray-200 dark:border-gray-700">
                                <div class="p-2 rounded-lg bg-purple-100 dark:bg-purple-900/30">
                                    <Database class="w-4 h-4 text-purple-600 dark:text-purple-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Docker Agents</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Container monitoring</p>
                                </div>
                                <div class="ml-auto text-right">
                                    <div class="text-lg font-bold text-gray-900 dark:text-gray-100">
                                        {diagnosticsData()?.dockerAgents?.hostsOnline}/{diagnosticsData()?.dockerAgents?.hostsTotal}
                                    </div>
                                    <div class="text-[10px] text-gray-500 dark:text-gray-400">online</div>
                                </div>
                            </div>
                            <div class="space-y-2 text-xs">
                                <MetricRow label="With Token Binding" value={diagnosticsData()?.dockerAgents?.hostsWithTokenBinding} />
                                <MetricRow label="Need Attention" value={diagnosticsData()?.dockerAgents?.hostsNeedingAttention} />
                                <MetricRow label="Outdated Version" value={diagnosticsData()?.dockerAgents?.hostsOutdatedVersion ?? 0} />
                            </div>
                            <Show when={diagnosticsData()?.dockerAgents?.recommendedAgentVersion}>
                                <div class="mt-3 pt-2 border-t border-gray-100 dark:border-gray-700/50 text-xs text-gray-500 dark:text-gray-400">
                                    Recommended version: {diagnosticsData()?.dockerAgents?.recommendedAgentVersion}
                                </div>
                            </Show>
                        </Card>
                    </Show>

                    {/* Alerts Configuration */}
                    <Show when={diagnosticsData()?.alerts}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-gray-200 dark:border-gray-700">
                                <div class="p-2 rounded-lg bg-rose-100 dark:bg-rose-900/30">
                                    <AlertTriangle class="w-4 h-4 text-rose-600 dark:text-rose-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Alerts Configuration</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Alert system status</p>
                                </div>
                            </div>
                            <div class="flex flex-wrap gap-2">
                                <span class={`px-2 py-1 rounded text-xs font-medium ${diagnosticsData()?.alerts?.legacyThresholdsDetected
                                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                                    : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                    }`}>
                                    Legacy thresholds: {diagnosticsData()?.alerts?.legacyThresholdsDetected ? 'Detected' : 'Migrated'}
                                </span>
                                <span class={`px-2 py-1 rounded text-xs font-medium ${diagnosticsData()?.alerts?.missingCooldown
                                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                                    : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                    }`}>
                                    Cooldown: {diagnosticsData()?.alerts?.missingCooldown ? 'Missing' : 'Configured'}
                                </span>
                                <span class={`px-2 py-1 rounded text-xs font-medium ${diagnosticsData()?.alerts?.missingGroupingWindow
                                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                                    : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                    }`}>
                                    Grouping: {diagnosticsData()?.alerts?.missingGroupingWindow ? 'Disabled' : 'Enabled'}
                                </span>
                            </div>
                            <Show when={(diagnosticsData()?.alerts?.notes?.length || 0) > 0}>
                                <ul class="mt-3 pt-2 border-t border-gray-100 dark:border-gray-700/50 space-y-1 text-xs text-gray-500 dark:text-gray-400 list-disc pl-4">
                                    <For each={diagnosticsData()?.alerts?.notes || []}>
                                        {(note) => <li>{note}</li>}
                                    </For>
                                </ul>
                            </Show>
                        </Card>
                    </Show>

                    {/* OpenCode AI Status */}
                    <Show when={diagnosticsData()?.openCode}>
                        <Card padding="md">
                            <div class="flex items-center gap-3 mb-4 pb-3 border-b border-gray-200 dark:border-gray-700">
                                <div class="p-2 rounded-lg bg-indigo-100 dark:bg-indigo-900/30">
                                    <Sparkles class="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
                                </div>
                                <div>
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">AI Assistant</h4>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">OpenCode Sidecar</p>
                                </div>
                                <div class="ml-auto">
                                    <StatusBadge
                                        status={diagnosticsData()?.openCode?.running ? 'online' : (diagnosticsData()?.openCode?.enabled ? 'offline' : 'unknown')}
                                        label={diagnosticsData()?.openCode?.running ? 'Running' : (diagnosticsData()?.openCode?.enabled ? 'Stopped' : 'Disabled')}
                                    />
                                </div>
                            </div>
                            <div class="space-y-2 text-xs">
                                <MetricRow label="Model" value={diagnosticsData()?.openCode?.model} />
                                <MetricRow label="Port" value={diagnosticsData()?.openCode?.port} mono />
                                <MetricRow label="Status" value={diagnosticsData()?.openCode?.healthy ? 'Healthy' : 'Unhealthy'} />
                            </div>
                            <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700/50 flex items-center justify-between text-xs">
                                <span class="text-gray-500 dark:text-gray-400">MCP Connection</span>
                                <div class="flex items-center gap-1.5">
                                    {diagnosticsData()?.openCode?.mcpConnected ?
                                        <CheckCircle class="w-3.5 h-3.5 text-green-500" /> :
                                        <XCircle class="w-3.5 h-3.5 text-red-500" />
                                    }
                                    <span class={diagnosticsData()?.openCode?.mcpConnected ? 'text-green-700 dark:text-green-300' : 'text-gray-500'}>
                                        {diagnosticsData()?.openCode?.mcpConnected ? 'Connected' : 'Disconnected'}
                                    </span>
                                </div>
                            </div>
                            <Show when={(diagnosticsData()?.openCode?.notes?.length || 0) > 0}>
                                <ul class="mt-3 bg-amber-50 dark:bg-amber-900/10 p-2 rounded text-xs text-amber-700 dark:text-amber-400 list-disc pl-4">
                                    <For each={diagnosticsData()?.openCode?.notes || []}>
                                        {(note) => <li>{note}</li>}
                                    </For>
                                </ul>
                            </Show>
                        </Card>
                    </Show>
                </div>

                {/* Errors Section */}
                <Show when={(diagnosticsData()?.errors?.length || 0) > 0}>
                    <Card padding="md" class="border-red-200 dark:border-red-800 bg-red-50/50 dark:bg-red-900/20">
                        <div class="flex items-center gap-3 mb-3">
                            <XCircle class="w-5 h-5 text-red-600 dark:text-red-400" />
                            <h4 class="text-sm font-semibold text-red-900 dark:text-red-100">Errors Detected</h4>
                        </div>
                        <ul class="space-y-2 text-xs text-red-700 dark:text-red-300">
                            <For each={diagnosticsData()?.errors || []}>
                                {(error) => (
                                    <li class="flex items-start gap-2 p-2 bg-red-100 dark:bg-red-900/40 rounded">
                                        <span class="text-red-500">•</span>
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
