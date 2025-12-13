import { Component, createSignal, For, Show } from 'solid-js';
import { showError, showSuccess } from '@/utils/toast';
import type { WizardState } from '../SetupWizard';

interface ConnectStepProps {
    state: WizardState;
    updateState: (updates: Partial<WizardState>) => void;
    onNext: () => void;
    onBack: () => void;
    onSkip: () => void;
}

type Platform = 'proxmox' | 'docker' | 'kubernetes';

interface DiscoveredNode {
    ip: string;
    port: number;
    type: string;
    hostname?: string;
}

export const ConnectStep: Component<ConnectStepProps> = (props) => {
    const [selectedPlatform, setSelectedPlatform] = createSignal<Platform | null>(null);
    const [isScanning, setIsScanning] = createSignal(false);
    const [discoveredNodes, setDiscoveredNodes] = createSignal<DiscoveredNode[]>([]);
    const [showManualForm, setShowManualForm] = createSignal(false);
    const [isConnecting, setIsConnecting] = createSignal(false);

    // Manual form fields
    const [host, setHost] = createSignal('');
    const [port, setPort] = createSignal('8006');
    const [tokenId, setTokenId] = createSignal('');
    const [tokenSecret, setTokenSecret] = createSignal('');

    const platforms = [
        { id: 'proxmox' as Platform, name: 'Proxmox VE', icon: '🖥️', desc: 'Hypervisor & containers', ports: ['8006'] },
        { id: 'docker' as Platform, name: 'Docker', icon: '🐳', desc: 'Container hosts', ports: ['2375', '2376'] },
        { id: 'kubernetes' as Platform, name: 'Kubernetes', icon: '☸️', desc: 'Container orchestration', ports: ['6443'] },
    ];

    const runDiscovery = async () => {
        setIsScanning(true);
        setDiscoveredNodes([]);

        try {
            const response = await fetch('/api/discover', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ subnet: 'auto' }),
            });

            if (!response.ok) throw new Error('Discovery failed');

            // Poll for results
            for (let i = 0; i < 10; i++) {
                await new Promise(r => setTimeout(r, 2000));
                const results = await fetch('/api/discover/results');
                if (results.ok) {
                    const data = await results.json();
                    if (data.nodes && data.nodes.length > 0) {
                        setDiscoveredNodes(data.nodes.filter((n: DiscoveredNode) =>
                            selectedPlatform() === 'proxmox' ? ['pve', 'pbs', 'pmg'].includes(n.type) : true
                        ));
                        break;
                    }
                }
            }

            if (discoveredNodes().length === 0) {
                showSuccess('Scan complete - no nodes found. Try manual setup.');
            }
        } catch (error) {
            showError('Discovery failed. Try manual setup.');
        } finally {
            setIsScanning(false);
        }
    };

    const connectNode = async (node?: DiscoveredNode) => {
        setIsConnecting(true);

        try {
            const nodeData = node ? {
                type: node.type === 'pbs' ? 'pbs' : node.type === 'pmg' ? 'pmg' : 'pve',
                name: node.hostname || node.ip,
                host: node.ip,
                port: node.port,
            } : {
                type: 'pve',
                name: host().replace(/:\d+$/, ''),
                host: host(),
                port: parseInt(port()),
                tokenId: tokenId(),
                tokenValue: tokenSecret(),
            };

            const response = await fetch('/api/nodes', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(nodeData),
            });

            if (!response.ok) throw new Error(await response.text());

            props.updateState({ nodeAdded: true, nodeName: nodeData.name });
            showSuccess(`Connected to ${nodeData.name}!`);
            props.onNext();
        } catch (error) {
            showError(`Connection failed: ${error}`);
        } finally {
            setIsConnecting(false);
        }
    };

    return (
        <div class="bg-white/10 backdrop-blur-xl rounded-2xl border border-white/20 overflow-hidden">
            <div class="p-6 border-b border-white/10">
                <h2 class="text-2xl font-bold text-white">Connect Your Infrastructure</h2>
                <p class="text-white/70 mt-1">Add monitoring for your systems</p>
            </div>

            <div class="p-6">
                {/* Platform selection */}
                <Show when={!selectedPlatform()}>
                    <div class="grid grid-cols-3 gap-4 mb-6">
                        <For each={platforms}>
                            {(platform) => (
                                <button
                                    onClick={() => setSelectedPlatform(platform.id)}
                                    class="bg-white/5 hover:bg-white/10 border border-white/10 hover:border-white/30 rounded-xl p-4 text-center transition-all"
                                >
                                    <div class="text-3xl mb-2">{platform.icon}</div>
                                    <div class="text-white font-medium">{platform.name}</div>
                                    <div class="text-white/50 text-xs mt-1">{platform.desc}</div>
                                </button>
                            )}
                        </For>
                    </div>
                </Show>

                {/* Proxmox connection */}
                <Show when={selectedPlatform() === 'proxmox'}>
                    <div class="space-y-4">
                        <div class="flex items-center gap-2 mb-4">
                            <button
                                onClick={() => setSelectedPlatform(null)}
                                class="text-white/60 hover:text-white"
                            >
                                ←
                            </button>
                            <span class="text-xl">🖥️</span>
                            <span class="text-white font-medium">Proxmox VE</span>
                        </div>

                        {/* Options */}
                        <div class="grid grid-cols-2 gap-3">
                            <button
                                onClick={runDiscovery}
                                disabled={isScanning()}
                                class="bg-blue-500/20 hover:bg-blue-500/30 border border-blue-400/30 rounded-xl p-4 text-left transition-all"
                            >
                                <div class="text-lg mb-1">🔍 Auto-Discover</div>
                                <div class="text-sm text-white/60">Scan your network</div>
                            </button>
                            <button
                                onClick={() => setShowManualForm(true)}
                                class="bg-white/5 hover:bg-white/10 border border-white/20 rounded-xl p-4 text-left transition-all"
                            >
                                <div class="text-lg mb-1">✏️ Manual Setup</div>
                                <div class="text-sm text-white/60">Enter details</div>
                            </button>
                        </div>

                        {/* Scanning indicator */}
                        <Show when={isScanning()}>
                            <div class="bg-blue-500/10 border border-blue-400/20 rounded-xl p-4 text-center">
                                <div class="animate-spin inline-block w-6 h-6 border-2 border-blue-400 border-t-transparent rounded-full mb-2" />
                                <p class="text-blue-200">Scanning network...</p>
                            </div>
                        </Show>

                        {/* Discovered nodes */}
                        <Show when={discoveredNodes().length > 0}>
                            <div class="space-y-2">
                                <p class="text-sm text-white/60">Found {discoveredNodes().length} node(s):</p>
                                <For each={discoveredNodes()}>
                                    {(node) => (
                                        <button
                                            onClick={() => connectNode(node)}
                                            disabled={isConnecting()}
                                            class="w-full bg-green-500/10 hover:bg-green-500/20 border border-green-400/30 rounded-xl p-3 flex items-center justify-between transition-all"
                                        >
                                            <div class="text-left">
                                                <div class="text-white font-medium">{node.hostname || node.ip}</div>
                                                <div class="text-white/50 text-sm">{node.ip}:{node.port}</div>
                                            </div>
                                            <span class="text-green-400">Connect →</span>
                                        </button>
                                    )}
                                </For>
                            </div>
                        </Show>

                        {/* Manual form */}
                        <Show when={showManualForm()}>
                            <div class="space-y-3 bg-white/5 rounded-xl p-4">
                                <input
                                    type="text"
                                    value={host()}
                                    onInput={(e) => setHost(e.currentTarget.value)}
                                    class="w-full px-4 py-3 bg-white/10 border border-white/20 rounded-xl text-white placeholder-white/40"
                                    placeholder="Host (e.g., 192.168.1.100)"
                                />
                                <input
                                    type="text"
                                    value={tokenId()}
                                    onInput={(e) => setTokenId(e.currentTarget.value)}
                                    class="w-full px-4 py-3 bg-white/10 border border-white/20 rounded-xl text-white placeholder-white/40"
                                    placeholder="API Token ID (e.g., root@pam!pulse)"
                                />
                                <input
                                    type="password"
                                    value={tokenSecret()}
                                    onInput={(e) => setTokenSecret(e.currentTarget.value)}
                                    class="w-full px-4 py-3 bg-white/10 border border-white/20 rounded-xl text-white placeholder-white/40"
                                    placeholder="API Token Secret"
                                />
                                <button
                                    onClick={() => connectNode()}
                                    disabled={isConnecting() || !host()}
                                    class="w-full py-3 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white font-medium rounded-xl transition-all"
                                >
                                    {isConnecting() ? 'Connecting...' : 'Connect'}
                                </button>
                            </div>
                        </Show>
                    </div>
                </Show>

                {/* Docker placeholder */}
                <Show when={selectedPlatform() === 'docker'}>
                    <div class="text-center py-8">
                        <div class="text-4xl mb-4">🐳</div>
                        <p class="text-white/70 mb-4">
                            Docker monitoring requires the Pulse Agent.<br />
                            Install it on your Docker host after setup.
                        </p>
                        <button
                            onClick={props.onNext}
                            class="px-6 py-3 bg-blue-500 hover:bg-blue-600 text-white rounded-xl"
                        >
                            Continue Setup →
                        </button>
                    </div>
                </Show>

                {/* Kubernetes placeholder */}
                <Show when={selectedPlatform() === 'kubernetes'}>
                    <div class="text-center py-8">
                        <div class="text-4xl mb-4">☸️</div>
                        <p class="text-white/70 mb-4">
                            Kubernetes monitoring requires the Pulse Agent.<br />
                            Deploy via Helm after setup.
                        </p>
                        <button
                            onClick={props.onNext}
                            class="px-6 py-3 bg-blue-500 hover:bg-blue-600 text-white rounded-xl"
                        >
                            Continue Setup →
                        </button>
                    </div>
                </Show>
            </div>

            {/* Actions */}
            <div class="p-6 bg-black/20 flex gap-3">
                <button
                    onClick={props.onBack}
                    class="px-6 py-3 bg-white/10 hover:bg-white/20 text-white rounded-xl"
                >
                    ← Back
                </button>
                <div class="flex-1" />
                <button
                    onClick={props.onSkip}
                    class="px-6 py-3 bg-white/10 hover:bg-white/20 text-white/60 rounded-xl"
                >
                    Skip for now
                </button>
            </div>
        </div>
    );
};
