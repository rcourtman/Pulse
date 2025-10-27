import type { Component } from 'solid-js';
import { Show, createMemo } from 'solid-js';
import type { DockerHost } from '@/types/api';

interface StatCardProps {
  label: string;
  value: number | string;
  sublabel?: string;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info';
  onClick?: () => void;
  isActive?: boolean;
}

const StatCard: Component<StatCardProps> = (props) => {
  const baseClass = 'flex flex-col gap-1 px-4 py-3 rounded-lg border transition-all duration-200';

  const variantClass = () => {
    if (props.isActive) {
      return 'bg-blue-100 dark:bg-blue-900/40 border-blue-300 dark:border-blue-700 ring-2 ring-blue-500/20';
    }

    switch (props.variant) {
      case 'success':
        return 'bg-green-50 dark:bg-green-950/20 border-green-200 dark:border-green-800';
      case 'warning':
        return 'bg-yellow-50 dark:bg-yellow-950/20 border-yellow-200 dark:border-yellow-800';
      case 'error':
        return 'bg-red-50 dark:bg-red-950/20 border-red-200 dark:border-red-800';
      case 'info':
        return 'bg-blue-50 dark:bg-blue-950/20 border-blue-200 dark:border-blue-800';
      default:
        return 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700';
    }
  };

  const hoverClass = () => props.onClick ? 'cursor-pointer hover:shadow-md hover:scale-[1.02]' : '';

  return (
    <button
      type="button"
      class={`${baseClass} ${variantClass()} ${hoverClass()}`}
      onClick={props.onClick}
      disabled={!props.onClick}
    >
      <div class="text-2xl font-bold text-gray-900 dark:text-gray-100">
        {props.value}
      </div>
      <div class="text-xs font-medium text-gray-600 dark:text-gray-400 uppercase tracking-wide">
        {props.label}
      </div>
      <Show when={props.sublabel}>
        <div class="text-xs text-gray-500 dark:text-gray-500">
          {props.sublabel}
        </div>
      </Show>
    </button>
  );
};

export interface DockerSummaryStats {
  hosts: {
    total: number;
    online: number;
    degraded: number;
    offline: number;
  };
  containers: {
    total: number;
    running: number;
    stopped: number;
    error: number;
  };
  services?: {
    total: number;
    healthy: number;
    degraded: number;
  };
  resources?: {
    avgCpu: number | null;
    avgMemory: number | null;
  };
}

type SummaryFilter = { type: 'host-status' | 'container-state' | 'service-health'; value: string } | null;

interface DockerSummaryStatsProps {
  hosts: DockerHost[];
  onFilterChange?: (filter: SummaryFilter) => void;
  activeFilter?: { type: string; value: string } | null;
}

export const DockerSummaryStatsBar: Component<DockerSummaryStatsProps> = (props) => {
  const stats = (): DockerSummaryStats => {
    const hosts = props.hosts || [];

    // Host stats
    let onlineHosts = 0;
    let degradedHosts = 0;
    let offlineHosts = 0;

    hosts.forEach((host) => {
      const status = host.status?.toLowerCase() ?? 'unknown';
      switch (status) {
        case 'online':
          onlineHosts++;
          break;
        case 'degraded':
        case 'warning':
        case 'maintenance':
          degradedHosts++;
          break;
        case 'offline':
        case 'error':
        case 'unreachable':
          offlineHosts++;
          break;
        default:
          degradedHosts++;
          break;
      }
    });

    // Container stats
    let totalContainers = 0;
    let runningContainers = 0;
    let stoppedContainers = 0;
    let errorContainers = 0;

    // Service stats
    let totalServices = 0;
    let healthyServices = 0;
    let degradedServices = 0;

    // Resource stats
    let totalCpu = 0;
    let totalMemory = 0;
    let cpuSamples = 0;
    let memorySamples = 0;

    hosts.forEach(host => {
      // Count containers
      const containers = host.containers || [];
      totalContainers += containers.length;

      containers.forEach(container => {
        const state = container.state?.toLowerCase();
        if (state === 'running') {
          runningContainers++;
          // Sum CPU/Memory for running containers
          if (typeof container.cpuPercent === 'number' && !Number.isNaN(container.cpuPercent)) {
            totalCpu += container.cpuPercent;
            cpuSamples++;
          }
          if (typeof container.memoryPercent === 'number' && !Number.isNaN(container.memoryPercent)) {
            totalMemory += container.memoryPercent;
            memorySamples++;
          }
        } else if (state === 'exited' || state === 'stopped' || state === 'created' || state === 'paused') {
          stoppedContainers++;
        } else if (state === 'restarting' || state === 'dead' || state === 'removing' || state === 'error' || state === 'failed') {
          errorContainers++;
        }
      });

      // Count services
      const services = host.services || [];
      totalServices += services.length;

      services.forEach(service => {
        const desired = service.desiredTasks ?? 0;
        const running = service.runningTasks ?? 0;
        if (desired > 0 && running >= desired) {
          healthyServices++;
        } else if (desired > 0) {
          degradedServices++;
        }
      });
    });

    return {
      hosts: {
        total: hosts.length,
        online: onlineHosts,
        degraded: degradedHosts,
        offline: offlineHosts,
      },
      containers: {
        total: totalContainers,
        running: runningContainers,
        stopped: stoppedContainers,
        error: errorContainers,
      },
      services: totalServices > 0 ? {
        total: totalServices,
        healthy: healthyServices,
        degraded: degradedServices,
      } : undefined,
      resources: (cpuSamples > 0 || memorySamples > 0) ? {
        avgCpu: cpuSamples > 0 ? totalCpu / cpuSamples : null,
        avgMemory: memorySamples > 0 ? totalMemory / memorySamples : null,
      } : undefined,
    };
  };

  const summary = createMemo(stats);

  const hostSublabel = () => {
    const hosts = summary().hosts;
    const parts = [`${hosts.online} online`];
    if (hosts.degraded > 0) {
      parts.push(`${hosts.degraded} degraded`);
    }
    if (hosts.offline > 0) {
      parts.push(`${hosts.offline} offline`);
    }
    return parts.join(', ');
  };

  const servicesSublabel = () => {
    const services = summary().services;
    if (!services) return '';
    const parts = [`${services.healthy} healthy`];
    if (services.degraded > 0) {
      parts.push(`${services.degraded} degraded`);
    }
    return parts.join(', ');
  };

  const resourceValue = () => {
    const avgCpu = summary().resources?.avgCpu ?? null;
    return avgCpu !== null ? Math.round(avgCpu) : '—';
  };

  const resourceSublabel = () => {
    const avgMemory = summary().resources?.avgMemory ?? null;
    if (avgMemory === null) return undefined;
    return `${Math.round(avgMemory)}% mem`;
  };

  const resourceVariant = (): StatCardProps['variant'] => {
    const avgCpu = summary().resources?.avgCpu ?? null;
    if (avgCpu === null) return 'info';
    if (avgCpu > 80) return 'error';
    if (avgCpu > 60) return 'warning';
    return 'info';
  };

  const isActive = (type: string, value: string) => {
    return props.activeFilter?.type === type && props.activeFilter?.value === value;
  };

  return (
    <div class="space-y-3">
      <div class="flex items-center justify-between">
        <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wide">
          Overview
        </h2>
        <Show when={props.activeFilter}>
          <button
            type="button"
            onClick={() => props.onFilterChange?.(null)}
            class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
          >
            Clear filter
          </button>
        </Show>
      </div>

      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        {/* Hosts */}
        <StatCard
          label="Total Hosts"
          value={summary().hosts.total}
          sublabel={hostSublabel()}
          variant="default"
        />

        <Show when={summary().hosts.degraded > 0}>
          <StatCard
            label="Degraded Hosts"
            value={summary().hosts.degraded}
            variant="warning"
            onClick={() => props.onFilterChange?.({ type: 'host-status', value: 'degraded' })}
            isActive={isActive('host-status', 'degraded')}
          />
        </Show>

        <Show when={summary().hosts.offline > 0}>
          <StatCard
            label="Offline Hosts"
            value={summary().hosts.offline}
            variant="error"
            onClick={() => props.onFilterChange?.({ type: 'host-status', value: 'offline' })}
            isActive={isActive('host-status', 'offline')}
          />
        </Show>

        {/* Containers */}
        <StatCard
          label="Running"
          value={summary().containers.running}
          sublabel={`of ${summary().containers.total} containers`}
          variant="success"
          onClick={() => props.onFilterChange?.({ type: 'container-state', value: 'running' })}
          isActive={isActive('container-state', 'running')}
        />

        <Show when={summary().containers.stopped > 0}>
          <StatCard
            label="Stopped"
            value={summary().containers.stopped}
            variant="warning"
            onClick={() => props.onFilterChange?.({ type: 'container-state', value: 'stopped' })}
            isActive={isActive('container-state', 'stopped')}
          />
        </Show>

        <Show when={summary().containers.error > 0}>
          <StatCard
            label="Error"
            value={summary().containers.error}
            variant="error"
            onClick={() => props.onFilterChange?.({ type: 'container-state', value: 'error' })}
            isActive={isActive('container-state', 'error')}
          />
        </Show>

        {/* Services */}
        <Show when={summary().services}>
          <StatCard
            label="Services"
            value={summary().services!.total}
            sublabel={servicesSublabel()}
            variant={summary().services!.degraded > 0 ? 'warning' : 'success'}
          />
        </Show>

        {/* Resources */}
        <Show when={summary().resources}>
          <StatCard
            label="Avg CPU"
            value={resourceValue()}
            sublabel={resourceSublabel()}
            variant={resourceVariant()}
          />
        </Show>
      </div>
    </div>
  );
};
