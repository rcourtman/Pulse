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
    const [port, _setPort] = createSignal('8006');
    const [tokenId, setTokenId] = createSignal('');
    const [tokenSecret, setTokenSecret] = createSignal('');

    const platforms = [
        { id: 'proxmox' as Platform, name: 'Proxmox VE', desc: 'Hypervisor & containers', ports: ['8006'] },
        { id: 'docker' as Platform, name: 'Docker', desc: 'Container hosts', ports: ['2375', '2376'] },
        { id: 'kubernetes' as Platform, name: 'Kubernetes', desc: 'Container orchestration', ports: ['6443'] },
    ];

    const PlatformIcon: Component<{ platform: Platform; class?: string }> = (iconProps) => {
        const iconClass = iconProps.class || 'w-10 h-10';
        switch (iconProps.platform) {
            case 'proxmox':
                // Official Proxmox VE logo (from simple-icons)
                return (
                    <svg class={iconClass} viewBox="0 0 24 24" fill="#E57000">
                        <path d="M4.928 1.825c-1.09.553-1.09.64-.07 1.78 5.655 6.295 7.004 7.782 7.107 7.782.139.017 7.971-8.542 8.058-8.801.034-.07-.208-.312-.519-.536-.415-.312-.864-.433-1.712-.467-1.59-.104-2.144.242-4.115 2.455-.899 1.003-1.66 1.833-1.66 1.833-.017 0-.76-.813-1.642-1.798S8.473 2.1 8.127 1.91c-.796-.45-2.421-.484-3.2-.086zM1.297 4.367C.45 4.695 0 5.007 0 5.248c0 .121 1.331 1.678 2.94 3.459 1.625 1.78 2.939 3.268 2.939 3.302 0 .035-1.331 1.522-2.94 3.303C1.314 17.11.017 18.683.035 18.822c.086.467 1.504 1.055 2.541 1.055 1.678-.018 2.058-.312 5.603-4.202 1.78-1.954 3.233-3.614 3.233-3.666 0-.069-1.435-1.694-3.199-3.63-2.3-2.508-3.423-3.632-3.96-3.874-.812-.398-2.126-.467-2.956-.138zm18.467.12c-.502.26-1.764 1.505-3.943 3.891-1.763 1.937-3.199 3.562-3.199 3.631 0 .07 1.453 1.712 3.234 3.666 3.544 3.89 3.925 4.184 5.602 4.202 1.038 0 2.455-.588 2.542-1.055.017-.156-1.28-1.712-2.905-3.493-1.608-1.78-2.94-3.285-2.94-3.32 0-.034 1.332-1.539 2.94-3.32C22.72 6.91 24.017 5.352 24 5.214c-.087-.45-1.366-.968-2.473-1.038-.795-.034-1.21.035-1.763.312zM7.954 16.973c-2.144 2.369-3.908 4.374-3.943 4.46-.034.07.208.312.52.537.414.311.864.432 1.711.467 1.574.103 2.161-.26 4.15-2.508.864-.968 1.608-1.78 1.625-1.78s.761.812 1.643 1.798c2.023 2.248 2.559 2.576 4.132 2.49.848-.035 1.297-.156 1.712-.467.311-.225.553-.467.519-.536-.087-.26-7.92-8.819-8.058-8.801-.069 0-1.867 1.954-4.011 4.34z" />
                    </svg>
                );
            case 'docker':
                // Official Docker Moby whale logo
                return (
                    <svg class={iconClass} viewBox="0 0 24 24" fill="#2496ED">
                        <path d="M13.983 11.078h2.119a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.119a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 0 0 .186-.186V3.574a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 0 0 .186-.186V6.29a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.887c0 .102.082.185.185.186m-2.93 0h2.12a.186.186 0 0 0 .184-.186V6.29a.185.185 0 0 0-.185-.185H8.1a.185.185 0 0 0-.185.185v1.887c0 .102.083.185.185.186m-2.964 0h2.119a.186.186 0 0 0 .185-.186V6.29a.185.185 0 0 0-.185-.185H5.136a.186.186 0 0 0-.186.185v1.887c0 .102.084.185.186.186m5.893 2.715h2.118a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 0 0 .185-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.186.186 0 0 0-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 0 0-.75.748 11.376 11.376 0 0 0 .692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 0 0 3.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z" />
                    </svg>
                );
            case 'kubernetes':
                // Official Kubernetes logo (from simple-icons)
                return (
                    <svg class={iconClass} viewBox="0 0 24 24" fill="#326CE5">
                        <path d="M10.204 14.35l.007.01-.999 2.413a5.171 5.171 0 0 1-2.075-2.597l2.578-.437.004.005a.44.44 0 0 1 .484.606zm-.833-2.129a.44.44 0 0 0 .173-.756l.002-.011L7.585 9.7a5.143 5.143 0 0 0-.73 3.255l2.514-.725.002-.009zm1.145-1.98a.44.44 0 0 0 .699-.337l.01-.005.15-2.62a5.144 5.144 0 0 0-3.01 1.442l2.147 1.523.004-.002zm.76 2.75l.723.349.722-.347.18-.78-.5-.623h-.804l-.5.623.179.779zm1.5-3.095a.44.44 0 0 0 .7.336l.008.003 2.134-1.513a5.188 5.188 0 0 0-2.992-1.442l.148 2.615.002.001zm10.876 5.97l-5.773 7.181a1.6 1.6 0 0 1-1.248.594l-9.261.003a1.6 1.6 0 0 1-1.247-.596l-5.776-7.18a1.583 1.583 0 0 1-.307-1.34L2.1 5.573c.108-.47.425-.864.863-1.073L11.305.513a1.606 1.606 0 0 1 1.385 0l8.345 3.985c.438.209.755.604.863 1.073l2.062 8.955c.108.47-.005.963-.308 1.34zm-3.289-2.057c-.042-.01-.103-.026-.145-.034-.174-.033-.315-.025-.479-.038-.35-.037-.638-.067-.895-.148-.105-.04-.18-.165-.216-.216l-.201-.059a6.45 6.45 0 0 0-.105-2.332 6.465 6.465 0 0 0-.936-2.163c.052-.047.15-.133.177-.159.008-.09.001-.183.094-.282.197-.185.444-.338.743-.522.142-.084.273-.137.415-.242.032-.024.076-.062.11-.089.24-.191.295-.52.123-.736-.172-.216-.506-.236-.745-.045-.034.027-.08.062-.111.088-.134.116-.217.23-.33.35-.246.25-.45.458-.673.609-.097.056-.239.037-.303.033l-.19.135a6.545 6.545 0 0 0-4.146-2.003l-.012-.223c-.065-.062-.143-.115-.163-.25-.022-.268.015-.557.057-.905.023-.163.061-.298.068-.475.001-.04-.001-.099-.001-.142 0-.306-.224-.555-.5-.555-.275 0-.499.249-.499.555l.001.014c0 .041-.002.092 0 .128.006.177.044.312.067.475.042.348.078.637.056.906a.545.545 0 0 1-.162.258l-.012.211a6.424 6.424 0 0 0-4.166 2.003 8.373 8.373 0 0 1-.18-.128c-.09.012-.18.04-.297-.029-.223-.15-.427-.358-.673-.608-.113-.12-.195-.234-.329-.349-.03-.026-.077-.062-.111-.088a.594.594 0 0 0-.348-.132.481.481 0 0 0-.398.176c-.172.216-.117.546.123.737l.007.005.104.083c.142.105.272.159.414.242.299.185.546.338.743.522.076.082.09.226.1.288l.16.143a6.462 6.462 0 0 0-1.02 4.506l-.208.06c-.055.072-.133.184-.215.217-.257.081-.546.11-.895.147-.164.014-.305.006-.48.039-.037.007-.09.02-.133.03l-.004.002-.007.002c-.295.071-.484.342-.423.608.061.267.349.429.645.365l.007-.001.01-.003.129-.029c.17-.046.294-.113.448-.172.33-.118.604-.217.87-.256.112-.009.23.069.288.101l.217-.037a6.5 6.5 0 0 0 2.88 3.596l-.09.218c.033.084.069.199.044.282-.097.252-.263.517-.452.813-.091.136-.185.242-.268.399-.02.037-.045.095-.064.134-.128.275-.034.591.213.71.248.12.556-.007.69-.282v-.002c.02-.039.046-.09.062-.127.07-.162.094-.301.144-.458.132-.332.205-.68.387-.897.05-.06.13-.082.215-.105l.113-.205a6.453 6.453 0 0 0 4.609.012l.106.192c.086.028.18.042.256.155.136.232.229.507.342.84.05.156.074.295.145.457.016.037.043.09.062.129.133.276.442.402.69.282.247-.118.341-.435.213-.71-.02-.039-.045-.096-.065-.134-.083-.156-.177-.261-.268-.398-.19-.296-.346-.541-.443-.793-.04-.13.007-.21.038-.294-.018-.022-.059-.144-.083-.202a6.499 6.499 0 0 0 2.88-3.622c.064.01.176.03.213.038.075-.05.144-.114.28-.104.266.039.54.138.87.256.154.06.277.128.448.173.036.01.088.019.13.028l.009.003.007.001c.297.064.584-.098.645-.365.06-.266-.128-.537-.423-.608zM16.4 9.701l-1.95 1.746v.005a.44.44 0 0 0 .173.757l.003.01 2.526.728a5.199 5.199 0 0 0-.108-1.674A5.208 5.208 0 0 0 16.4 9.7zm-4.013 5.325a.437.437 0 0 0-.404-.232.44.44 0 0 0-.372.233h-.002l-1.268 2.292a5.164 5.164 0 0 0 3.326.003l-1.27-2.296h-.01zm1.888-1.293a.44.44 0 0 0-.27.036.44.44 0 0 0-.214.572l-.003.004 1.01 2.438a5.15 5.15 0 0 0 2.081-2.615l-2.6-.44-.004.005z" />
                    </svg>
                );
            default:
                return null;
        }
    };

    const runDiscovery = async () => {
        setIsScanning(true);
        setDiscoveredNodes([]);

        const headers: Record<string, string> = { 'Content-Type': 'application/json' };
        if (props.state.apiToken) {
            headers['X-API-Token'] = props.state.apiToken;
        }

        try {
            const response = await fetch('/api/discover', {
                method: 'POST',
                headers,
                credentials: 'include',
                body: JSON.stringify({ subnet: 'auto' }),
            });

            if (!response.ok) throw new Error('Discovery failed');

            // Poll for results
            for (let i = 0; i < 10; i++) {
                await new Promise(r => setTimeout(r, 2000));
                const results = await fetch('/api/discover/results', {
                    headers: props.state.apiToken ? { 'X-API-Token': props.state.apiToken } : {},
                    credentials: 'include',
                });
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
        } catch (_error) {
            showError('Discovery failed. Try manual setup.');
        } finally {
            setIsScanning(false);
        }
    };

    const connectNode = async (node?: DiscoveredNode) => {
        setIsConnecting(true);

        const headers: Record<string, string> = { 'Content-Type': 'application/json' };
        if (props.state.apiToken) {
            headers['X-API-Token'] = props.state.apiToken;
        }

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
                headers,
                credentials: 'include',
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
                                    <div class="flex justify-center mb-2"><PlatformIcon platform={platform.id} /></div>
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
                            <PlatformIcon platform="proxmox" class="w-6 h-6" />
                            <span class="text-white font-medium">Proxmox VE</span>
                        </div>

                        {/* Options */}
                        <div class="grid grid-cols-2 gap-3">
                            <button
                                onClick={runDiscovery}
                                disabled={isScanning()}
                                class="bg-blue-500/20 hover:bg-blue-500/30 border border-blue-400/30 rounded-xl p-4 text-left transition-all"
                            >
                                <div class="text-lg mb-1 flex items-center gap-2"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg> Auto-Discover</div>
                                <div class="text-sm text-white/60">Scan your network</div>
                            </button>
                            <button
                                onClick={() => setShowManualForm(true)}
                                class="bg-white/5 hover:bg-white/10 border border-white/20 rounded-xl p-4 text-left transition-all"
                            >
                                <div class="text-lg mb-1 flex items-center gap-2"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg> Manual Setup</div>
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
                        <div class="flex justify-center mb-4"><PlatformIcon platform="docker" class="w-12 h-12" /></div>
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
                        <div class="flex justify-center mb-4"><PlatformIcon platform="kubernetes" class="w-12 h-12" /></div>
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
