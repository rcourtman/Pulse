import { describe, expect, it } from 'vitest';
import {
  getUpdateCheckActionLabel,
  getUpdateAvailabilityHeading,
  getUpdateBuildBadges,
  getUpdateCheckModeLabel,
  getUpdatePrimaryStatusLabel,
  UPDATES_PANEL_COPY,
} from '../updatesPresentation';

describe('updatesPresentation', () => {
  it('returns canonical build badges', () => {
    expect(
      getUpdateBuildBadges({ isDevelopment: true, isDocker: true, isSourceBuild: true }).map(
        (badge) => badge.label,
      ),
    ).toEqual(['Development', 'Docker', 'Source']);
  });

  it('returns canonical updates panel framing copy', () => {
    expect(UPDATES_PANEL_COPY).toEqual({
      title: 'Updates',
      description: 'Manage version checks and automatic update preferences.',
      currentVersionLabel: 'Current Version',
      checkNowLabel: 'Check Now',
      checkingLabel: 'Checking...',
      updatePreferencesTitle: 'Update Preferences',
      autoUpdateTitle: 'Automatic Stable Updates',
      autoUpdateDescription:
        'Supported host installs can automatically apply stable releases. Pre-release testing always stays manual.',
      previewChannelTitle: 'Pre-release builds stay on a manual preview channel.',
      previewChannelDescription:
        'Use this on staging or internal validation environments. Automatic stable updates stay disabled on pre-release builds so preview installs do not drift between channels unattended.',
      previewChannelAutoUpdateNotice:
        'Automatic stable updates are unavailable while the pre-release preview channel is selected.',
      checkIntervalLabel: 'Check Interval',
      preferredTimeLabel: 'Preferred Time',
    });
  });

  it('returns canonical update status copy', () => {
    expect(getUpdateAvailabilityHeading(true)).toBe('Available');
    expect(getUpdateAvailabilityHeading(false)).toBe('Status');
    expect(getUpdatePrimaryStatusLabel(true)).toBe('Update Ready');
    expect(getUpdatePrimaryStatusLabel(false)).toBe('Up to date');
    expect(getUpdateCheckModeLabel(true)).toBe('Auto-check enabled');
    expect(getUpdateCheckModeLabel(false)).toBe('Manual checks only');
    expect(getUpdateCheckActionLabel(true)).toBe('Checking...');
    expect(getUpdateCheckActionLabel(false)).toBe('Check Now');
  });
});
