import { createMemo, Show, For } from 'solid-js';
import { Dynamic } from 'solid-js/web';

import Server from 'lucide-solid/icons/server';
import Box from 'lucide-solid/icons/box';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Bell from 'lucide-solid/icons/bell';
import CheckCircle from 'lucide-solid/icons/check-circle';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import XCircle from 'lucide-solid/icons/x-circle';

import {
    ALERTS_OVERVIEW_PATH,
    INFRASTRUCTURE_PATH,
    WORKLOADS_PATH,
    buildStoragePath,
} from '@/routing/resourceLinks';
import { Card } from '@/components/shared/Card';
import { type ActionItem } from './dashboardHelpers';
import { MiniDonut, MiniGauge } from './Visualizations';

interface DashboardHeroProps {
    title?: string;
    totalResources: number;
    criticalAlerts: number;
    warningAlerts: number;
    byStatus: Record<string, number>;
    infrastructure: { total: number; online: number };
    workloads: { total: number; running: number };
    storage: { capacityPercent: number; totalUsed: number; totalCapacity: number };
    alerts: { activeCritical: number; activeWarning: number; total: number };
    topIssues?: ActionItem[];
}

export function DashboardHero(props: DashboardHeroProps) {
    const allIssues = createMemo(() => props.topIssues ?? []);
    const needsTicker = createMemo(() => allIssues().length > 3);

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
                    description: `${props.criticalAlerts} critical alert${props.criticalAlerts === 1 ? '' : 's'} requires immediate attention.`,
                    icon: XCircle,
                    color: 'text-red-600 dark:text-red-400',
                    bg: 'bg-red-50 dark:bg-red-900/20',
                    border: 'border-red-200 dark:border-red-800',
                    pulseColor: 'bg-red-500',
                    gradient: 'from-red-500/10 to-transparent'
                };
            case 'warning':
                return {
                    label: 'System Warning',
                    description: `${props.warningAlerts} warning alert${props.warningAlerts === 1 ? '' : 's'} active.`,
                    icon: AlertTriangle,
                    color: 'text-amber-600 dark:text-amber-400',
                    bg: 'bg-amber-50 dark:bg-amber-900/20',
                    border: 'border-amber-200 dark:border-amber-800',
                    pulseColor: 'bg-amber-500',
                    gradient: 'from-amber-500/10 to-transparent'
                };
            case 'healthy':
            default:
                return {
                    label: 'All Systems Operational',
                    description: 'Infrastructure and workloads are running normally.',
                    icon: CheckCircle,
                    color: 'text-emerald-600 dark:text-emerald-400',
                    bg: 'bg-emerald-50 dark:bg-emerald-900/20',
                    border: 'border-emerald-200 dark:border-emerald-800',
                    pulseColor: 'bg-emerald-500',
                    gradient: 'from-emerald-500/10 to-transparent'
                };
        }
    });



    return (
        <div class="grid grid-cols-1 lg:grid-cols-12 gap-4 mb-2">
            {/* Primary Status Card (Spans 4 columns) */}
            <Card
                padding="none"
                class={`lg:col-span-4 relative overflow-hidden flex flex-col justify-between border-l-4 shadow-lg hover:shadow-xl transition-shadow duration-300 ${statusConfig().bg} ${statusConfig().color.replace('text-', 'border-l-')}`}
            >
                {/* Refined Background Gradient */}
                <div class={`absolute inset-0 bg-gradient-to-br ${statusConfig().gradient} opacity-60 pointer-events-none mix-blend-overlay`} />
                <div class="absolute -right-10 -top-10 w-40 h-40 bg-white dark:bg-gray-800 opacity-10 rounded-full blur-3xl pointer-events-none" />

                <div class="relative p-6 z-10 flex flex-col h-full justify-between">
                    <div>
                        <div class="flex items-start justify-between mb-6">
                            <div class={`p-2.5 rounded-xl shadow-inner ${status() === 'healthy' ? 'bg-emerald-100/80 dark:bg-emerald-900/40' : status() === 'warning' ? 'bg-amber-100/80 dark:bg-amber-900/40' : 'bg-red-100/80 dark:bg-red-900/40'}`}>
                                <Dynamic component={statusConfig().icon} class={`w-7 h-7 drop-shadow-sm ${statusConfig().color}`} />
                            </div>

                            {/* Pulse Indicator */}
                            <div class="flex items-center gap-2.5 bg-white/50 dark:bg-black/20 px-2.5 py-1 rounded-full backdrop-blur-sm border border-white/20 dark:border-white/5">
                                <span class="relative flex h-2.5 w-2.5">
                                    <span class={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${statusConfig().pulseColor}`}></span>
                                    <span class={`relative inline-flex rounded-full h-2.5 w-2.5 ${statusConfig().pulseColor}`}></span>
                                </span>
                                <span class="text-[10px] font-bold uppercase tracking-widest opacity-70">System Status</span>
                            </div>
                        </div>

                        <h2 class="text-2xl font-extrabold tracking-tight text-gray-900 dark:text-white mb-3 drop-shadow-sm">
                            {statusConfig().label}
                        </h2>

                        <Show
                            when={allIssues().length > 0}
                            fallback={
                                <p class="text-sm font-medium text-gray-700 dark:text-gray-200 opacity-90 leading-relaxed max-w-[95%]">
                                    {statusConfig().description}
                                </p>
                            }
                        >
                            <div class="mt-2">
                                <p class="text-[10px] uppercase tracking-wider font-bold text-gray-500 dark:text-gray-400 mb-1.5 opacity-80">
                                    Active Incidents
                                    <Show when={allIssues().length > 1}>
                                        <span class="ml-1 font-normal">({allIssues().length})</span>
                                    </Show>
                                </p>
                                <div
                                    class="overflow-hidden relative group/ticker"
                                    style={{ "max-height": needsTicker() ? "108px" : undefined }}
                                >
                                    <div
                                        class={needsTicker() ? "group-hover/ticker:[animation-play-state:paused]" : ""}
                                        style={needsTicker() ? {
                                            animation: `ticker-up ${allIssues().length * 3}s linear infinite`,
                                        } : {}}
                                    >
                                        <For each={needsTicker() ? [...allIssues(), ...allIssues()] : allIssues()}>
                                            {(item) => (
                                                <a href={item.link} class="group/item flex items-center gap-2.5 p-2 -mx-2 rounded-lg hover:bg-white/40 dark:hover:bg-black/20 transition-colors backdrop-blur-sm">
                                                    <span class={`flex h-2 w-2 shrink-0 rounded-full ring-2 ring-white/20 ${item.priority === 'critical' ? 'bg-red-500' : 'bg-amber-500'}`} />
                                                    <span class="text-xs font-semibold text-gray-800 dark:text-gray-100 truncate group-hover/item:underline decoration-black/20 dark:decoration-white/20 underline-offset-2">
                                                        {item.label}
                                                    </span>
                                                </a>
                                            )}
                                        </For>
                                    </div>
                                    <Show when={needsTicker()}>
                                        <style>{`@keyframes ticker-up { 0% { transform: translateY(0); } 100% { transform: translateY(-50%); } }`}</style>
                                    </Show>
                                </div>
                            </div>
                        </Show>
                    </div>
                </div>
            </Card>

            {/* Metrics Grid (Spans 8 columns) */}
            <div class="lg:col-span-8 grid grid-cols-2 sm:grid-cols-4 gap-3">
                {/* Infrastructure */}
                <a href={INFRASTRUCTURE_PATH} class="group block">
                    <Card padding="none" class="h-full p-4 hover:border-blue-400 dark:hover:border-blue-500 transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700">
                        <div class="flex items-start justify-between mb-4">
                            <div class="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-500 dark:text-blue-400 group-hover:bg-blue-100 dark:group-hover:bg-blue-900/30 transition-colors">
                                <Server class="w-5 h-5" />
                            </div>
                            <MiniDonut
                                size={28}
                                strokeWidth={3}
                                data={[
                                    { value: props.infrastructure.online, color: 'text-blue-500 dark:text-blue-400' },
                                    { value: props.infrastructure.total - props.infrastructure.online, color: 'text-gray-200 dark:text-gray-700' }
                                ]}
                            />
                        </div>
                        <div class="space-y-0.5">
                            <div class="text-xs text-gray-500 dark:text-gray-400 font-semibold uppercase tracking-wide flex items-center gap-1">
                                Infrastructure
                                <span class="text-[10px] font-normal text-gray-400 ml-auto">{props.infrastructure.online} Online</span>
                            </div>
                            <div class="text-2xl font-bold text-gray-900 dark:text-white tracking-tight">
                                {props.infrastructure.total}
                            </div>
                        </div>
                    </Card>
                </a>

                {/* Workloads */}
                <a href={WORKLOADS_PATH} class="group block">
                    <Card padding="none" class="h-full p-4 hover:border-purple-400 dark:hover:border-purple-500 transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700">
                        <div class="flex items-center justify-between mb-4">
                            <div class="p-2 rounded-lg bg-purple-50 dark:bg-purple-900/20 text-purple-500 dark:text-purple-400 group-hover:bg-purple-100 dark:group-hover:bg-purple-900/30 transition-colors">
                                <Box class="w-5 h-5" />
                            </div>
                            <span class="text-[10px] font-bold px-2 py-0.5 rounded-full border bg-gray-50 border-gray-200 text-gray-600 dark:bg-gray-800 dark:border-gray-700 dark:text-gray-400">
                                {props.workloads.running} RUNNING
                            </span>
                        </div>
                        <div class="space-y-0.5">
                            <div class="text-xs text-gray-500 dark:text-gray-400 font-semibold uppercase tracking-wide">Workloads</div>
                            <div class="text-2xl font-bold text-gray-900 dark:text-white tracking-tight">
                                {props.workloads.total}
                            </div>
                        </div>
                    </Card>
                </a>

                {/* Storage */}
                <a href={buildStoragePath()} class="group block">
                    <Card padding="none" class="h-full p-4 hover:border-cyan-400 dark:hover:border-cyan-500 transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700">
                        <div class="flex items-start justify-between mb-4">
                            <div class="p-2 rounded-lg bg-cyan-50 dark:bg-cyan-900/20 text-cyan-600 dark:text-cyan-400 group-hover:bg-cyan-100 dark:group-hover:bg-cyan-900/30 transition-colors">
                                <HardDrive class="w-5 h-5" />
                            </div>
                            <MiniGauge
                                percent={props.storage.capacityPercent}
                                size={32}
                                strokeWidth={3.5}
                                color={props.storage.capacityPercent > 90 ? 'text-red-500' : 'text-cyan-500'}
                            />
                        </div>
                        <div class="space-y-0.5">
                            <div class="text-xs text-gray-500 dark:text-gray-400 font-semibold uppercase tracking-wide">Storage Used</div>
                            <div class="text-2xl font-bold text-gray-900 dark:text-white tracking-tight flex items-baseline gap-0.5">
                                {Math.round(props.storage.capacityPercent)}<span class="text-sm font-normal text-gray-400">%</span>
                            </div>
                        </div>
                    </Card>
                </a>

                {/* Alerts */}
                <a href={ALERTS_OVERVIEW_PATH} class="group block">
                    <Card padding="none" class="h-full p-4 hover:border-amber-400 dark:hover:border-amber-500 transition-all duration-300 cursor-pointer hover:shadow-lg hover:-translate-y-0.5 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700">
                        <div class="flex items-start justify-between mb-4">
                            <div class={`p-2 rounded-lg transition-colors ${props.alerts.activeCritical > 0 || props.alerts.activeWarning > 0 ? 'bg-amber-50 dark:bg-amber-900/20 text-amber-500 dark:text-amber-400' : 'bg-gray-50 dark:bg-gray-800 text-gray-400 group-hover:text-amber-500 group-hover:bg-amber-50 dark:group-hover:bg-amber-900/20'}`}>
                                <Bell class="w-5 h-5" />
                            </div>
                            <div class="flex flex-col items-end gap-1">
                                <Show when={props.alerts.activeCritical > 0}>
                                    <div class="flex items-center gap-1.5 px-1.5 py-0.5 bg-red-50 dark:bg-red-900/30 border border-red-100 dark:border-red-800 rounded">
                                        <div class="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse" />
                                        <span class="text-[9px] font-bold text-red-700 dark:text-red-300">{props.alerts.activeCritical} CRIT</span>
                                    </div>
                                </Show>
                                <Show when={props.alerts.activeWarning > 0}>
                                    <span class="text-[9px] font-semibold text-amber-600 dark:text-amber-400">{props.alerts.activeWarning} WARN</span>
                                </Show>
                            </div>
                        </div>
                        <div class="space-y-0.5">
                            <div class="text-xs text-gray-500 dark:text-gray-400 font-semibold uppercase tracking-wide">Active Alerts</div>
                            <div class="text-2xl font-bold text-gray-900 dark:text-white tracking-tight">
                                {props.alerts.total}
                            </div>
                        </div>
                    </Card>
                </a>
            </div>
        </div>
    );
}
