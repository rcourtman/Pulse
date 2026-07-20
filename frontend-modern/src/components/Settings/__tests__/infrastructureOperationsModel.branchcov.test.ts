import { describe, expect, it } from 'vitest';
import { getPowerShellInstallProfileEnvFromFlags } from '../infrastructureOperationsModel';

describe('getPowerShellInstallProfileEnvFromFlags', () => {
  it('returns an empty array for an empty flags list', () => {
    expect(getPowerShellInstallProfileEnvFromFlags([])).toEqual([]);
  });

  it('translates --enable-docker into the PULSE_ENABLE_DOCKER assignment', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--enable-docker'])).toEqual([
      `$env:PULSE_ENABLE_DOCKER="true"`,
    ]);
  });

  it('translates --disable-host into the PULSE_ENABLE_HOST=false assignment', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--disable-host'])).toEqual([
      `$env:PULSE_ENABLE_HOST="false"`,
    ]);
  });

  it('translates --enable-kubernetes into the PULSE_ENABLE_KUBERNETES assignment', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--enable-kubernetes'])).toEqual([
      `$env:PULSE_ENABLE_KUBERNETES="true"`,
    ]);
  });

  it('translates --enable-proxmox into the PULSE_ENABLE_PROXMOX assignment', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--enable-proxmox'])).toEqual([
      `$env:PULSE_ENABLE_PROXMOX="true"`,
    ]);
  });

  it('preserves flag order across a multi-flag profile (docker profile)', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--enable-docker', '--disable-host'])).toEqual([
      `$env:PULSE_ENABLE_DOCKER="true"`,
      `$env:PULSE_ENABLE_HOST="false"`,
    ]);
  });

  it('reads the proxmox type from the combined "--proxmox-type <value>" token (default branch)', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type pve'])).toEqual([
      `$env:PULSE_PROXMOX_TYPE="pve"`,
    ]);
  });

  it('trims surrounding whitespace from a combined proxmox-type value', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type    pbs   '])).toEqual([
      `$env:PULSE_PROXMOX_TYPE="pbs"`,
    ]);
  });

  it('ignores a combined "--proxmox-type " token with an empty value', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type '])).toEqual([]);
  });

  it('ignores a combined "--proxmox-type" token followed by only whitespace', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type    '])).toEqual([]);
  });

  it('reads the proxmox type from the split "--proxmox-type" then next-token form', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type', 'pve'])).toEqual([
      `$env:PULSE_PROXMOX_TYPE="pve"`,
    ]);
  });

  it('trims the next token in the split form', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type', '   pbs   '])).toEqual([
      `$env:PULSE_PROXMOX_TYPE="pbs"`,
    ]);
  });

  it('skips the proxmox-type assignment when "--proxmox-type" is the last token', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type'])).toEqual([]);
  });

  it('skips the proxmox-type assignment when the next token is an empty string', () => {
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type', ''])).toEqual([]);
  });

  it('skips the proxmox-type assignment when the next token is whitespace-only and does not consume it', () => {
    // The whitespace token is NOT consumed (index is only advanced inside the
    // truthy branch), so it is itself processed by the default arm on the next
    // iteration and also yields nothing.
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type', '   '])).toEqual([]);
  });

  it('consumes the next token as the proxmox type even when it resembles another flag', () => {
    // The split form greedily eats the following token as the type value,
    // so the flag-like "--enable-docker" is swallowed rather than processed.
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type', '--enable-docker'])).toEqual([
      `$env:PULSE_PROXMOX_TYPE="--enable-docker"`,
    ]);
  });

  it('ignores unknown flags entirely', () => {
    expect(
      getPowerShellInstallProfileEnvFromFlags(['--bogus', '--enable-kubernetes', 'who-knows']),
    ).toEqual([`$env:PULSE_ENABLE_KUBERNETES="true"`]);
  });

  it('produces both assignments for the proxmox-pve profile flags', () => {
    expect(
      getPowerShellInstallProfileEnvFromFlags(['--enable-proxmox', '--proxmox-type pve']),
    ).toEqual([`$env:PULSE_ENABLE_PROXMOX="true"`, `$env:PULSE_PROXMOX_TYPE="pve"`]);
  });

  it('produces both assignments for the proxmox-pbs profile flags', () => {
    expect(
      getPowerShellInstallProfileEnvFromFlags(['--enable-proxmox', '--proxmox-type pbs']),
    ).toEqual([`$env:PULSE_ENABLE_PROXMOX="true"`, `$env:PULSE_PROXMOX_TYPE="pbs"`]);
  });

  it('does not recognize the "--proxmox-type=<value>" equals form', () => {
    // Neither the exact `--proxmox-type` case nor the default
    // `startsWith('--proxmox-type ')` (space) arm matches the equals form,
    // so it silently yields no assignment.
    expect(getPowerShellInstallProfileEnvFromFlags(['--proxmox-type=pve'])).toEqual([]);
  });
});
