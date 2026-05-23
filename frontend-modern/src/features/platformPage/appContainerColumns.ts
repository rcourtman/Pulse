import type { Resource } from '@/types/resource';

// Default-hidden columns for app-container workloads. `disk` and `tags` are
// existing hides; `backup` is added because the workload-table Backup column
// is driven exclusively by `resource.proxmox.lastBackup`, which Proxmox VE
// populates from its vzdump scheduler / PBS integration. App containers
// (Docker, Podman, TrueNAS apps) have no equivalent source — the cell is
// not just empty, the concept doesn't exist at this integration layer.
export const APP_CONTAINER_BASE_DEFAULT_HIDDEN_COLUMNS = ['disk', 'tags', 'backup'] as const;

export const APP_CONTAINER_COLUMN_LABEL_OVERRIDES = {
  disk: 'Writable layer',
} as const;

const hasFiniteMetric = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

export function buildAppContainerDefaultHiddenColumnIds(
  containers: readonly Resource[],
): string[] {
  const hasNetworkIOTelemetry = containers.some(
    (container) =>
      hasFiniteMetric(container.network?.rxBytes) || hasFiniteMetric(container.network?.txBytes),
  );
  const hasDiskIOTelemetry = containers.some(
    (container) =>
      hasFiniteMetric(container.diskIO?.readRate) || hasFiniteMetric(container.diskIO?.writeRate),
  );

  return [
    ...APP_CONTAINER_BASE_DEFAULT_HIDDEN_COLUMNS,
    ...(hasNetworkIOTelemetry ? [] : ['netIo']),
    ...(hasDiskIOTelemetry ? [] : ['diskIo']),
  ];
}
