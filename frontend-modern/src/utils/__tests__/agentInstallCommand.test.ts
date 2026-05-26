import { describe, expect, it } from 'vitest';
import {
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
    expect(command).toContain('printf %s \'token-123\' > "$token_file"');
    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain('--insecure');
  });

  it('shell-quotes canonical URL and token-file bootstrap transport', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: "https://pulse.example/base path/agent's",
      token: "tok'en",
    });

    expect(command).toContain(
      "curl -fsSL 'https://pulse.example/base path/agent'\"'\"'s/install.sh' -o \"$install_script\"",
    );
    expect(command).toContain("--url 'https://pulse.example/base path/agent'\"'\"'s'");
    expect(command).toContain("printf %s 'tok'\"'\"'en' > \"$token_file\"");
    expect(command).toContain('--token-file "$token_file"');
    expect(command).not.toContain("--token 'tok");
  });

  it('runs a non-root preflight before privilege escalation for Unix installs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
    });

    const preflightIndex = command.indexOf('--preflight-only');
    const sudoIndex = command.indexOf('sudo bash "$install_script"');

    expect(command).toContain('tmp_dir=$(mktemp -d)');
    expect(command).toContain('trap \'rm -rf "$tmp_dir"\' EXIT');
    expect(command).toContain('bash "$install_script" --url');
    expect(command).toContain('--output json');
    expect(command).toContain('--non-interactive');
    expect(preflightIndex).toBeGreaterThan(-1);
    expect(sudoIndex).toBeGreaterThan(preflightIndex);
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

    expect(command).toContain(
      "curl -fsSL --cacert '/etc/pulse/custom-ca.pem' 'https://pulse.example/install.sh' -o \"$install_script\"",
    );
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain("--cacert '/etc/pulse/custom-ca.pem'");
  });

  it('preserves explicit insecure transport for self-signed https installs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      insecure: true,
    });

    expect(command).toContain(
      'curl -kfsSL \'https://pulse.example/install.sh\' -o "$install_script"',
    );
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain('--insecure');
  });

  it('omits token transport entirely when optional auth uses tokenless install commands', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
    });

    expect(command).toContain(
      'curl -fsSL \'https://pulse.example/install.sh\' -o "$install_script"',
    );
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).not.toContain('--token');
    expect(command).not.toContain('token_file=');
  });

  it('builds shared Windows install transport with token, insecure TLS, and custom CA continuity', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example/base/',
      token: 'token-123',
      insecure: true,
      caCertPath: 'C:\\Pulse\\custom-ca.cer',
    });

    expect(command).toContain(
      '$pulseTmp=Join-Path ([System.IO.Path]::GetTempPath()) ("pulse-agent-install-"+[System.Guid]::NewGuid().ToString("N"))',
    );
    expect(command).toContain('$pulseScriptUrl="https://pulse.example/base/install.ps1"');
    expect(command).toContain(
      '[System.IO.File]::WriteAllText($pulseTokenFile, "token-123", [System.Text.Encoding]::ASCII)',
    );
    expect(command).toContain('-TokenFile $pulseTokenFile');
    expect(command).toContain('-PreflightOnly $true');
    expect(command).toContain('-Output "json"');
    expect(command).toContain('-NonInteractive $true');
    expect(command).toContain('-Insecure $true');
    expect(command).toContain('-CACertPath "C:\\Pulse\\custom-ca.cer"');
    expect(command).toContain('Invoke-WebRequest -Uri $pulseScriptUrl -UseBasicParsing -OutFile $pulseInstallScript');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
  });

  it('supports tokenless shared Windows install transport for optional auth', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
    });

    expect(command).toContain('$pulseScriptUrl="https://pulse.example/install.ps1"');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
    expect(command).not.toContain('-TokenFile $pulseTokenFile');
    expect(command).toContain('-PreflightOnly $true');
    expect(command).toContain('-NonInteractive $true');
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

    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain('--enable-docker \\\n    --disable-host \\\n    --enable-commands');
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

    expect(command).not.toContain('$env:PULSE_TOKEN="token-123"');
    expect(command).toContain('-TokenFile $pulseTokenFile');
    expect(command).toContain('$env:PULSE_ENABLE_PROXMOX="true"');
    expect(command).toContain('$env:PULSE_PROXMOX_TYPE="pbs"');
    expect(command).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
  });

  it('passes insecure runtime continuity for plain-http Windows installs', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'http://pulse.example:7655',
      token: 'token-123',
    });

    expect(command).toContain('-Url "http://pulse.example:7655"');
    expect(command).toContain('-Insecure $true');
    expect(command).toContain('-PreflightOnly $true');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
  });
});
