import { Component, createSignal, createEffect, Show, For } from 'solid-js';
import { AIAPI } from '@/api/ai';
import type { FailurePrediction, ResourceCorrelation } from '@/types/aiIntelligence';

/**
 * AIInsightsPanel displays AI-learned predictions and correlations
 * Shows failure predictions with confidence levels and resource dependencies
 */
export const AIInsightsPanel: Component<{ resourceId?: string; showWhenEmpty?: boolean }> = (props) => {
    const [predictions, setPredictions] = createSignal<FailurePrediction[]>([]);
    const [correlations, setCorrelations] = createSignal<ResourceCorrelation[]>([]);
    const [loading, setLoading] = createSignal(false);
    const [expanded, setExpanded] = createSignal(false);
    const [locked, setLocked] = createSignal(false);
    const [lockedCount, setLockedCount] = createSignal(0);
    const [upgradeUrl, setUpgradeUrl] = createSignal('https://pulsemonitor.app/pro');
    const [error, setError] = createSignal('');
    const showWhenEmpty = () => Boolean(props.showWhenEmpty);

    const loadData = async () => {
        setLoading(true);
        setError('');
        try {
            const [predResp, corrResp] = await Promise.all([
                AIAPI.getPredictions(props.resourceId),
                AIAPI.getCorrelations(props.resourceId),
            ]);
            const licenseLocked = Boolean(predResp.license_required || corrResp.license_required);
            setLocked(licenseLocked);
            setUpgradeUrl(predResp.upgrade_url || corrResp.upgrade_url || 'https://pulsemonitor.app/pro');
            if (licenseLocked) {
                const predCount = predResp.count || 0;
                const corrCount = corrResp.count || 0;
                setLockedCount(predCount + corrCount);
                setPredictions([]);
                setCorrelations([]);
            } else {
                setLockedCount(0);
                setPredictions(predResp.predictions || []);
                setCorrelations(corrResp.correlations || []);
            }
        } catch (e) {
            console.error('Failed to load AI insights:', e);
            setError('Failed to load AI insights.');
        } finally {
            setLoading(false);
        }
    };

    createEffect(() => {
        loadData();
    });

    const totalInsights = () => predictions().length + correlations().length;
    const displayedCount = () => (locked() ? lockedCount() : totalInsights());
    const shouldShow = () => showWhenEmpty() || loading() || displayedCount() > 0;

    // Format days until in a human-readable way
    const formatDaysUntil = (days: number) => {
        if (days < 0) return 'Overdue';
        if (days < 1) return 'Today';
        if (days < 2) return 'Tomorrow';
        return `In ${Math.round(days)} days`;
    };

    // Get severity color based on days until and confidence
    const getPredictionColor = (pred: FailurePrediction) => {
        if (pred.is_overdue || pred.days_until < 0) return 'text-red-600 dark:text-red-400';
        if (pred.days_until < 3) return 'text-amber-600 dark:text-amber-400';
        if (pred.days_until < 7) return 'text-yellow-600 dark:text-yellow-400';
        return 'text-blue-600 dark:text-blue-400';
    };

    // Get event type display name
    const getEventDisplayName = (eventType: string) => {
        const names: Record<string, string> = {
            high_memory: 'High Memory',
            high_cpu: 'High CPU',
            disk_full: 'Disk Full',
            oom: 'Out of Memory',
            restart: 'Restart',
            unresponsive: 'Unresponsive',
            backup_failed: 'Backup Failure',
        };
        if (names[eventType]) {
            return names[eventType];
        }
        return eventType
            .split(/[_-]/)
            .map((part) => {
                if (part === 'cpu') return 'CPU';
                if (part === 'vm') return 'VM';
                if (part === 'pbs') return 'PBS';
                if (part === 'raid') return 'RAID';
                return part.charAt(0).toUpperCase() + part.slice(1);
            })
            .join(' ');
    };

    return (
        <Show when={shouldShow()}>
            <div class="bg-gradient-to-r from-purple-50 to-indigo-50 dark:from-purple-900/20 dark:to-indigo-900/20 border border-purple-200 dark:border-purple-700 rounded-lg overflow-hidden">
                {/* Header */}
                <button
                    type="button"
                    onClick={() => setExpanded(!expanded())}
                    class="w-full px-4 py-3 flex items-center justify-between hover:bg-purple-100/50 dark:hover:bg-purple-800/30 transition-colors"
                >
                    <div class="flex items-center gap-2">
                        <svg class="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                        </svg>
                        <span class="font-medium text-purple-900 dark:text-purple-100">
                            AI Insights
                        </span>
                        <Show when={displayedCount() > 0}>
                            <span class="px-2 py-0.5 text-xs font-medium bg-purple-200 dark:bg-purple-700 text-purple-800 dark:text-purple-200 rounded-full">
                                {displayedCount()}
                            </span>
                        </Show>
                        <Show when={locked()}>
                            <span class="px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 rounded-full">
                                Locked
                            </span>
                        </Show>
                    </div>
                    <svg
                        class={`w-5 h-5 text-purple-600 dark:text-purple-400 transition-transform ${expanded() ? 'rotate-180' : ''}`}
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                    >
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                    </svg>
                </button>

                {/* Content */}
                <Show when={expanded()}>
                    <div class="px-4 pb-4 space-y-4">
                        {/* Loading state */}
                        <Show when={loading()}>
                            <div class="text-sm text-gray-500 dark:text-gray-400 flex items-center gap-2">
                                <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                                Loading insights...
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
                                            Predictive intelligence is available with Pulse Pro. {lockedCount() || 0} insights are ready to review.
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

                        {/* Predictions */}
                        <Show when={!locked() && predictions().length > 0}>
                            <div>
                                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-1">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                    Failure Predictions
                                </h4>
                                <div class="space-y-2">
                                    <For each={predictions()}>
                                        {(pred) => (
                                            <div class="bg-white/50 dark:bg-gray-800/50 rounded-lg p-3 border border-purple-100 dark:border-purple-800">
                                                <div class="flex items-start justify-between gap-2">
                                                    <div class="flex-1">
                                                        <div class={`font-medium ${getPredictionColor(pred)}`}>
                                                            {getEventDisplayName(pred.event_type)}
                                                            <span class="ml-2 text-sm font-normal">
                                                                {formatDaysUntil(pred.days_until)}
                                                            </span>
                                                        </div>
                                                        <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                                            {pred.basis}
                                                        </p>
                                                    </div>
                                                    <div class="text-right shrink-0">
                                                        <div class="text-xs text-gray-500 dark:text-gray-400">
                                                            {Math.round(pred.confidence * 100)}% confidence
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        )}
                                    </For>
                                </div>
                            </div>
                        </Show>

                        {/* Correlations */}
                        <Show when={!locked() && correlations().length > 0}>
                            <div>
                                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-1">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                                    </svg>
                                    Resource Dependencies
                                </h4>
                                <div class="space-y-2">
                                    <For each={correlations()}>
                                        {(corr) => (
                                            <div class="bg-white/50 dark:bg-gray-800/50 rounded-lg p-3 border border-purple-100 dark:border-purple-800">
                                                <div class="flex items-center gap-2 text-sm">
                                                    <span class="font-medium text-gray-800 dark:text-gray-200">
                                                        {corr.source_name || corr.source_id}
                                                    </span>
                                                    <svg class="w-4 h-4 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8l4 4m0 0l-4 4m4-4H3" />
                                                    </svg>
                                                    <span class="font-medium text-gray-800 dark:text-gray-200">
                                                        {corr.target_name || corr.target_id}
                                                    </span>
                                                </div>
                                                <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                                    {corr.description || `${corr.event_pattern} (${corr.occurrences} observations)`}
                                                </p>
                                                <div class="flex items-center gap-3 mt-1 text-xs text-gray-500 dark:text-gray-400">
                                                    <span>Avg delay: {corr.avg_delay}</span>
                                                    <span>Confidence: {Math.round(corr.confidence * 100)}%</span>
                                                </div>
                                            </div>
                                        )}
                                    </For>
                                </div>
                            </div>
                        </Show>

                        {/* Empty state */}
                        <Show when={!loading() && !locked() && totalInsights() === 0}>
                            <p class="text-sm text-gray-500 dark:text-gray-400 text-center py-2">
                                No predictions or correlations detected yet. The AI will learn patterns over time.
                            </p>
                        </Show>
                    </div>
                </Show>
            </div>
        </Show>
    );
};
