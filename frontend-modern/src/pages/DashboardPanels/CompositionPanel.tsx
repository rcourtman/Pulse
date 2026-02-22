import { For, Show } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { Card } from '@/components/shared/Card';
import Server from 'lucide-solid/icons/server';
import AppWindow from 'lucide-solid/icons/app-window';
import Database from 'lucide-solid/icons/database';
import Box from 'lucide-solid/icons/box';
import Container from 'lucide-solid/icons/container';

interface CompositionPanelProps {
    infrastructureByType: Record<string, number>;
    workloadsByType: Record<string, number>;
}

const TYPE_ICONS: Record<string, any> = {
    vm: Server,
    lxc: Box,
    docker: Container,
    k8s: AppWindow,
    database: Database,
    unknown: Server,
};

const TYPE_LABELS: Record<string, string> = {
    vm: 'Virtual Machines',
    lxc: 'LXC Containers',
    docker: 'Docker Containers',
    k8s: 'Kubernetes Pods',
    database: 'Databases',
};

function DistributionItem(props: { type: string; count: number; total: number }) {
    const percent = () => Math.round((props.count / props.total) * 100);
    const Icon = () => TYPE_ICONS[props.type] ?? Server;

    return (
        <div class="space-y-1">
            <div class="flex items-center justify-between text-xs">
                <div class="flex items-center gap-2 text-base-content">
                    <Dynamic component={Icon()} class="w-3.5 h-3.5 text-muted" />
                    <span class="font-medium">{TYPE_LABELS[props.type] ?? props.type}</span>
                </div>
                <div class="flex items-center gap-1.5">
                    <span class="font-bold text-base-content">{props.count}</span>
                    <span class="text-slate-400 dark:text-slate-600">({percent()}%)</span>
                </div>
            </div>
            <div class="h-1.5 w-full bg-surface-alt rounded-full overflow-hidden">
                <div
                    class="h-full bg-blue-500 dark:bg-blue-600 rounded-full"
                    style={{ width: `${percent()}%` }}
                />
            </div>
        </div>
    );
}

export function CompositionPanel(props: CompositionPanelProps) {
    const infraTypes = () => Object.entries(props.infrastructureByType).filter((item): item is [string, number] => item[1] > 0);
    const workloadTypes = () => Object.entries(props.workloadsByType).filter((item): item is [string, number] => item[1] > 0);

    const totalInfra = () => Object.values(props.infrastructureByType).reduce((a, b) => a + b, 0);
    const totalWorkloads = () => Object.values(props.workloadsByType).reduce((a, b) => a + b, 0);

    return (
        <Card padding="none" class="px-4 py-3.5">
            <div class="flex items-center gap-2 mb-3">
                <h2 class="text-xs font-semibold text-muted uppercase tracking-wide">
                    Composition
                </h2>
            </div>

            <div class="space-y-4">
                <Show when={totalInfra() > 0}>
                    <div class="space-y-2">
                        <h3 class="text-[10px] font-bold text-muted uppercase tracking-wider">Infrastructure</h3>
                        <div class="space-y-3">
                            <For each={infraTypes()}>
                                {([type, count]) => (
                                    <DistributionItem type={type} count={count} total={totalInfra()} />
                                )}
                            </For>
                        </div>
                    </div>
                </Show>

                <Show when={totalInfra() > 0 && totalWorkloads() > 0}>
                    <div class="h-px bg-surface-alt" />
                </Show>

                <Show when={totalWorkloads() > 0}>
                    <div class="space-y-2">
                        <h3 class="text-[10px] font-bold text-muted uppercase tracking-wider">Workloads</h3>
                        <div class="space-y-3">
                            <For each={workloadTypes()}>
                                {([type, count]) => (
                                    <DistributionItem type={type} count={count} total={totalWorkloads()} />
                                )}
                            </For>
                        </div>
                    </div>
                </Show>

                <Show when={totalInfra() === 0 && totalWorkloads() === 0}>
                    <p class="text-xs text-muted py-2 text-center italic">No resources detected</p>
                </Show>
            </div>
        </Card>
    );
}

export default CompositionPanel;

