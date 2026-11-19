import { createMemo, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// A feature in preview, read from the build the server reports. The list has
// no feature at this stage yet, so the stage is set here and nowhere else.
const [version, setVersion] = createSignal<Record<string, unknown> | null>(null);
const [lastError, setLastError] = createSignal<string | null>(null);

vi.mock('../featureReleases.json', () => ({
  default: { infrastructureMap: { name: 'Map', stage: 'preview', guides: ['MAP.md'] } },
}));

vi.mock('@/stores/updates', () => ({
  updateStore: { versionInfo: () => version(), lastError: () => lastError() },
}));

import { getFeatureAvailability, isFeatureAvailable, isFeatureInPreview } from '../featureReleases';

describe('a feature in preview', () => {
  beforeEach(() => {
    setVersion(null);
    setLastError(null);
    vi.stubEnv('DEV', false);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('carries the Preview tag', () => {
    expect(isFeatureInPreview('infrastructureMap')).toBe(true);
  });

  it('is on in a beta or release candidate', () => {
    setVersion({ channel: 'rc' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
  });

  it('is off in a stable build and in a source build', () => {
    setVersion({ channel: 'stable' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ channel: '' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({});
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
  });

  it('follows the channel alone on a server that says it is a development build', () => {
    setVersion({ channel: 'stable', isDevelopment: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ isDevelopment: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    setVersion({ channel: 'rc', isDevelopment: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
  });

  it('is on in a page the end-to-end suite has marked, whatever the channel', () => {
    setVersion({ channel: 'stable', featureReleaseTestBuild: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
  });

  it('waits while the version is unknown and stays off if it cannot be read', () => {
    expect(getFeatureAvailability('infrastructureMap')).toBe('pending');
    expect(isFeatureAvailable('infrastructureMap')).toBe(false);
    setLastError('Failed to check for updates');
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
  });

  it('turns on for anything already on the page when the version arrives', () => {
    createRoot((dispose) => {
      // A memo is recomputed only if it tracked what changed.
      const available = createMemo(() => isFeatureAvailable('infrastructureMap'));
      expect(available()).toBe(false);
      setVersion({ channel: 'rc' });
      expect(available()).toBe(true);
      dispose();
    });
  });

  it('is on in a development build whatever the server says', () => {
    vi.stubEnv('DEV', true);
    setVersion({ channel: 'stable' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
  });
});
