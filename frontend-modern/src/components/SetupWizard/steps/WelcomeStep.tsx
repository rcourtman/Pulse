import { Component, createSignal, Show } from 'solid-js';
import { showError, showSuccess } from '@/utils/toast';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

interface WelcomeStepProps {
    onNext: () => void;
    bootstrapToken: string;
    setBootstrapToken: (token: string) => void;
    isUnlocked: boolean;
    setIsUnlocked: (unlocked: boolean) => void;
}

export const WelcomeStep: Component<WelcomeStepProps> = (props) => {
    const [isValidating, setIsValidating] = createSignal(false);
    const [tokenPath, setTokenPath] = createSignal('');
    const [isDocker, setIsDocker] = createSignal(false);
    const [inContainer, setInContainer] = createSignal(false);
    const [lxcCtid, setLxcCtid] = createSignal('');

    // Fetch bootstrap info on mount
    const fetchBootstrapInfo = async () => {
        try {
            const response = await apiFetch('/api/security/status');
            if (response.ok) {
                const data = await response.json();
                if (data.bootstrapTokenPath) {
                    setTokenPath(data.bootstrapTokenPath);
                    setIsDocker(data.isDocker || false);
                    setInContainer(data.inContainer || false);
                    setLxcCtid(data.lxcCtid || '');
                }
            }
        } catch (error) {
            logger.error('Failed to fetch bootstrap info:', error);
        }
    };

    // Call on component load
    fetchBootstrapInfo();

    const handleUnlock = async () => {
        if (!props.bootstrapToken.trim()) {
            showError('Please enter the bootstrap token');
            return;
        }

        setIsValidating(true);
        try {
            await apiFetch('/api/security/validate-bootstrap-token', {
                method: 'POST',
                body: JSON.stringify({ token: props.bootstrapToken.trim() }),
            });

            props.setIsUnlocked(true);
            showSuccess('Token verified!');
            props.onNext();
        } catch (_error) {
            if (_error instanceof Error && _error.message !== 'Invalid bootstrap token') {
                // In case apiFetch throws other errors
            }
            showError('Invalid bootstrap token. Please check and try again.');
        } finally {
            setIsValidating(false);
        }
    };

    const getTokenCommand = () => {
        const path = tokenPath() || '/etc/pulse/.bootstrap_token';
        if (isDocker()) {
            return `docker exec <container> cat ${path}`;
        }
        if (inContainer() && lxcCtid()) {
            return `pct exec ${lxcCtid()} -- cat ${path}`;
        }
        if (inContainer()) {
            return `pct exec <ctid> -- cat ${path}`;
        }
        return `cat ${path}`;
    };

    return (
        <div class="text-center">
            {/* Logo */}
            <div class="mb-8">
                <div class="inline-flex items-center justify-center w-24 h-24 rounded-full bg-gradient-to-br from-blue-500 to-indigo-600 shadow-2xl shadow-blue-500/30 mb-6">
                    <svg width="56" height="56" viewBox="0 0 256 256" class="text-white">
                        <circle class="fill-current opacity-20" cx="128" cy="128" r="122" />
                        <circle class="fill-none stroke-current" stroke-width="14" cx="128" cy="128" r="84" opacity="0.9" />
                        <circle class="fill-current" cx="128" cy="128" r="26" />
                    </svg>
                </div>
                <h1 class="text-4xl font-bold text-white mb-3">
                    Welcome to Pulse
                </h1>
                <p class="text-xl text-blue-200/80">
                    Unified infrastructure monitoring
                </p>
            </div>

            {/* Bootstrap token unlock */}
            <Show when={!props.isUnlocked}>
                <div class="bg-white/10 backdrop-blur-xl rounded-2xl p-6 border border-white/20 text-left">
                    <h3 class="text-lg font-semibold text-white mb-2">Unlock Setup</h3>
                    <p class="text-sm text-white/70 mb-4">
                        Retrieve the bootstrap token from your host:
                    </p>
                    <div class="bg-black/30 rounded-lg p-3 font-mono text-sm text-green-400 mb-4">
                        {getTokenCommand()}
                    </div>
                    <input
                        type="text"
                        value={props.bootstrapToken}
                        onInput={(e) => props.setBootstrapToken(e.currentTarget.value)}
                        onKeyPress={(e) => e.key === 'Enter' && handleUnlock()}
                        class="w-full px-4 py-3 bg-white/10 border border-white/20 rounded-xl text-white placeholder-white/40 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
                        placeholder="Paste your bootstrap token"
                        autofocus
                    />
                    <button
                        onClick={handleUnlock}
                        disabled={isValidating() || !props.bootstrapToken.trim()}
                        class="w-full mt-4 py-3 px-6 bg-gradient-to-r from-blue-500 to-indigo-600 hover:from-blue-600 hover:to-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed text-white font-medium rounded-xl transition-all shadow-lg shadow-blue-500/25"
                    >
                        {isValidating() ? 'Validating...' : 'Continue →'}
                    </button>
                </div>
            </Show>

            <Show when={props.isUnlocked}>
                <button
                    onClick={props.onNext}
                    class="py-4 px-8 bg-gradient-to-r from-blue-500 to-indigo-600 hover:from-blue-600 hover:to-indigo-700 text-white text-lg font-medium rounded-xl transition-all shadow-lg shadow-blue-500/25"
                >
                    Get Started →
                </button>
            </Show>
        </div>
    );
};
