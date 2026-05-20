import type { Resource } from '@/types/resource';

export const APP_CONTAINER_BASE_DEFAULT_HIDDEN_COLUMNS = ['disk', 'tags'] as const;

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
