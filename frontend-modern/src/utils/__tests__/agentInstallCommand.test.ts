import { describe, expect, it } from 'vitest';
import {
  buildPowerShellInstallScriptBootstrap,
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
  normalizeInstallerBaseUrl,
  resolveInstallerBaseUrl,
} from '../agentInstallCommand';

describe('agentInstallCommand', () => {
  it('includes insecure transport continuity for plain-http Pulse URLs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'http://pulse.example:7655',
      token: 'token-123',
    });

    expect(command).toContain("--url 'http://pulse.example:7655'");
    expect(command).toContain("--token 'token-123'");
    expect(command).toContain('--insecure');
  });

  it('shell-quotes canonical URL and token transport', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: "https://pulse.example/base path/agent's",
      token: "tok'en",
    });

    expect(command).toContain("curl -fsSL 'https://pulse.example/base path/agent'\"'\"'s/install.sh'");
    expect(command).toContain("--url 'https://pulse.example/base path/agent'\"'\"'s'");
    expect(command).toContain("--token 'tok'\"'\"'en'");
  });

  it('normalizes trailing slashes before building installer transport', () => {
    expect(normalizeInstallerBaseUrl('https://pulse.example/base///')).toBe(
      'https://pulse.example/base',
    );

    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example/base/',
      token: 'token-123',
    });

    expect(command).toContain("curl -fsSL 'https://pulse.example/base/install.sh'");
    expect(command).toContain("--url 'https://pulse.example/base'");
    expect(command).not.toContain('//install.sh');
    expect(command).not.toContain("--url 'https://pulse.example/base/'");
  });

  it('falls back to the canonical endpoint when the custom override is only whitespace', () => {
    expect(resolveInstallerBaseUrl('   ', 'https://pulse.example/base/')).toBe(
      'https://pulse.example/base',
    );
  });

  it('preserves explicit custom CA transport for the first installer fetch and runtime', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      caCertPath: '/etc/pulse/custom-ca.pem',
    });

    expect(command).toContain("curl -fsSL --cacert '/etc/pulse/custom-ca.pem' 'https://pulse.example/install.sh'");
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).toContain("--token 'token-123'");
    expect(command).toContain("--cacert '/etc/pulse/custom-ca.pem'");
  });

  it('preserves explicit insecure transport for self-signed https installs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      insecure: true,
    });

    expect(command).toContain("curl -kfsSL 'https://pulse.example/install.sh'");
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).toContain("--token 'token-123'");
    expect(command).toContain('--insecure');
  });

  it('omits token transport entirely when optional auth uses tokenless install commands', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
    });

    expect(command).toContain("curl -fsSL 'https://pulse.example/install.sh'");
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).not.toContain('--token');
  });

  it('builds shared Windows install transport with token, insecure TLS, and custom CA continuity', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example/base/',
      token: 'token-123',
      insecure: true,
      caCertPath: 'C:\\Pulse\\custom-ca.cer',
    });

    expect(command).toContain('$env:PULSE_URL="https://pulse.example/base"');
    expect(command).toContain('$env:PULSE_TOKEN="token-123"');
    expect(command).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(command).toContain('$env:PULSE_CACERT="C:\\Pulse\\custom-ca.cer"');
    expect(command).toContain('$pulseScriptUrl="https://pulse.example/base/install.ps1"');
  });

  it('supports tokenless shared Windows install transport for optional auth', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
    });

    expect(command).toContain('$env:PULSE_URL="https://pulse.example"');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
    expect(command).toContain(buildPowerShellInstallScriptBootstrap('https://pulse.example'));
  });

  it('fails closed when the install endpoint URL is blank', () => {
    expect(() =>
      buildUnixAgentInstallCommand({
        baseUrl: '   ',
        token: 'token-123',
      }),
    ).toThrow('Pulse install endpoint URL is required.');

    expect(() =>
      buildWindowsAgentInstallCommand({
        baseUrl: '   ',
        token: 'token-123',
      }),
    ).toThrow('Pulse install endpoint URL is required.');
  });

  it('preserves extra installer args for shared Unix install transport', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      extraArgs: ['--enable-docker', '--disable-host', '--enable-commands'],
    });

    expect(command).toContain("--token 'token-123'");
    expect(command).toContain('--enable-docker --disable-host --enable-commands');
  });

  it('preserves extra env assignments for shared Windows install transport', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      extraEnvAssignments: [
        '$env:PULSE_ENABLE_PROXMOX="true"',
        '$env:PULSE_PROXMOX_TYPE="pbs"',
        '$env:PULSE_ENABLE_COMMANDS="true"',
      ],
    });

    expect(command).toContain('$env:PULSE_TOKEN="token-123"');
    expect(command).toContain('$env:PULSE_ENABLE_PROXMOX="true"');
    expect(command).toContain('$env:PULSE_PROXMOX_TYPE="pbs"');
    expect(command).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
  });
});
