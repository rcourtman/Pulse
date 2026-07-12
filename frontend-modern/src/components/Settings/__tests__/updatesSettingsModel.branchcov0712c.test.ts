/**
 * Additional branch-coverage tests for buildUpdateInstallGuide in
 * updatesSettingsModel. This file is deliberately complementary to the sibling
 * updatesSettingsModel.branchcov0712.test.ts: it targets semantic branches that
 * the sibling does not exercise (optional-chain short-circuits on nullish
 * `updateInfo`, isProRuntime/dockerUpdate precedence, the docker OR's first
 * operand alone, the `(!deploymentType && isDocker)` fallback routed through the
 * community and pro-unavailable sub-arms, template-literal interpolation edges,
 * and a malformed-plan cast). v8's branch counter is coarse and reports these as
 * covered via neighbouring cases; the tests below pin the distinct runtime paths
 * and their concrete outputs.
 */
import { describe, expect, it } from 'vitest';
import { buildUpdateInstallGuide } from '../updatesSettingsModel';
import type {
  DockerUpdateCommands,
  UpdateInfo,
  UpdatePlan,
  VersionInfo,
} from '@/api/updates';

// ---- Fixtures ---------------------------------------------------------------
// Mirror the shapes used by the sibling branchcov0712 file so the two stay
// consistent. The model only consumes narrow Picks of these types.

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
const makeDockerUpdate = (
  overrides: Partial<DockerUpdateCommands> = {},
): DockerUpdateCommands => ({
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

// ---- buildUpdateInstallGuide (complementary branch-coverage) ----------------

describe('buildUpdateInstallGuide — optional-chain short-circuit on nullish updateInfo', () => {
  // The sibling tests always pass a *present* updateInfo object (setting
  // dockerUpdate: undefined). These two tests drive the `updateInfo?.dockerUpdate`
  // optional-chain short-circuit by passing nullish updateInfo itself, which
  // routes the docker arm to the community and pro-unavailable sub-branches
  // respectively and also short-circuits `updateInfo?.latestVersion`.

  it('short-circuits updateInfo?.dockerUpdate to falsy when updateInfo is null, routing to community guidance (not pro) and interpolating "undefined" into headerSummary', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      null,
      makeUpdatePlan({ canAutoUpdate: false }),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.headerSummary).toBe('Version undefined is ready to install');
    expect(guide?.introText).toBe('Follow these steps to update manually:');
    expect(guide?.steps).toHaveLength(2);
    expect(guide?.steps[0]).toStrictEqual({
      id: 'docker-compose-update',
      title: 'Pull the new image and recreate the container',
      command: 'docker compose pull && docker compose up -d',
    });
    expect(guide?.steps[1]?.id).toBe('docker-manual-note');
  });

  it('short-circuits updateInfo?.dockerUpdate to falsy when updateInfo is undefined, routing to the pro-unavailable notice when isProRuntime is true', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      undefined,
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      true,
    );
    expect(guide?.headerSummary).toBe('Version undefined is ready to install');
    expect(guide?.introText).toBe('Follow these steps to update:');
    expect(guide?.steps).toHaveLength(1);
    expect(guide?.steps[0]?.id).toBe('docker-pro-unavailable');
    expect(guide?.steps[0]?.note).toContain('license server did not provide Docker update commands');
  });
});

describe('buildUpdateInstallGuide — isProRuntime / dockerUpdate precedence', () => {
  // The sibling dockerUpdate-present tests all pass isProRuntime: false. This
  // pins that dockerUpdate takes precedence over isProRuntime: when both are
  // truthy, the digest-pinned pro steps win and the pro-unavailable notice is
  // never reached.

  it('prefers digest-pinned pro docker steps over the isProRuntime notice when both dockerUpdate and isProRuntime are truthy', () => {
    const dockerUpdate = makeDockerUpdate();
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      true,
    );
    expect(guide?.introText).toBe('Run these two commands on the Docker host to update:');
    expect(guide?.steps[0]).toStrictEqual({
      id: 'docker-pro-pull',
      title: 'Pull the new Pulse Pro image (pinned to this release\u2019s digest)',
      command: dockerUpdate.composePullCommand,
      commandCodeClass: PRO_STEP_CODE_CLASS,
    });
    expect(guide?.steps[1]?.id).toBe('docker-pro-up');
    expect(guide?.steps.some((s) => s.id === 'docker-pro-unavailable')).toBe(false);
  });
});

describe('buildUpdateInstallGuide — docker OR operand coverage', () => {
  // The sibling docker-arm tests always set isDocker: true alongside
  // deploymentType: 'docker'. This exercises the first OR operand being true on
  // its own (the second operand is not evaluated), confirming the arm is entered
  // regardless of isDocker when deploymentType is explicitly 'docker'.

  it('enters the docker arm via deploymentType==="docker" even when isDocker is false (first OR operand true, second not evaluated)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: false }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide).not.toBeNull();
    expect(guide?.steps[0]?.id).toBe('docker-compose-update');
  });

  // The sibling fallback test (deploymentType undefined + isDocker true) only
  // routes through the dockerUpdate-present sub-arm. These two tests route the
  // fallback through the community and pro-unavailable sub-arms respectively.

  it('routes the (!deploymentType && isDocker) fallback to community guidance when not pro and no dockerUpdate', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: undefined, isDocker: true }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan(),
      'v9.9.9',
      'curl',
      false,
    );
    // headerSummary comes from updateInfo.latestVersion, not dockerImageTag.
    expect(guide?.headerSummary).toBe('Version v6.0.5 is ready to install');
    expect(guide?.steps[0]?.id).toBe('docker-compose-update');
    // dockerImageTag 'v9.9.9' is interpolated into the non-compose note.
    expect(guide?.steps[1]?.note).toContain('docker pull rcourtman/pulse:v9.9.9');
  });

  it('routes the (!deploymentType && isDocker) fallback to the pro-unavailable notice when isProRuntime and no dockerUpdate', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: undefined, isDocker: true }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      true,
    );
    expect(guide?.introText).toBe('Follow these steps to update:');
    expect(guide?.steps[0]?.id).toBe('docker-pro-unavailable');
  });
});

describe('buildUpdateInstallGuide — headerSummary interpolation for nullish updateInfo across arms', () => {
  // The sibling "Version undefined" test only covers the proxmox arm with null
  // updateInfo. These pin the same short-circuit behaviour (updateInfo?.latestVersion
  // -> undefined) for undefined updateInfo in the systemd and development arms,
  // and for null updateInfo in the docker community arm.

  it('interpolates "undefined" into headerSummary for the systemd arm when updateInfo is undefined', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      undefined,
      makeUpdatePlan(),
      'v6.0.5',
      'curl -fsSL https://example.invalid/pulse.tar.gz',
      false,
    );
    expect(guide?.headerSummary).toBe('Version undefined is ready to install');
    // The systemd steps are still produced in full.
    expect(guide?.steps.map((s) => s.id)).toStrictEqual([
      'systemd-stop',
      'systemd-download',
      'systemd-start',
    ]);
  });

  it('interpolates "undefined" into headerSummary for the development arm when updateInfo is null', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'development' }),
      null,
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.headerSummary).toBe('Version undefined is ready to install');
    expect(guide?.steps.map((s) => s.id)).toStrictEqual([
      'development-pull',
      'development-build',
    ]);
  });
});

describe('buildUpdateInstallGuide — template-literal and malformed-input edges', () => {
  // dockerImageTag is interpolated verbatim into the community note. An empty
  // tag still renders a well-formed (if odd) command string; this pins the
  // template-literal interpolation branch.
  it('interpolates an empty dockerImageTag into the community note (template-literal edge)', () => {
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate: undefined }),
      makeUpdatePlan(),
      '',
      'curl',
      false,
    );
    expect(guide?.steps[1]?.note).toContain('docker pull rcourtman/pulse:');
    // No trailing version text after the colon.
    expect(guide?.steps[1]?.note).toMatch(/docker pull rcourtman\/pulse:,/);
  });

  // A present updateInfo whose latestVersion is absent cannot be expressed
  // through the typed UpdateInfoPick (latestVersion is required), so cast a
  // malformed object to exercise the same interpolation path via a distinct
  // runtime shape.
  it('interpolates "undefined" into headerSummary when updateInfo is present but latestVersion is missing (cast malformed updateInfo)', () => {
    const malformedUpdateInfo = {
      dockerUpdate: undefined,
    } as unknown as Parameters<typeof buildUpdateInstallGuide>[1];
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      malformedUpdateInfo,
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.headerSummary).toBe('Version undefined is ready to install');
  });

  // updatePlan.canAutoUpdate is a required boolean on the Pick; an undefined
  // value (malformed via cast) must still take the ternary falsy arm.
  it('uses the manual-only intro when updatePlan.canAutoUpdate is undefined (cast malformed plan -> ternary falsy arm)', () => {
    const malformedPlan = {
      canAutoUpdate: undefined,
    } as unknown as UpdatePlanPick;
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'systemd' }),
      makeUpdateInfo(),
      malformedPlan,
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.introText).toBe('Follow these steps to update manually:');
  });
});

describe('buildUpdateInstallGuide — buildProDockerUpdateSteps loginCommand falsy variants', () => {
  // The sibling test covers loginCommand undefined (falsy). An empty-string
  // loginCommand is also falsy but a distinct runtime value, and a non-empty
  // loginCommand is truthy. These pin both falsy variants' concrete outputs.

  it('omits the login note when loginCommand is an empty string (falsy empty-string arm)', () => {
    const dockerUpdate = makeDockerUpdate({ loginCommand: '' });
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
  });

  it('includes the login note with the exact interpolated command when loginCommand is a non-empty custom string', () => {
    const customLogin = 'echo hunter2 | docker login registry.example.invalid -u ops --password-stdin';
    const dockerUpdate = makeDockerUpdate({ loginCommand: customLogin });
    const guide = buildUpdateInstallGuide(
      makeVersionInfo({ deploymentType: 'docker', isDocker: true }),
      makeUpdateInfo({ dockerUpdate }),
      makeUpdatePlan(),
      'v6.0.5',
      'curl',
      false,
    );
    expect(guide?.steps).toHaveLength(3);
    expect(guide?.steps[2]?.id).toBe('docker-pro-login-note');
    expect(guide?.steps[2]?.note).toBe(
      `If the pull is denied, sign in to the registry first (replace <activation-key> with the key from Settings \u2192 License): ${customLogin}`,
    );
  });
});
