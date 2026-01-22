import { Component, createEffect, createSignal, For, Show } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { logger } from '@/utils/logger';
import type { InfrastructureChange } from '@/types/aiIntelligence';

const DEFAULT_UPGRADE_URL = 'https://pulserelay.pro/';

export const AIRecentChangesPanel: Component<{ hours?: number; showWhenEmpty?: boolean }> = (props) => {
    const [changes, setChanges] = createSignal<InfrastructureChange[]>([]);
    const [loading, setLoading] = createSignal(false);
    const [locked, setLocked] = createSignal(false);
    const [lockedCount, setLockedCount] = createSignal(0);
    const [upgradeUrl, setUpgradeUrl] = createSignal(DEFAULT_UPGRADE_URL);
    const [error, setError] = createSignal('');
    const showWhenEmpty = () => Boolean(props.showWhenEmpty);
    const hours = () => props.hours ?? 24;

    const loadData = async () => {
        setLoading(true);
        setError('');
        try {
            const response = await AIAPI.getRecentChanges(hours());
            const licenseLocked = Boolean(response.license_required);
            setLocked(licenseLocked);
            setUpgradeUrl(response.upgrade_url || DEFAULT_UPGRADE_URL);
            if (licenseLocked) {
                setLockedCount(response.count || 0);
                setChanges([]);
            } else {
                setLockedCount(0);
                setChanges(response.changes || []);
            }
        } catch (e) {
            logger.error('Failed to load AI change history:', e);
            setError('Failed to load recent changes.');
        } finally {
            setLoading(false);
        }
    };

    createEffect(() => {
        void loadData();
    });

    const displayedChanges = () => changes().slice(0, 6);
    const shouldShow = () => showWhenEmpty() || loading() || displayedChanges().length > 0 || lockedCount() > 0;

    const formatRelativeTime = (ts: string) => {
        const date = new Date(ts);
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);

        if (diffMins < 1) return 'just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        if (diffDays < 7) return `${diffDays}d ago`;
        return date.toLocaleDateString();
    };

    const changeTypeLabel = (changeType: string) => {
        const labels: Record<string, string> = {
            created: 'Created',
            deleted: 'Deleted',
            config: 'Config change',
            status: 'Status change',
            migrated: 'Migrated',
            restarted: 'Restarted',
            backed_up: 'Backup completed',
        };
        if (labels[changeType]) {
            return labels[changeType];
        }
        return changeType
            .split(/[_-]/)
            .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
            .join(' ');
    };

    const changeTypeBadge = (changeType: string) => {
        const styles: Record<string, string> = {
            created: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
            deleted: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
            config: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
            status: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
            migrated: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
            restarted: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
            backed_up: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
        };
        return styles[changeType] || 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300';
    };

    return (
        <Show when={shouldShow()}>
            <div class="bg-gradient-to-r from-slate-50 to-teal-50 dark:from-slate-900/20 dark:to-teal-900/20 border border-slate-200 dark:border-slate-700 rounded-lg overflow-hidden">
                <div class="px-4 py-3 flex items-center justify-between border-b border-slate-200/70 dark:border-slate-700/70">
                    <div class="flex items-center gap-2">
                        <svg class="w-5 h-5 text-teal-600 dark:text-teal-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 4H7a2 2 0 01-2-2V7a2 2 0 012-2h5l5 5v9a2 2 0 01-2 2z" />
                        </svg>
                        <span class="font-medium text-slate-900 dark:text-slate-100">Operational Memory</span>
                        <Show when={lockedCount() > 0 && locked()}>
                            <span class="px-2 py-0.5 text-xs font-medium bg-teal-200 dark:bg-teal-700 text-teal-900 dark:text-teal-100 rounded-full">
                                {lockedCount()}
                            </span>
                        </Show>
                        <Show when={!locked() && displayedChanges().length > 0}>
                            <span class="px-2 py-0.5 text-xs font-medium bg-teal-200 dark:bg-teal-700 text-teal-900 dark:text-teal-100 rounded-full">
                                {displayedChanges().length}
                            </span>
                        </Show>
                        <Show when={locked()}>
                            <span class="px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 rounded-full">
                                Locked
                            </span>
                        </Show>
                    </div>
                    <span class="text-xs text-slate-500 dark:text-slate-400">{hours()}h window</span>
                </div>
                <div class="px-4 pb-4 space-y-3">
                    <Show when={loading()}>
                        <div class="text-sm text-slate-500 dark:text-slate-400 flex items-center gap-2">
                            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                            Loading changes...
                        </div>
                    </Show>

                    <Show when={error() && !loading()}>
                        <div class="text-sm text-red-600 dark:text-red-400">
                            {error()}
                        </div>
                    </Show>

                    <Show when={locked() && !loading()}>
                        <div class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 p-3 text-sm text-amber-800 dark:text-amber-200">
                            <div class="flex items-center justify-between gap-3">
                                <div>
                                    <p class="font-medium">Pulse Pro required</p>
                                    <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                                        Operational memory is available with Pulse Pro. {lockedCount() || 0} changes are ready to review.
                                    </p>
                                </div>
                                <a
                                    class="text-xs font-medium text-amber-800 dark:text-amber-200 underline"
                                    href={upgradeUrl()}
                                    target="_blank"
                                    rel="noreferrer"
                                >
                                    Upgrade
                                </a>
                            </div>
                        </div>
                    </Show>

                    <Show when={!locked() && !loading()}>
                        <Show
                            when={displayedChanges().length > 0}
                            fallback={
                                <p class="text-sm text-slate-500 dark:text-slate-400 text-center py-2">
                                    No recent changes detected. The AI will surface configuration and state changes here.
                                </p>
                            }
                        >
                            <div class="space-y-2">
                                <For each={displayedChanges()}>
                                    {(change) => (
                                        <div class="bg-white/60 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-800 rounded-lg p-3">
                                            <div class="flex items-start justify-between gap-3">
                                                <div class="flex-1">
                                                    <div class="flex items-center gap-2 flex-wrap">
                                                        <span class={`px-2 py-0.5 text-[10px] font-semibold rounded-full ${changeTypeBadge(change.change_type)}`}>
                                                            {changeTypeLabel(change.change_type)}
                                                        </span>
                                                        <span class="text-sm font-medium text-slate-800 dark:text-slate-200">
                                                            {change.resource_name || change.resource_id}
                                                        </span>
                                                    </div>
                                                    <p class="text-xs text-slate-600 dark:text-slate-400 mt-1">
                                                        {change.description || 'Change detected by Pulse Patrol.'}
                                                    </p>
                                                </div>
                                                <span class="text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
                                                    {formatRelativeTime(change.detected_at)}
                                                </span>
                                            </div>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </Show>
                    </Show>
                </div>
            </div>
        </Show>
    );
};
