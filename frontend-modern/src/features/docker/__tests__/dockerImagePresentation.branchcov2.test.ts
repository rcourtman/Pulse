import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import { getDockerImageOperationalPresentation } from '../dockerImagePresentation';

const image = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'docker-image:host:nginx',
  name: 'nginx',
  displayName: 'nginx:latest',
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'running',
  type: 'docker-image',
  lastSeen: 1_700_000_000_000,
  docker: { runtime: 'docker', image: 'nginx:latest', imageId: 'sha256:abc' },
  ...overrides,
});

const container = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'app-container:docker-host:web',
  name: 'web',
  displayName: 'web',
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'running',
  type: 'app-container',
  lastSeen: 1_700_000_000_000,
  docker: { runtime: 'docker', agentId: 'agent-1', containerId: 'cid-web', image: 'nginx:latest' },
  ...overrides,
});

describe('dockerImagePresentation.branchcov2', () => {
  describe('imageIdentityTokens (exercised via consumer matching)', () => {
    it('matches a container whose docker.image equals the image docker.image token', () => {
      const result = getDockerImageOperationalPresentation(image(), [
        container({ docker: { runtime: 'docker', image: 'nginx:latest' } }),
      ]);
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('web');
    });

    it('matches a container whose docker.imageId equals the image docker.imageId token', () => {
      const result = getDockerImageOperationalPresentation(image(), [
        container({
          id: 'app-container:docker-host:by-digest',
          name: 'by-digest',
          docker: {
            runtime: 'docker',
            imageId: 'sha256:abc',
          },
        }),
      ]);
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('by-digest');
    });

    it('matches a container whose docker.image equals the image resource id', () => {
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker' } }),
        [
          container({
            id: 'app-container:docker-host:by-rid',
            name: 'by-rid',
            docker: { runtime: 'docker', image: 'docker-image:host:nginx' },
          }),
        ],
      );
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('by-rid');
    });

    it('matches a container whose docker.image equals the image resource name', () => {
      const result = getDockerImageOperationalPresentation(
        image({ name: 'nginx', docker: { runtime: 'docker' } }),
        [
          container({
            id: 'app-container:docker-host:by-name',
            name: 'by-name',
            docker: { runtime: 'docker', image: 'nginx' },
          }),
        ],
      );
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('by-name');
    });

    it('matches a container whose docker.image equals the image resource displayName', () => {
      const result = getDockerImageOperationalPresentation(
        image({ name: 'other', docker: { runtime: 'docker' } }),
        [
          container({
            id: 'app-container:docker-host:by-display',
            name: 'by-display',
            docker: { runtime: 'docker', image: 'nginx:latest' },
          }),
        ],
      );
      // displayName default is 'nginx:latest'
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('by-display');
    });

    it('matches a container whose docker.image equals one of docker.repoTags (spread arm)', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            repoTags: ['nginx:1.25', 'nginx:stable'],
          },
        }),
        [
          container({
            id: 'app-container:docker-host:by-tag',
            name: 'by-tag',
            docker: { runtime: 'docker', image: 'nginx:stable' },
          }),
        ],
      );
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('by-tag');
    });

    it('uses the ?? [] fallback when docker.repoTags is undefined', () => {
      // docker present but repoTags absent; a bogus tag must not match, real token must.
      const result = getDockerImageOperationalPresentation(image(), [
        container({
          id: 'app-container:docker-host:nope',
          name: 'nope',
          docker: { runtime: 'docker', image: 'never-a-token' },
        }),
        container({
          id: 'app-container:docker-host:real',
          name: 'real',
          docker: { runtime: 'docker', image: 'nginx:latest' },
        }),
      ]);
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('real');
    });

    it('still builds tokens from id/name/displayName when docker is absent entirely', () => {
      const result = getDockerImageOperationalPresentation(
        image({ name: 'standalone', displayName: 'standalone', docker: undefined }),
        [
          container({
            id: 'app-container:docker-host:by-name',
            name: 'by-name',
            docker: { runtime: 'docker', image: 'standalone' },
          }),
        ],
      );
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('by-name');
    });

    it('filters blank/whitespace identity values out of the token set', () => {
      // Every identity field blank/whitespace -> only docker.image provides a usable token.
      const result = getDockerImageOperationalPresentation(
        image({
          id: '   ',
          name: '',
          displayName: '  ',
          docker: { runtime: 'docker', image: 'only-token' },
        }),
        [
          // A container referencing the (blank) id/name must NOT match.
          container({
            id: 'app-container:docker-host:blank',
            name: 'blank',
            docker: { runtime: 'docker', image: '   ' },
          }),
          container({
            id: 'app-container:docker-host:hit',
            name: 'hit',
            docker: { runtime: 'docker', image: 'only-token' },
          }),
        ],
      );
      expect(result.consumerCount).toBe(1);
      expect(result.consumerSummary).toBe('hit');
    });

    it('ignores a container whose docker.image and docker.imageId are both blank', () => {
      const result = getDockerImageOperationalPresentation(image(), [
        container({
          id: 'app-container:docker-host:blank',
          name: 'blank',
          docker: { runtime: 'docker', image: '   ', imageId: '' },
        }),
      ]);
      expect(result.consumerCount).toBe(0);
      expect(result.consumerSummary).toBe('Unused');
    });
  });

  describe('resourceLabel (exercised via consumer labels)', () => {
    const imageWithToken = image({ docker: { runtime: 'docker', image: 'nginx:latest' } });

    it('prefers the trimmed resource name when present', () => {
      const result = getDockerImageOperationalPresentation(imageWithToken, [
        container({ name: '  web-app  ', displayName: 'ignored', id: 'should-not-show' }),
      ]);
      expect(result.consumerSummary).toBe('web-app');
    });

    it('falls back to trimmed displayName when name is blank', () => {
      const result = getDockerImageOperationalPresentation(imageWithToken, [
        container({ name: '   ', displayName: '  Web App  ', id: 'should-not-show' }),
      ]);
      expect(result.consumerSummary).toBe('Web App');
    });

    it('falls back to the raw resource id when name and displayName are both blank', () => {
      const result = getDockerImageOperationalPresentation(imageWithToken, [
        container({ name: '', displayName: '  ', id: 'app-container:docker-host:fallback' }),
      ]);
      // NOTE: resourceLabel returns resource.id untrimmed (asymmetric vs name/displayName).
      expect(result.consumerSummary).toBe('app-container:docker-host:fallback');
    });
  });

  describe('summarizeConsumers (exercised via consumerSummary)', () => {
    const tokenImage = image({ docker: { runtime: 'docker', image: 'nginx:latest' } });
    const matching = (name: string, id: string): Resource =>
      container({
        id,
        name,
        displayName: name,
        docker: { runtime: 'docker', image: 'nginx:latest' },
      });

    it('returns "Unused" when there are no consumers and reportedCount <= 0', () => {
      const result = getDockerImageOperationalPresentation(tokenImage, []);
      expect(result.consumerSummary).toBe('Unused');
      expect(result.consumerCount).toBe(0);
    });

    it('clamps a negative reportedCount to 0 and still returns "Unused"', () => {
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker', image: 'nginx:latest', imageContainers: -3 } }),
        [],
      );
      expect(result.consumerSummary).toBe('Unused');
      expect(result.consumerCount).toBe(0);
    });

    it('treats a missing imageContainers as 0 via ?? and returns "Unused"', () => {
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker', image: 'nginx:latest' } }),
        [],
      );
      expect(result.consumerSummary).toBe('Unused');
    });

    it('uses the singular form when reportedCount === 1 and no real consumers', () => {
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker', image: 'nginx:latest', imageContainers: 1 } }),
        [],
      );
      expect(result.consumerSummary).toBe('1 container');
      expect(result.consumerCount).toBe(1);
    });

    it('uses the plural form when reportedCount > 1 and no real consumers', () => {
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker', image: 'nginx:latest', imageContainers: 3 } }),
        [],
      );
      expect(result.consumerSummary).toBe('3 containers');
      expect(result.consumerCount).toBe(3);
    });

    it('renders a single consumer label verbatim', () => {
      const result = getDockerImageOperationalPresentation(tokenImage, [matching('web', 'c1')]);
      expect(result.consumerSummary).toBe('web');
      expect(result.consumerCount).toBe(1);
    });

    it('joins exactly two consumer labels with a comma', () => {
      const result = getDockerImageOperationalPresentation(tokenImage, [
        matching('web', 'c1'),
        matching('api', 'c2'),
      ]);
      expect(result.consumerSummary).toBe('web, api');
      expect(result.consumerCount).toBe(2);
    });

    it('collapses additional consumers beyond the first two into "+N" (extra > 0 arm)', () => {
      const result = getDockerImageOperationalPresentation(tokenImage, [
        matching('web', 'c1'),
        matching('api', 'c2'),
        matching('worker', 'c3'),
      ]);
      expect(result.consumerSummary).toBe('web, api +1');
      expect(result.consumerCount).toBe(3);
    });

    it('shows a growing +N count as extras increase (extra > 0 arm, N>1)', () => {
      const result = getDockerImageOperationalPresentation(tokenImage, [
        matching('web', 'c1'),
        matching('api', 'c2'),
        matching('worker', 'c3'),
        matching('scheduler', 'c4'),
      ]);
      expect(result.consumerSummary).toBe('web, api +2');
      expect(result.consumerCount).toBe(4);
    });

    it('reports a discrepancy when reportedCount exceeds the real consumer count', () => {
      // SUSPECTED SOURCE BUG: consumerCount (5) and the named summary ("web, api") disagree;
      // there is no "+3" accounting for the unrepresented containers.
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker', image: 'nginx:latest', imageContainers: 5 } }),
        [matching('web', 'c1'), matching('api', 'c2')],
      );
      expect(result.consumerCount).toBe(5);
      expect(result.consumerSummary).toBe('web, api');
    });
  });

  describe('getDockerImageOperationalPresentation (update-state branches)', () => {
    it('returns the danger branch when an updateStatus has a non-empty error', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { error: 'registry unreachable' },
          },
        }),
        [],
      );
      expect(result).toStrictEqual({
        consumerCount: 0,
        consumerSummary: 'Unused',
        updateLabel: 'Check failed',
        updateDetail: 'registry unreachable',
        updateTone: 'danger',
      });
    });

    it('trims the error text for updateDetail', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { error: '  boom  ' },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Check failed');
      expect(result.updateDetail).toBe('boom');
      expect(result.updateTone).toBe('danger');
    });

    it('does NOT enter the danger branch when error is whitespace-only (falls through)', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { error: '   ', updateAvailable: true },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Update available');
      expect(result.updateTone).toBe('warning');
    });

    it('does NOT enter the danger branch when error is undefined (falls through)', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { updateAvailable: false },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Current');
      expect(result.updateTone).toBe('success');
    });

    it('returns the warning branch when some state has updateAvailable === true', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { updateAvailable: true },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Update available');
      expect(result.updateDetail).toBe(
        'At least one running container is behind the latest reported digest.',
      );
      expect(result.updateTone).toBe('warning');
    });

    it('returns the success branch when some state has updateAvailable === false (and none true)', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { updateAvailable: false },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Current');
      expect(result.updateDetail).toBe('No newer digest was reported by the last image check.');
      expect(result.updateTone).toBe('success');
    });

    it('returns the muted branch when updateStatus is present but updateAvailable is undefined', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { currentDigest: 'sha256:1' },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Not checked');
      expect(result.updateDetail).toBe('No update comparison has been reported for this image.');
      expect(result.updateTone).toBe('muted');
    });

    it('returns the muted branch when there is no updateStatus at all', () => {
      const result = getDockerImageOperationalPresentation(image(), []);
      expect(result.updateLabel).toBe('Not checked');
      expect(result.updateTone).toBe('muted');
    });

    it('prefers the danger branch over updateAvailable === true (precedence)', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { updateAvailable: true, error: 'check blew up' },
          },
        }),
        [],
      );
      expect(result.updateLabel).toBe('Check failed');
      expect(result.updateTone).toBe('danger');
    });

    it('prefers updateAvailable === true over false across mixed states (some() short-circuit)', () => {
      const result = getDockerImageOperationalPresentation(
        image({
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { updateAvailable: false },
          },
        }),
        [
          container({
            name: 'stale',
            id: 'c1',
            docker: {
              runtime: 'docker',
              image: 'nginx:latest',
              updateStatus: { updateAvailable: true },
            },
          }),
        ],
      );
      expect(result.updateLabel).toBe('Update available');
      expect(result.updateTone).toBe('warning');
    });

    it('lets a matching container drive the danger branch even when the image has no error', () => {
      const result = getDockerImageOperationalPresentation(image(), [
        container({
          name: 'broken',
          id: 'c1',
          docker: {
            runtime: 'docker',
            image: 'nginx:latest',
            updateStatus: { error: 'container check failed' },
          },
        }),
      ]);
      expect(result.updateLabel).toBe('Check failed');
      expect(result.updateDetail).toBe('container check failed');
      expect(result.updateTone).toBe('danger');
      expect(result.consumerSummary).toBe('broken');
    });

    it('ignores a non-matching container updateStatus (not in consumers spread)', () => {
      const result = getDockerImageOperationalPresentation(image(), [
        container({
          name: 'unrelated',
          id: 'c1',
          docker: {
            runtime: 'docker',
            image: 'totally-different:image',
            updateStatus: { error: 'should-be-ignored' },
          },
        }),
      ]);
      // Container did not match image tokens -> its updateStatus is excluded.
      expect(result.updateLabel).toBe('Not checked');
      expect(result.updateTone).toBe('muted');
      expect(result.consumerCount).toBe(0);
    });

    it('uses the default empty containers array when the second argument is omitted', () => {
      const result = getDockerImageOperationalPresentation(image());
      expect(result).toStrictEqual({
        consumerCount: 0,
        consumerSummary: 'Unused',
        updateLabel: 'Not checked',
        updateDetail: 'No update comparison has been reported for this image.',
        updateTone: 'muted',
      });
    });

    it('computes consumerCount as max(reportedCount, consumers.length)', () => {
      const result = getDockerImageOperationalPresentation(
        image({ docker: { runtime: 'docker', image: 'nginx:latest', imageContainers: 2 } }),
        [
          container({
            name: 'a',
            id: 'c1',
            docker: { runtime: 'docker', image: 'nginx:latest' },
          }),
          container({
            name: 'b',
            id: 'c2',
            docker: { runtime: 'docker', image: 'nginx:latest' },
          }),
          container({
            name: 'c',
            id: 'c3',
            docker: { runtime: 'docker', image: 'nginx:latest' },
          }),
        ],
      );
      // reportedCount=2, real consumers=3 -> max wins on the count, summary shows all three.
      expect(result.consumerCount).toBe(3);
      expect(result.consumerSummary).toBe('a, b +1');
    });
  });
});
