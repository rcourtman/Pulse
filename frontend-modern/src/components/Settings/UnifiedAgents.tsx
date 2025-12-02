import { Component, createSignal, Show, For, onMount, createEffect, createMemo } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { MonitoringAPI } from '@/api/monitoring';
import { SecurityAPI } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import type { HostLookupResponse } from '@/types/api';
import type { APITokenRecord } from '@/api/security';
import { HOST_AGENT_SCOPE, DOCKER_REPORT_SCOPE } from '@/constants/apiScopes';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { logger } from '@/utils/logger';

const TOKEN_PLACEHOLDER = '<api-token>';
const pulseUrl = () => getPulseBaseUrl();

const buildDefaultTokenName = () => {
    const now = new Date();
    const iso = now.toISOString().slice(0, 16); // YYYY-MM-DDTHH:MM
    const stamp = iso.replace('T', ' ').replace(/:/g, '-');
    return `Agent ${stamp}`;
};

type AgentPlatform = 'linux' | 'macos' | 'windows';

const commandsByPlatform: Record<
    AgentPlatform,
    {
        title: string;
        description: string;
        snippets: { label: string; command: string; note?: string | any }[];
    }
> = {
    linux: {
        title: 'Install on Linux',
        description:
            'The unified installer downloads the agent binary and configures the appropriate service for your system.',
        snippets: [
            {
                label: 'Install',
                command: `curl -fsSL ${pulseUrl()}/install.sh | sudo bash -s -- --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
                note: (
                    <span>
                        Automatically detects your init system (systemd, OpenRC, Unraid, Synology) and configures the appropriate service. Works on Debian, Ubuntu, Fedora, Alpine, Gentoo, Unraid, Synology, and more.
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
                command: `curl -fsSL ${pulseUrl()}/install.sh | sudo bash -s -- --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
                note: (
                    <span>
                        Creates <code>/Library/LaunchDaemons/com.pulse.agent.plist</code> and starts the agent automatically.
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
                command: `irm ${pulseUrl()}/install.ps1 | iex`,
                note: (
                    <span>
                        Run in PowerShell as Administrator. The script will prompt for the Pulse URL and API token, download the agent binary, and install it as a Windows service with automatic startup.
                    </span>
                ),
            },
            {
                label: 'Install with parameters (PowerShell)',
                command: `$env:PULSE_URL="${pulseUrl()}"; $env:PULSE_TOKEN="${TOKEN_PLACEHOLDER}"; irm ${pulseUrl()}/install.ps1 | iex`,
                note: (
                    <span>
                        Non-interactive installation. Set environment variables before running to skip prompts.
                    </span>
                ),
            },
        ],
    },
};

export const UnifiedAgents: Component = () => {
    const { state } = useWebSocket();

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
    const [enableDocker, setEnableDocker] = createSignal(false); // Default to false - user must opt-in for Docker monitoring

    createEffect(() => {
        if (requiresToken()) {
            setConfirmedNoToken(false);
        } else {
            setCurrentToken(null);
            setLatestRecord(null);
        }
    });

    const commandSections = createMemo(() =>
        Object.entries(commandsByPlatform).map(([platform, meta]) => ({
            platform: platform as AgentPlatform,
            ...meta,
        })),
    );

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
                const response = await fetch('/api/security/status', { credentials: 'include' });
                if (response.ok) {
                    const data = (await response.json()) as SecurityStatus;
                    setSecurityStatus(data);
                }
            } catch (err) {
                if (!hasLoggedSecurityStatusError) {
                    hasLoggedSecurityStatusError = true;
                    logger.error('Failed to load security status', err);
                }
            }
        };
        fetchSecurityStatus();
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
            // Generate token with BOTH scopes
            const scopes = [HOST_AGENT_SCOPE, DOCKER_REPORT_SCOPE];
            const { token, record } = await SecurityAPI.createToken(desiredName, scopes);

            setCurrentToken(token);
            setLatestRecord(record);
            setTokenName('');
            setConfirmedNoToken(false);
            notificationStore.success('Token generated with Host and Docker permissions.', 4000);
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

    const getDockerFlag = () => enableDocker() ? ' --enable-docker' : '';

    const getUninstallCommand = () => {
        return `curl -fsSL ${pulseUrl()}/install.sh | sudo bash -s -- --uninstall`;
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
        }>();

        // Process Host Agents
        hosts.forEach(h => {
            const key = h.hostname || h.id;
            unified.set(key, {
                id: h.id,
                hostname: h.hostname || 'Unknown',
                displayName: h.displayName,
                types: ['host'],
                status: h.status || 'unknown',
                version: h.agentVersion,
                lastSeen: h.lastSeen,
                isLegacy: h.isLegacy
            });
        });

        // Process Docker Agents (merge if same hostname)
        dockerHosts.forEach(d => {
            const key = d.hostname || d.id;
            const existing = unified.get(key);
            if (existing) {
                if (!existing.types.includes('docker')) {
                    existing.types.push('docker');
                }
                // Update version/status if newer
                if (!existing.version && d.agentVersion) existing.version = d.agentVersion;
                if (d.isLegacy) existing.isLegacy = true;
            } else {
                unified.set(key, {
                    id: d.id,
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
                    entry.types = Array.from(new Set([...entry.types, 'host']));
                }
                if (prevTypes.has('docker') && !currentTypes.has('docker') && dockerHosts.length > 0) {
                    // Docker type disappeared but we still have docker data overall
                    entry.types = Array.from(new Set([...entry.types, 'docker']));
                }
            }

            newHostTypes.set(key, new Set(entry.types));
        });
        previousHostTypes = newHostTypes;

        return Array.from(unified.values()).sort((a, b) => a.hostname.localeCompare(b.hostname));
    });

    const legacyAgents = createMemo(() => allHosts().filter(h => h.isLegacy));
    const hasLegacyAgents = createMemo(() => legacyAgents().length > 0);

    const removedDockerHosts = createMemo(() => {
        const removed = state.removedDockerHosts || [];
        return removed.sort((a, b) => b.removedAt - a.removedAt);
    });
    const hasRemovedDockerHosts = createMemo(() => removedDockerHosts().length > 0);

    const getUpgradeCommand = (_hostname: string) => {
        const token = resolvedToken();
        return `curl -fsSL ${pulseUrl()}/install.sh | sudo bash -s -- --url ${pulseUrl()} --token ${token}`;
    };

    const handleRemoveAgent = async (id: string, type: 'host' | 'docker') => {
        if (!confirm('Are you sure you want to remove this agent? This will stop monitoring but will not uninstall the agent from the remote machine.')) return;

        try {
            if (type === 'host') {
                await MonitoringAPI.deleteHostAgent(id);
            } else {
                await MonitoringAPI.deleteDockerHost(id);
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

    return (
        <div class="space-y-6">
            <Card padding="lg" class="space-y-5">
                <div class="space-y-1">
                    <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Add a unified agent</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400">
                        Monitor server metrics (CPU, RAM, Disk) and Docker containers with a single agent.
                    </p>
                </div>

                <div class="space-y-5">
                    <Show when={requiresToken()}>
                        <div class="space-y-3">
                            <div class="space-y-1">
                                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Generate API token</p>
                                <p class="text-sm text-gray-600 dark:text-gray-400">
                                    Create a fresh token scoped for both Host and Docker monitoring.
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

                    <Show when={commandsUnlocked()}>
                        <div class="space-y-3">
                            <div class="flex items-center justify-between">
                                <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Installation commands</h4>
                                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                                    <input
                                        type="checkbox"
                                        checked={enableDocker()}
                                        onChange={(e) => setEnableDocker(e.currentTarget.checked)}
                                        class="rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                                    />
                                    Enable Docker monitoring
                                </label>
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
                                                            // Append docker flag if enabled
                                                            if (enableDocker()) {
                                                                // For PowerShell, we need to handle the env var or args differently
                                                                if (cmd.includes('$env:PULSE_URL')) {
                                                                    // Env var style: add $env:PULSE_ENABLE_DOCKER="true";
                                                                    cmd = `$env:PULSE_ENABLE_DOCKER="true"; ` + cmd;
                                                                } else if (cmd.includes('irm')) {
                                                                    // Simple irm style: no args passed to script directly in this snippet style
                                                                    // Actually, the simple irm style relies on prompts, so flags don't apply directly unless we change the snippet
                                                                    // But for the bash script, we append flags
                                                                    if (!cmd.includes('irm')) {
                                                                        cmd += getDockerFlag();
                                                                    }
                                                                } else {
                                                                    cmd += getDockerFlag();
                                                                }
                                                            }
                                                            return cmd;
                                                        };

                                                        return (
                                                            <div class="space-y-2">
                                                                <div class="flex items-center justify-between gap-3">
                                                                    <h6 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                                                                        {snippet.label}
                                                                    </h6>
                                                                    <button
                                                                        type="button"
                                                                        onClick={async () => {
                                                                            const success = await copyToClipboard(copyCommand());
                                                                            if (typeof window !== 'undefined' && window.showToast) {
                                                                                window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                                                                            }
                                                                        }}
                                                                        class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors bg-blue-600 text-white hover:bg-blue-700"
                                                                    >
                                                                        Copy command
                                                                    </button>
                                                                </div>
                                                                <pre class="overflow-x-auto rounded-md bg-gray-900/90 p-3 text-xs text-gray-100">
                                                                    <code>{copyCommand()}</code>
                                                                </pre>
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
                                    Advanced options (uninstall & manual install)
                                </summary>
                                <div class="mt-3 space-y-4">
                                    <div>
                                        <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Uninstall</p>
                                        <div class="mt-2 flex items-center gap-2">
                                            <code class="flex-1 break-all rounded bg-gray-900 px-3 py-2 font-mono text-xs text-red-400 dark:bg-gray-950">
                                                {getUninstallCommand()}
                                            </code>
                                            <button
                                                type="button"
                                                onClick={async () => {
                                                    const success = await copyToClipboard(getUninstallCommand());
                                                    if (typeof window !== 'undefined' && window.showToast) {
                                                        window.showToast(success ? 'success' : 'error', success ? 'Copied to clipboard' : 'Failed to copy to clipboard');
                                                    }
                                                }}
                                                class="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:bg-red-900/30 dark:text-red-300 dark:hover:bg-red-900/50"
                                            >
                                                Copy
                                            </button>
                                        </div>
                                        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                                            Stops the agent, removes the binary, the systemd unit, and related files.
                                        </p>
                                    </div>
                                </div>
                            </details>
                        </div>
                    </Show>
                </div>
            </Card>

            <Card padding="lg" class="space-y-4">
                <div class="space-y-1">
                    <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Managed Agents</h3>
                    <p class="text-sm text-gray-600 dark:text-gray-400">
                        Overview of all agents currently reporting to Pulse.
                    </p>
                </div>

                <Show when={hasLegacyAgents()}>
                    <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-700 dark:bg-amber-900/20">
                        <div class="flex items-start gap-3">
                            <svg class="h-5 w-5 flex-shrink-0 text-amber-500 dark:text-amber-400 mt-0.5" viewBox="0 0 20 20" fill="currentColor">
                                <path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
                            </svg>
                            <div class="flex-1 space-y-2">
                                <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                                    {legacyAgents().length} legacy agent{legacyAgents().length > 1 ? 's' : ''} detected
                                </p>
                                <p class="text-sm text-amber-700 dark:text-amber-300">
                                    Legacy agents (pulse-host-agent, pulse-docker-agent) are deprecated. Upgrade to the unified agent for auto-updates and combined host + Docker monitoring.
                                </p>
                                <p class="text-xs text-amber-600 dark:text-amber-400">
                                    Run this command on each legacy host to upgrade:
                                </p>
                                <div class="flex items-center gap-2">
                                    <code class="flex-1 break-all rounded bg-amber-100 px-3 py-2 font-mono text-xs text-amber-900 dark:bg-amber-900/40 dark:text-amber-100">
                                        {getUpgradeCommand('')}
                                    </code>
                                    <button
                                        type="button"
                                        onClick={async () => {
                                            const success = await copyToClipboard(getUpgradeCommand(''));
                                            if (typeof window !== 'undefined' && window.showToast) {
                                                window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                                            }
                                        }}
                                        class="rounded-lg bg-amber-200 px-3 py-1.5 text-xs font-medium text-amber-800 transition-colors hover:bg-amber-300 dark:bg-amber-800 dark:text-amber-100 dark:hover:bg-amber-700"
                                    >
                                        Copy
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </Show>

                <Card padding="none" tone="glass" class="overflow-hidden rounded-lg">
                    <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                        <thead class="bg-gray-50 dark:bg-gray-800">
                            <tr>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Hostname</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Type</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Status</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Version</th>
                                <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Last Seen</th>
                                <th scope="col" class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Actions</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
                            <For each={allHosts()} fallback={
                                <tr>
                                    <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                                        No agents installed yet.
                                    </td>
                                </tr>
                            }>
                                {(agent) => (
                                    <tr>
                                        <td class="whitespace-nowrap px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">
                                            {agent.displayName || agent.hostname}
                                            <Show when={agent.displayName && agent.displayName !== agent.hostname}>
                                                <span class="ml-2 text-xs text-gray-500">({agent.hostname})</span>
                                            </Show>
                                        </td>
                                        <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                            <div class="flex gap-1">
                                                <For each={agent.types}>
                                                    {(type) => (
                                                        <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${type === 'host'
                                                            ? 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300'
                                                            : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300'
                                                            }`}>
                                                            {type === 'host' ? 'Host' : 'Docker'}
                                                        </span>
                                                    )}
                                                </For>
                                            </div>
                                        </td>
                                        <td class="whitespace-nowrap px-4 py-3 text-sm">
                                            <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${connectedFromStatus(agent.status)
                                                ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
                                                : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                                                }`}>
                                                {agent.status}
                                            </span>
                                        </td>
                                        <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                            {agent.version || '—'}
                                            <Show when={agent.isLegacy}>
                                                <span class="ml-2 inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-200" title="This agent is using an old version. Please update to the unified agent.">
                                                    Legacy
                                                </span>
                                            </Show>
                                        </td>
                                        <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                            {agent.lastSeen ? formatRelativeTime(agent.lastSeen) : '—'}
                                        </td>
                                        <td class="whitespace-nowrap px-4 py-3 text-right text-sm font-medium">
                                            <button
                                                onClick={() => handleRemoveAgent(agent.id, agent.types[0])}
                                                class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                            >
                                                Remove
                                            </button>
                                        </td>
                                    </tr>
                                )}
                            </For>
                        </tbody>
                    </table>
                </Card>
            </Card>

            <Show when={hasRemovedDockerHosts()}>
                <Card padding="lg" class="space-y-4">
                    <div class="space-y-1">
                        <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Removed Docker Hosts</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">
                            Docker hosts that were removed and are blocked from re-enrolling. Allow re-enrollment to let them report again.
                        </p>
                    </div>

                    <Card padding="none" tone="glass" class="overflow-hidden rounded-lg">
                        <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                            <thead class="bg-gray-50 dark:bg-gray-800">
                                <tr>
                                    <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Hostname</th>
                                    <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Host ID</th>
                                    <th scope="col" class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Removed</th>
                                    <th scope="col" class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Actions</th>
                                </tr>
                            </thead>
                            <tbody class="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
                                <For each={removedDockerHosts()}>
                                    {(host) => (
                                        <tr>
                                            <td class="whitespace-nowrap px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">
                                                {host.displayName || host.hostname || 'Unknown'}
                                            </td>
                                            <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400 font-mono text-xs">
                                                {host.id.slice(0, 8)}...
                                            </td>
                                            <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                                {formatRelativeTime(host.removedAt)}
                                            </td>
                                            <td class="whitespace-nowrap px-4 py-3 text-right text-sm font-medium">
                                                <button
                                                    onClick={() => handleAllowReenroll(host.id, host.hostname)}
                                                    class="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                                                >
                                                    Allow re-enroll
                                                </button>
                                            </td>
                                        </tr>
                                    )}
                                </For>
                            </tbody>
                        </table>
                    </Card>
                </Card>
            </Show>
        </div >
    );
};
