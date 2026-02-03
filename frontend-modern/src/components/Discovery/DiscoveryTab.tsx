import { Component, Show, For, createSignal, createResource, onCleanup, createEffect } from 'solid-js';
import type { ResourceType, DiscoveryProgress } from '../../types/discovery';
import {
    getDiscovery,
    triggerDiscovery,
    updateDiscoveryNotes,
    formatDiscoveryAge,
    getCategoryDisplayName,
    getConfidenceLevel,
} from '../../api/discovery';
import { GuestMetadataAPI } from '../../api/guestMetadata';
import { eventBus } from '../../stores/events';

interface DiscoveryTabProps {
    resourceType: ResourceType;
    hostId: string;
    resourceId: string;
    hostname: string;
    /** Canonical guest ID used for metadata API calls */
    guestId?: string;
    /** Current custom URL for this guest */
    customUrl?: string;
    /** Called after a URL is saved or deleted so the parent can update its state */
    onCustomUrlChange?: (url: string) => void;
}

// Construct the resource ID in the same format the backend uses
const makeResourceId = (type: ResourceType, hostId: string, resourceId: string) => {
    return `${type}:${hostId}:${resourceId}`;
};

export const DiscoveryTab: Component<DiscoveryTabProps> = (props) => {
    const [isScanning, setIsScanning] = createSignal(false);
    const [editingNotes, setEditingNotes] = createSignal(false);

    // --- Guest URL editing state ---
    const [urlValue, setUrlValue] = createSignal(props.customUrl ?? '');
    const [urlSaving, setUrlSaving] = createSignal(false);

    // Keep local value in sync with prop changes
    createEffect(() => {
        setUrlValue(props.customUrl ?? '');
    });

    const handleSaveUrl = async () => {
        if (!props.guestId) return;
        const trimmed = urlValue().trim();
        setUrlSaving(true);
        try {
            await GuestMetadataAPI.updateMetadata(props.guestId, { customUrl: trimmed });
            props.onCustomUrlChange?.(trimmed);
        } catch (err) {
            console.error('Failed to save guest URL:', err);
        } finally {
            setUrlSaving(false);
        }
    };

    const handleDeleteUrl = async () => {
        if (!props.guestId) return;
        setUrlSaving(true);
        try {
            await GuestMetadataAPI.updateMetadata(props.guestId, { customUrl: '' });
            setUrlValue('');
            props.onCustomUrlChange?.('');
        } catch (err) {
            console.error('Failed to remove guest URL:', err);
        } finally {
            setUrlSaving(false);
        }
    };
    const [notesText, setNotesText] = createSignal('');
    const [saveError, setSaveError] = createSignal<string | null>(null);
    const [scanError, setScanError] = createSignal<string | null>(null);
    const [scanProgress, setScanProgress] = createSignal<DiscoveryProgress | null>(null);
    const [scanSuccess, setScanSuccess] = createSignal(false);

    // Fetch discovery data
    const [discovery, { refetch, mutate }] = createResource(
        () => ({ type: props.resourceType, host: props.hostId, id: props.resourceId }),
        async (params) => {
            try {
                return await getDiscovery(params.type, params.host, params.id);
            } catch {
                return null;
            }
        }
    );

    // Handle triggering a new discovery
    const handleTriggerDiscovery = async (force = false) => {
        setIsScanning(true);
        setScanProgress(null);
        setScanError(null);
        setScanSuccess(false);
        try {
            // triggerDiscovery runs the discovery and returns the result
            await triggerDiscovery(props.resourceType, props.hostId, props.resourceId, {
                force,
                hostname: props.hostname,
            });
            // Scan complete - hide progress bar
            setIsScanning(false);
            // Refetch to get the saved discovery data (shows brief loading state)
            await refetch();
            // Now show success - data is already visible
            setScanSuccess(true);
            // Clear success after user has seen it
            setTimeout(() => setScanSuccess(false), 2000);
        } catch (err) {
            console.error('Discovery failed:', err);
            // Extract error message for display
            const message = err instanceof Error ? err.message : 'Discovery scan failed';
            // Provide helpful message if it's a connection issue
            if (message.includes('no connected agent')) {
                setScanError('No agent available. Enable "Pulse Commands" in Settings → Unified Agents for this host.');
            } else {
                setScanError(message);
            }
            setIsScanning(false);
            setScanProgress(null);
        }
    };

    // Handle saving notes
    const handleSaveNotes = async () => {
        setSaveError(null);
        try {
            await updateDiscoveryNotes(props.resourceType, props.hostId, props.resourceId, {
                user_notes: notesText(),
            });
            setEditingNotes(false);
            await refetch();
        } catch (err) {
            setSaveError(err instanceof Error ? err.message : 'Failed to save notes');
        }
    };

    // Start editing notes
    const startEditingNotes = () => {
        const currentNotes = discovery()?.user_notes || '';
        setNotesText(currentNotes);
        setEditingNotes(true);
    };

    // Subscribe to WebSocket progress updates
    const resourceId = () => makeResourceId(props.resourceType, props.hostId, props.resourceId);

    createEffect(() => {
        const unsubscribe = eventBus.on('ai_discovery_progress', (progress) => {
            // Only update if this progress is for our resource
            if (progress && progress.resource_id === resourceId()) {
                setScanProgress(progress);

                // If scan completed or failed, refresh the data and clear scanning state
                if (progress.status === 'completed' || progress.status === 'failed') {
                    setIsScanning(false);
                    // Fetch the updated discovery data
                    // Use a small delay to ensure the backend has persisted the data
                    setTimeout(async () => {
                        try {
                            const result = await getDiscovery(props.resourceType, props.hostId, props.resourceId);
                            if (result) {
                                mutate(result);
                            }
                        } catch (err) {
                            console.error('Failed to fetch discovery after completion:', err);
                        }
                        setScanProgress(null);
                    }, 500);
                }
            }
        });

        onCleanup(() => {
            unsubscribe();
        });
    });

    const confidenceInfo = () => {
        const d = discovery();
        if (!d || d.confidence === undefined || d.confidence === null) return null;
        return getConfidenceLevel(d.confidence);
    };

    // Check if discovery has meaningful data (not empty/unknown results)
    const hasValidDiscovery = () => {
        const d = discovery();
        if (!d) return false;
        // Consider valid if:
        // - Has a service name that's not empty and not "unknown" (case-insensitive)
        // - OR has a confidence value above 0
        // - OR has any discovered ports, facts, paths, etc.
        const hasServiceName = d.service_name && d.service_name.toLowerCase() !== 'unknown';
        const hasConfidence = typeof d.confidence === 'number' && d.confidence > 0;
        const hasPorts = d.ports && d.ports.length > 0;
        const hasFacts = d.facts && d.facts.length > 0;
        const hasPaths = (d.config_paths && d.config_paths.length > 0) ||
            (d.data_paths && d.data_paths.length > 0) ||
            (d.log_paths && d.log_paths.length > 0);
        const hasCliAccess = !!d.cli_access;

        return hasServiceName || hasConfidence || hasPorts || hasFacts || hasPaths || hasCliAccess;
    };

    // Accessor that returns discovery only when valid (for use with Show component)
    const validDiscovery = () => hasValidDiscovery() ? discovery() : null;

    return (
        <div class="space-y-4">
            {/* Loading state */}
            <Show when={discovery.loading}>
                <div class="flex items-center justify-center py-8">
                    <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full"></div>
                    <span class="ml-2 text-sm text-gray-500 dark:text-gray-400">Loading discovery...</span>
                </div>
            </Show>

            {/* Scan Progress Bar */}
            <Show when={scanProgress() && isScanning()}>
                <div class="rounded border border-blue-200 bg-blue-50 p-3 shadow-sm dark:border-blue-800 dark:bg-blue-900/30">
                    <div class="flex items-center justify-between mb-2">
                        <div class="flex items-center gap-2">
                            <div class="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full"></div>
                            <span class="text-sm font-medium text-blue-700 dark:text-blue-300">
                                {scanProgress()?.current_step || 'Scanning...'}
                            </span>
                        </div>
                        <span class="text-xs text-blue-600 dark:text-blue-400">
                            {Math.round(scanProgress()?.percent_complete || 0)}%
                        </span>
                    </div>
                    <div class="w-full bg-blue-200 dark:bg-blue-800 rounded-full h-2 overflow-hidden">
                        <div
                            class="bg-blue-500 h-2 rounded-full transition-all duration-300"
                            style={{ width: `${scanProgress()?.percent_complete || 0}%` }}
                        ></div>
                    </div>
                    <Show when={scanProgress()?.current_command}>
                        <div class="mt-2 text-xs text-blue-600 dark:text-blue-400">
                            Running: <code class="font-mono">{scanProgress()?.current_command}</code>
                        </div>
                    </Show>
                    <Show when={scanProgress()?.elapsed_ms}>
                        <div class="mt-1 text-xs text-blue-500 dark:text-blue-500">
                            Elapsed: {((scanProgress()?.elapsed_ms || 0) / 1000).toFixed(1)}s
                        </div>
                    </Show>
                </div>
            </Show>

            {/* Scan Success */}
            <Show when={scanSuccess()}>
                <div class="mb-4 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20">
                    <div class="flex items-center gap-2">
                        <svg class="w-5 h-5 text-green-500 dark:text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                        </svg>
                        <p class="text-sm font-medium text-green-800 dark:text-green-200">Discovery complete!</p>
                    </div>
                </div>
            </Show>

            {/* Scan Error */}
            <Show when={scanError()}>
                <div class="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-900/20">
                    <div class="flex items-start gap-3">
                        <svg class="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                        </svg>
                        <div>
                            <p class="text-sm font-medium text-red-800 dark:text-red-200">Discovery Failed</p>
                            <p class="text-sm text-red-700 dark:text-red-300 mt-1">{scanError()}</p>
                        </div>
                        <button
                            onClick={() => setScanError(null)}
                            class="ml-auto text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                        >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                </div>
            </Show>

            {/* No discovery yet */}
            <Show when={!discovery.loading && !discovery()}>
                <div class="text-center py-8">
                    <div class="text-gray-500 dark:text-gray-400 mb-4">
                        <svg class="w-12 h-12 mx-auto mb-2 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                        </svg>
                        <p class="text-sm">No discovery data yet</p>
                        <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">
                            Run a discovery scan to identify services and configurations
                        </p>
                    </div>
                    <button
                        onClick={() => handleTriggerDiscovery(true)}
                        disabled={isScanning()}
                        class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                        {isScanning() ? (
                            <span class="flex items-center">
                                <span class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full mr-2"></span>
                                Scanning...
                            </span>
                        ) : (
                            'Run Discovery'
                        )}
                    </button>
                </div>
            </Show>

            {/* Discovery exists but has no meaningful data - show re-scan option */}
            <Show when={!discovery.loading && discovery() && !hasValidDiscovery()}>
                <div class="text-center py-8">
                    <div class="text-gray-500 dark:text-gray-400 mb-4">
                        <svg class="w-12 h-12 mx-auto mb-2 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <p class="text-sm">Unknown Service</p>
                        <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">
                            Discovery completed but couldn't identify a known service.
                        </p>
                        <Show when={discovery()?.updated_at}>
                            <p class="text-xs text-gray-400 dark:text-gray-500 mt-2">
                                Last scanned: {formatDiscoveryAge(discovery()!.updated_at)}
                            </p>
                        </Show>
                    </div>
                    <button
                        onClick={() => handleTriggerDiscovery(true)}
                        disabled={isScanning()}
                        class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                        {isScanning() ? (
                            <span class="flex items-center justify-center">
                                <span class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full mr-2"></span>
                                Scanning...
                            </span>
                        ) : (
                            'Re-scan Discovery'
                        )}
                    </button>
                </div>
            </Show>

            {/* Discovery data with valid results */}
            <Show when={validDiscovery()}>
                {(d) => (
                    <div class="space-y-4">
                        {/* Service Header */}
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="flex items-start justify-between">
                                <div>
                                    <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                                        {d().service_name || 'Unknown Service'}
                                    </h3>
                                    <Show when={d().service_version}>
                                        <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                                            Version {d().service_version}
                                        </p>
                                    </Show>
                                </div>
                                <Show when={d().category && d().category !== 'unknown'}>
                                    <span class="inline-block rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                        {getCategoryDisplayName(d().category)}
                                    </span>
                                </Show>
                            </div>

                            <Show when={confidenceInfo()}>
                                <p class={`text-xs mt-2 ${confidenceInfo()!.color}`}>
                                    {confidenceInfo()!.label} ({Math.round((d().confidence || 0) * 100)}%)
                                </p>
                            </Show>
                        </div>

                        {/* CLI Access */}
                        <Show when={d().cli_access}>
                            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">
                                    CLI Access
                                </div>
                                <code class="block bg-gray-100 dark:bg-gray-800 rounded px-2 py-1.5 text-xs text-gray-800 dark:text-gray-200 font-mono overflow-x-auto">
                                    {d().cli_access}
                                </code>
                            </div>
                        </Show>

                        {/* Configuration, Data & Log Paths */}
                        <Show when={d().config_paths?.length > 0 || d().data_paths?.length > 0 || d().log_paths?.length > 0}>
                            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                <Show when={d().config_paths?.length > 0}>
                                    <div class="mb-3">
                                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-1">
                                            Config Paths
                                        </div>
                                        <div class="space-y-1">
                                            <For each={d().config_paths}>
                                                {(path) => (
                                                    <code class="block text-xs text-gray-600 dark:text-gray-300 font-mono">
                                                        {path}
                                                    </code>
                                                )}
                                            </For>
                                        </div>
                                    </div>
                                </Show>
                                <Show when={d().data_paths?.length > 0}>
                                    <div class="mb-3">
                                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-1">
                                            Data Paths
                                        </div>
                                        <div class="space-y-1">
                                            <For each={d().data_paths}>
                                                {(path) => (
                                                    <code class="block text-xs text-gray-600 dark:text-gray-300 font-mono">
                                                        {path}
                                                    </code>
                                                )}
                                            </For>
                                        </div>
                                    </div>
                                </Show>
                                <Show when={d().log_paths?.length > 0}>
                                    <div>
                                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-1">
                                            Log Paths
                                        </div>
                                        <div class="space-y-1">
                                            <For each={d().log_paths}>
                                                {(path) => (
                                                    <code class="block text-xs text-gray-600 dark:text-gray-300 font-mono">
                                                        {path}
                                                    </code>
                                                )}
                                            </For>
                                        </div>
                                    </div>
                                </Show>
                            </div>
                        </Show>

                        {/* Ports */}
                        <Show when={d().ports?.length > 0}>
                            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">
                                    Listening Ports
                                </div>
                                <div class="flex flex-wrap gap-1">
                                    <For each={d().ports}>
                                        {(port) => (
                                            <span class="inline-block rounded bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                                                {port.port}/{port.protocol}
                                                <Show when={port.process}>
                                                    <span class="text-gray-500 dark:text-gray-400 ml-1">({port.process})</span>
                                                </Show>
                                            </span>
                                        )}
                                    </For>
                                </div>
                            </div>
                        </Show>

                        {/* Key Facts */}
                        <Show when={d().facts?.length > 0}>
                            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">
                                    Discovered Facts
                                </div>
                                <div class="space-y-1.5">
                                    <For each={d().facts.slice(0, 8)}>
                                        {(fact) => (
                                            <div class="flex items-center justify-between text-xs">
                                                <span class="text-gray-600 dark:text-gray-400">{fact.key}</span>
                                                <span class="font-medium text-gray-800 dark:text-gray-200 truncate ml-2 max-w-[60%]" title={fact.value}>
                                                    {fact.value}
                                                </span>
                                            </div>
                                        )}
                                    </For>
                                </div>
                            </div>
                        </Show>

                        {/* User Notes */}
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="flex items-center justify-between mb-2">
                                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                                    Your Notes
                                </div>
                                <Show when={!editingNotes()}>
                                    <button
                                        onClick={startEditingNotes}
                                        class="text-xs text-blue-600 dark:text-blue-400 hover:underline"
                                    >
                                        {d().user_notes ? 'Edit' : 'Add notes'}
                                    </button>
                                </Show>
                            </div>

                            <Show
                                when={editingNotes()}
                                fallback={
                                    <Show
                                        when={d().user_notes}
                                        fallback={
                                            <p class="text-xs text-gray-400 dark:text-gray-500 italic">
                                                No notes yet. Add notes to document important information.
                                            </p>
                                        }
                                    >
                                        <p class="text-xs text-gray-600 dark:text-gray-300 whitespace-pre-wrap">
                                            {d().user_notes}
                                        </p>
                                    </Show>
                                }
                            >
                                <div class="space-y-2">
                                    <textarea
                                        value={notesText()}
                                        onInput={(e) => setNotesText(e.currentTarget.value)}
                                        placeholder="Add notes about this resource (API tokens, passwords, important info)..."
                                        class="w-full h-24 px-2 py-1.5 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
                                    />
                                    <Show when={saveError()}>
                                        <p class="text-xs text-red-600 dark:text-red-400">{saveError()}</p>
                                    </Show>
                                    <div class="flex gap-2">
                                        <button
                                            onClick={handleSaveNotes}
                                            class="px-3 py-1 bg-blue-600 text-white text-xs rounded hover:bg-blue-700 transition-colors"
                                        >
                                            Save
                                        </button>
                                        <button
                                            onClick={() => setEditingNotes(false)}
                                            class="px-3 py-1 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs rounded hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
                                        >
                                            Cancel
                                        </button>
                                    </div>
                                </div>
                            </Show>
                        </div>

                        {/* AI Reasoning (collapsible) */}
                        <Show when={d().ai_reasoning}>
                            <details class="rounded border border-gray-200 bg-white/70 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                <summary class="p-3 text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/50">
                                    AI Reasoning
                                </summary>
                                <div class="px-3 pb-3">
                                    <p class="text-xs text-gray-600 dark:text-gray-300">
                                        {d().ai_reasoning}
                                    </p>
                                </div>
                            </details>
                        </Show>

                        {/* Web Interface URL */}
                        <Show when={props.guestId}>
                            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">
                                    Web Interface URL
                                </div>
                                <div class="flex items-center gap-2">
                                    <input
                                        type="url"
                                        class="flex-1 text-xs px-2.5 py-1.5 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                                        placeholder="https://192.168.1.100:8080"
                                        value={urlValue()}
                                        onInput={(e) => setUrlValue(e.currentTarget.value)}
                                        onKeyDown={(e) => { if (e.key === 'Enter') handleSaveUrl(); }}
                                        disabled={urlSaving()}
                                    />
                                    <button
                                        type="button"
                                        class="px-2.5 py-1.5 text-xs font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
                                        disabled={urlSaving() || urlValue().trim() === (props.customUrl ?? '')}
                                        onClick={handleSaveUrl}
                                    >
                                        Save
                                    </button>
                                    <Show when={props.customUrl}>
                                        <button
                                            type="button"
                                            class="px-2.5 py-1.5 text-xs font-medium rounded-md text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 disabled:opacity-50 transition-colors"
                                            disabled={urlSaving()}
                                            onClick={handleDeleteUrl}
                                            title="Remove URL"
                                        >
                                            Remove
                                        </button>
                                    </Show>
                                </div>
                                <p class="mt-1.5 text-[10px] text-gray-400 dark:text-gray-500">
                                    Add a URL to quickly access this guest's web interface from the dashboard.
                                </p>
                            </div>
                        </Show>

                        {/* Footer with Update button */}
                        <div class="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
                            <span class="text-xs text-gray-500 dark:text-gray-400">
                                Last updated: {formatDiscoveryAge(d().updated_at)}
                            </span>
                            <button
                                onClick={() => handleTriggerDiscovery(true)}
                                disabled={isScanning()}
                                class="px-3 py-1.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs rounded hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center gap-1.5"
                            >
                                <Show
                                    when={isScanning()}
                                    fallback={
                                        <>
                                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                            </svg>
                                            Update Discovery
                                        </>
                                    }
                                >
                                    <span class="animate-spin h-3.5 w-3.5 border-2 border-gray-500 border-t-transparent rounded-full"></span>
                                    Scanning...
                                </Show>
                            </button>
                        </div>
                    </div>
                )}
            </Show>
        </div>
    );
};

export default DiscoveryTab;
