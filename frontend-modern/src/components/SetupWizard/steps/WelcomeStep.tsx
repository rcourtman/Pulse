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
            const data = await apiFetchJSON<{ bootstrapTokenPath?: string; isDocker?: boolean; inContainer?: boolean; lxcCtid?: string }>('/api/security/status');
            if (data?.bootstrapTokenPath) {
                setTokenPath(data.bootstrapTokenPath);
                setIsDocker(data.isDocker || false);
                setInContainer(data.inContainer || false);
                setLxcCtid(data.lxcCtid || '');
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
            const response = await apiFetch('/api/security/validate-bootstrap-token', {
                method: 'POST',
                body: JSON.stringify({ token: props.bootstrapToken.trim() }),
            });

            if (!response.ok) {
                throw new Error('Invalid bootstrap token');
            }

            props.setIsUnlocked(true);
            showSuccess('Token verified!');
            props.onNext();
        } catch (_error) {
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
        <div class="text-center relative">
            {/* Background Glow */}
            <div class="absolute inset-0 flex items-center justify-center pointer-events-none -z-10 opacity-50 dark:opacity-30">
                <div class="w-[500px] h-[500px] bg-blue-500 rounded-full blur-[120px] animate-pulse-slow"></div>
            </div>

            {/* Logo */}
            <div class="mb-10 relative z-10">
                <img
                    src="/logo.svg"
                    alt="Pulse Logo"
                    class="w-24 h-24 rounded-lg mb-8 mx-auto animate-pulse-logo shadow-xl dark:shadow-blue-900/20"
                />
                <h1 class="text-4xl sm:text-5xl font-bold tracking-tight text-slate-900 dark:text-white mb-4 animate-fade-in delay-100">
                    Welcome to Pulse
                </h1>
                <p class="text-xl text-slate-500 dark:text-blue-200/80 font-light animate-fade-in delay-200 max-w-md mx-auto">
                    Unified infrastructure intelligence
                </p>
            </div>

            {/* Bootstrap token unlock */}
            <Show when={!props.isUnlocked}>
                <div class="card p-8 max-w-lg mx-auto backdrop-blur-xl bg-white/70 dark:bg-slate-800/70 border border-slate-200/50 dark:border-slate-700/50 text-left shadow-2xl animate-slide-up delay-300 relative overflow-hidden group rounded-2xl">
                    <div class="absolute inset-0 bg-gradient-to-br from-white/40 to-transparent dark:from-white/5 dark:to-transparent pointer-events-none"></div>

                    <div class="relative z-10">
                        <h3 class="text-xl font-semibold text-slate-900 dark:text-white mb-2 tracking-tight">Unlock Setup</h3>
                        <p class="text-sm text-slate-500 dark:text-slate-300 mb-6">
                            Run the following command on your host to retrieve the secure bootstrap token:
                        </p>

                        <div class="bg-slate-50 dark:bg-black/40 rounded-xl p-4 font-mono text-sm text-slate-800 dark:text-emerald-400 mb-6 border border-slate-200/80 dark:border-slate-700/50 shadow-inner overflow-x-auto flex items-center">
                            <span class="opacity-50 select-none mr-3 text-slate-400 dark:text-slate-500">$</span>
                            <code class="whitespace-nowrap">{getTokenCommand()}</code>
                        </div>

                        <div class="space-y-4">
                            <input
                                type="text"
                                value={props.bootstrapToken}
                                onInput={(e) => props.setBootstrapToken(e.currentTarget.value)}
                                onKeyPress={(e) => e.key === 'Enter' && handleUnlock()}
                                class="w-full px-5 py-3.5 bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-700 rounded-xl text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all font-mono shadow-sm"
                                placeholder="Paste your bootstrap token"
                                autofocus
                            />

                            <button
                                onClick={handleUnlock}
                                disabled={isValidating() || !props.bootstrapToken.trim()}
                                class="w-full py-3.5 px-6 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:bg-slate-300 dark:disabled:bg-slate-700 disabled:text-slate-500 disabled:cursor-not-allowed text-white font-medium rounded-xl shadow-[0_4px_14px_0_rgba(37,99,235,0.39)] hover:shadow-[0_6px_20px_rgba(37,99,235,0.23)] transition-all flex justify-center items-center gap-2 duration-200"
                            >
                                {isValidating() ? (
                                    <>
                                        <svg class="animate-spin -ml-1 mr-2 h-5 w-5 text-white" fill="none" viewBox="0 0 24 24">
                                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                        </svg>
                                        Validating...
                                    </>
                                ) : 'Continue to Setup →'}
                            </button>
                        </div>
                    </div>
                </div>
            </Show>

            <Show when={props.isUnlocked}>
                <div class="animate-enter delay-200">
                    <button
                        onClick={props.onNext}
                        class="py-4 px-10 bg-blue-600 hover:bg-blue-700 text-white text-lg font-medium rounded-xl shadow-[0_4px_14px_0_rgba(37,99,235,0.39)] hover:shadow-[0_6px_20px_rgba(37,99,235,0.23)] transition-all duration-300 hover:-translate-y-1"
                    >
                        Get Started →
                    </button>
                </div>
            </Show>
        </div>
    );
};
