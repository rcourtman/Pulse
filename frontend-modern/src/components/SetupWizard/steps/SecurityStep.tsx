import { Component, createSignal, Show } from 'solid-js';
import { showError, showSuccess } from '@/utils/toast';
import { setApiToken as setApiClientToken, apiFetchJSON } from '@/utils/apiClient';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { WizardState } from '../SetupWizard';

interface SecurityStepProps {
    state: WizardState;
    updateState: (updates: Partial<WizardState>) => void;
    bootstrapToken: string;
    onNext: () => void;
    onBack: () => void;
}

export const SecurityStep: Component<SecurityStepProps> = (props) => {
    const [username, setUsername] = createSignal(props.state.username || 'admin');
    const [useCustomPassword, setUseCustomPassword] = createSignal(false);
    const [password, setPassword] = createSignal('');
    const [confirmPassword, setConfirmPassword] = createSignal('');
    const [isSettingUp, setIsSettingUp] = createSignal(false);

    const generatePassword = () => {
        const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';
        let pass = '';
        for (let i = 0; i < 16; i++) {
            pass += chars.charAt(Math.floor(Math.random() * chars.length));
        }
        return pass;
    };

    const generateToken = (): string => {
        const array = new Uint8Array(24);
        crypto.getRandomValues(array);
        return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
    };

    const handleSetup = async () => {
        if (useCustomPassword()) {
            if (!password()) {
                showError('Please enter a password');
                return;
            }
            if (password().length < 12) {
                showError('Password must be at least 12 characters');
                return;
            }
            if (password() !== confirmPassword()) {
                showError('Passwords do not match');
                return;
            }
        }

        setIsSettingUp(true);
        const finalPassword = useCustomPassword() ? password() : generatePassword();
        const token = generateToken();

        try {
            setApiClientToken(token);

            await apiFetchJSON('/api/security/quick-setup', {
                method: 'POST',
                headers: {
                    'X-Setup-Token': props.bootstrapToken,
                },
                body: JSON.stringify({
                    username: username(),
                    password: finalPassword,
                    apiToken: token,
                    force: false,
                    setupToken: props.bootstrapToken,
                }),
            });

            props.updateState({
                username: username(),
                password: finalPassword,
                apiToken: token,
            });

            if (typeof window !== 'undefined') {
                try {
                    sessionStorage.setItem(
                        STORAGE_KEYS.SETUP_CREDENTIALS,
                        JSON.stringify({
                            username: username(),
                            password: finalPassword,
                            apiToken: token,
                            createdAt: new Date().toISOString(),
                        }),
                    );
                } catch (_err) {
                    // Ignore storage errors (private browsing, quota limits, etc.)
                }
            }

            showSuccess('Security configured!');
            props.onNext();
        } catch (error) {
            showError(`Setup failed: ${error}`);
        } finally {
            setIsSettingUp(false);
        }
    };

    return (
        <div class="max-w-lg mx-auto bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 overflow-hidden animate-fade-in relative rounded-xl">            <div class="p-8 border-b border-slate-200 dark:border-slate-700 relative z-10 text-center">
            <h2 class="text-3xl font-bold tracking-tight text-slate-900 dark:text-white">Secure Your Dashboard</h2>
            <p class="text-slate-500 dark:text-blue-200/80 mt-2 font-light">Create your root administrator account</p>
        </div>

            <div class="p-8 space-y-6 relative z-10">
                {/* Username */}
                <div>
                    <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Username</label>
                    <input
                        type="text"
                        value={username()}
                        onInput={(e) => setUsername(e.currentTarget.value)}
                        class="w-full px-5 py-3.5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all font-mono"
                        placeholder="admin"
                    />
                </div>

                {/* Password choice */}
                <div>
                    <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-3">Password</label>
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
                        <button
                            type="button"
                            onClick={() => setUseCustomPassword(false)}
                            class={`py-3 px-4 rounded-lg text-sm font-medium transition-all duration-200 border ${!useCustomPassword()
                                ? 'bg-blue-600 border-blue-600 text-white'
                                : 'bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700'
                                }`}
                        >
                            Generate Secure
                        </button>
                        <button
                            type="button"
                            onClick={() => setUseCustomPassword(true)}
                            class={`py-3 px-4 rounded-lg text-sm font-medium transition-all duration-200 border ${useCustomPassword()
                                ? 'bg-blue-600 border-blue-600 text-white'
                                : 'bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700'
                                }`}
                        >
                            Custom Password
                        </button>
                    </div>

                    <Show when={useCustomPassword()}>
                        <div class="space-y-4 animate-fade-in">
                            <input
                                type="password"
                                value={password()}
                                onInput={(e) => setPassword(e.currentTarget.value)}
                                class="w-full px-5 py-3.5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all font-mono"
                                placeholder="Password"
                            />
                            <input
                                type="password"
                                value={confirmPassword()}
                                onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                                class="w-full px-5 py-3.5 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all font-mono"
                                placeholder="Confirm password"
                            />
                        </div>
                    </Show>

                    <Show when={!useCustomPassword()}>
                        <div class="bg-blue-50 dark:bg-slate-800 border border-blue-200 dark:border-blue-900/50 rounded-lg p-4 animate-fade-in shadow-inner">
                            <p class="text-sm text-blue-800 dark:text-blue-200 font-medium">
                                A secure 16-character password will be generated and shown after setup.
                            </p>
                        </div>
                    </Show>
                </div>

                {/* Info */}
                <div class="bg-slate-50 dark:bg-slate-900 rounded-lg p-4 border border-slate-200 dark:border-slate-700">
                    <p class="text-sm text-slate-600 dark:text-slate-300">
                        This creates your admin account and an API token for automation.
                        Credentials will be displayed once - save them securely!
                    </p>
                </div>
            </div>

            {/* Actions */}
            <div class="p-8 bg-slate-50 dark:bg-slate-900 flex gap-4 border-t border-slate-200 dark:border-slate-700 relative z-10">
                <button
                    onClick={props.onBack}
                    class="px-6 py-3.5 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 font-medium rounded-lg transition-all"
                >
                    ← Back
                </button>
                <button
                    onClick={handleSetup}
                    disabled={isSettingUp()}
                    class="flex-1 py-3.5 px-6 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:bg-slate-300 dark:disabled:bg-slate-700 disabled:text-slate-500 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-all flex justify-center items-center gap-2 duration-200"
                >
                    {isSettingUp() ? (
                        <>
                            <svg class="animate-spin -ml-1 mr-2 h-5 w-5 text-white" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                            Setting up...
                        </>
                    ) : (
                        'Create Account →'
                    )}
                </button>
            </div>
        </div>
    );
};
