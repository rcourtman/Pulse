import { describe, expect, it } from 'vitest';
import { getDockerImageRegistryLink } from '../dockerImageReference';

describe('getDockerImageRegistryLink', () => {
  it.each([
    ['nginx:latest', 'https://hub.docker.com/_/nginx/tags'],
    ['library/redis:7.4', 'https://hub.docker.com/_/redis/tags'],
    ['docker.io/library/postgres@sha256:abc', 'https://hub.docker.com/_/postgres/tags'],
    ['index.docker.io/example/widget:1.2', 'https://hub.docker.com/r/example/widget/tags'],
    [
      'registry-1.docker.io/example/nested/widget:stable',
      'https://hub.docker.com/r/example/nested/widget/tags',
    ],
    [
      'quay.io/prometheus/node-exporter:v1.9.1',
      'https://quay.io/repository/prometheus/node-exporter?tab=tags',
    ],
  ])('maps %s to its public tags page', (image, href) => {
    expect(getDockerImageRegistryLink(image)).toEqual({
      href,
      label: 'View image tags',
    });
  });

  it.each([
    undefined,
    '',
    '   ',
    'ghcr.io/example/widget:latest',
    'registry.example.com/example/widget:latest',
    'localhost:5000/example/widget:latest',
    'invalid image:latest',
    String.raw`docker.io\example\widget:latest`,
  ])('does not invent a public link for %s', (image) => {
    expect(getDockerImageRegistryLink(image)).toBeNull();
  });
});
