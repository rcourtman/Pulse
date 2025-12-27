import { Component, Show } from 'solid-js';
import type { DockerContainerUpdateStatus } from '@/types/api';

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
                            class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400"
                            title={`Update check failed: ${props.updateStatus?.error}`}
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
                    class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                    title={`Image update available. Current: ${props.updateStatus?.currentDigest?.slice(0, 19) || 'unknown'}... Latest: ${props.updateStatus?.latestDigest?.slice(0, 19) || 'unknown'}...`}
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

    return (
        <Show when={hasUpdate()}>
            <span
                class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                title={`Image update available`}
            >
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                </svg>
            </span>
        </Show>
    );
};

export default UpdateBadge;
