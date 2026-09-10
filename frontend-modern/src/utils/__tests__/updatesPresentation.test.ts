import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  getUpdateCheckActionLabel,
  formatUpdateReleaseDate,
  getUpdateAvailabilityHeading,
  getUpdateBuildBadges,
  getUpdateCheckedLabel,
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
      title: 'Pulse server updates',
      description:
        'Manage the Pulse server runtime. Pulse Agent updates are diagnosed under Infrastructure.',
      currentVersionLabel: 'Server version',
      checkNowLabel: 'Check Now',
      checkingLabel: 'Checking...',
      updatePreferencesTitle: 'Update Preferences',
      autoUpdateTitle: 'Automatic Stable Updates',
      autoUpdateDescription:
        'Supported host installs can automatically apply stable releases. Preview testing always stays manual.',
      previewChannelTitle: 'Preview builds stay on a manual channel.',
      previewChannelDescription:
        'Use beta builds for user testing and release candidates only when they may become stable without product changes. Keep preview installs on staging or internal validation environments.',
      previewChannelAutoUpdateNotice:
        'Automatic stable updates are unavailable while the Preview channel is selected.',
      autoUpdateScheduleNote:
        'The host checks for stable releases once a day overnight and applies them automatically.',
    });
  });

  it('returns canonical update status copy', () => {
    expect(getUpdateAvailabilityHeading(true)).toBe('Available');
    expect(getUpdateAvailabilityHeading(false)).toBe('Status');
    expect(getUpdatePrimaryStatusLabel(true)).toBe('Update Ready');
    expect(getUpdatePrimaryStatusLabel(false)).toBe('Up to date');
    expect(getUpdatePrimaryStatusLabel(false, true)).toBe('Source build');
    expect(getUpdatePrimaryStatusLabel(true, true)).toBe('Source build');
    expect(getUpdateCheckModeLabel(true)).toBe('Auto-check enabled');
    expect(getUpdateCheckModeLabel(false)).toBe('Manual checks only');
    expect(getUpdateCheckModeLabel(true, true)).toBe('Release updates disabled');
    expect(getUpdateCheckActionLabel(true)).toBe('Checking...');
    expect(getUpdateCheckActionLabel(false)).toBe('Check Now');
  });

  describe('getUpdateCheckedLabel', () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it('labels a missing or unset check as not checked', () => {
      expect(getUpdateCheckedLabel(null)).toBe('Not checked yet');
      expect(getUpdateCheckedLabel(undefined)).toBe('Not checked yet');
      expect(getUpdateCheckedLabel(0)).toBe('Not checked yet');
    });

    it('states the age of the check the verdict came from', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-08-10T12:00:00Z'));
      expect(getUpdateCheckedLabel(new Date('2026-08-10T09:00:00Z').getTime())).toBe(
        'Checked 3 hours ago',
      );
      expect(getUpdateCheckedLabel(new Date('2026-08-10T11:59:30Z').getTime())).toBe(
        'Checked 30s ago',
      );
      expect(getUpdateCheckedLabel(new Date('2026-08-09T11:00:00Z').getTime())).toBe(
        'Checked 1 day ago',
      );
    });
  });
});

it('omits unknown and invalid release dates while formatting known dates', () => {
  for (const value of [undefined, '', '0001-01-01T00:00:00Z', 'invalid']) {
    expect(formatUpdateReleaseDate(value)).toBeUndefined();
  }
  const date = '2026-09-10T14:06:01Z';
  expect(formatUpdateReleaseDate(date)).toBe(new Date(date).toLocaleDateString());
});
