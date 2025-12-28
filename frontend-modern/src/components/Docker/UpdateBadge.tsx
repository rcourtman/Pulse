import { Component, Show, createSignal } from 'solid-js';
import type { DockerContainerUpdateStatus } from '@/types/api';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import { MonitoringAPI } from '@/api/monitoring';


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
                            class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400 cursor-help"
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
                    class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 cursor-help"
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
                class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 cursor-help"
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
}

type UpdateState = 'idle' | 'confirming' | 'updating' | 'success' | 'error';

/**
 * UpdateButton displays a clickable button to trigger container updates.
 * Includes confirmation, loading states, and error handling.
 */
export const UpdateButton: Component<UpdateButtonProps> = (props) => {
    const [state, setState] = createSignal<UpdateState>('idle');
    const [errorMessage, setErrorMessage] = createSignal<string>('');

    const hasUpdate = () => props.updateStatus?.updateAvailable === true;

    const handleClick = async (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();

        if (state() === 'idle') {
            // Show confirmation
            setState('confirming');
            return;
        }

        if (state() === 'confirming') {
            // User confirmed, trigger update
            setState('updating');
            try {
                await MonitoringAPI.updateDockerContainer(
                    props.hostId,
                    props.containerId,
                    props.containerName
                );
                setState('success');
                props.onUpdateTriggered?.();
                // Reset after 3 seconds
                setTimeout(() => setState('idle'), 3000);
            } catch (err) {
                setErrorMessage((err as Error).message || 'Failed to trigger update');
                setState('error');
                // Reset after 5 seconds
                setTimeout(() => setState('idle'), 5000);
            }
        }
    };

    const handleCancel = (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();
        setState('idle');
    };

    const getButtonClass = () => {
        const base = 'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium transition-all';
        switch (state()) {
            case 'confirming':
                return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 cursor-pointer hover:bg-amber-200 dark:hover:bg-amber-900/60`;
            case 'updating':
                return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 cursor-wait`;
            case 'success':
                return `${base} bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300`;
            case 'error':
                return `${base} bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300 cursor-help`;
            default:
                return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 cursor-pointer hover:bg-blue-200 dark:hover:bg-blue-900/60`;
        }
    };

    const getTooltip = () => {
        switch (state()) {
            case 'confirming':
                return 'Click again to confirm update';
            case 'updating':
                return 'Update in progress...';
            case 'success':
                return 'Update command sent! Container will restart shortly.';
            case 'error':
                return `Error: ${errorMessage()}`;
            default:
                if (!props.updateStatus) return 'Update container';
                const current = props.updateStatus.currentDigest?.slice(0, 12) || 'unknown';
                const latest = props.updateStatus.latestDigest?.slice(0, 12) || 'unknown';
                return `Click to update\nCurrent: ${current}...\nLatest: ${latest}...`;
        }
    };

    return (
        <Show when={hasUpdate()}>
            <div class="inline-flex items-center gap-1" data-prevent-toggle>
                <button
                    type="button"
                    class={getButtonClass()}
                    onClick={handleClick}
                    onMouseDown={(e) => { e.stopPropagation(); }}
                    disabled={state() === 'updating'}
                    data-prevent-toggle
                    onMouseEnter={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        showTooltip(getTooltip(), rect.left + rect.width / 2, rect.top, {
                            align: 'center',
                            direction: 'up'
                        });
                    }}
                    onMouseLeave={() => hideTooltip()}
                >
                    <Show when={state() === 'updating'}>
                        {/* Spinner */}
                        <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                        </svg>
                    </Show>
                    <Show when={state() === 'success'}>
                        {/* Check icon */}
                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                        </svg>
                    </Show>
                    <Show when={state() === 'error'}>
                        {/* X icon */}
                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </Show>
                    <Show when={state() === 'idle' || state() === 'confirming'}>
                        {/* Upload/update icon */}
                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                        </svg>
                    </Show>
                    <Show when={!props.compact}>
                        <span>
                            {state() === 'confirming' ? 'Confirm?' :
                                state() === 'updating' ? 'Updating...' :
                                    state() === 'success' ? 'Queued!' :
                                        state() === 'error' ? 'Failed' : 'Update'}
                        </span>
                    </Show>
                </button>
                <Show when={state() === 'confirming'}>
                    <button
                        type="button"
                        class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
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
    );
};

export default UpdateBadge;

