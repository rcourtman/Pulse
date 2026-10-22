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
import { getProRuntimeSwitchPresentation } from '../proRuntimeSwitchPresentation';
import type { ProRuntimeSwitch } from '@/api/updates';

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
      description: 'Keep Pulse up to date. Agent updates are diagnosed under Infrastructure.',
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

    it('measures a held check from the supplied presentation clock', () => {
      const checkedAt = Date.parse('2026-10-11T04:00:00Z');
      expect(getUpdateCheckedLabel(checkedAt, checkedAt + 60_000)).toBe('Checked 1 min ago');
      expect(getUpdateCheckedLabel(checkedAt, checkedAt + 120_000)).toBe('Checked 2 mins ago');
      expect(getUpdateCheckedLabel(checkedAt, checkedAt - 60_000)).toBe('Checked 0s ago');
      expect(getUpdateCheckedLabel(null, checkedAt)).toBe('Not checked yet');
      expect(getUpdateCheckedLabel(NaN, checkedAt)).toBe('Not checked yet');
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

const proRuntimeSwitchBase: ProRuntimeSwitch = {
  available: false,
  currentVersion: '6.5.0',
  installInPlace: false,
};

const pinnedRef = 'registry.pulserelay.pro/pulse/pulse-pro@sha256:' + 'ab'.repeat(32);
const proDockerCommands = {
  version: 'v6.5.0',
  image: 'registry.pulserelay.pro/pulse/pulse-pro',
  imageDigest: 'sha256:' + 'ab'.repeat(32),
  loginCommand:
    "printf '%s' '<activation-key>' | docker login registry.pulserelay.pro -u 'lic_test' --password-stdin",
  composePullCommand: `PULSE_IMAGE='${pinnedRef}' docker compose pull`,
  composeUpCommand: `PULSE_IMAGE='${pinnedRef}' docker compose up -d`,
};

describe('getProRuntimeSwitchPresentation', () => {
  it('offers an in-place switch naming the locked features and the target', () => {
    const view = getProRuntimeSwitchPresentation({
      ...proRuntimeSwitchBase,
      available: true,
      installInPlace: true,
      version: '6.5.0',
    });
    expect(view.kind).toBe('in_place');
    if (view.kind !== 'in_place') return;
    expect(view.version).toBe('v6.5.0');
    expect(view.body).toContain('Patrol modes, Audit Log, RBAC and Reporting');
    expect(view.body).toContain('Pulse Pro v6.5.0');
  });

  it('gives Docker installs protected login first and preserves the pinned broker commands', () => {
    const view = getProRuntimeSwitchPresentation({
      ...proRuntimeSwitchBase,
      available: true,
      docker: proDockerCommands,
    });
    expect(view.kind).toBe('docker');
    if (view.kind !== 'docker') return;
    expect(view.steps.map((step) => step.command)).toEqual([
      "docker login registry.pulserelay.pro --username 'lic_test'",
      proDockerCommands.composePullCommand,
      proDockerCommands.composeUpCommand,
    ]);
    expect(view.steps[0].note).toContain('Docker’s hidden Password prompt');
    expect(JSON.stringify(view)).not.toContain('<activation-key>');
    expect(view.note).toContain('PULSE_IMAGE');
    expect(view.steps.some((step) => step.command?.includes('rcourtman/pulse'))).toBe(false);
  });

  it('skips the login step when the broker sends none', () => {
    const view = getProRuntimeSwitchPresentation({
      ...proRuntimeSwitchBase,
      available: true,
      docker: { ...proDockerCommands, loginCommand: undefined },
    });
    if (view.kind !== 'docker') throw new Error(`expected docker, got ${view.kind}`);
    expect(view.steps.map((step) => step.id)).toEqual(['pro-runtime-pull', 'pro-runtime-up']);
  });

  it('explains an older Pro release with both versions', () => {
    const view = getProRuntimeSwitchPresentation({
      ...proRuntimeSwitchBase,
      currentVersion: '6.6.0',
      reason: 'older_release',
      version: '6.5.0',
    });
    expect(view.kind).toBe('older_release');
    if (view.kind !== 'older_release') return;
    expect(view.body).toContain('v6.5.0');
    expect(view.body).toContain('v6.6.0');
  });

  it('explains a preview-only Pro build to a stable-channel install', () => {
    const view = getProRuntimeSwitchPresentation({
      ...proRuntimeSwitchBase,
      reason: 'prerelease_only',
      version: '6.1.0-rc.1',
    });
    expect(view.kind).toBe('prerelease_only');
    if (view.kind !== 'prerelease_only') return;
    expect(view.body).toContain('v6.1.0-rc.1');
    expect(view.body).toContain('stable');
  });

  it('shows a quiet notice when the license server cannot be reached', () => {
    expect(
      getProRuntimeSwitchPresentation({ ...proRuntimeSwitchBase, reason: 'unavailable' }).kind,
    ).toBe('unavailable');
  });

  it.each([
    'already_pro',
    'not_activated',
    'not_eligible',
    'source_build',
    'unsupported_deployment',
  ] as const)('stays hidden for %s', (reason) => {
    expect(getProRuntimeSwitchPresentation({ ...proRuntimeSwitchBase, reason }).kind).toBe(
      'hidden',
    );
  });

  it('stays hidden without a result', () => {
    expect(getProRuntimeSwitchPresentation(null).kind).toBe('hidden');
    expect(getProRuntimeSwitchPresentation(undefined).kind).toBe('hidden');
  });
});
