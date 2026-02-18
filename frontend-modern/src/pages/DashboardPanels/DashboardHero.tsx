import { createMemo, Show } from 'solid-js';
import { Dynamic } from 'solid-js/web';

import Server from 'lucide-solid/icons/server';
import Boxes from 'lucide-solid/icons/boxes';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Archive from 'lucide-solid/icons/archive';
import CheckCircle from 'lucide-solid/icons/check-circle';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import XCircle from 'lucide-solid/icons/x-circle';

import {
    ALERTS_OVERVIEW_PATH,
    INFRASTRUCTURE_PATH,
    WORKLOADS_PATH,
    buildRecoveryPath,
    buildStoragePath,
} from '@/routing/resourceLinks';
import { Card } from '@/components/shared/Card';
import { MiniDonut, MiniGauge } from './Visualizations';
import { formatBytes, formatRelativeTime } from '@/utils/format';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';

const CARD_THEMES = {
    blue: {
        iconBg: 'bg-blue-50 dark:bg-blue-900/20',
        iconColor: 'text-blue-500 dark:text-blue-400',
        hoverBorder: 'hover:border-blue-400 dark:hover:border-blue-500',
        hoverIconBg: 'group-hover:bg-blue-100 dark:group-hover:bg-blue-900/30',
    },
    purple: {
        iconBg: 'bg-purple-50 dark:bg-purple-900/20',
        iconColor: 'text-purple-500 dark:text-purple-400',
        hoverBorder: 'hover:border-purple-400 dark:hover:border-purple-500',
        hoverIconBg: 'group-hover:bg-purple-100 dark:group-hover:bg-purple-900/30',
    },
    cyan: {
        iconBg: 'bg-cyan-50 dark:bg-cyan-900/20',
        iconColor: 'text-cyan-600 dark:text-cyan-400',
        hoverBorder: 'hover:border-cyan-400 dark:hover:border-cyan-500',
        hoverIconBg: 'group-hover:bg-cyan-100 dark:group-hover:bg-cyan-900/30',
    },
    emerald: {
        iconBg: 'bg-emerald-50 dark:bg-emerald-900/20',
        iconColor: 'text-emerald-600 dark:text-emerald-400',
        hoverBorder: 'hover:border-emerald-400 dark:hover:border-emerald-500',
        hoverIconBg: 'group-hover:bg-emerald-100 dark:group-hover:bg-emerald-900/30',
    },
} as const;

function SeverityChip(props: { severity: 'warning' | 'critical' }) {
    const isWarning = () => props.severity === 'warning';
    return (
        <span
            class={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-semibold ${
                isWarning()
                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                    : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
            }`}
        >
            <span class={`w-1.5 h-1.5 rounded-full ${isWarning() ? 'bg-amber-500' : 'bg-red-500'}`} />
            {isWarning() ? 'Warning' : 'Critical'}
        </span>
    );
}

interface DashboardHeroProps {
    criticalAlerts: number;
    warningAlerts: number;
    infrastructure: { total: number; online: number };
    workloads: { total: number; running: number; stopped: number };
    storage: {
        capacityPercent: number;
        totalUsed: number;
        totalCapacity: number;
        warningCount: number;
        criticalCount: number;
    };
    alerts: { activeCritical: number; activeWarning: number; total: number };
    recovery: DashboardRecoverySummary;
    topCPU: Array<{ id: string; name: string; percent: number }>;
}

export function DashboardHero(props: DashboardHeroProps) {
    const status = createMemo(() => {
        if (props.criticalAlerts > 0) return 'critical';
        if (props.warningAlerts > 0) return 'warning';
        return 'healthy';
    });

    const statusConfig = createMemo(() => {
        switch (status()) {
            case 'critical':
                return {
                    label: 'Critical Status',
                    icon: XCircle,
                    color: 'text-red-600 dark:text-red-400',
                    bg: 'bg-red-50 dark:bg-red-900/20',
                    pulseColor: 'bg-red-500',
                };
            case 'warning':
                return {
                    label: 'System Warning',
                    icon: AlertTriangle,
                    color: 'text-amber-600 dark:text-amber-400',
                    bg: 'bg-amber-50 dark:bg-amber-900/20',
                    pulseColor: 'bg-amber-500',
                };
            default:
                return {
                    label: 'All Systems Operational',
                    icon: CheckCircle,
                    color: 'text-emerald-600 dark:text-emerald-400',
                    bg: 'bg-emerald-50 dark:bg-emerald-900/20',
                    pulseColor: 'bg-emerald-500',
                };
        }
    });

    const isRecoveryStale = createMemo(() => {
        const ts = props.recovery.latestEventTimestamp;
        if (typeof ts !== 'number' || !Number.isFinite(ts)) return false;
        return Date.now() - ts > 24 * 60 * 60_000;
    });

    const infrastructureHasIssue = createMemo(() => props.infrastructure.total - props.infrastructure.online > 0);
    const workloadsHasIssue = createMemo(() => props.workloads.stopped > 0);

    const storageSeverity = createMemo<'warning' | 'critical' | null>(() => {
        if (props.storage.criticalCount > 0) return 'critical';
        if (props.storage.warningCount > 0) return 'warning';
        return null;
    });

    const recoverySeverity = createMemo<'warning' | 'critical' | null>(() => {
        if ((props.recovery.byOutcome.failed ?? 0) > 0) return 'critical';
        if (isRecoveryStale()) return 'warning';
        return null;
    });

    const badgeBase =
        'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold border';

    return (
        <div class="space-y-3">
            <Card padding="none" class="border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800">
                <div class="px-4 py-2.5 flex items-center justify-between gap-3">
                    <div class="flex items-center gap-2.5 min-w-0">
                        <div class={`relative rounded-lg p-1.5 ${statusConfig().bg}`}>
                            <Dynamic component={statusConfig().icon} class={`w-4 h-4 ${statusConfig().color}`} />
                            <Show when={status() === 'critical'}>
                                <span class="absolute -right-0.5 -top-0.5 flex h-2.5 w-2.5">
                                    <span
                                        class={`absolute inline-flex h-full w-full rounded-full opacity-75 animate-ping ${statusConfig().pulseColor}`}
                                    />
                                    <span
                                        class={`relative inline-flex h-2.5 w-2.5 rounded-full ${statusConfig().pulseColor}`}
                                    />
                                </span>
                            </Show>
                        </div>
                        <span class="text-sm font-semibold text-gray-800 dark:text-gray-100 truncate">
                            {statusConfig().label}
                        </span>
                    </div>

                    <div class="flex items-center gap-2 shrink-0">
                        <Show when={props.alerts.activeCritical > 0}>
                            <span
                                class={`${badgeBase} bg-red-50 text-red-700 border-red-100 dark:bg-red-900/30 dark:text-red-300 dark:border-red-800`}
                            >
                                <span class="w-1.5 h-1.5 rounded-full bg-red-500" />
                                {props.alerts.activeCritical} Critical
                            </span>
                        </Show>

                        <Show when={props.alerts.activeWarning > 0}>
                            <span
                                class={`${badgeBase} bg-amber-50 text-amber-700 border-amber-100 dark:bg-amber-900/30 dark:text-amber-300 dark:border-amber-800`}
                            >
                                <span class="w-1.5 h-1.5 rounded-full bg-amber-500" />
                                {props.alerts.activeWarning} Warning
                            </span>
                        </Show>

                        <a
                            href={ALERTS_OVERVIEW_PATH}
                            class="text-xs font-semibold text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white transition-colors"
                        >
                            View Alerts →
                        </a>
                    </div>
                </div>
            </Card>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <a href={INFRASTRUCTURE_PATH} class="group block">
                    <Card
                        padding="none"
                        class={`h-full p-4 ${CARD_THEMES.blue.hoverBorder} transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700`}
                    >
                        <div class="flex items-start justify-between gap-2 mb-3">
                            <div class="flex items-center gap-2 min-w-0">
                                <div
                                    class={`p-2 rounded-lg transition-colors ${CARD_THEMES.blue.iconBg} ${CARD_THEMES.blue.iconColor} ${CARD_THEMES.blue.hoverIconBg}`}
                                >
                                    <Server class="w-5 h-5" />
                                </div>
                                <h3 class="text-sm font-semibold text-gray-900 dark:text-white truncate">Infrastructure</h3>
                                <Show when={infrastructureHasIssue()}>
                                    <SeverityChip severity="critical" />
                                </Show>
                            </div>
                            <MiniDonut
                                size={28}
                                strokeWidth={3}
                                data={[
                                    { value: props.infrastructure.online, color: 'text-blue-500 dark:text-blue-400' },
                                    {
                                        value: props.infrastructure.total - props.infrastructure.online,
                                        color: 'text-gray-200 dark:text-gray-700',
                                    },
                                ]}
                            />
                        </div>

                        <div class="text-2xl font-bold text-gray-900 dark:text-white">{props.infrastructure.total}</div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">Total Nodes</div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-2 truncate">
                            {props.infrastructure.online} online
                            <Show when={props.topCPU[0]}>
                                <span>
                                    {' '}
                                    · Top CPU: {props.topCPU[0].name} {Math.round(props.topCPU[0].percent)}%
                                </span>
                            </Show>
                        </div>
                    </Card>
                </a>

                <a href={WORKLOADS_PATH} class="group block">
                    <Card
                        padding="none"
                        class={`h-full p-4 ${CARD_THEMES.purple.hoverBorder} transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700`}
                    >
                        <div class="flex items-start justify-between gap-2 mb-3">
                            <div class="flex items-center gap-2 min-w-0">
                                <div
                                    class={`p-2 rounded-lg transition-colors ${CARD_THEMES.purple.iconBg} ${CARD_THEMES.purple.iconColor} ${CARD_THEMES.purple.hoverIconBg}`}
                                >
                                    <Boxes class="w-5 h-5" />
                                </div>
                                <h3 class="text-sm font-semibold text-gray-900 dark:text-white truncate">Workloads</h3>
                                <Show when={workloadsHasIssue()}>
                                    <SeverityChip severity="warning" />
                                </Show>
                            </div>
                            <span class="text-[10px] font-bold px-2 py-0.5 rounded-full border bg-gray-50 border-gray-200 text-gray-600 dark:bg-gray-800 dark:border-gray-700 dark:text-gray-400">
                                {props.workloads.running} RUNNING
                            </span>
                        </div>

                        <div class="text-2xl font-bold text-gray-900 dark:text-white">{props.workloads.total}</div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">Total</div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                            {props.workloads.running} running · {props.workloads.stopped} stopped
                        </div>
                    </Card>
                </a>

                <a href={buildStoragePath()} class="group block">
                    <Card
                        padding="none"
                        class={`h-full p-4 ${CARD_THEMES.cyan.hoverBorder} transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700`}
                    >
                        <div class="flex items-start justify-between gap-2 mb-3">
                            <div class="flex items-center gap-2 min-w-0">
                                <div
                                    class={`p-2 rounded-lg transition-colors ${CARD_THEMES.cyan.iconBg} ${CARD_THEMES.cyan.iconColor} ${CARD_THEMES.cyan.hoverIconBg}`}
                                >
                                    <HardDrive class="w-5 h-5" />
                                </div>
                                <h3 class="text-sm font-semibold text-gray-900 dark:text-white truncate">Storage</h3>
                                <Show when={storageSeverity()}>{(severity) => <SeverityChip severity={severity()} />}</Show>
                            </div>
                            <MiniGauge
                                percent={props.storage.capacityPercent}
                                size={32}
                                strokeWidth={3.5}
                                color={props.storage.capacityPercent > 90 ? 'text-red-500' : 'text-cyan-500'}
                            />
                        </div>

                        <div class="text-2xl font-bold text-gray-900 dark:text-white flex items-baseline gap-0.5">
                            {Math.round(props.storage.capacityPercent)}
                            <span class="text-sm font-normal text-gray-400">%</span>
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">Capacity Used</div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                            {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
                        </div>
                    </Card>
                </a>

                <a href={buildRecoveryPath()} class="group block">
                    <Card
                        padding="none"
                        class={`h-full p-4 ${CARD_THEMES.emerald.hoverBorder} transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700`}
                    >
                        <div class="flex items-start justify-between gap-2 mb-3">
                            <div class="flex items-center gap-2 min-w-0">
                                <div
                                    class={`p-2 rounded-lg transition-colors ${CARD_THEMES.emerald.iconBg} ${CARD_THEMES.emerald.iconColor} ${CARD_THEMES.emerald.hoverIconBg}`}
                                >
                                    <Archive class="w-5 h-5" />
                                </div>
                                <h3 class="text-sm font-semibold text-gray-900 dark:text-white truncate">Recovery</h3>
                                <Show when={recoverySeverity()}>{(severity) => <SeverityChip severity={severity()} />}</Show>
                            </div>

                            <Show when={props.recovery.hasData}>
                                <MiniDonut
                                    size={28}
                                    strokeWidth={3}
                                    data={[
                                        {
                                            value: props.recovery.byOutcome.success ?? 0,
                                            color: 'text-emerald-500 dark:text-emerald-400',
                                        },
                                        { value: props.recovery.byOutcome.failed ?? 0, color: 'text-red-400 dark:text-red-400' },
                                        {
                                            value: props.recovery.byOutcome.warning ?? 0,
                                            color: 'text-amber-400 dark:text-amber-400',
                                        },
                                    ]}
                                />
                            </Show>
                        </div>

                        <Show
                            when={props.recovery.hasData}
                            fallback={<div class="text-2xl font-bold text-gray-900 dark:text-white">—</div>}
                        >
                            <div class="text-2xl font-bold text-gray-900 dark:text-white">{props.recovery.totalProtected}</div>
                        </Show>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">Protected</div>

                        <Show
                            when={props.recovery.hasData}
                            fallback={<div class="text-xs text-gray-500 dark:text-gray-400 mt-2">No data</div>}
                        >
                            <div class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                                {(props.recovery.byOutcome.success ?? 0)} ok · {(props.recovery.byOutcome.failed ?? 0)} failed
                                {' · Last '}
                                {formatRelativeTime(props.recovery.latestEventTimestamp ?? undefined, {
                                    compact: true,
                                    emptyText: '—',
                                })}
                            </div>
                        </Show>
                    </Card>
                </a>
            </div>
        </div>
    );
}
