import type { Resource } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

export type DockerImageUpdateTone = 'danger' | 'warning' | 'success' | 'muted';

const trimmed = (value: unknown): string => asTrimmedString(value) ?? '';

export type DockerImageOperationalPresentation = {
  consumerCount: number;
  consumerSummary: string;
  updateLabel: string;
  updateDetail: string;
  updateTone: DockerImageUpdateTone;
};

const imageIdentityTokens = (image: Resource): Set<string> =>
  new Set(
    [
      image.id,
      image.name,
      image.displayName,
      image.docker?.image,
      image.docker?.imageId,
      ...(image.docker?.repoTags ?? []),
    ]
      .map(trimmed)
      .filter((value): value is string => value.length > 0),
  );

const containerUsesImage = (container: Resource, tokens: ReadonlySet<string>): boolean =>
  [container.docker?.image, container.docker?.imageId]
    .map(trimmed)
    .some((value) => value.length > 0 && tokens.has(value));

const resourceLabel = (resource: Resource): string =>
  trimmed(resource.name) || trimmed(resource.displayName) || resource.id;

const summarizeConsumers = (consumers: readonly Resource[], reportedCount: number): string => {
  if (consumers.length === 0) {
    if (reportedCount <= 0) return 'Unused';
    return `${reportedCount} container${reportedCount === 1 ? '' : 's'}`;
  }
  const labels = consumers.map(resourceLabel);
  const visible = labels.slice(0, 2);
  const extra = labels.length - visible.length;
  return `${visible.join(', ')}${extra > 0 ? ` +${extra}` : ''}`;
};

export function getDockerImageOperationalPresentation(
  image: Resource,
  containers: readonly Resource[] = [],
): DockerImageOperationalPresentation {
  const tokens = imageIdentityTokens(image);
  const consumers = containers.filter((container) => containerUsesImage(container, tokens));
  const reportedCount = Math.max(0, image.docker?.imageContainers ?? 0);
  const consumerCount = Math.max(reportedCount, consumers.length);
  const updateStates = [image, ...consumers]
    .map((resource) => resource.docker?.updateStatus)
    .filter((state): state is NonNullable<typeof state> => state !== undefined);
  const failed = updateStates.find((state) => trimmed(state.error).length > 0);

  if (failed) {
    return {
      consumerCount,
      consumerSummary: summarizeConsumers(consumers, reportedCount),
      updateLabel: 'Check failed',
      updateDetail: trimmed(failed.error),
      updateTone: 'danger',
    };
  }
  if (updateStates.some((state) => state.updateAvailable === true)) {
    return {
      consumerCount,
      consumerSummary: summarizeConsumers(consumers, reportedCount),
      updateLabel: 'Update available',
      updateDetail: 'At least one running container is behind the latest reported digest.',
      updateTone: 'warning',
    };
  }
  if (updateStates.some((state) => state.updateAvailable === false)) {
    return {
      consumerCount,
      consumerSummary: summarizeConsumers(consumers, reportedCount),
      updateLabel: 'Current',
      updateDetail: 'No newer digest was reported by the last image check.',
      updateTone: 'success',
    };
  }
  return {
    consumerCount,
    consumerSummary: summarizeConsumers(consumers, reportedCount),
    updateLabel: 'Not checked',
    updateDetail: 'No update comparison has been reported for this image.',
    updateTone: 'muted',
  };
}
