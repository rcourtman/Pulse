import { Component, createEffect, createSignal, For, Show } from 'solid-js';
import { AIAPI } from '@/api/ai';
import type { RemediationRecord, RemediationStats } from '@/types/aiIntelligence';

const DEFAULT_UPGRADE_URL = 'https://pulsemonitor.app/pro';

export const AIImpactTimelinePanel: Component<{ hours?: number; showWhenEmpty?: boolean }> = (props) => {
    const [remediations, setRemediations] = createSignal<RemediationRecord[]>([]);
    const [stats, setStats] = createSignal<RemediationStats | null>(null);
    const [loading, setLoading] = createSignal(false);
    const [locked, setLocked] = createSignal(false);
    const [upgradeUrl, setUpgradeUrl] = createSignal(DEFAULT_UPGRADE_URL);
    const [error, setError] = createSignal('');
    const showWhenEmpty = () => Boolean(props.showWhenEmpty);
    const hours = () => props.hours ?? 168;

    const loadData = async () => {
        setLoading(true);
        setError('');
        try {
            const response = await AIAPI.getRemediations({ hours: hours(), limit: 6 });
            setLocked(Boolean(response.license_required));
            setUpgradeUrl(response.upgrade_url || DEFAULT_UPGRADE_URL);
            setStats(response.stats || null);
            setRemediations(response.remediations || []);
        } catch (e) {
            console.error('Failed to load AI impact timeline:', e);
            setError('Failed to load AI impact timeline.');
        } finally {
            setLoading(false);
        }
    };

    createEffect(() => {
        void loadData();
    });

    const shouldShow = () => showWhenEmpty() || loading() || remediations().length > 0 || (stats()?.total || 0) > 0;

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

    const outcomeBadge = (outcome: string) => {
        const styles: Record<string, string> = {
            resolved: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
            partial: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
            failed: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
        };
        return styles[outcome] || 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300';
    };

    const truncateText = (text: string, limit: number) => {
        if (text.length <= limit) return text;
        return `${text.slice(0, limit - 3)}...`;
    };

    const statValue = (key: keyof RemediationStats) => stats()?.[key] ?? 0;

    return (
        <Show when={shouldShow()}>
            <div class="bg-gradient-to-r from-emerald-50 to-sky-50 dark:from-emerald-900/20 dark:to-sky-900/20 border border-emerald-200 dark:border-emerald-700 rounded-lg overflow-hidden">
                <div class="px-4 py-3 flex items-center justify-between border-b border-emerald-200/70 dark:border-emerald-700/70">
                    <div class="flex items-center gap-2">
                        <svg class="w-5 h-5 text-emerald-600 dark:text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                        </svg>
                        <span class="font-medium text-emerald-900 dark:text-emerald-100">Pulse AI Impact</span>
                        <Show when={locked()}>
                            <span class="px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 rounded-full">
                                Locked
                            </span>
                        </Show>
                    </div>
                    <span class="text-xs text-emerald-600 dark:text-emerald-300">{Math.round(hours() / 24)} days</span>
                </div>
                <div class="px-4 pb-4 space-y-3">
                    <Show when={loading()}>
                        <div class="text-sm text-emerald-700 dark:text-emerald-200 flex items-center gap-2">
                            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                            Loading impact...
                        </div>
                    </Show>

                    <Show when={error() && !loading()}>
                        <div class="text-sm text-red-600 dark:text-red-400">
                            {error()}
                        </div>
                    </Show>

                    <Show when={stats() && !loading()}>
                        <div class="grid grid-cols-2 gap-2 text-xs">
                            <div class="rounded-lg bg-white/70 dark:bg-gray-900/40 border border-emerald-100 dark:border-emerald-800 px-2 py-1">
                                <span class="text-emerald-600 dark:text-emerald-300">Resolved</span>
                                <span class="ml-1 font-semibold text-emerald-900 dark:text-emerald-100">{statValue('resolved')}</span>
                            </div>
                            <div class="rounded-lg bg-white/70 dark:bg-gray-900/40 border border-emerald-100 dark:border-emerald-800 px-2 py-1">
                                <span class="text-emerald-600 dark:text-emerald-300">Auto-fix</span>
                                <span class="ml-1 font-semibold text-emerald-900 dark:text-emerald-100">{statValue('automatic')}</span>
                            </div>
                            <div class="rounded-lg bg-white/70 dark:bg-gray-900/40 border border-emerald-100 dark:border-emerald-800 px-2 py-1">
                                <span class="text-emerald-600 dark:text-emerald-300">Partial</span>
                                <span class="ml-1 font-semibold text-emerald-900 dark:text-emerald-100">{statValue('partial')}</span>
                            </div>
                            <div class="rounded-lg bg-white/70 dark:bg-gray-900/40 border border-emerald-100 dark:border-emerald-800 px-2 py-1">
                                <span class="text-emerald-600 dark:text-emerald-300">Failed</span>
                                <span class="ml-1 font-semibold text-emerald-900 dark:text-emerald-100">{statValue('failed')}</span>
                            </div>
                        </div>
                    </Show>

                    <Show when={locked() && !loading()}>
                        <div class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 p-3 text-sm text-amber-800 dark:text-amber-200">
                            <div class="flex items-center justify-between gap-3">
                                <div>
                                    <p class="font-medium">Pulse Pro required</p>
                                    <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                                        Pulse AI has handled {statValue('resolved')} remediations in the last {Math.round(hours() / 24)} days. Upgrade to view the receipts.
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
                            when={remediations().length > 0}
                            fallback={
                                <p class="text-sm text-emerald-700 dark:text-emerald-200 text-center py-2">
                                    No remediations logged yet. Pulse AI will highlight fixes here as they happen.
                                </p>
                            }
                        >
                            <div class="space-y-2">
                                <For each={remediations()}>
                                    {(rec) => (
                                        <div class="bg-white/70 dark:bg-gray-900/40 border border-emerald-100 dark:border-emerald-800 rounded-lg p-3">
                                            <div class="flex items-start justify-between gap-2">
                                                <div class="flex-1">
                                                    <div class="flex items-center gap-2 flex-wrap">
                                                        <span class={`px-2 py-0.5 text-[10px] font-semibold rounded-full ${outcomeBadge(rec.outcome)}`}>
                                                            {rec.outcome}
                                                        </span>
                                                        <Show when={rec.automatic}>
                                                            <span class="px-2 py-0.5 text-[10px] font-semibold rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                                                                Auto-Fix
                                                            </span>
                                                        </Show>
                                                        <span class="text-xs font-medium text-emerald-900 dark:text-emerald-100">
                                                            {rec.action}
                                                        </span>
                                                    </div>
                                                    <p class="text-xs text-emerald-700 dark:text-emerald-200 mt-1">
                                                        {rec.problem}
                                                    </p>
                                                    <Show when={rec.note}>
                                                        <p class="text-[11px] text-emerald-600 dark:text-emerald-300 mt-1">
                                                            {rec.note}
                                                        </p>
                                                    </Show>
                                                    <Show when={rec.output}>
                                                        <p class="text-[11px] text-emerald-600 dark:text-emerald-300 mt-1">
                                                            Evidence: {truncateText(rec.output || '', 140)}
                                                        </p>
                                                    </Show>
                                                </div>
                                                <span class="text-[11px] text-emerald-600 dark:text-emerald-300 whitespace-nowrap">
                                                    {formatRelativeTime(rec.timestamp)}
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
