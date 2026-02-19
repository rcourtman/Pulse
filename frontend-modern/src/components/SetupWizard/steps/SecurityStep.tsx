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
        <div class="bg-slate-800 rounded-md border border-slate-700 overflow-hidden shadow-sm">
            <div class="p-6 border-b border-slate-700">
                <h2 class="text-2xl font-bold text-white">Secure Your Dashboard</h2>
                <p class="text-slate-300 mt-1">Create your admin account</p>
            </div>

            <div class="p-6 space-y-6">
                {/* Username */}
                <div>
                    <label class="block text-sm font-medium text-slate-300 mb-2">Username</label>
                    <input
                        type="text"
                        value={username()}
                        onInput={(e) => setUsername(e.currentTarget.value)}
                        class="w-full px-4 py-3 bg-slate-900 border border-slate-700 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder="admin"
                    />
                </div>

                {/* Password choice */}
                <div>
                    <label class="block text-sm font-medium text-slate-300 mb-3">Password</label>
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
                        <button
                            type="button"
                            onClick={() => setUseCustomPassword(false)}
                            class={`py-3 px-4 rounded-md text-sm font-medium transition-all ${!useCustomPassword()
                                ? 'bg-blue-500 text-white'
                                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                                }`}
                        >
                            Generate Secure
                        </button>
                        <button
                            type="button"
                            onClick={() => setUseCustomPassword(true)}
                            class={`py-3 px-4 rounded-md text-sm font-medium transition-all ${useCustomPassword()
                                ? 'bg-blue-500 text-white'
                                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                                }`}
                        >
                            Custom Password
                        </button>
                    </div>

                    <Show when={useCustomPassword()}>
                        <div class="space-y-3">
                            <input
                                type="password"
                                value={password()}
                                onInput={(e) => setPassword(e.currentTarget.value)}
                                class="w-full px-4 py-3 bg-slate-900 border border-slate-700 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                placeholder="Password"
                            />
                            <input
                                type="password"
                                value={confirmPassword()}
                                onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                                class="w-full px-4 py-3 bg-slate-900 border border-slate-700 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                placeholder="Confirm password"
                            />
                        </div>
                    </Show>

                    <Show when={!useCustomPassword()}>
                        <div class="bg-blue-900/30 border border-blue-800 rounded-md p-4">
                            <p class="text-sm text-blue-200">
                                A secure 16-character password will be generated and shown after setup.
                            </p>
                        </div>
                    </Show>
                </div>

                {/* Info */}
                <div class="bg-slate-700/50 rounded-md p-4">
                    <p class="text-sm text-slate-300">
                        This creates your admin account and an API token for automation.
                        Credentials will be displayed once - save them securely!
                    </p>
                </div>
            </div>

            {/* Actions */}
            <div class="p-6 bg-slate-900/50 flex gap-3 border-t border-slate-700">
                <button
                    onClick={props.onBack}
                    class="px-6 py-3 bg-slate-700 hover:bg-slate-600 text-white rounded-md transition-all"
                >
                    ← Back
                </button>
                <button
                    onClick={handleSetup}
                    disabled={isSettingUp()}
                    class="flex-1 py-3 px-6 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white font-medium rounded-md transition-all"
                >
                    {isSettingUp() ? 'Setting up...' : 'Create Account →'}
                </button>
            </div>
        </div>
    );
};
