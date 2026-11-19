import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  getFeatureAvailability,
  getFeatureStage,
  getGuideFeature,
  isFeatureGuideAvailable,
  isFeatureAvailable,
  isFeatureInPreview,
  normalizeFeatureStage,
  resolveFeatureAvailability,
} from '../featureReleases';

const releaseBuild = { development: false, preview: false };
const previewBuild = { development: false, preview: true };
// The server has said it is no development build, and not yet which channel.
const unknownBuild = { development: false, preview: undefined };
const developmentBuild = { development: true, preview: undefined };
// The server has said nothing yet.
const unansweredBuild = { development: undefined, preview: undefined };

describe('feature release stages', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('shows a feature at build in a development build only', () => {
    expect(resolveFeatureAvailability('build', developmentBuild)).toBe('on');
    expect(resolveFeatureAvailability('build', previewBuild)).toBe('off');
    expect(resolveFeatureAvailability('build', releaseBuild)).toBe('off');
    expect(resolveFeatureAvailability('build', unknownBuild)).toBe('off');
  });

  it('shows a feature in preview in development and Preview builds, never in a stable one', () => {
    expect(resolveFeatureAvailability('preview', developmentBuild)).toBe('on');
    expect(resolveFeatureAvailability('preview', previewBuild)).toBe('on');
    expect(resolveFeatureAvailability('preview', releaseBuild)).toBe('off');
  });

  it('waits for the build to say what it is before showing a preview feature', () => {
    expect(resolveFeatureAvailability('preview', unknownBuild)).toBe('pending');
    expect(resolveFeatureAvailability('preview', unansweredBuild)).toBe('pending');
  });

  it('waits for the build to say whether it is a development build', () => {
    // A production bundle shows a feature at build only where the version reply
    // makes it a development build, which only the end-to-end suite's mark does.
    expect(resolveFeatureAvailability('build', unansweredBuild)).toBe('pending');
    expect(resolveFeatureAvailability('build', { development: true, preview: false })).toBe('on');
    // A development build of a pre-release still shows it. A published one does not.
    expect(resolveFeatureAvailability('build', { development: true, preview: true })).toBe('on');
    expect(resolveFeatureAvailability('build', { development: false, preview: true })).toBe('off');
  });

  it('shows a released feature in every build', () => {
    expect(resolveFeatureAvailability('released', releaseBuild)).toBe('on');
    expect(resolveFeatureAvailability('released', previewBuild)).toBe('on');
    expect(resolveFeatureAvailability('released', unknownBuild)).toBe('on');
  });

  it('treats a stage it does not know as not yet shown', () => {
    expect(normalizeFeatureStage('preview')).toBe('preview');
    expect(normalizeFeatureStage('released')).toBe('released');
    expect(normalizeFeatureStage('Released')).toBe('build');
    expect(normalizeFeatureStage(true)).toBe('build');
    expect(normalizeFeatureStage(undefined)).toBe('build');
  });

  // Moving the map past build is Richard's call after its review. This fails
  // on purpose when the stage changes, so the change is never a side effect.
  it('holds the map at build', () => {
    expect(getFeatureStage('infrastructureMap')).toBe('build');
    expect(isFeatureInPreview('infrastructureMap')).toBe(false);
  });

  it('shows the map in this development build and not in a release build', () => {
    expect(getFeatureAvailability('infrastructureMap')).toBe('on');
    expect(isFeatureAvailable('infrastructureMap')).toBe(true);

    // A production bundle shows nothing until the server has said what it is.
    vi.stubEnv('DEV', false);
    expect(getFeatureAvailability('infrastructureMap')).toBe('pending');
    expect(isFeatureAvailable('infrastructureMap')).toBe(false);
  });

  it('knows which feature a guide belongs to, however the address is spelled', () => {
    expect(getGuideFeature('MAP')).toBe('infrastructureMap');
    expect(getGuideFeature('map')).toBe('infrastructureMap');
    expect(getGuideFeature('docs/MAP.md')).toBe('infrastructureMap');
    expect(getGuideFeature('MAP.MD')).toBe('infrastructureMap');
    expect(getGuideFeature('ROADMAP')).toBeUndefined();
    expect(getGuideFeature('README')).toBeUndefined();
    expect(getGuideFeature('')).toBeUndefined();
  });

  it('opens the map guide in this development build and not in a release build', () => {
    expect(isFeatureGuideAvailable('MAP')).toBe(true);

    vi.stubEnv('DEV', false);
    expect(isFeatureGuideAvailable('MAP')).toBe(false);
    expect(isFeatureGuideAvailable('map.md')).toBe(false);
    expect(isFeatureGuideAvailable('README')).toBe(true);
    expect(isFeatureGuideAvailable('AVAILABILITY')).toBe(true);
  });
});
