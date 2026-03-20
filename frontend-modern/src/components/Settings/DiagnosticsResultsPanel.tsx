import { Component, For, Show } from 'solid-js';
import type { JSX } from 'solid-js';
import { formatRelativeTime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import Activity from 'lucide-solid/icons/activity';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import CheckCircle from 'lucide-solid/icons/check-circle';
import Cpu from 'lucide-solid/icons/cpu';
import Database from 'lucide-solid/icons/database';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Network from 'lucide-solid/icons/network';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Server from 'lucide-solid/icons/server';
import Shield from 'lucide-solid/icons/shield';
import Sparkles from 'lucide-solid/icons/sparkles';
import XCircle from 'lucide-solid/icons/x-circle';
import { StatusDot } from '@/components/shared/StatusDot';
import { getSimpleStatusIndicator, getStatusIndicatorBadgeToneClasses } from '@/utils/status';
import { getSemanticTonePresentation } from '@/utils/semanticTonePresentation';
import { DIAGNOSTICS_EMPTY_PBS_MESSAGE } from '@/utils/diagnosticsPresentation';
import type { DiagnosticsData } from '@/components/Settings/diagnosticsModel';

const DiagnosticCard: Component<{
  children: JSX.Element;
  icon: Component<{ class?: string }>;
  status?: 'success' | 'warning' | 'error' | 'info';
  title: string;
}> = (props) => {
  const tone = () => getSemanticTonePresentation(props.status || 'info');

  return (
    <div class={`rounded-md border p-4 transition-all hover:shadow-sm ${tone().panelClass}`}>
      <div class="mb-3 flex items-center gap-3">
        <div class={`rounded-md bg-surface p-2 ${tone().iconClass}`}>
          <props.icon class="h-4 w-4" />
        </div>
        <h4 class="text-sm font-semibold text-base-content">{props.title}</h4>
      </div>
      <div class="space-y-1.5 text-xs text-muted">{props.children}</div>
    </div>
  );
};

const StatusBadge: Component<{
  label?: string;
  status: 'online' | 'offline' | 'warning' | 'unknown';
}> = (props) => {
  const indicator = () => getSimpleStatusIndicator(props.status);

  return (
    <span
      class={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide ${getStatusIndicatorBadgeToneClasses(indicator().variant)}`}
    >
      <StatusDot
        variant={indicator().variant}
        size="xs"
        ariaHidden={true}
        class="translate-y-[0.5px]"
      />
      {props.label || indicator().label}
    </span>
  );
};

const MetricRow: Component<{
  label: string;
  mono?: boolean;
  value: string | number | undefined;
}> = (props) => (
  <div class="flex items-center justify-between border-b border-border-subtle py-1.5 last:border-0">
    <span class="text-muted">{props.label}</span>
    <span class={`text-base-content ${props.mono ? 'font-mono text-[11px]' : 'font-medium'}`}>
      {props.value ?? 'Unknown'}
    </span>
  </div>
);

interface DiagnosticsResultsPanelProps {
  diagnosticsData: DiagnosticsData | null;
  loading: boolean;
  onRunDiagnostics: () => void;
}

export const DiagnosticsResultsPanel: Component<DiagnosticsResultsPanelProps> = (props) => {
  return (
    <Show
      when={props.diagnosticsData}
      fallback={
        <Card padding="lg" class="text-center">
          <div class="py-12">
            <Activity class="mx-auto mb-4 h-12 w-12 text-muted" />
            <h3 class="mb-2 text-lg font-medium text-base-content">No diagnostics data yet</h3>
            <p class="mb-6 text-sm text-muted">
              Click "Run Diagnostics" above to test connections and inspect system status
            </p>
            <button
              type="button"
              onClick={props.onRunDiagnostics}
              disabled={props.loading}
              class="inline-flex min-h-10 items-center gap-2 rounded-md bg-blue-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50 sm:min-h-9"
            >
              <RefreshCw class={`h-4 w-4 ${props.loading ? 'animate-spin' : ''}`} />
              Run Diagnostics
            </button>
          </div>
        </Card>
      }
    >
      <div class="space-y-6">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <DiagnosticCard title="System Runtime" icon={Cpu} status="info">
            <MetricRow
              label="OS / Arch"
              value={`${props.diagnosticsData?.system?.os || '?'} / ${props.diagnosticsData?.system?.arch || '?'}`}
            />
            <MetricRow label="Go Runtime" value={props.diagnosticsData?.system?.goVersion} mono />
            <MetricRow label="CPU Cores" value={props.diagnosticsData?.system?.numCPU} />
            <MetricRow label="Goroutines" value={props.diagnosticsData?.system?.numGoroutine} />
            <MetricRow label="Memory" value={`${props.diagnosticsData?.system?.memoryMB || 0} MB`} />
          </DiagnosticCard>

          <DiagnosticCard
            title="PVE Nodes"
            icon={Server}
            status={props.diagnosticsData?.nodes?.every((node) => node.connected) ? 'success' : 'warning'}
          >
            <div class="mb-2 flex items-center justify-between">
              <span>Total Nodes</span>
              <span class="text-lg font-bold text-base-content">
                {props.diagnosticsData?.nodes?.length || 0}
              </span>
            </div>
            <div class="space-y-1">
              <For each={props.diagnosticsData?.nodes || []}>
                {(node) => (
                  <div class="flex items-center justify-between border-b border-border-subtle py-1 last:border-0">
                    <span class="max-w-[120px] truncate" title={node.host}>
                      {node.name}
                    </span>
                    <StatusBadge status={node.connected ? 'online' : 'offline'} />
                  </div>
                )}
              </For>
            </div>
          </DiagnosticCard>

          <DiagnosticCard
            title="PBS Instances"
            icon={HardDrive}
            status={
              props.diagnosticsData?.pbs?.every((pbs) => pbs.connected)
                ? 'success'
                : props.diagnosticsData?.pbs?.length
                  ? 'warning'
                  : 'info'
            }
          >
            <Show
              when={(props.diagnosticsData?.pbs?.length || 0) > 0}
              fallback={<div class="py-4 text-center text-muted">{DIAGNOSTICS_EMPTY_PBS_MESSAGE}</div>}
            >
              <div class="mb-2 flex items-center justify-between">
                <span>Total Instances</span>
                <span class="text-lg font-bold text-base-content">
                  {props.diagnosticsData?.pbs?.length || 0}
                </span>
              </div>
              <div class="space-y-1">
                <For each={props.diagnosticsData?.pbs || []}>
                  {(pbs) => (
                    <div class="flex items-center justify-between border-b border-border-subtle py-1 last:border-0">
                      <span class="max-w-[120px] truncate" title={pbs.host}>
                        {pbs.name}
                      </span>
                      <StatusBadge status={pbs.connected ? 'online' : 'offline'} />
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </DiagnosticCard>

          <DiagnosticCard
            title="Network Discovery"
            icon={Network}
            status={props.diagnosticsData?.discovery?.enabled ? 'success' : 'info'}
          >
            <MetricRow
              label="Status"
              value={props.diagnosticsData?.discovery?.enabled ? 'Enabled' : 'Disabled'}
            />
            <MetricRow
              label="Subnet"
              value={props.diagnosticsData?.discovery?.configuredSubnet || 'auto'}
              mono
            />
            <MetricRow
              label="Scan Interval"
              value={props.diagnosticsData?.discovery?.scanInterval || 'default'}
            />
            <MetricRow
              label="Last Scan"
              value={formatRelativeTime(props.diagnosticsData?.discovery?.lastScanStartedAt, {
                compact: true,
                emptyText: 'Never',
              })}
            />
            <MetricRow
              label="Servers Found"
              value={props.diagnosticsData?.discovery?.lastResultServers ?? 0}
            />
          </DiagnosticCard>
        </div>

        <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Show when={props.diagnosticsData?.metricsStore}>
            <Card padding="md">
              <div class="mb-4 flex items-center gap-3 border-b border-border pb-3">
                <div class="rounded-md bg-blue-100 p-2 dark:bg-blue-900">
                  <Database class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <h4 class="text-sm font-semibold text-base-content">Metrics Store</h4>
                  <p class="text-xs text-muted">History persistence health</p>
                </div>
                <div class="ml-auto">
                  <StatusBadge
                    status={
                      props.diagnosticsData?.metricsStore?.status === 'healthy'
                        ? 'online'
                        : props.diagnosticsData?.metricsStore?.status === 'buffering'
                          ? 'warning'
                          : props.diagnosticsData?.metricsStore?.status === 'empty'
                            ? 'warning'
                            : 'offline'
                    }
                    label={props.diagnosticsData?.metricsStore?.status || 'unknown'}
                  />
                </div>
              </div>
              <div class="space-y-2 text-xs">
                <MetricRow
                  label="Enabled"
                  value={props.diagnosticsData?.metricsStore?.enabled ? 'Yes' : 'No'}
                />
                <MetricRow
                  label="DB Size"
                  value={`${Math.round((props.diagnosticsData?.metricsStore?.dbSize ?? 0) / (1024 * 1024))} MB`}
                />
                <MetricRow
                  label="Total Points"
                  value={props.diagnosticsData?.metricsStore?.totalPoints ?? 0}
                />
                <MetricRow
                  label="Raw Points"
                  value={props.diagnosticsData?.metricsStore?.rawCount ?? 0}
                />
                <MetricRow
                  label="Minute Points"
                  value={props.diagnosticsData?.metricsStore?.minuteCount ?? 0}
                />
                <MetricRow
                  label="Hourly Points"
                  value={props.diagnosticsData?.metricsStore?.hourlyCount ?? 0}
                />
                <MetricRow
                  label="Daily Points"
                  value={props.diagnosticsData?.metricsStore?.dailyCount ?? 0}
                />
                <MetricRow
                  label="Buffer Size"
                  value={props.diagnosticsData?.metricsStore?.bufferSize ?? 0}
                />
              </div>
              <Show when={(props.diagnosticsData?.metricsStore?.notes?.length || 0) > 0}>
                <div class="mt-3 rounded-md border border-amber-200 bg-amber-50 p-2 dark:border-amber-800 dark:bg-amber-900">
                  <div class="flex items-start gap-2 text-xs text-amber-700 dark:text-amber-300">
                    <AlertTriangle class="mt-0.5 h-4 w-4 flex-shrink-0" />
                    <div class="space-y-1">
                      <For each={props.diagnosticsData?.metricsStore?.notes || []}>
                        {(note) => <div>{note}</div>}
                      </For>
                    </div>
                  </div>
                </div>
              </Show>
              <Show when={props.diagnosticsData?.metricsStore?.error}>
                <div class="mt-3 rounded-md border border-red-200 bg-red-50 p-2 text-xs text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300">
                  {props.diagnosticsData?.metricsStore?.error}
                </div>
              </Show>
            </Card>
          </Show>

          <Show when={props.diagnosticsData?.apiTokens}>
            <Card padding="md">
              <div class="mb-4 flex items-center gap-3 border-b border-border pb-3">
                <div class="rounded-md bg-blue-100 p-2 dark:bg-blue-900">
                  <Shield class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <h4 class="text-sm font-semibold text-base-content">API Tokens</h4>
                  <p class="text-xs text-muted">Authentication status</p>
                </div>
                <div class="ml-auto">
                  <StatusBadge
                    status={props.diagnosticsData?.apiTokens?.enabled ? 'online' : 'warning'}
                    label={props.diagnosticsData?.apiTokens?.enabled ? 'Enabled' : 'Disabled'}
                  />
                </div>
              </div>
              <div class="space-y-2 text-xs">
                <MetricRow
                  label="Configured Tokens"
                  value={props.diagnosticsData?.apiTokens?.tokenCount}
                />
                <MetricRow
                  label="Unused Tokens"
                  value={props.diagnosticsData?.apiTokens?.unusedTokenCount ?? 0}
                />
              </div>
            </Card>
          </Show>

          <Show when={props.diagnosticsData?.dockerAgents}>
            <Card padding="md">
              <div class="mb-4 flex items-center gap-3 border-b border-border pb-3">
                <div class="rounded-md bg-blue-100 p-2 dark:bg-blue-900">
                  <Database class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <h4 class="text-sm font-semibold text-base-content">Container Runtime Agents</h4>
                  <p class="text-xs text-muted">Agent-backed container runtime monitoring</p>
                </div>
                <div class="ml-auto text-right">
                  <div class="text-lg font-bold text-base-content">
                    {props.diagnosticsData?.dockerAgents?.agentsOnline}/
                    {props.diagnosticsData?.dockerAgents?.agentsTotal}
                  </div>
                  <div class="text-[10px] text-muted">online</div>
                </div>
              </div>
              <div class="space-y-2 text-xs">
                <MetricRow
                  label="With Token Binding"
                  value={props.diagnosticsData?.dockerAgents?.agentsWithTokenBinding}
                />
                <MetricRow
                  label="Need Attention"
                  value={props.diagnosticsData?.dockerAgents?.agentsNeedingAttention}
                />
                <MetricRow
                  label="Outdated Version"
                  value={props.diagnosticsData?.dockerAgents?.agentsOutdatedVersion ?? 0}
                />
              </div>
              <Show when={props.diagnosticsData?.dockerAgents?.recommendedAgentVersion}>
                <div class="mt-3 border-t border-border-subtle pt-2 text-xs text-muted">
                  Recommended version: {props.diagnosticsData?.dockerAgents?.recommendedAgentVersion}
                </div>
              </Show>
            </Card>
          </Show>

          <Show when={props.diagnosticsData?.alerts}>
            <Card padding="md">
              <div class="mb-4 flex items-center gap-3 border-b border-border pb-3">
                <div class="rounded-md bg-rose-100 p-2 dark:bg-rose-900">
                  <AlertTriangle class="h-4 w-4 text-rose-600 dark:text-rose-400" />
                </div>
                <div>
                  <h4 class="text-sm font-semibold text-base-content">Alerts Configuration</h4>
                  <p class="text-xs text-muted">Alert system status</p>
                </div>
              </div>
              <div class="flex flex-wrap gap-2">
                <span
                  class={`rounded px-2 py-1 text-xs font-medium ${
                    getStatusIndicatorBadgeToneClasses(
                      props.diagnosticsData?.alerts?.missingCooldown ? 'warning' : 'success',
                    )
                  }`}
                >
                  Cooldown:{' '}
                  {props.diagnosticsData?.alerts?.missingCooldown ? 'Missing' : 'Configured'}
                </span>
                <span
                  class={`rounded px-2 py-1 text-xs font-medium ${
                    getStatusIndicatorBadgeToneClasses(
                      props.diagnosticsData?.alerts?.missingGroupingWindow ? 'warning' : 'success',
                    )
                  }`}
                >
                  Grouping:{' '}
                  {props.diagnosticsData?.alerts?.missingGroupingWindow ? 'Disabled' : 'Enabled'}
                </span>
              </div>
              <Show when={(props.diagnosticsData?.alerts?.notes?.length || 0) > 0}>
                <ul class="mt-3 list-disc space-y-1 border-t border-border-subtle pt-2 pl-4 text-xs text-muted">
                  <For each={props.diagnosticsData?.alerts?.notes || []}>
                    {(note) => <li>{note}</li>}
                  </For>
                </ul>
              </Show>
            </Card>
          </Show>

          <Show when={props.diagnosticsData?.aiChat}>
            <Card padding="md">
              <div class="mb-4 flex items-center gap-3 border-b border-border pb-3">
                <div class="rounded-md bg-blue-100 p-2 dark:bg-blue-900">
                  <Sparkles class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <h4 class="text-sm font-semibold text-base-content">Pulse Assistant</h4>
                  <p class="text-xs text-muted">Pulse Assistant Service</p>
                </div>
                <div class="ml-auto">
                  <StatusBadge
                    status={
                      props.diagnosticsData?.aiChat?.running
                        ? 'online'
                        : props.diagnosticsData?.aiChat?.enabled
                          ? 'offline'
                          : 'unknown'
                    }
                    label={
                      props.diagnosticsData?.aiChat?.running
                        ? 'Running'
                        : props.diagnosticsData?.aiChat?.enabled
                          ? 'Stopped'
                          : 'Disabled'
                    }
                  />
                </div>
              </div>
              <div class="space-y-2 text-xs">
                <MetricRow label="Model" value={props.diagnosticsData?.aiChat?.model} />
                <MetricRow label="Port" value={props.diagnosticsData?.aiChat?.port} mono />
                <MetricRow
                  label="Status"
                  value={props.diagnosticsData?.aiChat?.healthy ? 'Healthy' : 'Unhealthy'}
                />
              </div>
              <div class="mt-3 flex items-center justify-between border-t border-border-subtle pt-3 text-xs">
                <span class="text-muted">MCP Connection</span>
                <div class="flex items-center gap-1.5">
                  <Show
                    when={props.diagnosticsData?.aiChat?.mcpConnected}
                    fallback={<XCircle class="h-3.5 w-3.5 text-rose-400" />}
                  >
                    <CheckCircle class="h-3.5 w-3.5 text-emerald-400" />
                  </Show>
                  <span
                    class={
                      props.diagnosticsData?.aiChat?.mcpConnected
                        ? 'text-green-700 dark:text-green-300'
                        : 'text-slate-500'
                    }
                  >
                    {props.diagnosticsData?.aiChat?.mcpConnected ? 'Connected' : 'Disconnected'}
                  </span>
                </div>
              </div>
              <Show when={(props.diagnosticsData?.aiChat?.notes?.length || 0) > 0}>
                <ul class="mt-3 list-disc rounded bg-amber-50 p-2 pl-4 text-xs text-amber-700 dark:bg-amber-900 dark:text-amber-400">
                  <For each={props.diagnosticsData?.aiChat?.notes || []}>
                    {(note) => <li>{note}</li>}
                  </For>
                </ul>
              </Show>
            </Card>
          </Show>
        </div>

        <Show when={(props.diagnosticsData?.errors?.length || 0) > 0}>
          <Card padding="md" class="border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-900">
            <div class="mb-3 flex items-center gap-3">
              <XCircle class="h-5 w-5 text-red-600 dark:text-red-400" />
              <h4 class="text-sm font-semibold text-red-900 dark:text-red-100">Errors Detected</h4>
            </div>
            <ul class="space-y-2 text-xs text-red-700 dark:text-red-300">
              <For each={props.diagnosticsData?.errors || []}>
                {(error) => (
                  <li class="flex items-start gap-2 rounded bg-red-100 p-2 dark:bg-red-900">
                    <span class="text-rose-400">•</span>
                    <span>{error}</span>
                  </li>
                )}
              </For>
            </ul>
          </Card>
        </Show>
      </div>
    </Show>
  );
};
