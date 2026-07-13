import { describe, expect, it } from 'vitest';
import { getSecurityAuthRestartInstruction } from '@/utils/securityAuthPresentation';

// The sibling `securityAuthPresentation.test.ts` already exercises the
// `docker`, `proxmoxve`, `development`, and no-arg (undefined) arms of
// `getSecurityAuthRestartInstruction`. This file raises branch coverage by
// exercising the remaining arms of the `deploymentType` switch:
//   - the shared `systemd` / `manual` case group
//   - unrecognized non-empty strings that fall through to `default`
//   - case-sensitivity of the switch (e.g. "DOCKER" != "docker")
//   - boundary / malformed inputs (empty string, null, non-string runtime values)
//
// `deploymentType` is typed as `string | undefined`, so deliberately-wrong
// inputs are cast via `as unknown as DeploymentTypeParam` to keep strict
// type-check clean without `any` / `@ts-ignore` / `@ts-expect-error`.

type DeploymentTypeParam = Parameters<typeof getSecurityAuthRestartInstruction>[0];

describe('getSecurityAuthRestartInstruction — branch coverage (batch 0713)', () => {
  describe('shared `systemd` / `manual` case group', () => {
    it('returns the sudo systemctl restart instruction for systemd', () => {
      expect(getSecurityAuthRestartInstruction('systemd')).toEqual({
        label: 'Restart the service:',
        command: 'sudo systemctl restart pulse',
      });
    });

    it('returns the same sudo systemctl instruction for manual', () => {
      const manual = getSecurityAuthRestartInstruction('manual');
      const systemd = getSecurityAuthRestartInstruction('systemd');

      // Source declares `case 'systemd': case 'manual':` as a shared
      // fallthrough, so both inputs must produce identical output.
      expect(manual).toEqual(systemd);
      expect(manual).toEqual({
        label: 'Restart the service:',
        command: 'sudo systemctl restart pulse',
      });
      // The shared arm never exposes a secondaryLabel.
      expect(manual.secondaryLabel).toBeUndefined();
    });

    it('routes systemd/manual to the sudo-prefixed command, not the dev command', () => {
      // Confirms the systemd/manual branch is distinct from the
      // `development` arm which uses `npm run dev:restart`.
      expect(getSecurityAuthRestartInstruction('systemd').command).toBe(
        'sudo systemctl restart pulse',
      );
      expect(getSecurityAuthRestartInstruction('manual').command).not.toBe(
        'npm run dev:restart',
      );
    });
  });

  describe('default arm — unrecognized deployment types', () => {
    it('returns the generic fallback for an unknown non-empty deployment type', () => {
      const result = getSecurityAuthRestartInstruction('kubernetes');
      expect(result).toEqual({
        label: 'Restart Pulse using your deployment method',
      });
      expect(result.command).toBeUndefined();
      expect(result.secondaryLabel).toBeUndefined();
    });

    it('returns the generic fallback for another unknown deployment type', () => {
      expect(getSecurityAuthRestartInstruction('podman')).toEqual({
        label: 'Restart Pulse using your deployment method',
      });
    });

    it('is case-sensitive: "DOCKER" does not match the docker case', () => {
      // The switch uses strict string equality, so any case variant of a
      // known deployment type must fall through to default.
      const upper = getSecurityAuthRestartInstruction('DOCKER');
      expect(upper).toEqual({
        label: 'Restart Pulse using your deployment method',
      });
      expect(upper.command).toBeUndefined();

      // Same expectation for mixed-case / Systemd variants.
      expect(getSecurityAuthRestartInstruction('Systemd')).toEqual({
        label: 'Restart Pulse using your deployment method',
      });
    });

    it('falls through to default for an empty string', () => {
      // The empty string matches no `case` clause, so it hits default.
      const result = getSecurityAuthRestartInstruction('');
      expect(result.label).toBe('Restart Pulse using your deployment method');
      expect(result.command).toBeUndefined();
    });

    it('falls through to default for null (deliberately malformed input)', () => {
      const result = getSecurityAuthRestartInstruction(
        null as unknown as DeploymentTypeParam,
      );
      expect(result).toEqual({
        label: 'Restart Pulse using your deployment method',
      });
      expect(result.command).toBeUndefined();
      expect(result.secondaryLabel).toBeUndefined();
    });

    it('falls through to default for a non-string runtime value (deliberately malformed)', () => {
      const result = getSecurityAuthRestartInstruction(
        42 as unknown as DeploymentTypeParam,
      );
      expect(result).toEqual({
        label: 'Restart Pulse using your deployment method',
      });
    });
  });
});
