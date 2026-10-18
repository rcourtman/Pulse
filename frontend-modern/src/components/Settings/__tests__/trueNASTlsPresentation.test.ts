import { describe, expect, it } from 'vitest';
import { getTrueNASTlsPolicy } from '../trueNASTlsPresentation';

const secure = {
  host: 'nas.example.invalid',
  port: '443',
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: '',
};

describe('TrueNAS requested TLS policy', () => {
  it('requires the native chain and hostname checks without a pin', () => {
    expect(getTrueNASTlsPolicy(secure).title).toBe('Trusted certificate chain required');
  });
  it('requires both the pin and the native chain checks, unlike Proxmox pin precedence', () => {
    expect(getTrueNASTlsPolicy({ ...secure, fingerprint: 'AB:'.repeat(31) + 'AB' })).toMatchObject({
      title: 'Certificate chain and fingerprint required',
      tone: 'info',
    });
  });
  it('retains pin checks even when the chain checks are skipped', () => {
    const policy = getTrueNASTlsPolicy({
      ...secure,
      fingerprint: ' SHA256:' + 'ab '.repeat(32),
      insecureSkipVerify: true,
    });
    expect(policy.title).toBe('Certificate fingerprint required');
    expect(policy.description).toContain('hostname and expiry checks are skipped');
    expect(policy.description).toContain('pin is still required');
  });
  it.each(['', '   ', ' SHA256: : '])('does not claim pinning for a native-empty pin %j', (pin) => {
    expect(getTrueNASTlsPolicy({ ...secure, fingerprint: pin, insecureSkipVerify: true })).toMatchObject({
      title: 'Certificate identity not verified',
      tone: 'danger',
    });
  });
  it.each(['ab', 'zz'.repeat(32), 'ab\t'.repeat(32)])('does not call invalid pin %j verified', (pin) => {
    expect(getTrueNASTlsPolicy({ ...secure, fingerprint: pin }).title).toBe(
      'Certificate fingerprint is invalid',
    );
  });
  it.each([true, false])('warns about explicit HTTP regardless of the HTTPS checkbox (%s)', (useHttps) => {
    expect(getTrueNASTlsPolicy({ ...secure, host: '  HTTP://nas.example.invalid', useHttps })).toMatchObject({
      title: 'HTTP does not protect credentials',
      tone: 'danger',
    });
  });
  it('does not imply a pin can secure HTTP, including an invalid pin', () => {
    expect(getTrueNASTlsPolicy({ ...secure, useHttps: false, fingerprint: 'bad' }).title).toBe(
      'HTTP does not protect credentials',
    );
  });
  it.each(['', '   '])('matches the native HTTPS default with separate port %j', (port) => {
    expect(getTrueNASTlsPolicy({ ...secure, host: 'http://nas.example.invalid:80', useHttps: false, port }).title).toBe(
      'Trusted certificate chain required',
    );
  });
  it.each(['0', '-1', '65536', 'bad', '1.5'])('does not claim valid policy with port %j', (port) => {
    expect(getTrueNASTlsPolicy({ ...secure, port }).title).toBe('Connection port is invalid');
  });
  it('lets an explicit HTTPS scheme override an unchecked HTTPS checkbox', () => {
    expect(getTrueNASTlsPolicy({ ...secure, host: 'https://nas.example.invalid', useHttps: false }).title).toBe(
      'Trusted certificate chain required',
    );
  });
  it('does not claim supported transport for another URL scheme', () => {
    expect(getTrueNASTlsPolicy({ ...secure, host: 'wss://nas.example.invalid' }).title).toBe(
      'Endpoint scheme is not supported',
    );
  });
});
