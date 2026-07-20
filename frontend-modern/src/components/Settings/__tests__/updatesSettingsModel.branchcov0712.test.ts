/**
 * Branch-coverage tests for the exported helpers in updatesSettingsModel.
 * Each block targets a single function and drives both arms of every
 * conditional / ternary / optional-chain / guard / early-return branch that is
 * reachable from the public surface.
 *
 * Targets: buildUpdateInstallGuide, getUpdateChannelCardOptions,
 * buildIdleDockerComposeCommand, buildIdleDockerUpdateCommand.
 */
import { describe, expect, it } from 'vitest';
import {
  buildIdleDockerComposeCommand,
  buildIdleDockerUpdateCommand,
  buildUpdateInstallGuide,
  getUpdateChannelCardOptions,
} from '../updatesSettingsModel';
import type { DockerUpdateCommands, UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';

// ---- Fixtures ---------------------------------------------------------------
// Mirror the shapes used by the sibling UpdateInstallGuide.test.tsx so the two
// files stay consistent. The model only consumes narrow Picks of these types.

type VersionInfoPick = Pick<VersionInfo, 'deploymentType' | 'isDocker'>;
type UpdateInfoPick = Pick<UpdateInfo, 'latestVersion' | 'dockerUpdate'>;
type UpdatePlanPick = Pick<UpdatePlan, 'canAutoUpdate'>;

const makeVersionInfo = (overrides: Partial<VersionInfoPick> = {}): VersionInfoPick => ({
  deploymentType: 'systemd',
  isDocker: false,
  ...overrides,
});

const makeUpdateInfo = (overrides: Partial<UpdateInfoPick> = {}): UpdateInfoPick => ({
  latestVersion: 'v6.0.5',
  ...overrides,
});

const makeUpdatePlan = (overrides: Partial<UpdatePlanPick> = {}): UpdatePlanPick => ({
  canAutoUpdate: false,
  ...overrides,
});

const digest = 'sha256:' + 'ab'.repeat(32);
const pinnedRef = `registry.pulserelay.pro/pulse/pulse-pro@${digest}`;
const makeDockerUpdate = (overrides: Partial<DockerUpdateCommands> = {}): DockerUpdateCommands => ({
  version: 'v6.0.5',
  image: 'registry.pulserelay.pro/pulse/pulse-pro',
  imageDigest: digest,
  loginCommand:
    "printf '%s' '<activation-key>' | docker login registry.pulserelay.pro -u 'lic_test' --password-stdin",
  composePullCommand: `PULSE_IMAGE='${pinnedRef}' docker compose pull`,
  composeUpCommand: `PULSE_IMAGE='${pinnedRef}' docker compose up -d`,
  ...overrides,
});

const PRO_STEP_CODE_CLASS =
  'block rounded-md border border-border bg-base p-3 font-mono text-sm text-green-400 whitespace-pre-wrap break-all';
const SYSTEMD_DOWNLOAD_CODE_CLASS =
  'block rounded-md border border-border bg-base p-3 font-mono text-sm text-base-content whitespace-pre-wrap break-all';

// ---- buildIdleDockerComposeCommand -----------------------------------------
// Single return path; assert the exact constant.

describe('buildIdleDockerComposeCommand', () => {
  it('returns the community docker compose pull+up command', () => {
    expect(buildIdleDockerComposeCommand()).toBe('docker compose pull && docker compose up -d');
  });
});

// ---- buildIdleDockerUpdateCommand -------------------------------------------
// Single return path; assert the exact constant.

describe('buildIdleDockerUpdateCommand', () => {
  it('returns the community docker pull/stop/rm command', () => {
    expect(buildIdleDockerUpdateCommand()).toBe(
      'docker pull rcourtman/pulse:latest && docker stop pulse && docker rm pulse',
    );
  });
});

// ---- getUpdateChannelCardOptions --------------------------------------------
// Branches: the `versionInfo?.isSourceBuild` optional chain. Four reachable
// arms: input undefined (chain short-circuits -> disabled undefined), input
// null (chain short-circuits -> disabled undefined), isSourceBuild true
// (disabled true), isSourceBuild false (disabled false). The full option
// shape (value/title/description/tone) is constant across all arms.

describe('getUpdateChannelCardOptions', () => {
  it('returns both options with disabled undefined when versionInfo is omitted (optional-chain short-circuit)', () => {
    expect(getUpdateChannelCardOptions()).toStrictEqual([
      {
        value: 'stable',
        title: 'Stable',
        description: 'Production-ready releases for paid and self-hosted environments',
        tone: 'success',
        disabled: undefined,
      },
      {
        value: 'rc',
        title: 'Pre-release',
        description: 'Early preview builds for staging, internal validation, and opt-in testers',
        tone: 'accent',
        disabled: undefined,
      },
    ]);
  });

  it('returns both options with disabled undefined when versionInfo is null (optional-chain short-circuit)', () => {
    const options = getUpdateChannelCardOptions(null);
    expect(options[0]?.disabled).toBeUndefined();
    expect(options[1]?.disabled).toBeUndefined();
  });

  it('disables both options for a source build (isSourceBuild true arm)', () => {
    const options = getUpdateChannelCardOptions({ isSourceBuild: true });
    expect(options[0]?.disabled).toBe(true);
    expect(options[1]?.disabled).toBe(true);
    // value/tone are unaffected by the branch.
    expect(options[0]?.value).toBe('stable');
    expect(options[0]?.tone).toBe('success');
    expect(options[1]?.value).toBe('rc');
    expect(options[1]?.tone).toBe('accent');
  });

  it('leaves both options enabled for a packaged build (isSourceBuild false arm)', () => {
    const options = getUpdateChannelCardOptions({ isSourceBuild: false });
    expect(options[0]?.disabled).toBe(false);
    expect(options[1]?.disabled).toBe(false);
    // Titles/descriptions are the constant copy.
    expect(options[0]?.title).toBe('Stable');
    expect(options[1]?.title).toBe('Pre-release');
  });
});

// ---- buildUpdateInstallGuide ------------------------------------------------
// Branches covered below:
//  - !versionInfo guard (null and undefined -> null)
//  - introText ternary: canAutoUpdate truthy arm vs falsy/absent-plan arm
//  - proxmoxve deployment arm
//  - docker arm: deploymentType === 'docker' vs the (!deploymentType && isDocker) fallback
//  - docker arm: updateInfo?.dockerUpdate truthy (pro steps) vs absent
//  - pro steps inner: loginCommand present (note pushed) vs absent
//  - docker arm: isProRuntime true (pro-unavailable notice) vs community guidance
//  - systemd/manual arm (both OR operands)
//  - development arm
//  - final return null (unknown deploymentType, and undefined+!isDocker fallthrough)

describe('buildUpdateInstallGuide', () => {
  // -- !versionInfo guard ----------------------------------------------------

  it('returns null when versionInfo is null (!versionInfo guard)', () => {
    expect(
      buildUpdateInstallGuide(null, makeUpdateInfo(), makeUpdatePlan(), 'v6.0.5', 'curl', false),
    ).toBeNull();
  });

  it('returns null when versionInfo is undefined (!versionInfo guard)', () => {
    expect(
      buildUpdateInstallGuide(
        undefined,
        makeUpdateInfo(),
        makeUpdatePlan(),
        'v6.0.5',
        'curl',
        false,
      ),
    ).toBeNull();
  });

  // -- introText ternary -----------------------------------------------------

  it('uses the automatic-install intro when updatePlan.canAutoUpdate is true (ternary truthy arm)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      makeUpdateInfo(),
      makeUpdatePlan({ canAutoUpdate: true }),
      'v6.0.5',
      'curl -fsSL https://example.invalid/pulse.tar.gz',
      false,
    );
    expect(guide?.introText).toBe(
      'Click "Install Update" above for automatic installation, or update manually:',
    );
  });

  it('uses the manual-only intro when updatePlan.canAutoUpdate is false (ternary falsy arm)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      makeUpdateInfo(),
      makeUpdatePlan({ canAutoUpdate: false }),
      'v6.0.5',
      'curl -fsSL https://example.invalid/pulse.tar.gz',
      false,
    );
    expect(guide?.introText).toBe('Follow these steps to update manually:');
  });

  it('uses the manual-only intro when updatePlan is null (optional-chain short-circuit -> falsy arm)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      makeUpdateInfo(),
      null,
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.introText).toBe('Follow these steps to update manually:');
  });

  it('uses the manual-only intro when updatePlan is undefined (optional-chain short-circuit -> falsy arm)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      makeUpdateInfo(),
      undefined,
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.introText).toBe('Follow these steps to update manually:');
  });

  // -- proxmoxve arm ---------------------------------------------------------

  it('returns the proxmox LXC guide for the proxmoxve deployment arm', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'proxmoxve' }),
      makeUpdateInfo({ latestVersion: 'v6.0.5' }),
      makeUpdatePlan({ canAutoUpdate: false }),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide).toStrictEqual({
      headerTitle: 'Update Available',
      headerSummary: 'Version v6.0.5 is ready to install',
      introText: 'Follow these steps to update manually:',
      steps: [
        { id: 'open-console', title: 'Open your Pulse LXC console' },
        { id: 'run-update', title: 'Run the update command:', command: 'update' },
        {
          id: 'run-update-note',
          title: '',
          note: 'The script will automatically download and install the latest version.',
        },
      ],
    });
  });

  // -- docker arm: pro steps (dockerUpdate present) --------------------------

  it('returns digest-pinned pro docker steps when dockerUpdate is present (with login note)', () => {
    const dockerUpdate = makeDockerUpdate();
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    // The dockerUpdate arm overrides introText with the two-command copy.
    expect(guide?.introText).toBe('Run these two commands on the Docker host to update:');
    expect(guide?.steps).toHaveLength(3);
    expect(guide?.steps[0]).toStrictEqual({
      id: 'docker-pro-pull',
      title: 'Pull the new Pulse Pro image (pinned to this release\u2019s digest)',
      command: dockerUpdate.composePullCommand,
      commandCodeClass: PRO_STEP_CODE_CLASS,
    });
    expect(guide?.steps[1]).toStrictEqual({
      id: 'docker-pro-up',
      title: 'Recreate the container on the new image',
      command: dockerUpdate.composeUpCommand,
      commandCodeClass: PRO_STEP_CODE_CLASS,
    });
    // loginCommand truthy arm: a third note step is pushed.
    expect(guide?.steps[2]?.id).toBe('docker-pro-login-note');
    expect(guide?.steps[2]?.title).toBe('');
    expect(guide?.steps[2]?.note).toContain('sign in to the registry first');
    expect(guide?.steps[2]?.note).toContain(dockerUpdate.loginCommand as string);
  });

  it('omits the login note step when dockerUpdate has no loginCommand (loginCommand falsy arm)', () => {
    const dockerUpdate = makeDockerUpdate({ loginCommand: undefined });
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.steps).toHaveLength(2);
    expect(guide?.steps.map((s) => s.id)).toStrictEqual(['docker-pro-pull', 'docker-pro-up']);
    expect(guide?.steps.some((s) => s.id === 'docker-pro-login-note')).toBe(false);
  });

  it('reaches the pro steps arm via the (!deploymentType && isDocker) docker fallback, not just deploymentType==="docker"', () => {
    // Empty/undefined deploymentType with isDocker true must still hit the
    // docker branch via the second OR operand.
    const dockerUpdate = makeDockerUpdate();
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: undefined, isDocker: true }),
      makeUpdateInfo({ dockerUpdate }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.steps[0]?.id).toBe('docker-pro-pull');
    expect(guide?.headerSummary).toBe('Version v6.0.5 is ready to install');
  });

  // -- docker arm: isProRuntime, no dockerUpdate -----------------------------

  it('returns the pro-unavailable notice when isProRuntime and no dockerUpdate (isProRuntime true arm)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      true,
    );
    expect(guide?.introText).toBe('Follow these steps to update:');
    expect(guide?.steps).toHaveLength(1);
    expect(guide?.steps[0]?.id).toBe('docker-pro-unavailable');
    expect(guide?.steps[0]?.title).toBe('');
    // Concrete substrings of the withhold-community-image notice.
    expect(guide?.steps[0]?.note).toContain('Pulse Pro image');
    expect(guide?.steps[0]?.note).toContain('Private Release Access');
    expect(guide?.steps[0]?.note).toContain('Do not pull the public rcourtman/pulse image');
  });

  // -- docker arm: community guidance (not pro, no dockerUpdate) -------------

  it('returns community docker guidance with the image tag interpolated when not pro and no dockerUpdate', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan({ canAutoUpdate: true }),
      'v6.0.5',
      'curl',
      false,
    );
    // isProRuntime defaults to false too; verify the default-arg path.
    expect(guide?.steps).toHaveLength(2);
    expect(guide?.steps[0]).toStrictEqual({
      id: 'docker-compose-update',
      title: 'Pull the new image and recreate the container',
      command: 'docker compose pull && docker compose up -d',
    });
    expect(guide?.steps[1]?.id).toBe('docker-manual-note');
    // The dockerImageTag is interpolated into the non-compose note.
    expect(guide?.steps[1]?.note).toContain('docker pull rcourtman/pulse:v6.0.5');
    expect(guide?.steps[1]?.note).toContain('docker stop pulse && docker rm pulse');
    // introText ternary truthy arm flows through to the community guide.
    expect(guide?.introText).toBe(
      'Click "Install Update" above for automatic installation, or update manually:',
    );
  });

  it('honours the isProRuntime default (false) when the trailing arg is omitted', () => {
    // Calling with 5 args exercises the `isProRuntime = false` default.
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
    );
    expect(guide?.steps[0]?.id).toBe('docker-compose-update');
  });

  // -- systemd / manual arm --------------------------------------------------

  it('returns the systemd guide with the download command and code class (systemd arm)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      makeUpdateInfo({ latestVersion: 'v6.1.0' }),
      makeUpdatePlan({ canAutoUpdate: false }),
      'v6.1.0',
      'curl -fsSL https://example.invalid/pulse.tar.gz | tar',
      false,
    );
    expect(guide).toStrictEqual({
      headerTitle: 'Update Available',
      headerSummary: 'Version v6.1.0 is ready to install',
      introText: 'Follow these steps to update manually:',
      steps: [
        { id: 'systemd-stop', title: 'Stop the service', command: 'sudo systemctl stop pulse' },
        {
          id: 'systemd-download',
          title: 'Download and extract the new version',
          command: 'curl -fsSL https://example.invalid/pulse.tar.gz | tar',
          commandCodeClass: SYSTEMD_DOWNLOAD_CODE_CLASS,
        },
        { id: 'systemd-start', title: 'Start the service', command: 'sudo systemctl start pulse' },
      ],
    });
  });

  it('returns the same systemd-shaped guide for the manual deployment arm (second OR operand)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'manual' }),
      makeUpdateInfo(),
      makeUpdatePlan(),
      'v6.0.5',
      'curl-download',
      false,
    );
    expect(guide?.steps.map((s) => s.id)).toStrictEqual([
      'systemd-stop',
      'systemd-download',
      'systemd-start',
    ]);
    expect(guide?.steps[1]?.command).toBe('curl-download');
    expect(guide?.steps[1]?.commandCodeClass).toBe(SYSTEMD_DOWNLOAD_CODE_CLASS);
  });

  // -- development arm -------------------------------------------------------

  it('returns the git/make development guide for the development arm', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'development' }),
      makeUpdateInfo(),
      makeUpdatePlan({ canAutoUpdate: false }),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide).toStrictEqual({
      headerTitle: 'Update Available',
      headerSummary: 'Version v6.0.5 is ready to install',
      introText: 'Follow these steps to update manually:',
      steps: [
        {
          id: 'development-pull',
          title: 'Pull the latest changes',
          command: 'git pull origin main',
        },
        {
          id: 'development-build',
          title: 'Rebuild and restart',
          command: 'make build && make run',
        },
      ],
    });
  });

  // -- final return null -----------------------------------------------------

  it('returns null for an unrecognised non-empty deployment type (final fallthrough)', () => {
    expect(
      buildUpdateInstallGuide(
        makeVersionInfo({ deploymentType: 'kubernetes', isDocker: false }),
        makeUpdateInfo(),
        makeUpdatePlan(),
        'v6.0.5',
        'curl',
        false,
      ),
    ).toBeNull();
  });

  it('returns null when deploymentType is absent and isDocker is false (docker OR fallback fails -> fallthrough)', () => {
    expect(
      buildUpdateInstallGuide(
        makeVersionInfo({ deploymentType: undefined, isDocker: false }),
        makeUpdateInfo(),
        makeUpdatePlan(),
        'v6.0.5',
        'curl',
        false,
      ),
    ).toBeNull();
  });

  // -- suspected source-behaviour documentation ------------------------------
  // When updateInfo is absent the headerSummary interpolates undefined, producing
  // "Version undefined is ready to install". This test pins the current behaviour
  // (see GLM_REPORT.md suspected-bug note). No source change is made here.

  it('interpolates "undefined" into headerSummary when updateInfo is null (documented suspect behaviour)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'proxmoxve' }),
      null,
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.headerSummary).toBe('Version undefined is ready to install');
  });
});
