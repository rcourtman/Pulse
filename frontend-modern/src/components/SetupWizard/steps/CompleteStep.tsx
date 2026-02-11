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
} from '@/utils/conversionEvents';
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
        <div class="text-center">
            {/* Success animation */}
            <div class="mb-5">
                <div class="inline-flex items-center justify-center w-14 h-14 rounded-full bg-green-500 shadow-xl shadow-green-500/30 mb-3">
                    <svg class="w-7 h-7 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                </div>
                <h1 class="text-xl font-bold text-white mb-1">
                    Security Configured
                </h1>
                <p class="text-sm text-green-200/80">
                    Install the Unified Agent on each host you want to monitor
                </p>
            </div>

            {/* Connected Agents - show when agents connect */}
            <Show when={connectedAgents().length > 0}>
                <div class="bg-green-500/20 backdrop-blur-xl rounded-xl border border-green-400/30 p-4 text-left mb-4">
                    <h3 class="text-sm font-semibold text-green-300 mb-2 flex items-center gap-2">
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                        </svg>
                        Connected ({connectedAgents().length} host{connectedAgents().length !== 1 ? 's' : ''})
                    </h3>
                    <div class="space-y-2">
                        <For each={connectedAgents()}>
                            {(agent) => (
                                <div class="flex items-center justify-between bg-black/20 rounded-lg px-3 py-2">
                                    <div class="flex items-center gap-2">
                                        <span class="w-2 h-2 bg-green-400 rounded-full animate-pulse"></span>
                                        <span class="text-white text-sm font-medium">{agent.name}</span>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        <span class="text-[10px] text-green-300/80 bg-green-500/20 px-1.5 py-0.5 rounded">{agent.type}</span>
                                        <Show when={agent.host}>
                                            <span class="text-[10px] text-white/40">{agent.host}</span>
                                        </Show>
                                    </div>
                                </div>
                            )}
                        </For>
                    </div>
                </div>
            </Show>

            {/* Auto-detection info */}
            <div class="bg-white/10 backdrop-blur-xl rounded-xl border border-white/20 p-4 text-left mb-4">
                <h3 class="text-sm font-semibold text-white mb-3 flex items-center gap-2">
                    <svg class="w-4 h-4 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                    </svg>
                    Smart Auto-Detection
                </h3>

                <p class="text-white/70 text-xs mb-3">
                    This is the primary onboarding path. The installer auto-detects platform integrations on each host:
                </p>

                <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 mb-3">
                    <div class="bg-white/5 border border-white/10 rounded-lg p-2 text-center">
                        <div class="h-6 flex items-center justify-center mb-1">
                            <span class="text-lg">🐳</span>
                        </div>
                        <div class="text-white font-medium text-xs">Docker</div>
                        <p class="text-[9px] text-white/40">Container monitoring</p>
                    </div>
                    <div class="bg-white/5 border border-white/10 rounded-lg p-2 text-center">
                        <div class="h-6 flex items-center justify-center mb-1">
                            <span class="text-lg">☸️</span>
                        </div>
                        <div class="text-white font-medium text-xs">Kubernetes</div>
                        <p class="text-[9px] text-white/40">Cluster monitoring</p>
                    </div>
                    <div class="bg-white/5 border border-white/10 rounded-lg p-2 text-center">
                        <div class="h-6 flex items-center justify-center mb-1">
                            <ProxmoxIcon class="w-5 h-5 text-orange-400" />
                        </div>
                        <div class="text-white font-medium text-xs">Proxmox</div>
                        <p class="text-[9px] text-white/40">VM & container API</p>
                    </div>
                    <div class="bg-white/5 border border-white/10 rounded-lg p-2 text-center">
                        <div class="h-6 flex items-center justify-center mb-1">
                            <span class="text-sm text-cyan-300 font-semibold">NAS</span>
                        </div>
                        <div class="text-white font-medium text-xs">TrueNAS</div>
                        <p class="text-[9px] text-white/40">SCALE-aware service install</p>
                    </div>
                </div>

                <div class="bg-green-500/10 border border-green-400/30 rounded-lg p-2.5">
                    <div class="flex items-center gap-2">
                        <div class="w-4 h-4 rounded-full bg-green-500 flex items-center justify-center">
                            <svg class="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                            </svg>
                        </div>
                        <div class="flex-1">
                            <div class="flex items-center gap-2">
                                <span class="text-white font-medium text-xs">Host Metrics</span>
                                <span class="text-[9px] text-green-300 bg-green-500/20 px-1.5 py-0.5 rounded">Always included</span>
                            </div>
                            <p class="text-[10px] text-white/50">CPU, memory, disk, network on any Linux/macOS/Windows</p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Agent installation */}
            <div class="bg-white/10 backdrop-blur-xl rounded-xl border border-white/20 p-4 text-left mb-4">
                <div class="flex items-center justify-between mb-2">
                    <h3 class="text-sm font-semibold text-white flex items-center gap-2">
                        <svg class="w-4 h-4 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                        </svg>
                        Install Command
                    </h3>
                    <button
                        onClick={() => {
                            void generateNewToken('manual');
                        }}
                        disabled={generatingToken()}
                        class="text-xs bg-blue-500/20 text-blue-300 hover:bg-blue-500/30 hover:text-blue-200 px-2 py-1 rounded flex items-center gap-1 disabled:opacity-50"
                        title="Generate a new token for the next host"
                    >
                        {generatingToken() ? (
                            <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                            </svg>
                        ) : (
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                            </svg>
                        )}
                        New Token
                    </button>
                </div>
                <p class="text-white/70 text-xs mb-2">
                    Copy and run on each host you want monitored. <span class="text-green-300">A new token is generated automatically after each copy.</span>
                </p>

                <div class="bg-black/40 rounded-lg p-2.5 font-mono text-[10px] mb-2 relative group">
                    <code class="text-green-400 break-all block leading-relaxed">{getInstallCommand()}</code>
                    <button
                        onClick={handleCopyInstall}
                        class="absolute top-1.5 right-1.5 p-1.5 bg-white/10 hover:bg-white/20 rounded-md transition-all"
                        title="Copy command"
                    >
                        {copied() === 'install' ? (
                            <svg class="w-3.5 h-3.5 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                            </svg>
                        ) : (
                            <svg class="w-3.5 h-3.5 text-white/60" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                            </svg>
                        )}
                    </button>
                </div>

                <p class="text-[10px] text-white/40">
                    Agents auto-register with Pulse. Install on as many hosts as you like.
                </p>
            </div>

            {/* Credentials section (collapsible) */}
            <div class="bg-white/5 backdrop-blur rounded-lg border border-white/10 mb-4 overflow-hidden">
                <button
                    onClick={() => setShowCredentials(!showCredentials())}
                    class="w-full p-2.5 flex items-center justify-between text-left hover:bg-white/5 transition-all"
                >
                    <div class="flex items-center gap-2">
                        <svg class="w-3.5 h-3.5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                        </svg>
                        <span class="text-white/80 text-xs">Your Credentials</span>
                        <span class="text-[10px] text-amber-300/80 bg-amber-500/20 px-1.5 py-0.5 rounded">Save these</span>
                    </div>
                    <svg class={`w-3.5 h-3.5 text-white/40 transition-transform ${showCredentials() ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                    </svg>
                </button>

                <Show when={showCredentials()}>
                    <div class="p-2.5 pt-0 space-y-1.5">
                        <div class="bg-black/20 rounded-md p-2">
                            <div class="text-[10px] text-white/50 mb-0.5">Username</div>
                            <div class="text-white font-mono text-xs">{props.state.username}</div>
                        </div>

                        <div class="bg-black/20 rounded-md p-2">
                            <div class="text-[10px] text-white/50 mb-0.5">Password</div>
                            <div class="flex items-center justify-between">
                                <code class="text-white font-mono text-xs break-all">{props.state.password}</code>
                                <button
                                    onClick={() => handleCopy('password')}
                                    class="ml-2 p-0.5 hover:bg-white/10 rounded transition-all text-white/40 hover:text-white"
                                >
                                    {copied() === 'password' ? (
                                        <svg class="w-3 h-3 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                        </svg>
                                    ) : (
                                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                                        </svg>
                                    )}
                                </button>
                            </div>
                        </div>

                        <div class="bg-black/20 rounded-md p-2">
                            <div class="text-[10px] text-white/50 mb-0.5">API Token (for web login)</div>
                            <div class="flex items-center justify-between">
                                <code class="text-white font-mono text-[10px] break-all">{props.state.apiToken}</code>
                                <button
                                    onClick={() => handleCopy('token')}
                                    class="ml-2 p-0.5 hover:bg-white/10 rounded transition-all text-white/40 hover:text-white"
                                >
                                    {copied() === 'token' ? (
                                        <svg class="w-3 h-3 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                        </svg>
                                    ) : (
                                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                                        </svg>
                                    )}
                                </button>
                            </div>
                        </div>

                        <button
                            onClick={downloadCredentials}
                            class="w-full py-1.5 text-[10px] text-blue-300 hover:text-blue-200 flex items-center justify-center gap-1 hover:bg-white/5 rounded-md transition-all"
                        >
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                            Download credentials
                        </button>
                    </div>
                </Show>
            </div>

            {/* Launch button */}
            <button
                onClick={props.onComplete}
                class="w-full py-2.5 px-5 bg-blue-500 hover:bg-blue-600 text-white text-sm font-medium rounded-lg transition-all shadow-lg shadow-blue-500/25"
            >
                {connectedAgents().length > 0 ? 'Go to Dashboard' : 'Skip for Now'}
            </button>

            <p class="mt-2 text-[10px] text-white/40">
                You can add more agents anytime from Settings
            </p>

            {/* Optional upsell after core agent onboarding is complete */}
            <Show when={connectedAgents().length > 0}>
                <div class="bg-gradient-to-r from-blue-500/20 to-purple-500/20 backdrop-blur-xl rounded-xl border border-blue-400/30 p-4 text-left mt-4">
                    <div class="flex items-start gap-3">
                        <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-blue-500 text-white shadow-lg shadow-blue-500/30 shrink-0">
                            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
                            </svg>
                        </div>
                        <div class="flex-1 min-w-0">
                            <h3 class="text-sm font-semibold text-white mb-1">Monitor from Anywhere</h3>
                            <p class="text-xs text-white/70 mb-3">
                                Get push notifications and manage your infrastructure from your phone with Pulse Relay.
                            </p>
                            <Show
                                when={!trialStarted() && entitlements()?.subscription_state !== 'trial'}
                                fallback={
                                    <button
                                        type="button"
                                        onClick={handleSetupRelay}
                                        class="inline-flex items-center gap-1.5 rounded-lg bg-blue-500 hover:bg-blue-600 px-3 py-1.5 text-xs font-medium text-white shadow-sm transition-all"
                                    >
                                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
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
                                    class="inline-flex items-center gap-1.5 rounded-lg bg-blue-500 hover:bg-blue-600 px-3 py-1.5 text-xs font-medium text-white shadow-sm transition-all disabled:opacity-50"
                                >
                                    {trialStarting() ? (
                                        <>
                                            <svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
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
                            <p class="mt-2 text-[10px] text-white/40">14-day Pro trial &middot; No credit card required</p>
                        </div>
                    </div>
                </div>
            </Show>
        </div>
    );
};
