import { Component, createSignal, createEffect, onCleanup, Show, For } from 'solid-js';
// Note: For is still used for connectedAgents list
import { useNavigate } from '@solidjs/router';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { getPulseBaseUrl } from '@/utils/url';
import type { State } from '@/types/api';
import { SecurityAPI } from '@/api/security';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { showSuccess, showError } from '@/utils/toast';
import {
    trackAgentFirstConnected,
    trackAgentInstallCommandCopied,
    trackAgentInstallTokenGenerated,
    trackPaywallViewed,
    trackUpgradeClicked,
} from '@/utils/upgradeMetrics';
import { loadLicenseStatus, entitlements, getUpgradeActionUrlOrFallback } from '@/stores/license';
import type { WizardState } from '../SetupWizard';

interface CompleteStepProps {
    state: WizardState;
    onComplete: () => void;
}

// Platform auto-detection is now handled by the install script

interface ConnectedAgent {
    id: string;
    name: string;
    type: string;
    host: string;
    addedAt: Date;
}

const RELAY_SETTINGS_PATH = '/settings/system-relay';
const SETUP_WIZARD_TELEMETRY_SURFACE = 'setup_wizard_complete';

export const CompleteStep: Component<CompleteStepProps> = (props) => {
    const navigate = useNavigate();
    const [copied, setCopied] = createSignal<'password' | 'token' | 'install' | null>(null);
    const [showCredentials, setShowCredentials] = createSignal(false);
    const [connectedAgents, setConnectedAgents] = createSignal<ConnectedAgent[]>([]);
    const [currentInstallToken, setCurrentInstallToken] = createSignal(props.state.apiToken);
    const [generatingToken, setGeneratingToken] = createSignal(false);
    const [trialStarting, setTrialStarting] = createSignal(false);
    const [trialStarted, setTrialStarted] = createSignal(false);
    const [relayPaywallTracked, setRelayPaywallTracked] = createSignal(false);
    let firstConnectionTracked = false;

    // Only track Relay paywall exposure once it is actually shown.
    createEffect(() => {
        if (connectedAgents().length > 0 && !relayPaywallTracked()) {
            trackPaywallViewed('relay', 'setup_wizard');
            setRelayPaywallTracked(true);
        }
    });

    // Poll for agent connections since WebSocket isn't available during setup
    createEffect(() => {
        let pollInterval: number | undefined;
        let previousCount = 0;

        const checkForAgents = async () => {
            try {
                const state = await apiFetchJSON<State>('/api/state', {
                    headers: {
                        'X-API-Token': props.state.apiToken,
                    },
                });
                const nodes = state.nodes || [];
                const hosts = state.hosts || [];

                // Check if we have new connections
                const totalAgents = nodes.length + hosts.length;
                if (!firstConnectionTracked && totalAgents > 0) {
                    trackAgentFirstConnected(SETUP_WIZARD_TELEMETRY_SURFACE, 'first_agent');
                    firstConnectionTracked = true;
                }

                if (totalAgents > previousCount || totalAgents !== connectedAgents().length) {
                    // Group agents by hostname to avoid duplicates
                    const agentMap = new Map<string, ConnectedAgent>();

                    for (const node of nodes) {
                        const name = node.name || node.displayName || 'Unknown';
                        const existing = agentMap.get(name);
                        if (existing) {
                            // Add PVE to existing agent's types
                            if (!existing.type.includes('Proxmox')) {
                                existing.type = `${existing.type} + Proxmox VE`;
                            }
                            if (node.host && !existing.host) {
                                existing.host = node.host;
                            }
                        } else {
                            agentMap.set(name, {
                                id: node.id || `node-${name}`,
                                name,
                                type: 'Proxmox VE',
                                host: node.host || '',
                                addedAt: new Date(),
                            });
                        }
                    }

                    for (const host of hosts) {
                        const name = host.displayName || host.hostname || 'Unknown';
                        const existing = agentMap.get(name);
                        if (existing) {
                            // Add Host Agent to existing PVE node
                            if (!existing.type.includes('Host')) {
                                existing.type = `${existing.type} + Host Agent`;
                            }
                        } else {
                            agentMap.set(name, {
                                id: host.id || `host-${name}`,
                                name,
                                type: 'Host Agent',
                                host: '',
                                addedAt: new Date(),
                            });
                        }
                    }

                    setConnectedAgents(Array.from(agentMap.values()));

                    // Generate new token if agents increased
                    if (previousCount > 0 && totalAgents > previousCount) {
                        void generateNewToken('agent_added');
                    }

                    previousCount = totalAgents;
                }
            } catch (error) {
                logger.error('Failed to check for agents:', error);
            }
        };

        // Poll every 3 seconds
        pollInterval = window.setInterval(checkForAgents, 3000);

        // Initial check
        checkForAgents();

        onCleanup(() => {
            if (pollInterval) {
                window.clearInterval(pollInterval);
            }
        });
    });

    // Platform selection removed - installer now auto-detects Docker, Kubernetes, Proxmox

    const generateNewToken = async (reason: 'rotation' | 'agent_added' | 'manual' = 'rotation') => {
        if (generatingToken()) return;

        setGeneratingToken(true);
        try {
            // Don't specify scopes - tokens without scopes default to wildcard access
            const result = await SecurityAPI.createToken(`agent-install-${Date.now()}`);
            if (result.token) {
                setCurrentInstallToken(result.token);
                trackAgentInstallTokenGenerated(SETUP_WIZARD_TELEMETRY_SURFACE, reason);
            }
        } catch (error) {
            logger.error('Failed to generate new token:', error);
        } finally {
            setGeneratingToken(false);
        }
    };

    const handleCopy = async (type: 'password' | 'token' | 'install', value?: string) => {
        const copyValue = value || (type === 'password' ? props.state.password : props.state.apiToken);
        const success = await copyToClipboard(copyValue);
        if (success) {
            setCopied(type);
            setTimeout(() => setCopied(null), 2000);
        }
    };

    const handleCopyInstall = async () => {
        const command = getInstallCommand();
        const success = await copyToClipboard(command);
        if (success) {
            setCopied('install');
            setTimeout(() => setCopied(null), 2000);
            trackAgentInstallCommandCopied(SETUP_WIZARD_TELEMETRY_SURFACE, 'linux_install');

            // Auto-generate a new token for the next host
            // This ensures each copy gets a unique token without user intervention
            void generateNewToken('rotation');
        }
    };

    const downloadCredentials = () => {
        const baseUrl = getPulseBaseUrl();
        const content = `Pulse Credentials
==================
Generated: ${new Date().toISOString()}

Web Login:
----------
URL: ${baseUrl}
Username: ${props.state.username}
Password: ${props.state.password}

API Token:
----------
${props.state.apiToken}

Example: curl -H "X-API-Token: ${props.state.apiToken}" ${baseUrl}/api/state

Keep these credentials secure!
`;

        const blob = new Blob([content], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `pulse-credentials-${Date.now()}.txt`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    };

    const getInstallCommand = () => {
        const baseUrl = getPulseBaseUrl();
        // Simple command - the install script auto-detects Docker, Kubernetes, and Proxmox
        // Note: sudo removed - users should run as root (via su or sudo) since some systems (like Proxmox) don't have sudo installed
        return `curl -sSL ${baseUrl}/install.sh | bash -s -- --url "${baseUrl}" --token "${currentInstallToken()}"`;
    };

    const handleStartTrial = async () => {
        trackUpgradeClicked('setup_wizard', 'relay');
        if (trialStarting()) return;

        setTrialStarting(true);
        try {
            const res = await apiFetch('/api/license/trial/start', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-API-Token': props.state.apiToken,
                },
                body: JSON.stringify({}),
            });

            if (!res.ok) {
                // 409 means trial already started — treat as success.
                if (res.status === 409) {
                    setTrialStarted(true);
                    return;
                }
                const text = await res.text().catch(() => '');
                throw new Error(text || `Trial start failed (${res.status})`);
            }

            showSuccess('14-day Pro trial started! Set up Relay to monitor from your phone.');
            setTrialStarted(true);
            await loadLicenseStatus(true);
        } catch (err) {
            logger.warn('[CompleteStep] Failed to start trial; falling back to upgrade URL', err);
            showError('Unable to start trial. Redirecting to upgrade options...');
            const upgradeUrl = getUpgradeActionUrlOrFallback('relay');
            if (typeof window !== 'undefined') {
                window.location.href = upgradeUrl;
            }
        } finally {
            setTrialStarting(false);
        }
    };

    const handleSetupRelay = () => {
        props.onComplete();
        navigate(RELAY_SETTINGS_PATH);
    };

    return (
        <div class="max-w-2xl mx-auto bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 overflow-hidden animate-fade-in relative rounded-md p-6 sm:p-8 text-center text-slate-900 dark:text-white">            <div class="relative z-10">
            {/* Success animation */}
            <div class="mb-8">
                <div class="inline-flex items-center justify-center w-16 h-16 rounded-full bg-emerald-100 dark:bg-emerald-900 text-emerald-600 dark:text-emerald-400 mb-6 shadow-sm border border-emerald-200 dark:border-emerald-800">
                    <svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                </div>
                <h1 class="text-2xl sm:text-3xl font-bold tracking-tight text-slate-900 dark:text-white mb-2">
                    Security Configured
                </h1>
                <p class="text-slate-500 dark:text-emerald-300 font-light text-sm sm:text-base">
                    Install the Unified Agent on each host you want to monitor
                </p>
            </div>

            <Show when={connectedAgents().length > 0}>
                <div class="bg-emerald-50 dark:bg-emerald-900 rounded-md border border-emerald-200 dark:border-emerald-800 p-5 text-left mb-6 shadow-sm">
                    <h3 class="text-sm font-semibold text-emerald-800 dark:text-emerald-400 mb-3 flex items-center gap-2">
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                        </svg>
                        Connected ({connectedAgents().length} host{connectedAgents().length !== 1 ? 's' : ''})
                    </h3>
                    <div class="space-y-2">
                        <For each={connectedAgents()}>
                            {(agent) => (
                                <div class="flex items-center justify-between bg-white dark:bg-slate-900 rounded-md px-3 py-2.5 border border-slate-100 dark:border-slate-800 shadow-sm">
                                    <div class="flex items-center gap-2.5">
                                        <span class="w-2.5 h-2.5 bg-emerald-500 rounded-full animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.8)]"></span>
                                        <span class="text-slate-800 dark:text-white text-sm font-medium">{agent.name}</span>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        <span class="text-[10px] text-emerald-700 dark:text-emerald-300 bg-emerald-100 dark:bg-emerald-900 border border-emerald-200 dark:border-emerald-800 px-2 py-0.5 rounded-full font-medium">{agent.type}</span>
                                        <Show when={agent.host}>
                                            <span class="text-[10px] text-slate-400 dark:text-slate-500 font-mono">{agent.host}</span>
                                        </Show>
                                    </div>
                                </div>
                            )}
                        </For>
                    </div>
                </div>
            </Show>

            {/* Auto-detection info */}
            <div class="bg-white dark:bg-slate-900 rounded-md border border-slate-200 dark:border-slate-700 shadow-sm p-6 text-left mb-6">
                <h3 class="text-sm font-semibold text-slate-900 dark:text-white mb-3 flex items-center gap-2">
                    <svg class="w-4 h-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                    </svg>
                    Smart Auto-Detection
                </h3>

                <p class="text-slate-500 dark:text-slate-400 text-xs mb-4">
                    This is the primary onboarding path. The installer auto-detects platform integrations on each host:
                </p>

                <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
                    <div class="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md p-3 text-center transition-all hover:shadow-md">
                        <div class="h-6 flex items-center justify-center mb-2">
                            <span class="text-xl">🐳</span>
                        </div>
                        <div class="text-slate-800 dark:text-slate-200 font-semibold text-xs mb-0.5">Docker</div>
                        <p class="text-[9px] text-slate-500 dark:text-slate-400">Container monitoring</p>
                    </div>
                    <div class="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md p-3 text-center transition-all hover:shadow-md">
                        <div class="h-6 flex items-center justify-center mb-2">
                            <span class="text-xl">☸️</span>
                        </div>
                        <div class="text-slate-800 dark:text-slate-200 font-semibold text-xs mb-0.5">Kubernetes</div>
                        <p class="text-[9px] text-slate-500 dark:text-slate-400">Cluster monitoring</p>
                    </div>
                    <div class="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md p-3 text-center transition-all hover:shadow-md">
                        <div class="h-6 flex items-center justify-center mb-2">
                            <ProxmoxIcon class="w-5 h-5 text-orange-500" />
                        </div>
                        <div class="text-slate-800 dark:text-slate-200 font-semibold text-xs mb-0.5">Proxmox</div>
                        <p class="text-[9px] text-slate-500 dark:text-slate-400">VM & container API</p>
                    </div>
                    <div class="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md p-3 text-center transition-all hover:shadow-md">
                        <div class="h-6 flex items-center justify-center mb-2">
                            <span class="text-[13px] text-cyan-600 dark:text-cyan-400 font-bold tracking-tight">NAS</span>
                        </div>
                        <div class="text-slate-800 dark:text-slate-200 font-semibold text-xs mb-0.5">TrueNAS</div>
                        <p class="text-[9px] text-slate-500 dark:text-slate-400">SCALE-aware install</p>
                    </div>
                </div>

                <div class="bg-emerald-50 dark:bg-emerald-900 border border-emerald-200 dark:border-emerald-800 rounded-md p-3">
                    <div class="flex items-center gap-3">
                        <div class="w-5 h-5 rounded-full bg-emerald-100 dark:bg-emerald-800 flex items-center justify-center shrink-0">
                            <svg class="w-3 h-3 text-emerald-600 dark:text-emerald-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                            </svg>
                        </div>
                        <div class="flex-1">
                            <div class="flex items-center gap-2 mb-0.5">
                                <span class="text-slate-800 dark:text-slate-200 font-semibold text-xs">Host Metrics</span>
                                <span class="text-[9px] text-emerald-700 dark:text-emerald-300 bg-emerald-100 dark:bg-emerald-900 px-1.5 py-0.5 rounded-sm font-medium">Always included</span>
                            </div>
                            <p class="text-[10px] text-slate-500 dark:text-slate-400">CPU, memory, disk, network on any Linux/macOS/Windows</p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Agent installation */}
            <div class="bg-white dark:bg-slate-900 rounded-md border border-slate-200 dark:border-slate-700 shadow-sm p-6 text-left mb-6">
                <div class="flex items-center justify-between mb-3">
                    <h3 class="text-sm font-semibold text-slate-900 dark:text-white flex items-center gap-2">
                        <svg class="w-4 h-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                        </svg>
                        Install Command
                    </h3>
                    <button
                        onClick={() => {
                            void generateNewToken('manual');
                        }}
                        disabled={generatingToken()}
                        class="text-xs bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 hover:text-slate-900 dark:hover:text-white px-2.5 py-1.5 rounded-md flex items-center gap-1.5 disabled:opacity-50 transition-colors border border-transparent dark:border-slate-700"
                        title="Generate a new token for the next host"
                    >
                        {generatingToken() ? (
                            <svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                            </svg>
                        ) : (
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                            </svg>
                        )}
                        New Token
                    </button>
                </div>
                <p class="text-slate-500 dark:text-slate-400 text-xs mb-3">
                    Copy and run on each host you want monitored. <span class="text-emerald-600 dark:text-emerald-400 font-medium">A new token is generated automatically after each copy.</span>
                </p>

                <div class="bg-slate-50 dark:bg-slate-950 rounded-md p-4 font-mono text-xs mb-3 relative group border border-slate-200 dark:border-slate-700 shadow-inner">
                    <code class="text-emerald-600 dark:text-emerald-400 break-all block leading-relaxed pr-8">{getInstallCommand()}</code>
                    <button
                        onClick={handleCopyInstall}
                        class="absolute top-2.5 right-2.5 p-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-700 rounded-md transition-all shadow-sm group-hover:opacity-100 sm:opacity-0 focus:opacity-100"
                        title="Copy command"
                    >
                        {copied() === 'install' ? (
                            <svg class="w-4 h-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                            </svg>
                        ) : (
                            <svg class="w-4 h-4 text-slate-400 dark:text-slate-500 group-hover:text-slate-600 dark:group-hover:text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                            </svg>
                        )}
                    </button>
                </div>

                <p class="text-[11px] text-slate-400 dark:text-slate-500">
                    Agents auto-register with Pulse. Install on as many hosts as you like.
                </p>
            </div>

            {/* Credentials section (collapsible) */}
            <div class="bg-white dark:bg-slate-900 rounded-md border border-slate-200 dark:border-slate-700 mb-8 overflow-hidden shadow-sm">
                <button
                    onClick={() => setShowCredentials(!showCredentials())}
                    class="w-full p-4 flex items-center justify-between text-left hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors group"
                >
                    <div class="flex items-center gap-3">
                        <div class="w-8 h-8 rounded-md bg-amber-50 dark:bg-amber-900 flex items-center justify-center border border-amber-100 dark:border-amber-800">
                            <svg class="w-4 h-4 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                            </svg>
                        </div>
                        <div>
                            <span class="text-slate-800 dark:text-slate-200 font-semibold text-sm flex items-center gap-2">
                                Your Credentials
                                <span class="text-[10px] text-amber-700 dark:text-amber-400 bg-amber-100 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 px-2 py-0.5 rounded-full">Save these</span>
                            </span>
                        </div>
                    </div>
                    <svg class={`w-5 h-5 text-slate-400 transition-transform duration-200 group-hover:text-slate-600 dark:group-hover:text-slate-300 ${showCredentials() ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                    </svg>
                </button>

                <Show when={showCredentials()}>
                    <div class="p-4 pt-0 space-y-3 border-t border-slate-100 dark:border-slate-800 mt-2">
                        <div class="bg-slate-50 dark:bg-black border border-slate-200 dark:border-slate-800 rounded-md p-3">
                            <div class="text-[11px] font-medium text-slate-500 dark:text-slate-400 mb-1 uppercase tracking-wider">Username</div>
                            <div class="text-slate-900 dark:text-white font-mono text-sm">{props.state.username}</div>
                        </div>

                        <div class="bg-slate-50 dark:bg-black border border-slate-200 dark:border-slate-800 rounded-md p-3">
                            <div class="text-[11px] font-medium text-slate-500 dark:text-slate-400 mb-1 uppercase tracking-wider">Password</div>
                            <div class="flex items-center justify-between">
                                <code class="text-slate-900 dark:text-white font-mono text-sm break-all">{props.state.password}</code>
                                <button
                                    onClick={() => handleCopy('password')}
                                    class="ml-3 p-1.5 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-md transition-all shadow-sm shrink-0"
                                >
                                    {copied() === 'password' ? (
                                        <svg class="w-4 h-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                        </svg>
                                    ) : (
                                        <svg class="w-4 h-4 text-slate-400 dark:text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                                        </svg>
                                    )}
                                </button>
                            </div>
                        </div>

                        <div class="bg-slate-50 dark:bg-black border border-slate-200 dark:border-slate-800 rounded-md p-3">
                            <div class="text-[11px] font-medium text-slate-500 dark:text-slate-400 mb-1 uppercase tracking-wider">API Token (for web login)</div>
                            <div class="flex items-center justify-between">
                                <code class="text-slate-900 dark:text-white font-mono text-xs break-all pr-4">{props.state.apiToken}</code>
                                <button
                                    onClick={() => handleCopy('token')}
                                    class="ml-2 p-1.5 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-md transition-all shadow-sm shrink-0"
                                >
                                    {copied() === 'token' ? (
                                        <svg class="w-4 h-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                        </svg>
                                    ) : (
                                        <svg class="w-4 h-4 text-slate-400 dark:text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                                        </svg>
                                    )}
                                </button>
                            </div>
                        </div>

                        <button
                            onClick={downloadCredentials}
                            class="w-full mt-2 py-2.5 text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 flex items-center justify-center gap-1.5 bg-blue-50 dark:bg-blue-900 hover:bg-blue-100 dark:hover:bg-blue-900 rounded-md transition-colors border border-blue-100 dark:border-blue-900"
                        >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                            Download credentials
                        </button>
                    </div>
                </Show>
            </div>

            {/* Launch button */}
            <div class="pt-4 border-t border-slate-200 dark:border-slate-800">
                <button
                    onClick={props.onComplete}
                    class="w-full py-4 px-6 bg-blue-600 hover:bg-blue-700 text-white text-base font-semibold rounded-md transition-all duration-200"
                >
                    {connectedAgents().length > 0 ? 'Go to Dashboard' : 'Skip for Now'}
                </button>
                <p class="mt-4 text-xs text-slate-500 dark:text-slate-400">
                    You can add more agents anytime from Settings
                </p>
            </div>

            {/* Optional upsell after core agent onboarding is complete */}
            <Show when={connectedAgents().length > 0}>
                <div class="bg-indigo-50 dark:bg-indigo-900 rounded-md border border-indigo-100 dark:border-indigo-800 p-5 text-left mt-8 shadow-sm overflow-hidden relative">
                    <div class="flex items-start gap-4 relative z-10">
                        <div class="flex h-12 w-12 items-center justify-center rounded-md bg-indigo-600 text-white shrink-0 shadow-sm border border-indigo-500">
                            <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
                            </svg>
                        </div>
                        <div class="flex-1 min-w-0">
                            <h3 class="text-sm font-bold text-slate-900 dark:text-white mb-1">Monitor from Anywhere</h3>
                            <p class="text-xs text-slate-600 dark:text-indigo-200 mb-4 leading-relaxed">
                                Get push notifications and manage your infrastructure from your phone with Pulse Relay.
                            </p>
                            <Show
                                when={!trialStarted() && entitlements()?.subscription_state !== 'trial'}
                                fallback={
                                    <button
                                        type="button"
                                        onClick={handleSetupRelay}
                                        class="inline-flex items-center gap-2 rounded-md bg-indigo-600 hover:bg-indigo-700 px-4 py-2 text-xs font-semibold text-white shadow-sm transition-all"
                                    >
                                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                            <path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
                                        </svg>
                                        Set Up Relay
                                    </button>
                                }
                            >
                                <button
                                    type="button"
                                    onClick={() => void handleStartTrial()}
                                    disabled={trialStarting()}
                                    class="inline-flex items-center gap-2 rounded-md bg-indigo-600 hover:bg-indigo-700 px-4 py-2 text-xs font-semibold text-white shadow-sm transition-all disabled:opacity-50"
                                >
                                    {trialStarting() ? (
                                        <>
                                            <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                                            </svg>
                                            Starting trial...
                                        </>
                                    ) : (
                                        'Start Free Trial & Set Up Mobile'
                                    )}
                                </button>
                            </Show>
                            <p class="mt-3 text-[10px] text-slate-500 dark:text-indigo-300 font-medium tracking-wide">14-DAY PRO TRIAL &middot; NO CREDIT CARD REQUIRED</p>
                        </div>
                    </div>
                </div>
            </Show>
        </div>
        </div>
    );
};
