import { createMemo, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// A feature at build stage in a production bundle, read from the version reply.
// Mocked here so build-stage gating stays covered after features graduate to preview.
vi.mock('../featureReleases.json', () => ({
  default: {
    infrastructureMap: { name: 'Map', stage: 'build', guides: ['MAP.md'] },
    standaloneVms: { name: 'Machines VMs', stage: 'build', guides: [] },
  },
}));
const [version, setVersion] = createSignal<Record<string, unknown> | null>(null);
const [lastError, setLastError] = createSignal<string | null>(null);

vi.mock('@/stores/updates', () => ({
  updateStore: { versionInfo: () => version(), lastError: () => lastError() },
}));

import {
  FEATURE_RELEASE_TEST_BUILD_FIELD,
  getFeatureAvailability,
  getFeatureGuideAvailability,
  getFeatureStage,
  isFeatureAvailable,
  isFeatureGuideAvailable,
} from '../featureReleases';

describe('a feature at build, in a production bundle', () => {
  beforeEach(() => {
    setVersion(null);
    setLastError(null);
    vi.stubEnv('DEV', false);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('is the stage the map is held at', () => {
    expect(getFeatureStage('infrastructureMap')).toBe('build');
  });

  it('is off in every released build: stable, beta and release candidate', () => {
    setVersion({ channel: 'stable', isDevelopment: false });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ channel: 'rc', isDevelopment: false });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    expect(isFeatureGuideAvailable('MAP')).toBe(false);
  });

  it('is off on every server that says it is a development or source build', () => {
    // The installer's source option and a server run from a checkout both
    // report this, and both are people outside the project.
    setVersion({ channel: '', isDevelopment: true, isSourceBuild: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ channel: '', isDevelopment: false, isSourceBuild: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ channel: 'rc', isDevelopment: true, build: 'development' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
  });

  it('is on only in a page the end-to-end suite has marked as a development build', () => {
    expect(FEATURE_RELEASE_TEST_BUILD_FIELD).toBe('featureReleaseTestBuild');

    setVersion({ channel: 'stable', isDevelopment: false, featureReleaseTestBuild: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
    expect(isFeatureGuideAvailable('MAP')).toBe(true);

    // Only the exact mark counts.
    setVersion({ channel: 'stable', featureReleaseTestBuild: 'true' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ channel: 'stable', featureReleaseTestBuild: 1 });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
  });

  it('waits while the version is unknown and stays off if it cannot be read', () => {
    expect(getFeatureAvailability('infrastructureMap')).toBe('pending');
    expect(getFeatureGuideAvailability('MAP')).toBe('pending');
    expect(getFeatureGuideAvailability('README')).toBe('on');
    expect(isFeatureAvailable('infrastructureMap')).toBe(false);

    setLastError('Failed to check for updates');
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    expect(getFeatureGuideAvailability('MAP')).toBe('off');
  });

  it('settles for anything already on the page when the version arrives', () => {
    createRoot((dispose) => {
      // A memo is recomputed only if it tracked what changed, as a mounted
      // header button or route is.
      const availability = createMemo(() => getFeatureAvailability('infrastructureMap'));
      expect(availability()).toBe('pending');

      setVersion({ channel: 'stable' });
      expect(availability()).toBe('off');

      setVersion({ channel: 'stable', featureReleaseTestBuild: true });
      expect(availability()).toBe('on');
      dispose();
    });
  });

  it('keeps the standalone VM view off in stable, beta, RC and source installs', () => {
    expect(getFeatureStage('standaloneVms')).toBe('build');
    expect(isFeatureAvailable('standaloneVms')).toBe(false);
    for (const reply of [
      { version: '6.9.0', channel: 'stable' },
      { version: '6.9.0-beta.4', channel: 'rc' },
      { version: '6.9.0-rc.4', channel: 'rc' },
      { version: '6.9.0-dev', isDevelopment: true, isSourceBuild: true },
    ]) {
      setVersion(reply);
      expect(getFeatureAvailability('standaloneVms')).toBe('off');
    }

    setVersion({ featureReleaseTestBuild: true });
    expect(getFeatureAvailability('standaloneVms')).toBe('on');
  });
});
