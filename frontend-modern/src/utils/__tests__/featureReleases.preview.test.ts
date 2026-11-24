import { createMemo, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The real Preview-stage registry in a production bundle, read from the build the server reports.
const [version, setVersion] = createSignal<Record<string, unknown> | null>(null);
const [lastError, setLastError] = createSignal<string | null>(null);

vi.mock('@/stores/updates', () => ({
  updateStore: { versionInfo: () => version(), lastError: () => lastError() },
}));

import {
  getFeatureAvailability,
  getFeatureGuideAvailability,
  getFeatureStage,
  isFeatureAvailable,
  isFeatureGuideAvailable,
  isFeatureInPreview,
} from '../featureReleases';

describe('a feature in preview', () => {
  beforeEach(() => {
    setVersion(null);
    setLastError(null);
    vi.stubEnv('DEV', false);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('carries the Preview tag for promoted features', () => {
    expect(getFeatureStage('infrastructureMap')).toBe('preview');
    expect(isFeatureInPreview('infrastructureMap')).toBe(true);
    expect(getFeatureStage('standaloneVms')).toBe('preview');
    expect(isFeatureInPreview('standaloneVms')).toBe(true);
  });

  it('is on in a beta or release candidate', () => {
    setVersion({ channel: 'rc' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
    expect(isFeatureGuideAvailable('MAP')).toBe(true);
    expect(getFeatureAvailability('standaloneVms')).toBe('on');
  });

  it('is off in a stable build and in a source build', () => {
    for (const reply of [{ channel: 'stable' }, { channel: '' }, {}]) {
      setVersion(reply);
      expect(getFeatureAvailability('infrastructureMap')).toBe('off');
      expect(isFeatureGuideAvailable('MAP')).toBe(false);
      expect(getFeatureAvailability('standaloneVms')).toBe('off');
    }
  });

  it('follows the channel alone on a server that says it is a development build', () => {
    setVersion({ channel: 'stable', isDevelopment: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    expect(getFeatureAvailability('standaloneVms')).toBe('off');
    setVersion({ isDevelopment: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    expect(getFeatureAvailability('standaloneVms')).toBe('off');
    setVersion({ channel: 'rc', isDevelopment: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
    expect(getFeatureAvailability('standaloneVms')).toBe('on');
  });

  it('is on in a page the end-to-end suite has marked, whatever the channel', () => {
    setVersion({ channel: 'stable', featureReleaseTestBuild: true });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
    expect(getFeatureAvailability('standaloneVms')).toBe('on');
  });

  it('waits while the version is unknown and stays off if it cannot be read', () => {
    expect(getFeatureAvailability('infrastructureMap')).toBe('pending');
    expect(getFeatureGuideAvailability('MAP')).toBe('pending');
    expect(getFeatureAvailability('standaloneVms')).toBe('pending');
    expect(isFeatureAvailable('infrastructureMap')).toBe(false);
    expect(isFeatureAvailable('standaloneVms')).toBe(false);
    setLastError('Failed to check for updates');
    expect(getFeatureAvailability('infrastructureMap')).toBe('off');
    expect(getFeatureGuideAvailability('MAP')).toBe('off');
    expect(getFeatureAvailability('standaloneVms')).toBe('off');
  });

  it('turns on for anything already on the page when the version arrives', () => {
    createRoot((dispose) => {
      // A memo is recomputed only if it tracked what changed.
      const mapAvailable = createMemo(() => isFeatureAvailable('infrastructureMap'));
      const vmsAvailable = createMemo(() => isFeatureAvailable('standaloneVms'));
      expect(mapAvailable()).toBe(false);
      expect(vmsAvailable()).toBe(false);
      setVersion({ channel: 'rc' });
      expect(mapAvailable()).toBe(true);
      expect(vmsAvailable()).toBe(true);
      dispose();
    });
  });

  it('is on in a development build whatever the server says', () => {
    vi.stubEnv('DEV', true);
    setVersion({ channel: 'stable' });
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
    expect(getFeatureAvailability('standaloneVms')).toBe('on');
  });
});
