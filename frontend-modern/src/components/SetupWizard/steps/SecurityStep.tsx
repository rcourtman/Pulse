import { Component, createSignal, Show } from 'solid-js';
import { showError } from '@/utils/toast';
import { setApiToken as setApiClientToken, apiFetchJSON } from '@/utils/apiClient';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { WizardState } from '../SetupWizard';

interface SecurityStepProps {
  state: WizardState;
  updateState: (updates: Partial<WizardState>) => void;
  bootstrapToken: string;
  onComplete: () => void;
  onBack: () => void;
}

const GENERATED_PASSWORD_LENGTH = 20;
const GENERATED_PASSWORD_CHARS =
  'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';

export const SecurityStep: Component<SecurityStepProps> = (props) => {
  const [username, setUsername] = createSignal(props.state.username || 'admin');
  const [useCustomPassword, setUseCustomPassword] = createSignal(false);
  const [password, setPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');
  const [showPassword, setShowPassword] = createSignal(false);
  const [isSettingUp, setIsSettingUp] = createSignal(false);

  const generatePassword = () => {
    const password: string[] = [];
    const maxUnbiasedByte = 256 - (256 % GENERATED_PASSWORD_CHARS.length);
    const randomBytes = new Uint8Array(GENERATED_PASSWORD_LENGTH);

    while (password.length < GENERATED_PASSWORD_LENGTH) {
      crypto.getRandomValues(randomBytes);
      for (const byte of randomBytes) {
        if (byte >= maxUnbiasedByte) continue;
        password.push(GENERATED_PASSWORD_CHARS[byte % GENERATED_PASSWORD_CHARS.length]);
        if (password.length === GENERATED_PASSWORD_LENGTH) break;
      }
    }

    return password.join('');
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
            STORAGE_KEYS.SETUP_HANDOFF,
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

      props.onComplete();
    } catch (error) {
      showError(`Setup failed: ${error}`);
    } finally {
      setIsSettingUp(false);
    }
  };

  return (
    <div class="max-w-lg mx-auto bg-surface border border-border overflow-hidden relative rounded-md">
      <div class="p-8 border-b border-border relative z-10 text-center">
        <h2 class="text-2xl font-bold tracking-tight text-base-content">
          Create your admin account
        </h2>
        <p class="text-sm text-muted mt-2">
          The next step is to choose the first infrastructure source.
        </p>
      </div>
      <div class="p-8 space-y-6 relative z-10">
        <div>
          <label class="block text-sm font-medium text-base-content mb-2">Username</label>
          <input
            type="text"
            value={username()}
            onInput={(e) => setUsername(e.currentTarget.value)}
            class="w-full px-5 py-3.5 bg-surface border border-border rounded-md text-base-content placeholder-slate-400 focus:outline-none focus:ring-0 focus:border-blue-500 transition-colors font-mono"
            placeholder="admin"
          />
        </div>

        <div>
          <label class="block text-sm font-medium text-base-content mb-3">Password</label>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
            <button
              type="button"
              onClick={() => setUseCustomPassword(false)}
              class={`py-3 px-4 rounded-md text-sm font-medium transition-colors border ${
                !useCustomPassword()
                  ? 'bg-blue-600 border-blue-600 text-white'
                  : 'bg-surface border-border text-muted hover:bg-surface-hover'
              }`}
            >
              Auto-generate
            </button>
            <button
              type="button"
              onClick={() => setUseCustomPassword(true)}
              class={`py-3 px-4 rounded-md text-sm font-medium transition-colors border ${
                useCustomPassword()
                  ? 'bg-blue-600 border-blue-600 text-white'
                  : 'bg-surface border-border text-muted hover:bg-surface-hover'
              }`}
            >
              Custom password
            </button>
          </div>

          <Show when={useCustomPassword()}>
            <div class="space-y-2">
              <div class="relative">
                <input
                  type={showPassword() ? 'text' : 'password'}
                  value={password()}
                  onInput={(e) => setPassword(e.currentTarget.value)}
                  class="w-full px-5 py-3.5 pr-20 bg-surface border border-border rounded-md text-base-content placeholder-slate-400 focus:outline-none focus:ring-0 focus:border-blue-500 transition-colors font-mono"
                  placeholder="Password (min 12 characters)"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword())}
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-xs font-medium text-muted hover:text-base-content transition-colors px-2 py-1"
                >
                  {showPassword() ? 'Hide' : 'Show'}
                </button>
              </div>
              <input
                type={showPassword() ? 'text' : 'password'}
                value={confirmPassword()}
                onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                class="w-full px-5 py-3.5 bg-surface border border-border rounded-md text-base-content placeholder-slate-400 focus:outline-none focus:ring-0 focus:border-blue-500 transition-colors font-mono"
                placeholder="Confirm password"
              />
              <p class="text-xs text-muted">Minimum 12 characters.</p>
            </div>
          </Show>

          <Show when={!useCustomPassword()}>
            <p class="text-sm text-muted">
              A secure 20-character password will be generated and shown on the next screen.
            </p>
          </Show>
        </div>

        <div class="bg-base rounded-md p-4 border border-border text-left">
          <div class="text-[11px] font-semibold uppercase tracking-wide text-muted mb-2">
            On the next screen
          </div>
          <ul class="text-sm text-muted space-y-1">
            <li>• Your username and password</li>
            <li>• An admin API token for automation</li>
          </ul>
          <p class="mt-2 text-xs text-muted">Save them before continuing, they are shown once.</p>
        </div>
      </div>
      {/* Actions */}
      <div class="p-8 bg-base flex gap-4 border-t border-border relative z-10">
        <button
          onClick={props.onBack}
          class="px-6 py-3.5 bg-surface border border-border hover:bg-surface-hover text-base-content font-medium rounded-md transition-colors"
        >
          ← Back
        </button>
        <button
          onClick={handleSetup}
          disabled={isSettingUp()}
          class="flex-1 py-3.5 px-6 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:bg-surface-alt disabled:text-muted disabled:cursor-not-allowed text-white font-medium rounded-md transition-colors flex justify-center items-center gap-2 duration-200"
        >
          {isSettingUp() ? (
            <>
              <svg
                class="animate-spin -ml-1 mr-2 h-5 w-5 text-white"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              Setting up...
            </>
          ) : (
            'Create Account & Continue →'
          )}
        </button>
      </div>
    </div>
  );
};
