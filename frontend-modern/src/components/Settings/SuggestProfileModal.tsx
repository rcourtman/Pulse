import { Component, createSignal, Show, For, createMemo, createEffect, on, onMount } from 'solid-js';
import { AgentProfilesAPI, type ProfileSuggestion, type ConfigKeyDefinition, type ConfigValidationResult } from '@/api/agentProfiles';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { formatRelativeTime } from '@/utils/format';
import { KNOWN_SETTINGS_BY_KEY } from './agentProfileSettings';
import Lightbulb from 'lucide-solid/icons/lightbulb';
import AlertCircle from 'lucide-solid/icons/alert-circle';
import Check from 'lucide-solid/icons/check';
import Loader2 from 'lucide-solid/icons/loader-2';

interface SuggestProfileModalProps {
    onClose: () => void;
    onSuggestionAccepted: (suggestion: ProfileSuggestion) => void;
}

interface SuggestionHistoryItem {
    id: string;
    prompt: string;
    suggestion: ProfileSuggestion;
    createdAt: number;
}

interface SettingPreviewItem {
    key: string;
    label: string;
    description: string;
    value: unknown;
    defaultValue?: unknown;
    type?: string;
    known: boolean;
}

const createHistoryId = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const toTitleCase = (value: string) =>
    value
        .split('_')
        .filter(Boolean)
        .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
        .join(' ');

const formatDisplayValue = (value: unknown) => {
    if (value === null || value === undefined) return 'unset';
    if (typeof value === 'boolean') return value ? 'Enabled' : 'Disabled';
    if (typeof value === 'string') return value === '' ? '(empty)' : value;
    if (typeof value === 'number') return String(value);
    return JSON.stringify(value);
};

const formatValueBadgeClass = (value: unknown) => {
    if (typeof value === 'boolean') {
        return value
            ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
            : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300';
    }
    return 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
};

const hasValue = (value: unknown) => value !== null && value !== undefined;

export const SuggestProfileModal: Component<SuggestProfileModalProps> = (props) => {
    const [prompt, setPrompt] = createSignal('');
    const [loading, setLoading] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [suggestion, setSuggestion] = createSignal<ProfileSuggestion | null>(null);
    const [schema, setSchema] = createSignal<ConfigKeyDefinition[]>([]);
    const [schemaError, setSchemaError] = createSignal<string | null>(null);
    const [validation, setValidation] = createSignal<ConfigValidationResult | null>(null);
    const [validationError, setValidationError] = createSignal<string | null>(null);
    const [history, setHistory] = createSignal<SuggestionHistoryItem[]>([]);
    const [activeHistoryId, setActiveHistoryId] = createSignal<string | null>(null);
    const [showDefaults, setShowDefaults] = createSignal(false);

    // Example prompts for inspiration
    const examplePrompts = [
        'Create a profile for production servers with minimal logging',
        'Profile for Docker hosts that need container monitoring',
        'Kubernetes monitoring profile with all pods visible',
        'Development environment profile with debug logging',
    ];

    const schemaByKey = createMemo(() => new Map(schema().map(def => [def.key, def])));

    const activeHistoryItem = createMemo(() => {
        const currentId = activeHistoryId();
        if (!currentId) return null;
        return history().find(item => item.id === currentId) || null;
    });

    const settingsPreview = createMemo<SettingPreviewItem[]>(() => {
        const current = suggestion();
        if (!current) return [];

        const config = current.config || {};
        const items = Object.entries(config).map(([key, value]) => {
            const known = KNOWN_SETTINGS_BY_KEY.get(key);
            const definition = schemaByKey().get(key);
            return {
                key,
                label: known?.label ?? toTitleCase(key),
                description: known?.description ?? definition?.description ?? 'No description available.',
                value,
                defaultValue: definition?.defaultValue,
                type: definition?.type,
                known: Boolean(known || definition),
            };
        });

        return items.sort((a, b) => {
            if (a.known !== b.known) return a.known ? -1 : 1;
            return a.key.localeCompare(b.key);
        });
    });

    const omittedDefaults = createMemo<ConfigKeyDefinition[]>(() => {
        const current = suggestion();
        if (!current || schema().length === 0) return [];
        const presentKeys = new Set(Object.keys(current.config || {}));
        return schema().filter(def => !presentKeys.has(def.key) && hasValue(def.defaultValue));
    });

    const riskHints = createMemo(() => {
        const current = suggestion();
        if (!current) return [];
        const config = current.config || {};
        const hints: string[] = [];

        if (config.disable_auto_update === true) {
            hints.push('Auto updates are disabled. Plan manual patching for agents.');
        }
        if (config.disable_docker_update_checks === true) {
            hints.push('Docker update checks are disabled. Update visibility will be limited.');
        }
        if (config.enable_host === false) {
            hints.push('Host monitoring is disabled. Host metrics and command execution will stop.');
        }
        if (config.enable_docker === false) {
            hints.push('Docker monitoring is disabled. Container metrics and update tracking will stop.');
        }
        if (config.disable_ceph === true) {
            hints.push('Ceph monitoring is disabled. Cluster health checks will be skipped.');
        }

        return hints;
    });

    onMount(async () => {
        try {
            const defs = await AgentProfilesAPI.getConfigSchema();
            setSchema(defs);
            setSchemaError(null);
        } catch (err) {
            logger.error('Failed to load agent profile schema', err);
            setSchema([]);
            setSchemaError('Defaults and validation details are unavailable right now.');
        }
    });

    let validationRequestId = 0;
    createEffect(on(suggestion, (current) => {
        validationRequestId += 1;
        const requestId = validationRequestId;

        if (!current) {
            setValidation(null);
            setValidationError(null);
            return;
        }

        setValidationError(null);
        (async () => {
            try {
                const result = await AgentProfilesAPI.validateConfig(current.config || {});
                if (requestId !== validationRequestId) return;
                setValidation(result);
            } catch (err) {
                if (requestId !== validationRequestId) return;
                logger.error('Failed to validate profile suggestion', err);
                setValidationError('Validation unavailable right now. You can still review the draft.');
                setValidation(null);
            }
        })();
    }));

    const handleSubmit = async () => {
        const userPrompt = prompt().trim();
        if (!userPrompt) {
            setError('Please enter a description for the profile you need');
            return;
        }

        setLoading(true);
        setError(null);

        try {
            const result = await AgentProfilesAPI.suggestProfile({
                prompt: userPrompt,
            });
            const historyEntry: SuggestionHistoryItem = {
                id: createHistoryId(),
                prompt: userPrompt,
                suggestion: result,
                createdAt: Date.now(),
            };
            setHistory(prev => [historyEntry, ...prev].slice(0, 5));
            setActiveHistoryId(historyEntry.id);
            setSuggestion(result);
            setShowDefaults(false);
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

    const handleSelectHistory = (item: SuggestionHistoryItem) => {
        setSuggestion(item.suggestion);
        setPrompt(item.prompt);
        setActiveHistoryId(item.id);
        setError(null);
        setShowDefaults(false);
    };

    return (
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black opacity-50">
            <div class="w-full max-w-2xl bg-surface rounded-md shadow-sm border border-border mx-4 max-h-[90vh] overflow-hidden flex flex-col">
                {/* Header */}
                <div class="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
                    <div class="flex items-center gap-3">
                        <div class="flex items-center justify-center w-8 h-8 rounded-md bg-amber-100 dark:bg-amber-900">
                            <Lightbulb class="w-4 h-4 text-amber-600 dark:text-amber-400" />
                        </div>
                        <div>
                            <h3 class="text-lg font-semibold text-base-content">
                                Profile Ideas
                            </h3>
                            <p class="text-xs text-muted">
                                Describe what you need, and we'll help draft a profile
                            </p>
                        </div>
                    </div>
                    <button
                        type="button"
                        onClick={props.onClose}
                        class="p-1.5 rounded-md text-slate-500 hover:text-slate-700 hover:bg-slate-100 dark:hover:text-slate-300 dark:hover:bg-slate-800"
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
                        <label class="block text-sm font-medium text-base-content">
                            What kind of profile do you need?
                        </label>
                        <textarea
                            value={prompt()}
                            onInput={(e) => setPrompt(e.currentTarget.value)}
                            placeholder="Describe the agents and use case for this profile..."
                            rows={3}
                            class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-blue-400 dark:focus:ring-blue-800 resize-none"
                            disabled={loading()}
                        />
                        <Show when={suggestion()}>
                            <p class="text-xs text-muted">
                                Tip: edit the prompt and click Regenerate to refine the draft.
                            </p>
                        </Show>
                    </div>

                    {/* Example Prompts */}
                    <Show when={!suggestion()}>
                        <div class="space-y-2">
                            <span class="text-xs text-muted">Examples:</span>
                            <div class="flex flex-wrap gap-2">
                                <For each={examplePrompts}>
                                    {(example) => (
                                        <button
                                            type="button"
                                            onClick={() => handleUseExample(example)}
                                            class="text-xs px-2 py-1 rounded-md text-slate-600 hover:bg-surface-alt dark:text-slate-400 dark:hover:bg-slate-700 transition-colors"
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
                        <div class="flex items-start gap-2 p-3 rounded-md bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800">
                            <AlertCircle class="w-4 h-4 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
                            <p class="text-sm text-red-700 dark:text-red-300">{error()}</p>
                        </div>
                    </Show>

                    {/* Loading State */}
                    <Show when={loading()}>
                        <div class="flex items-center justify-center py-4">
                            <Loader2 class="w-5 h-5 text-blue-600 dark:text-blue-400 animate-spin" />
                            <span class="ml-3 text-sm text-muted">Generating suggestion...</span>
                        </div>
                    </Show>

                    {/* Suggestion Result */}
                    <Show when={suggestion()}>
                        {(sugg) => (
                            <div class="space-y-4">
                                {/* Draft Warning */}
                                <div class="flex items-start gap-2 p-3 rounded-md bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800">
                                    <AlertCircle class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />
                                    <p class="text-sm text-amber-700 dark:text-amber-300">
                                        Draft suggestion — review settings before creating the profile.
                                    </p>
                                </div>

                                <Show when={validationError()}>
                                    <div class="flex items-start gap-2 p-3 rounded-md bg-surface-alt border border-border">
                                        <AlertCircle class="w-4 h-4 text-muted mt-0.5 shrink-0" />
                                        <p class="text-xs text-muted">{validationError()}</p>
                                    </div>
                                </Show>

                                <Show when={schemaError()}>
                                    <div class="flex items-start gap-2 p-3 rounded-md bg-surface-alt border border-border">
                                        <AlertCircle class="w-4 h-4 text-muted mt-0.5 shrink-0" />
                                        <p class="text-xs text-muted">{schemaError()}</p>
                                    </div>
                                </Show>

                                <Show when={validation()?.errors?.length}>
                                    <div class="flex items-start gap-2 p-3 rounded-md bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800">
                                        <AlertCircle class="w-4 h-4 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
                                        <div class="space-y-1">
                                            <p class="text-sm font-medium text-red-700 dark:text-red-300">
                                                Fix these before saving.
                                            </p>
                                            <ul class="text-xs text-red-700 dark:text-red-300 space-y-1">
                                                <For each={validation()?.errors || []}>
                                                    {(issue) => (
                                                        <li class="flex items-start gap-2">
                                                            <span class="font-mono text-red-700 dark:text-red-200">{issue.key}</span>
                                                            <span>{issue.message}</span>
                                                        </li>
                                                    )}
                                                </For>
                                            </ul>
                                        </div>
                                    </div>
                                </Show>

                                <Show when={validation()?.warnings?.length}>
                                    <div class="flex items-start gap-2 p-3 rounded-md bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800">
                                        <AlertCircle class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />
                                        <div class="space-y-1">
                                            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">
                                                Review before saving (some may be ignored).
                                            </p>
                                            <ul class="text-xs text-amber-700 dark:text-amber-300 space-y-1">
                                                <For each={validation()?.warnings || []}>
                                                    {(issue) => (
                                                        <li class="flex items-start gap-2">
                                                            <span class="font-mono text-amber-700 dark:text-amber-200">{issue.key}</span>
                                                            <span>{issue.message}</span>
                                                        </li>
                                                    )}
                                                </For>
                                            </ul>
                                        </div>
                                    </div>
                                </Show>

                                <Show when={riskHints().length > 0}>
                                    <div class="flex items-start gap-2 p-3 rounded-md bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800">
                                        <AlertCircle class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />
                                        <div class="space-y-1">
                                            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">
                                                Review checklist
                                            </p>
                                            <ul class="text-xs text-amber-700 dark:text-amber-300 space-y-1">
                                                <For each={riskHints()}>
                                                    {(hint) => <li>{hint}</li>}
                                                </For>
                                            </ul>
                                        </div>
                                    </div>
                                </Show>

                                {/* Profile Preview */}
                                <div class="rounded-md border border-border overflow-hidden">
                                    {/* Name & Description */}
                                    <div class="p-4 bg-surface-alt border-b border-border space-y-2">
                                        <div>
                                            <h4 class="font-medium text-base-content">{sugg().name}</h4>
                                            <p class="text-sm text-muted mt-1">{sugg().description}</p>
                                        </div>
                                        <Show when={activeHistoryItem()}>
                                            <div class="text-xs text-muted break-words">
                                                <span class="font-medium text-muted">Prompt:</span>{' '}
                                                {activeHistoryItem()?.prompt}
                                            </div>
                                        </Show>
                                    </div>

                                    {/* Settings Preview + JSON */}
                                    <div class="p-4 space-y-4">
                                        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                            <div class="space-y-3">
                                                <h5 class="text-sm font-medium text-base-content">Settings Preview</h5>
                                                <Show when={settingsPreview().length > 0} fallback={
                                                    <p class="text-xs text-muted">No settings were suggested.</p>
                                                }>
                                                    <div class="space-y-2">
                                                        <For each={settingsPreview()}>
                                                            {(setting) => (
                                                                <div class="rounded-md border border-border p-3">
                                                                    <div class="flex items-start justify-between gap-3">
                                                                        <div class="space-y-1">
                                                                            <div class="flex items-center gap-2 flex-wrap">
                                                                                <span class="text-sm font-medium text-base-content">
                                                                                    {setting.label}
                                                                                </span>
                                                                                <span class="text-[10px] font-mono text-muted">
                                                                                    {setting.key}
                                                                                </span>
                                                                                <Show when={!setting.known}>
                                                                                    <span class="text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300">
                                                                                        Unknown (ignored)
                                                                                    </span>
                                                                                </Show>
                                                                            </div>
                                                                            <p class="text-xs text-muted">{setting.description}</p>
                                                                        </div>
                                                                        <div class="text-right shrink-0">
                                                                            <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${formatValueBadgeClass(setting.value)}`}>
                                                                                {formatDisplayValue(setting.value)}
                                                                            </span>
                                                                            <Show when={schema().length > 0}>
                                                                                <div class="text-[11px] text-muted mt-1">
                                                                                    Default{' '}
                                                                                    {hasValue(setting.defaultValue)
                                                                                        ? formatDisplayValue(setting.defaultValue)
                                                                                        : 'n/a'}
                                                                                </div>
                                                                            </Show>
                                                                        </div>
                                                                    </div>
                                                                </div>
                                                            )}
                                                        </For>
                                                    </div>
                                                </Show>

                                                <Show when={schema().length > 0}>
                                                    <div class="rounded-md border border-border p-3">
                                                        <div class="flex items-start justify-between gap-3">
                                                            <div>
                                                                <h6 class="text-xs font-semibold text-base-content uppercase tracking-wide">
                                                                    Defaults unchanged
                                                                </h6>
                                                                <p class="text-xs text-muted mt-1">
                                                                    Only overrides are shown. {omittedDefaults().length} settings stay at defaults.
                                                                </p>
                                                            </div>
                                                            <button
                                                                type="button"
                                                                onClick={() => setShowDefaults(!showDefaults())}
                                                                class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                                            >
                                                                {showDefaults() ? 'Hide defaults' : 'Show defaults'}
                                                            </button>
                                                        </div>
                                                        <Show when={showDefaults()}>
                                                            <div class="mt-3 space-y-2">
                                                                <Show when={omittedDefaults().length > 0} fallback={
                                                                    <p class="text-xs text-muted">No defaults to show.</p>
                                                                }>
                                                                    <For each={omittedDefaults()}>
                                                                        {(def) => (
                                                                            <div class="flex items-center justify-between text-xs text-muted">
                                                                                <span class="font-mono">{def.key}</span>
                                                                                <span>{formatDisplayValue(def.defaultValue)}</span>
                                                                            </div>
                                                                        )}
                                                                    </For>
                                                                </Show>
                                                            </div>
                                                        </Show>
                                                    </div>
                                                </Show>
                                            </div>
                                            <div class="space-y-3">
                                                <h5 class="text-sm font-medium text-base-content">Raw JSON</h5>
                                                <div class="bg-slate-900 dark:bg-slate-950 rounded-md p-3 overflow-x-auto">
                                                    <pre class="text-xs text-slate-300 font-mono">
                                                        {JSON.stringify(sugg().config, null, 2)}
                                                    </pre>
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Rationale */}
                                    <Show when={sugg().rationale && sugg().rationale.length > 0}>
                                        <div class="p-4 border-t border-border space-y-2">
                                            <h5 class="text-sm font-medium text-base-content">Rationale</h5>
                                            <ul class="space-y-1">
                                                <For each={sugg().rationale}>
                                                    {(reason) => (
                                                        <li class="flex items-start gap-2 text-sm text-muted">
                                                            <Check class="w-4 h-4 text-emerald-400 mt-0.5 shrink-0" />
                                                            <span>{reason}</span>
                                                        </li>
                                                    )}
                                                </For>
                                            </ul>
                                        </div>
                                    </Show>
                                </div>

                                <Show when={history().length > 1}>
                                    <div class="rounded-md border border-border p-4 space-y-2">
                                        <div class="flex items-center justify-between">
                                            <h5 class="text-sm font-medium text-base-content">Recent drafts</h5>
                                            <span class="text-xs text-muted">
                                                {history().length - 1} older
                                            </span>
                                        </div>
                                        <p class="text-xs text-muted">
                                            Switch to a previous suggestion.
                                        </p>
                                        <div class="space-y-2">
                                            <For each={history().filter(item => item.id !== activeHistoryId())}>
                                                {(item) => (
                                                    <button
                                                        type="button"
                                                        onClick={() => handleSelectHistory(item)}
                                                        class="w-full text-left rounded-md border border-border p-3 hover:border-blue-300 hover:bg-blue-50 dark:hover:border-blue-700 dark:hover:bg-blue-900 transition-colors"
                                                    >
                                                        <div class="flex items-center justify-between gap-3">
                                                            <span class="text-sm font-medium text-base-content">
                                                                {item.suggestion.name}
                                                            </span>
                                                            <span class="text-xs text-muted">
                                                                {formatRelativeTime(item.createdAt)}
                                                            </span>
                                                        </div>
                                                        <p class="text-xs text-muted mt-1 truncate">{item.prompt}</p>
                                                    </button>
                                                )}
                                            </For>
                                        </div>
                                    </div>
                                </Show>
                            </div>
                        )}
                    </Show>
                </div>

                {/* Footer */}
                <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-border bg-surface-alt shrink-0">
                    <button
                        type="button"
                        onClick={props.onClose}
                        class="rounded-md px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
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
                                class="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                                <Lightbulb class="w-4 h-4" />
                                {loading() ? 'Generating...' : 'Get Ideas'}
                            </button>
                        }
                    >
                        <button
                            type="button"
                            onClick={handleSubmit}
                            disabled={loading() || !prompt().trim()}
                            class="inline-flex items-center gap-2 rounded-md px-4 py-2 text-sm font-medium text-slate-700 transition-colors hover:bg-surface-alt dark:text-slate-200 dark:hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-60"
                            title="Regenerate using the current prompt"
                        >
                            <Lightbulb class="w-4 h-4" />
                            {loading() ? 'Generating...' : 'Try Again'}
                        </button>
                        <button
                            type="button"
                            onClick={handleAccept}
                            disabled={loading()}
                            class="inline-flex items-center gap-2 rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-green-700 disabled:cursor-not-allowed disabled:opacity-60"
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
