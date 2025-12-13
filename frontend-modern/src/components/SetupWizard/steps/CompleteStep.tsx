import { Component, createSignal } from 'solid-js';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import type { WizardState } from '../SetupWizard';

interface CompleteStepProps {
    state: WizardState;
    onComplete: () => void;
}

export const CompleteStep: Component<CompleteStepProps> = (props) => {
    const [copied, setCopied] = createSignal<'password' | 'token' | null>(null);

    const handleCopy = async (type: 'password' | 'token') => {
        const value = type === 'password' ? props.state.password : props.state.apiToken;
        const success = await copyToClipboard(value);
        if (success) {
            setCopied(type);
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

⚠️ Keep these credentials secure!
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

    return (
        <div class="text-center">
            {/* Success animation */}
            <div class="mb-8">
                <div class="inline-flex items-center justify-center w-24 h-24 rounded-full bg-gradient-to-br from-green-500 to-emerald-600 shadow-2xl shadow-green-500/30 mb-6">
                    <svg class="w-12 h-12 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                </div>
                <h1 class="text-3xl font-bold text-white mb-3">
                    You're All Set! 🎉
                </h1>
                <p class="text-xl text-green-200/80">
                    Pulse is ready to monitor your infrastructure
                </p>
            </div>

            {/* Credentials box */}
            <div class="bg-white/10 backdrop-blur-xl rounded-2xl border border-white/20 p-6 text-left mb-6">
                <div class="flex items-center justify-between mb-4">
                    <h3 class="text-lg font-semibold text-white">Your Credentials</h3>
                    <button
                        onClick={downloadCredentials}
                        class="text-sm text-blue-300 hover:text-blue-200 flex items-center gap-1"
                    >
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                        </svg>
                        Download
                    </button>
                </div>

                <div class="space-y-3">
                    {/* Username */}
                    <div class="bg-black/20 rounded-xl p-3">
                        <div class="text-xs text-white/60 mb-1">Username</div>
                        <div class="text-white font-mono">{props.state.username}</div>
                    </div>

                    {/* Password */}
                    <div class="bg-black/20 rounded-xl p-3">
                        <div class="text-xs text-white/60 mb-1">Password</div>
                        <div class="flex items-center justify-between">
                            <code class="text-white font-mono text-sm break-all">{props.state.password}</code>
                            <button
                                onClick={() => handleCopy('password')}
                                class="ml-2 p-1.5 hover:bg-white/10 rounded transition-all"
                            >
                                {copied() === 'password' ? '✓' : '📋'}
                            </button>
                        </div>
                    </div>

                    {/* API Token */}
                    <div class="bg-black/20 rounded-xl p-3">
                        <div class="text-xs text-white/60 mb-1">API Token</div>
                        <div class="flex items-center justify-between">
                            <code class="text-white font-mono text-xs break-all">{props.state.apiToken}</code>
                            <button
                                onClick={() => handleCopy('token')}
                                class="ml-2 p-1.5 hover:bg-white/10 rounded transition-all"
                            >
                                {copied() === 'token' ? '✓' : '📋'}
                            </button>
                        </div>
                    </div>
                </div>

                <div class="mt-4 p-3 bg-amber-500/20 border border-amber-400/30 rounded-xl">
                    <p class="text-amber-200 text-sm">
                        ⚠️ Save these now — they won't be shown again!
                    </p>
                </div>
            </div>

            {/* Quick links */}
            <div class="grid grid-cols-3 gap-3 mb-8">
                <a href="/proxmox/overview" class="bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl p-4 text-center transition-all">
                    <div class="text-2xl mb-2">📊</div>
                    <div class="text-sm text-white/80">Dashboard</div>
                </a>
                <a href="/settings/proxmox" class="bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl p-4 text-center transition-all">
                    <div class="text-2xl mb-2">⚙️</div>
                    <div class="text-sm text-white/80">Add More Nodes</div>
                </a>
                <a href="/settings/system-ai" class="bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl p-4 text-center transition-all">
                    <div class="text-2xl mb-2">🤖</div>
                    <div class="text-sm text-white/80">Configure AI</div>
                </a>
            </div>

            {/* Launch button */}
            <button
                onClick={props.onComplete}
                class="w-full py-4 px-8 bg-gradient-to-r from-green-500 to-emerald-600 hover:from-green-600 hover:to-emerald-700 text-white text-lg font-medium rounded-xl transition-all shadow-lg shadow-green-500/25"
            >
                Launch Pulse →
            </button>
        </div>
    );
};
