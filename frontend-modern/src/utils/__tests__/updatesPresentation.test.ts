import { describe, expect, it } from 'vitest';
import {
  getUpdateAvailabilityHeading,
  getUpdateBuildBadges,
  getUpdateCheckModeLabel,
  getUpdatePrimaryStatusLabel,
} from '../updatesPresentation';

describe('updatesPresentation', () => {
  it('returns canonical build badges', () => {
    expect(
      getUpdateBuildBadges({ isDevelopment: true, isDocker: true, isSourceBuild: true }).map(
        (badge) => badge.label,
      ),
    ).toEqual(['Development', 'Docker', 'Source']);
  });

  it('returns canonical update status copy', () => {
    expect(getUpdateAvailabilityHeading(true)).toBe('Available');
    expect(getUpdateAvailabilityHeading(false)).toBe('Status');
    expect(getUpdatePrimaryStatusLabel(true)).toBe('Update Ready');
    expect(getUpdatePrimaryStatusLabel(false)).toBe('Up to date');
    expect(getUpdateCheckModeLabel(true)).toBe('Auto-check enabled');
    expect(getUpdateCheckModeLabel(false)).toBe('Manual checks only');
  });
});
