import { Component, createSignal, Show, For } from 'solid-js';
import { AgentProfilesAPI, type ProfileSuggestion } from '@/api/agentProfiles';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import Sparkles from 'lucide-solid/icons/sparkles';
import AlertCircle from 'lucide-solid/icons/alert-circle';
import Check from 'lucide-solid/icons/check';
import Loader2 from 'lucide-solid/icons/loader-2';

interface SuggestProfileModalProps {
    onClose: () => void;
    onSuggestionAccepted: (suggestion: ProfileSuggestion) => void;
}

export const SuggestProfileModal: Component<SuggestProfileModalProps> = (props) => {
    const [prompt, setPrompt] = createSignal('');
    const [loading, setLoading] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [suggestion, setSuggestion] = createSignal<ProfileSuggestion | null>(null);

    // Example prompts for inspiration
    const examplePrompts = [
        'Create a profile for production servers with minimal logging',
        'Profile for Docker hosts that need container monitoring',
        'Kubernetes monitoring profile with all pods visible',
        'Development environment profile with debug logging',
    ];

    const handleSubmit = async () => {
        const userPrompt = prompt().trim();
        if (!userPrompt) {
            setError('Please enter a description for the profile you need');
            return;
        }

        setLoading(true);
        setError(null);
        setSuggestion(null);

        try {
            const result = await AgentProfilesAPI.suggestProfile({
                prompt: userPrompt,
            });
            setSuggestion(result);
        } catch (err) {
            logger.error('Failed to get profile suggestion', err);
            const message = err instanceof Error ? err.message : 'Failed to get suggestion';
            setError(message);
        } finally {
            setLoading(false);
        }
    };

    const handleAccept = () => {
        const currentSuggestion = suggestion();
        if (currentSuggestion) {
            props.onSuggestionAccepted(currentSuggestion);
            notificationStore.success(`Profile "${currentSuggestion.name}" ready to create`);
        }
    };

    const handleUseExample = (example: string) => {
        setPrompt(example);
    };

    return (
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div class="w-full max-w-2xl bg-white dark:bg-gray-900 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 mx-4 max-h-[90vh] overflow-hidden flex flex-col">
                {/* Header */}
                <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 shrink-0">
                    <div class="flex items-center gap-3">
                        <div class="flex items-center justify-center w-8 h-8 rounded-lg bg-purple-100 dark:bg-purple-900/30">
                            <Sparkles class="w-4 h-4 text-purple-600 dark:text-purple-400" />
                        </div>
                        <div>
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                                AI Profile Suggestion
                            </h3>
                            <p class="text-xs text-gray-500 dark:text-gray-400">
                                Describe what you need, and AI will draft a profile
                            </p>
                        </div>
                    </div>
                    <button
                        type="button"
                        onClick={props.onClose}
                        class="p-1.5 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:text-gray-300 dark:hover:bg-gray-800"
                    >
                        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                {/* Content */}
                <div class="px-6 py-4 space-y-4 overflow-y-auto flex-1">
                    {/* Prompt Input */}
                    <div class="space-y-2">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                            What kind of profile do you need?
                        </label>
                        <textarea
                            value={prompt()}
                            onInput={(e) => setPrompt(e.currentTarget.value)}
                            placeholder="Describe the agents and use case for this profile..."
                            rows={3}
                            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-purple-400 dark:focus:ring-purple-800/60 resize-none"
                            disabled={loading()}
                        />
                    </div>

                    {/* Example Prompts */}
                    <Show when={!suggestion()}>
                        <div class="space-y-2">
                            <span class="text-xs text-gray-500 dark:text-gray-400">Examples:</span>
                            <div class="flex flex-wrap gap-2">
                                <For each={examplePrompts}>
                                    {(example) => (
                                        <button
                                            type="button"
                                            onClick={() => handleUseExample(example)}
                                            class="text-xs px-2 py-1 rounded-md bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 transition-colors"
                                            disabled={loading()}
                                        >
                                            {example}
                                        </button>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>

                    {/* Error Message */}
                    <Show when={error()}>
                        <div class="flex items-start gap-2 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800">
                            <AlertCircle class="w-4 h-4 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
                            <p class="text-sm text-red-700 dark:text-red-300">{error()}</p>
                        </div>
                    </Show>

                    {/* Loading State */}
                    <Show when={loading()}>
                        <div class="flex items-center justify-center py-8">
                            <Loader2 class="w-6 h-6 text-purple-600 dark:text-purple-400 animate-spin" />
                            <span class="ml-3 text-gray-600 dark:text-gray-400">Generating suggestion...</span>
                        </div>
                    </Show>

                    {/* Suggestion Result */}
                    <Show when={suggestion()}>
                        {(sugg) => (
                            <div class="space-y-4">
                                {/* Draft Warning */}
                                <div class="flex items-start gap-2 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800">
                                    <AlertCircle class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />
                                    <p class="text-sm text-amber-700 dark:text-amber-300">
                                        This is a draft suggestion. Review the settings before creating the profile.
                                    </p>
                                </div>

                                {/* Profile Preview */}
                                <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
                                    {/* Name & Description */}
                                    <div class="p-4 bg-gray-50 dark:bg-gray-800/50 border-b border-gray-200 dark:border-gray-700">
                                        <h4 class="font-medium text-gray-900 dark:text-gray-100">{sugg().name}</h4>
                                        <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{sugg().description}</p>
                                    </div>

                                    {/* Config */}
                                    <div class="p-4 space-y-3">
                                        <h5 class="text-sm font-medium text-gray-700 dark:text-gray-300">Settings</h5>
                                        <div class="bg-gray-900 dark:bg-gray-950 rounded-md p-3 overflow-x-auto">
                                            <pre class="text-xs text-gray-300 font-mono">
                                                {JSON.stringify(sugg().config, null, 2)}
                                            </pre>
                                        </div>
                                    </div>

                                    {/* Rationale */}
                                    <Show when={sugg().rationale && sugg().rationale.length > 0}>
                                        <div class="p-4 border-t border-gray-200 dark:border-gray-700 space-y-2">
                                            <h5 class="text-sm font-medium text-gray-700 dark:text-gray-300">Rationale</h5>
                                            <ul class="space-y-1">
                                                <For each={sugg().rationale}>
                                                    {(reason) => (
                                                        <li class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-400">
                                                            <Check class="w-4 h-4 text-green-500 mt-0.5 shrink-0" />
                                                            <span>{reason}</span>
                                                        </li>
                                                    )}
                                                </For>
                                            </ul>
                                        </div>
                                    </Show>
                                </div>
                            </div>
                        )}
                    </Show>
                </div>

                {/* Footer */}
                <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 shrink-0">
                    <button
                        type="button"
                        onClick={props.onClose}
                        class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                    >
                        Cancel
                    </button>
                    <Show
                        when={suggestion()}
                        fallback={
                            <button
                                type="button"
                                onClick={handleSubmit}
                                disabled={loading() || !prompt().trim()}
                                class="inline-flex items-center gap-2 rounded-lg bg-purple-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-purple-700 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                                <Sparkles class="w-4 h-4" />
                                {loading() ? 'Generating...' : 'Suggest Profile'}
                            </button>
                        }
                    >
                        <button
                            type="button"
                            onClick={() => {
                                setSuggestion(null);
                                setError(null);
                            }}
                            class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                        >
                            Try Again
                        </button>
                        <button
                            type="button"
                            onClick={handleAccept}
                            class="inline-flex items-center gap-2 rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-green-700"
                        >
                            <Check class="w-4 h-4" />
                            Use This Profile
                        </button>
                    </Show>
                </div>
            </div>
        </div>
    );
};

export default SuggestProfileModal;
