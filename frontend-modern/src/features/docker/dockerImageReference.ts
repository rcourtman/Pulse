import { asTrimmedString } from '@/utils/stringUtils';

export type DockerImageRegistryLink = {
  href: string;
  label: string;
};

const DOCKER_HUB_REGISTRIES = new Set(['docker.io', 'index.docker.io', 'registry-1.docker.io']);

const encodeRepositoryPath = (repository: string): string =>
  repository
    .split('/')
    .filter(Boolean)
    .map((segment) => encodeURIComponent(segment))
    .join('/');

const stripImageTagAndDigest = (image: string): string => {
  const withoutDigest = image.split('@', 1)[0] ?? '';
  const lastSlash = withoutDigest.lastIndexOf('/');
  const lastColon = withoutDigest.lastIndexOf(':');
  return lastColon > lastSlash ? withoutDigest.slice(0, lastColon) : withoutDigest;
};

const hasExplicitRegistry = (firstSegment: string): boolean =>
  firstSegment === 'localhost' || firstSegment.includes('.') || firstSegment.includes(':');

/**
 * Returns a public tags page only when the registry has a stable, predictable
 * browser URL. Private and unknown registries deliberately return null rather
 * than producing a misleading or credential-leaking link.
 */
export function getDockerImageRegistryLink(
  rawImage: string | null | undefined,
): DockerImageRegistryLink | null {
  const image = asTrimmedString(rawImage);
  if (!image || /[\s\\]/.test(image)) return null;

  const reference = stripImageTagAndDigest(image);
  const parts = reference.split('/').filter(Boolean);
  if (parts.length === 0) return null;

  const firstSegment = (parts[0] ?? '').toLowerCase();
  const explicitRegistry = hasExplicitRegistry(firstSegment);
  const registry = explicitRegistry ? firstSegment : 'docker.io';
  const repositoryParts = explicitRegistry ? parts.slice(1) : parts;
  if (repositoryParts.length === 0) return null;

  if (DOCKER_HUB_REGISTRIES.has(registry)) {
    const normalizedRepository =
      repositoryParts.length === 1
        ? ['library', repositoryParts[0] ?? '']
        : repositoryParts[0]?.toLowerCase() === 'library'
          ? ['library', ...repositoryParts.slice(1)]
          : repositoryParts;
    const repository = encodeRepositoryPath(normalizedRepository.join('/'));
    if (!repository) return null;

    if (normalizedRepository[0] === 'library' && normalizedRepository.length === 2) {
      return {
        href: `https://hub.docker.com/_/${encodeURIComponent(normalizedRepository[1] ?? '')}/tags`,
        label: 'View image tags',
      };
    }
    return {
      href: `https://hub.docker.com/r/${repository}/tags`,
      label: 'View image tags',
    };
  }

  if (registry === 'quay.io') {
    const repository = encodeRepositoryPath(repositoryParts.join('/'));
    if (!repository) return null;
    return {
      href: `https://quay.io/repository/${repository}?tab=tags`,
      label: 'View image tags',
    };
  }

  return null;
}

/**
 * Splits an image reference before its repository name, so a narrow cell can
 * truncate the shared registry and namespace and keep the part that tells
 * images apart: `ghcr.io/pulse-demo/` + `backup-coordinator:2026.04`. A digest
 * never moves the split. A reference without a path keeps everything in the
 * head, like a Kubernetes name without a tail.
 */
export function splitDockerImageReference(reference: string): { head: string; tail: string } {
  const digestStart = reference.indexOf('@');
  const path = digestStart >= 0 ? reference.slice(0, digestStart) : reference;
  const cut = path.lastIndexOf('/');
  if (cut <= 0 || cut === path.length - 1) return { head: reference, tail: '' };
  return { head: reference.slice(0, cut + 1), tail: reference.slice(cut + 1) };
}
