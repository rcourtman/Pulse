import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import type { Host } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatBytes } from '@/utils/format';

interface ServersOverviewProps {
  hosts: Host[];
  connectionHealth: Record<string, boolean>;
}

const statusClass: Record<string, string> = {
  online: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20',
  degraded: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20',
  offline: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20',
};

const formatStatus = (status: string | undefined) => {
  if (!status) return 'unknown';
  const normalized = status.toLowerCase();
  if (normalized === 'online' || normalized === 'degraded' || normalized === 'offline') {
    return normalized;
  }
  return status;
};

export const ServersOverview: Component<ServersOverviewProps> = (props) => {
  const sortedHosts = createMemo(() =>
    [...props.hosts].sort((a, b) => a.hostname.localeCompare(b.hostname)),
  );

  return (
    <div class="space-y-6">
      <header class="flex flex-col gap-2">
        <h1 class="text-2xl font-semibold text-slate-900 dark:text-slate-100">Servers</h1>
        <p class="text-sm text-slate-600 dark:text-slate-400">
          Unified view of standalone hosts reporting via the Pulse host agent.
        </p>
      </header>

      <Show
        when={sortedHosts().length > 0}
        fallback={
          <EmptyState
            title="No servers reporting yet"
            description="Install the pulse-host-agent on a Linux, Windows, or macOS machine to have it appear here."
          />
        }
      >
        <div class="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
          <For each={sortedHosts()}>
            {(host) => {
              const status = formatStatus(host.status);
              const statusClasses =
                statusClass[status] ??
                'bg-slate-500/10 text-slate-600 dark:text-slate-300 border border-slate-500/20';
              const lastSeen = new Date(host.lastSeen || Date.now());
              const connectionKey = `host-${host.id}`;
              const isHealthy = props.connectionHealth[connectionKey] ?? status !== 'offline';
              const memoryUsage =
                typeof host.memory?.usage === 'number'
                  ? Math.round((host.memory.usage + Number.EPSILON) * 10) / 10
                  : undefined;

              return (
                <Card class="flex flex-col gap-4 p-4">
                  <div class="flex items-start justify-between gap-3">
                    <div>
                      <p class="text-sm font-medium text-slate-500 uppercase tracking-wide">
                        {host.platform ?? 'unknown'}
                      </p>
                      <h2 class="text-lg font-semibold text-slate-900 dark:text-slate-100">
                        {host.displayName || host.hostname}
                      </h2>
                      <p class="text-sm text-slate-500 dark:text-slate-400">
                        {host.osName}
                        {host.osVersion ? ` ${host.osVersion}` : ''}
                      </p>
                    </div>
                    <span class={`rounded-full px-3 py-1 text-xs font-medium ${statusClasses}`}>
                      {status}
                    </span>
                  </div>

                  <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                    <div>
                      <dt class="text-slate-500 dark:text-slate-400">CPU Usage</dt>
                      <dd class="font-semibold text-slate-900 dark:text-slate-100">
                        {typeof host.cpuUsage === 'number' ? `${host.cpuUsage.toFixed(1)}%` : '—'}
                      </dd>
                    </div>
                    <div>
                      <dt class="text-slate-500 dark:text-slate-400">Memory</dt>
                      <dd class="font-semibold text-slate-900 dark:text-slate-100">
                        {host.memory?.total
                          ? `${formatBytes(host.memory.used ?? 0)} / ${formatBytes(host.memory.total)}${
                              memoryUsage !== undefined ? ` (${memoryUsage.toFixed(1)}%)` : ''
                            }`
                          : '—'}
                      </dd>
                    </div>
                    <div>
                      <dt class="text-slate-500 dark:text-slate-400">Architecture</dt>
                      <dd class="font-semibold text-slate-900 dark:text-slate-100">
                        {host.architecture ?? '—'}
                      </dd>
                    </div>
                    <div>
                      <dt class="text-slate-500 dark:text-slate-400">Last Seen</dt>
                      <dd class="font-semibold text-slate-900 dark:text-slate-100">
                        {lastSeen.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                      </dd>
                    </div>
                    <div class="col-span-2">
                      <dt class="text-slate-500 dark:text-slate-400">Connection</dt>
                      <dd class="font-semibold text-slate-900 dark:text-slate-100">
                        {isHealthy ? 'Healthy' : 'Unreachable'}
                      </dd>
                    </div>
                  </dl>

                  <Show when={host.disks && host.disks.length > 0}>
                    <div class="rounded-md border border-slate-200 bg-slate-50 p-3 dark:border-slate-700/70 dark:bg-slate-900/70">
                      <p class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
                        Storage
                      </p>
                      <ul class="mt-2 space-y-1 text-xs text-slate-600 dark:text-slate-300">
                        <For each={host.disks}>
                          {(disk) => (
                            <li class="flex items-center justify-between">
                              <span class="truncate">
                                {disk.mountpoint || disk.device || 'disk'} •{' '}
                                {disk.type ? disk.type.toUpperCase() : '—'}
                              </span>
                              <span>
                                {formatBytes(disk.used ?? 0)} / {formatBytes(disk.total ?? 0)}
                                {typeof disk.usage === 'number'
                                  ? ` (${disk.usage.toFixed(1)}%)`
                                  : ''}
                              </span>
                            </li>
                          )}
                        </For>
                      </ul>
                    </div>
                  </Show>
                </Card>
              );
            }}
          </For>
        </div>
      </Show>
    </div>
  );
};
