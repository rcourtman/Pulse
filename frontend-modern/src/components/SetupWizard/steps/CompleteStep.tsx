import { Component, createSignal, createEffect, onCleanup, Show, For } from 'solid-js';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { SecurityAPI } from '@/api/security';
import type { WizardState } from '../SetupWizard';

interface CompleteStepProps {
    state: WizardState;
    onComplete: () => void;
}

type Platform = 'proxmox' | 'docker' | 'kubernetes' | 'host';

interface ConnectedAgent {
    id: string;
    name: string;
    type: string;
    host: string;
    addedAt: Date;
}

export const CompleteStep: Component<CompleteStepProps> = (props) => {
    const [copied, setCopied] = createSignal<'password' | 'token' | 'install' | null>(null);
    const [showCredentials, setShowCredentials] = createSignal(false);
    const [selectedPlatforms, setSelectedPlatforms] = createSignal<Platform[]>([]);
    const [connectedAgents, setConnectedAgents] = createSignal<ConnectedAgent[]>([]);
    const [currentInstallToken, setCurrentInstallToken] = createSignal(props.state.apiToken);
    const [generatingToken, setGeneratingToken] = createSignal(false);

    // Available optional platforms (host monitoring is always enabled)
    const platforms = [
        { id: 'proxmox' as Platform, name: 'Proxmox VE', desc: 'VMs & containers via API', icon: '🖥️' },
        { id: 'docker' as Platform, name: 'Docker', desc: 'Container monitoring', icon: '🐳' },
        { id: 'kubernetes' as Platform, name: 'Kubernetes', desc: 'Cluster monitoring', icon: '☸️' },
    ];

    // Poll for agent connections since WebSocket isn't available during setup
    createEffect(() => {
        let pollInterval: number | undefined;
        let previousCount = 0;

        const checkForAgents = async () => {
            try {
                console.log('[CompleteStep] Checking for agents with token:', props.state.apiToken?.slice(0, 8) + '...');

                const response = await fetch('/api/state', {
                    headers: {
                        'X-API-Token': props.state.apiToken,
                    },
                });

                console.log('[CompleteStep] API response status:', response.status);

                if (!response.ok) {
                    console.log('[CompleteStep] API returned non-OK status, skipping');
                    return;
                }

                const state = await response.json();
                const nodes = state.nodes || [];
                const hosts = state.hosts || [];

                console.log('[CompleteStep] Received:', { nodes: nodes.length, hosts: hosts.length, hostNames: hosts.map((h: any) => h.hostname || h.displayName) });

                // Check if we have new connections
                const totalAgents = nodes.length + hosts.length;

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
                        generateNewToken();
                    }

                    previousCount = totalAgents;
                }
            } catch (error) {
                console.error('Failed to check for agents:', error);
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

    const togglePlatform = (platform: Platform) => {
        const current = selectedPlatforms();
        if (current.includes(platform)) {
            setSelectedPlatforms(current.filter(p => p !== platform));
        } else {
            setSelectedPlatforms([...current, platform]);
        }
    };

    const generateNewToken = async () => {
        if (generatingToken()) return;

        setGeneratingToken(true);
        try {
            // Don't specify scopes - tokens without scopes default to wildcard access
            const result = await SecurityAPI.createToken(`agent-install-${Date.now()}`);
            if (result.token) {
                setCurrentInstallToken(result.token);
            }
        } catch (error) {
            console.error('Failed to generate new token:', error);
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
        // Host monitoring is always enabled by default, only add flags for optional integrations
        const platformFlags = selectedPlatforms()
            .filter(p => p !== 'host') // host is default, don't need flag
            .map(p => `--enable-${p}`)
            .join(' ');
        const flagsPart = platformFlags ? ` ${platformFlags}` : '';
        return `curl -sSL ${baseUrl}/install.sh | sudo bash -s -- --url "${baseUrl}" --token "${currentInstallToken()}"${flagsPart}`;
    };

    return (
        <div class="text-center">
            {/* Success animation */}
            <div class="mb-5">
                <div class="inline-flex items-center justify-center w-14 h-14 rounded-full bg-gradient-to-br from-green-500 to-emerald-600 shadow-xl shadow-green-500/30 mb-3">
                    <svg class="w-7 h-7 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                </div>
                <h1 class="text-xl font-bold text-white mb-1">
                    Security Configured
                </h1>
                <p class="text-sm text-green-200/80">
                    Install Pulse Agents on hosts you want to monitor
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

            {/* Platform selection */}
            <div class="bg-white/10 backdrop-blur-xl rounded-xl border border-white/20 p-4 text-left mb-4">
                <h3 class="text-sm font-semibold text-white mb-3">What does this host have?</h3>

                {/* Always included - Host monitoring */}
                <div class="bg-green-500/10 border border-green-400/30 rounded-lg p-2.5 mb-3">
                    <div class="flex items-center gap-2">
                        <div class="w-4 h-4 rounded-full bg-green-500 flex items-center justify-center">
                            <svg class="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                            </svg>
                        </div>
                        <div class="flex-1">
                            <div class="flex items-center gap-2">
                                <span class="text-white font-medium text-xs">Host Monitoring</span>
                                <span class="text-[9px] text-green-300 bg-green-500/20 px-1.5 py-0.5 rounded">Always included</span>
                            </div>
                            <p class="text-[10px] text-white/50">CPU, memory, disk, network on any Linux/macOS/Windows server</p>
                        </div>
                    </div>
                </div>

                {/* Optional integrations */}
                <p class="text-[10px] text-white/50 mb-2">Enable if this host runs:</p>
                <div class="grid grid-cols-3 gap-2">
                    <For each={platforms}>
                        {(platform) => (
                            <button
                                onClick={() => togglePlatform(platform.id)}
                                class={`p-2 rounded-lg text-left transition-all border ${selectedPlatforms().includes(platform.id)
                                    ? 'bg-blue-500/20 border-blue-400/50'
                                    : 'bg-white/5 border-white/10 hover:bg-white/10'
                                    }`}
                            >
                                <div class="flex items-center gap-1.5">
                                    <div class={`w-3.5 h-3.5 rounded border-2 flex items-center justify-center ${selectedPlatforms().includes(platform.id)
                                        ? 'bg-blue-500 border-blue-500'
                                        : 'border-white/40'
                                        }`}>
                                        <Show when={selectedPlatforms().includes(platform.id)}>
                                            <svg class="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                                                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                                            </svg>
                                        </Show>
                                    </div>
                                    <span class="text-white font-medium text-xs">{platform.name}</span>
                                </div>
                                <p class="text-[9px] text-white/40 ml-5 mt-0.5">{platform.desc}</p>
                            </button>
                        )}
                    </For>
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
                        onClick={generateNewToken}
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
                    Run on the host you want to monitor. <span class="text-amber-300">Use a different token for each host</span> - click "New Token" after each install.
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
                class="w-full py-2.5 px-5 bg-gradient-to-r from-blue-500 to-indigo-600 hover:from-blue-600 hover:to-indigo-700 text-white text-sm font-medium rounded-lg transition-all shadow-lg shadow-blue-500/25"
            >
                {connectedAgents().length > 0 ? 'Go to Dashboard' : 'Skip for Now'}
            </button>

            <p class="mt-2 text-[10px] text-white/40">
                You can add more agents anytime from Settings
            </p>
        </div>
    );
};
