import { Component, Show, createSignal, createEffect, createMemo } from 'solid-js';
import type { DockerContainerUpdateStatus } from '@/types/api';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import { MonitoringAPI } from '@/api/monitoring';
import {
    getContainerUpdateState,
    markContainerQueued,
    markContainerUpdateSuccess,
    markContainerUpdateError,
    clearContainerUpdateState,
    updateStates
} from '@/stores/containerUpdates';
import { shouldHideDockerUpdateActions, areSystemSettingsLoaded } from '@/stores/systemSettings';



interface UpdateBadgeProps {
    updateStatus?: DockerContainerUpdateStatus;
    compact?: boolean;
}

/**
 * UpdateBadge displays a visual indicator when a container image has an update available.
 * Uses a blue color scheme to differentiate from health/status badges.
 */
export const UpdateBadge: Component<UpdateBadgeProps> = (props) => {
    const hasUpdate = () => props.updateStatus?.updateAvailable === true;
    const hasError = () => Boolean(props.updateStatus?.error);

    return (
        <Show when={hasUpdate() || hasError()}>
            <Show
                when={hasUpdate()}
                fallback={
                    // Show subtle error indicator if check failed
                    <Show when={hasError()}>
                        <span
                            class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-surface-alt text-slate-500 dark:text-slate-400 cursor-help"
                            onMouseEnter={(e) => {
                                const rect = e.currentTarget.getBoundingClientRect();
                                showTooltip(`Update check failed: ${props.updateStatus?.error}`, rect.left + rect.width / 2, rect.top, {
                                    align: 'center',
                                    direction: 'up'
                                });
                            }}
                            onMouseLeave={() => hideTooltip()}
                        >
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                            <Show when={!props.compact}>
                                <span>Check failed</span>
                            </Show>
                        </span>
                    </Show>
                }
            >
                <span
                    class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-help"
                    onMouseEnter={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        const current = props.updateStatus?.currentDigest?.slice(0, 19) || 'unknown';
                        const latest = props.updateStatus?.latestDigest?.slice(0, 19) || 'unknown';
                        const content = `Image update available\nCurrent: ${current}...\nLatest: ${latest}...`;
                        showTooltip(content, rect.left + rect.width / 2, rect.top, {
                            align: 'center',
                            direction: 'up'
                        });
                    }}
                    onMouseLeave={() => hideTooltip()}
                >
                    <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                    </svg>
                    <Show when={!props.compact}>
                        <span>Update</span>
                    </Show>
                </span>
            </Show>
        </Show>
    );
};

/**
 * Compact version of UpdateBadge - just an icon with no text.
 * Use this in table cells where space is limited.
 */
export const UpdateIcon: Component<{ updateStatus?: DockerContainerUpdateStatus }> = (props) => {
    const hasUpdate = () => props.updateStatus?.updateAvailable === true;

    const getTooltip = () => {
        if (!props.updateStatus) return 'Image update available';

        const current = props.updateStatus.currentDigest?.slice(0, 12) || 'unknown';
        const latest = props.updateStatus.latestDigest?.slice(0, 12) || 'unknown';

        return `Update available\nCurrent: ${current}...\nLatest: ${latest}...`;
    };

    return (
        <Show when={hasUpdate()}>
            <span
                class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-help"
                onMouseEnter={(e) => {
                    const rect = e.currentTarget.getBoundingClientRect();
                    showTooltip(getTooltip(), rect.left + rect.width / 2, rect.top, {
                        align: 'center',
                        direction: 'up'
                    });
                }}
                onMouseLeave={() => hideTooltip()}
            >
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                </svg>
            </span>
        </Show>
    );
};

interface UpdateButtonProps {
    updateStatus?: DockerContainerUpdateStatus;
    hostId: string;
    containerId: string;
    containerName: string;
    compact?: boolean;
    onUpdateTriggered?: () => void;
    externalState?: 'updating' | 'queued' | 'error';
}


type UpdateState = 'idle' | 'confirming' | 'updating' | 'success' | 'error';

/**
 * UpdateButton displays a clickable button to trigger container updates.
 * Uses a persistent store to maintain state across WebSocket refreshes.
 * 
 * If the server has disabled Docker update actions (via PULSE_DISABLE_DOCKER_UPDATE_ACTIONS
 * or the Settings UI), this component will render a read-only UpdateBadge instead,
 * allowing users to see that updates are available without being able to trigger them.
 * 
 * While system settings are loading, the button displays in a disabled/loading state
 * to prevent premature clicks before the server configuration is known.
 */
export const UpdateButton: Component<UpdateButtonProps> = (props) => {
    const [localState, setLocalState] = createSignal<'idle' | 'confirming'>('idle');
    const [errorMessage, setErrorMessage] = createSignal<string>('');

    // Reactive check for whether settings are loaded and what they say
    const settingsLoaded = () => areSystemSettingsLoaded();
    const shouldHideButton = () => shouldHideDockerUpdateActions();

    // Get persistent state from store - this survives WebSocket updates
    const storeState = createMemo(() => {
        // Access updateStates() to create reactive dependency
        updateStates();
        return getContainerUpdateState(props.hostId, props.containerId);
    });

    // Derived state: check store first, then external prop, then local state
    const currentState = (): UpdateState => {
        const stored = storeState();
        if (stored) {
            switch (stored.state) {
                case 'queued':
                case 'updating':
                    return 'updating';
                case 'success':
                    return 'success';
                case 'error':
                    return 'error';
            }
        }
        if (props.externalState === 'updating') return 'updating';
        if (props.externalState === 'queued') return 'updating';
        if (props.externalState === 'error') return 'error';
        return localState();
    };

    // Watch for update completion - when updateAvailable becomes false, the update succeeded
    createEffect(() => {
        const stored = storeState();
        if (stored && (stored.state === 'queued' || stored.state === 'updating')) {
            // If the container no longer has an update available, the update succeeded!
            if (props.updateStatus?.updateAvailable === false) {
                markContainerUpdateSuccess(props.hostId, props.containerId);
            }
        }
    });

    const hasUpdate = () => props.updateStatus?.updateAvailable === true || currentState() !== 'idle';

    const handleClick = async (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();

        const state = currentState();

        // Prevent clicking if already updating
        if (state === 'updating' || state === 'success' || state === 'error') return;

        if (state === 'idle') {
            // Show confirmation
            setLocalState('confirming');
            return;
        }

        if (state === 'confirming') {
            // User confirmed, trigger update
            // Immediately set store state so it persists
            markContainerQueued(props.hostId, props.containerId);
            setLocalState('idle'); // Reset local state

            try {
                await MonitoringAPI.updateDockerContainer(
                    props.hostId,
                    props.containerId,
                    props.containerName
                );
                // Command queued successfully - store already has 'queued' state
                // The effect above will detect when updateAvailable becomes false
                props.onUpdateTriggered?.();
            } catch (err) {
                const message = (err as Error).message || 'Failed to trigger update';
                setErrorMessage(message);
                markContainerUpdateError(props.hostId, props.containerId, message);
            }
        }
    };

    const handleCancel = (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();
        setLocalState('idle');
        // Also clear any store state if canceling
        clearContainerUpdateState(props.hostId, props.containerId);
    };

    const getButtonClass = () => {
        const base = 'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium transition-all';
        switch (currentState()) {
            case 'confirming':
                return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 cursor-pointer hover:bg-amber-200 dark:hover:bg-amber-900`;
            case 'updating':
                return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-wait`;
            case 'success':
                return `${base} bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300`;
            case 'error':
                return `${base} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 cursor-help`;
            default:
                return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-pointer hover:bg-blue-200 dark:hover:bg-blue-900`;
        }
    };

    const getTooltip = () => {
        const stored = storeState();
        switch (currentState()) {
            case 'confirming':
                return 'Click again to confirm update';
            case 'updating': {
                const elapsed = stored ? Math.round((Date.now() - stored.startedAt) / 1000) : 0;
                // Show the current step if available from backend
                const step = stored?.message || 'Processing...';
                if (elapsed > 60) {
                    return `${step} (${Math.floor(elapsed / 60)}m ${elapsed % 60}s)`;
                }
                return `${step} (${elapsed}s)`;
            }
            case 'success':
                return '✓ Update completed successfully!';
            case 'error':
                return `✗ Update failed: ${stored?.message || errorMessage() || 'Unknown error'}`;
            default:
                if (!props.updateStatus) return 'Update container';
                const current = props.updateStatus.currentDigest?.slice(0, 12) || 'unknown';
                const latest = props.updateStatus.latestDigest?.slice(0, 12) || 'unknown';
                return `Click to update\nCurrent: ${current}...\nLatest: ${latest}...`;
        }
    };

    // Compute if the button should be disabled due to loading or settings
    const isButtonDisabled = () => currentState() === 'updating' || !settingsLoaded();

    return (
        <Show when={hasUpdate()}>
            {/* Case 1: Settings loaded and updates are disabled - show read-only badge */}
            <Show when={settingsLoaded() && shouldHideButton()}>
                <UpdateBadge updateStatus={props.updateStatus} compact={props.compact} />
            </Show>

            {/* Case 2: Settings loading OR settings loaded with updates enabled - show button */}
            <Show when={!settingsLoaded() || !shouldHideButton()}>
                <div class="inline-flex items-center gap-1" data-prevent-toggle>
                    <button
                        type="button"
                        class={getButtonClass()}
                        onClick={handleClick}
                        onMouseDown={(e) => { e.stopPropagation(); }}
                        disabled={isButtonDisabled()}
                        data-prevent-toggle
                        onMouseEnter={(e) => {
                            const rect = e.currentTarget.getBoundingClientRect();
                            const tooltip = !settingsLoaded()
                                ? 'Loading settings...'
                                : getTooltip();
                            showTooltip(tooltip, rect.left + rect.width / 2, rect.top, {
                                align: 'center',
                                direction: 'up'
                            });
                        }}
                        onMouseLeave={() => hideTooltip()}
                    >
                        {/* Loading state - settings haven't loaded yet */}
                        <Show when={!settingsLoaded()}>
                            <svg class="w-3 h-3 animate-pulse opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                            </svg>
                        </Show>
                        {/* Normal states - settings loaded */}
                        <Show when={settingsLoaded()}>
                            <Show when={currentState() === 'updating'}>
                                {/* Spinner */}
                                <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                                </svg>
                            </Show>
                            <Show when={currentState() === 'success'}>
                                {/* Check icon */}
                                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                                </svg>
                            </Show>
                            <Show when={currentState() === 'error'}>
                                {/* X icon */}
                                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </Show>
                            <Show when={currentState() === 'idle' || currentState() === 'confirming'}>
                                {/* Upload/update icon */}
                                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                                </svg>
                            </Show>
                        </Show>
                        <Show when={!props.compact}>
                            <span class={!settingsLoaded() ? 'opacity-50' : ''}>
                                {!settingsLoaded() ? 'Update' :
                                    currentState() === 'confirming' ? 'Confirm?' :
                                        currentState() === 'updating' ? 'Updating...' :
                                            currentState() === 'success' ? 'Queued!' :
                                                currentState() === 'error' ? 'Failed' : 'Update'}
                            </span>
                        </Show>
                    </button>
                    <Show when={settingsLoaded() && currentState() === 'confirming'}>
                        <button
                            type="button"
                            class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-surface-alt text-slate-600 dark:text-slate-300 hover:bg-slate-300 dark:hover:bg-slate-600 transition-colors"
                            onClick={handleCancel}
                            title="Cancel"
                        >
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </Show>
                </div>
            </Show>
        </Show>
    );
};

export default UpdateBadge;

