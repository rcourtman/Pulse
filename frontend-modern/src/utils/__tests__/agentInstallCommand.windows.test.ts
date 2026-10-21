import { spawn } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer, type Server } from 'node:https';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';

import {
  buildPowerShellInstallScriptBootstrap,
  buildWindowsAgentInstallCommand,
  buildWindowsAgentUninstallCommand,
} from '@/utils/agentInstallCommand';

const powerShellRuntime = process.platform === 'win32' ? 'powershell.exe' : undefined;

const installerScript = `
param([string]$Url, [string]$TokenFile)
$phase = if ($env:PULSE_PREFLIGHT_ONLY -eq "true") { "preflight" } else { "install" }
Add-Content -LiteralPath $env:PULSE_TEST_MARKER -Value $phase
`.trim();

// A recorder, not the production installer: execute the copied producer over
// real HTTPS without touching SCM, agent state, credentials or the trust store.
const uninstallRecorder = `
param(
  [bool]$Uninstall,
  [string]$Url = $env:PULSE_URL,
  [string]$Token = $env:PULSE_TOKEN,
  [string]$TokenFile = $env:PULSE_TOKEN_FILE,
  [string]$AgentId = $env:PULSE_AGENT_ID,
  [string]$Hostname = $env:PULSE_HOSTNAME,
  [bool]$Insecure,
  [string]$CACertPath,
  [string]$ServerFingerprint,
  [bool]$PreflightOnly,
  [string]$Output,
  [bool]$NonInteractive
)
if (-not $Uninstall -or -not $NonInteractive -or $PreflightOnly) {
  throw 'Expected one noninteractive removal, not installation or preflight.'
}
[ordered]@{
  url = $Url; token = $Token; tokenFile = $TokenFile;
  agentId = $AgentId; hostname = $Hostname;
  insecure = $Insecure; ca = $CACertPath; fingerprint = $ServerFingerprint
} | ConvertTo-Json -Compress | Set-Content -LiteralPath $env:PULSE_TEST_MARKER -Encoding ascii
`.trim();

const runPowerShell = (command: string, env: NodeJS.ProcessEnv) =>
  new Promise<{ stdout: string; stderr: string }>((resolve, reject) => {
    const child = spawn(
      powerShellRuntime!,
      [
        '-NoLogo',
        '-NoProfile',
        '-NonInteractive',
        '-ExecutionPolicy',
        'Bypass',
        '-Command',
        command,
      ],
      { env: { ...process.env, ...env }, windowsHide: true, timeout: 25_000 },
    );
    let stdout = '';
    let stderr = '';
    child.stdout.setEncoding('utf8').on('data', (chunk) => (stdout += chunk));
    child.stderr.setEncoding('utf8').on('data', (chunk) => (stderr += chunk));
    child.on('error', reject);
    child.on('close', (code) => {
      if (code === 0) resolve({ stdout, stderr });
      else reject(new Error(`PowerShell exited ${code}.\nstdout:\n${stdout}\nstderr:\n${stderr}`));
    });
  });

describe.runIf(Boolean(powerShellRuntime))('Windows install command TLS runtime', () => {
  let server: Server | undefined;
  let baseUrl: string;
  let certificate: Buffer;
  let suiteDirectory: string;
  let testDirectory: string;
  let markerPath: string;
  let certificatePath: string;
  let requests: string[] = [];

  beforeAll(async () => {
    suiteDirectory = await mkdtemp(join(tmpdir(), 'pulse-windows-tls-suite-'));
    const pfxPath = join(suiteDirectory, 'server.pfx');
    const derPath = join(suiteDirectory, 'server.cer');
    const quote = (value: string) => `'${value.replace(/'/g, "''")}'`;
    await runPowerShell(
      `$ErrorActionPreference="Stop"; ` +
        `if ($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5 -or $PSVersionTable.PSVersion.Minor -ne 1) { throw 'Windows PowerShell 5.1 required' }; ` +
        `$rsa=[System.Security.Cryptography.RSACryptoServiceProvider]::new(2048); ` +
        `try { ` +
        `$request=[System.Security.Cryptography.X509Certificates.CertificateRequest]::new(` +
        `"CN=localhost", $rsa, [System.Security.Cryptography.HashAlgorithmName]::SHA256, ` +
        `[System.Security.Cryptography.RSASignaturePadding]::Pkcs1); ` +
        // DER SubjectAlternativeName containing DNS:localhost. Building the
        // extension directly keeps this fixture compatible with .NET Framework.
        `[byte[]]$subjectAlternativeName=0x30,0x0b,0x82,0x09,0x6c,0x6f,0x63,0x61,0x6c,0x68,0x6f,0x73,0x74; ` +
        `$request.CertificateExtensions.Add(` +
        `[System.Security.Cryptography.X509Certificates.X509Extension]::new(` +
        `"2.5.29.17", $subjectAlternativeName, $false)); ` +
        `$certificate=$request.CreateSelfSigned(` +
        `[System.DateTimeOffset]::UtcNow.AddMinutes(-5), ` +
        `[System.DateTimeOffset]::UtcNow.AddDays(1)); ` +
        `try { ` +
        `[System.IO.File]::WriteAllBytes(${quote(pfxPath)}, ` +
        `$certificate.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, "pulse-test")); ` +
        `[System.IO.File]::WriteAllBytes(${quote(derPath)}, ` +
        `$certificate.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Cert)) ` +
        `} finally { $certificate.Dispose() } ` +
        `} finally { $rsa.Dispose() }`,
      {},
    );
    const [pfx, der] = await Promise.all([readFile(pfxPath), readFile(derPath)]);
    const lines = der.toString('base64').match(/.{1,64}/g) ?? [];
    certificate = Buffer.from(
      `-----BEGIN CERTIFICATE-----\n${lines.join('\n')}\n-----END CERTIFICATE-----\n`,
    );
    const testServer = createServer({ pfx, passphrase: 'pulse-test' }, (request, response) => {
      requests.push(request.url ?? '');
      if (request.url === '/refused/install.ps1') {
        response.writeHead(503).end('Synthetic installer unavailable');
        return;
      }
      if (request.url === '/uninstall/install.ps1') {
        response.writeHead(200, { 'content-type': 'text/plain' }).end(uninstallRecorder);
        return;
      }
      if (request.url !== '/install.ps1') {
        response.writeHead(404).end();
        return;
      }
      response.writeHead(200, { 'content-type': 'text/plain' }).end(installerScript);
    });
    server = testServer;
    await new Promise<void>((resolve) => testServer.listen(0, resolve));
    const address = testServer.address();
    if (!address || typeof address === 'string') throw new Error('HTTPS test server did not bind.');
    baseUrl = `https://localhost:${address.port}`;
  }, 30_000);

  beforeEach(async () => {
    requests = [];
    testDirectory = await mkdtemp(join(tmpdir(), 'pulse-windows-tls-'));
    markerPath = join(testDirectory, 'result.txt');
    certificatePath = join(testDirectory, 'server.crt');
    await writeFile(certificatePath, certificate);
  });

  afterEach(async () => {
    if (testDirectory) await rm(testDirectory, { recursive: true, force: true });
  });

  afterAll(async () => {
    if (server) {
      await new Promise<void>((resolve, reject) =>
        server!.close((error) => (error ? reject(error) : resolve())),
      );
    }
    if (suiteDirectory) await rm(suiteDirectory, { recursive: true, force: true });
  });

  it('executes the inline bootstrap over self-signed HTTPS without a callback runspace', async () => {
    await runPowerShell(buildPowerShellInstallScriptBootstrap(baseUrl), {
      PULSE_INSECURE_SKIP_VERIFY: 'true',
      PULSE_TEST_MARKER: markerPath,
    });

    await expect(readFile(markerPath, 'utf8')).resolves.toMatch(/install\r?\n/);
  }, 30_000);

  it('executes preflight and install through the insecure Windows command', async () => {
    await runPowerShell(buildWindowsAgentInstallCommand({ baseUrl, insecure: true }), {
      PULSE_TEST_MARKER: markerPath,
    });

    await expect(readFile(markerPath, 'utf8')).resolves.toMatch(/preflight\r?\ninstall\r?\n/);
  }, 30_000);

  it('executes preflight and install through the PEM custom-CA Windows command', async () => {
    await runPowerShell(buildWindowsAgentInstallCommand({ baseUrl, caCertPath: certificatePath }), {
      PULSE_TEST_MARKER: markerPath,
    });

    await expect(readFile(markerPath, 'utf8')).resolves.toMatch(/preflight\r?\ninstall\r?\n/);
  }, 30_000);

  it('executes copied removal with selected CA and identity, not ambient installation credentials', async () => {
    await runPowerShell(
      buildWindowsAgentUninstallCommand({
        baseUrl: `${baseUrl}/uninstall`,
        caCertPath: certificatePath,
        agentId: "agent'$()",
        hostname: "host'$name",
      }),
      {
        PULSE_TEST_MARKER: markerPath,
        PULSE_URL: 'https://wrong.invalid',
        PULSE_TOKEN: 'synthetic-ambient-install-token',
        PULSE_TOKEN_FILE: 'synthetic-wrong-token-file',
        PULSE_AGENT_ID: 'wrong-agent',
        PULSE_HOSTNAME: 'wrong-host',
      },
    );
    expect(JSON.parse(await readFile(markerPath, 'utf8'))).toEqual({
      url: '',
      token: '',
      tokenFile: '',
      agentId: "agent'$()",
      hostname: "host'$name",
      insecure: false,
      ca: certificatePath,
      fingerprint: '',
    });
    expect(requests).toEqual(['/uninstall/install.ps1']);
  }, 30_000);

  it.each(['Continue', 'Ignore'])(
    'makes an HTTPS 503 catchable under %s and restores the selected trust callback',
    async (preference) => {
      const command = buildWindowsAgentUninstallCommand({
        baseUrl: `${baseUrl}/refused`,
        caCertPath: certificatePath,
      });
      const result = await runPowerShell(
        `$ErrorActionPreference='${preference}'; ` +
          `$before=[System.Net.ServicePointManager]::ServerCertificateValidationCallback; ` +
          `try { ${command}; throw 'Download refusal was ignored' } catch { ` +
          `if ($_.Exception.Message -eq 'Download refusal was ignored') { throw }; ` +
          `if ([System.Net.ServicePointManager]::ServerCertificateValidationCallback -ne $before -or ` +
          `[PulseInstallerCertificateValidator]::CustomCa -ne $null) { throw 'Download trust was not restored' }; ` +
          `if ($ErrorActionPreference -ne '${preference}') { throw 'Caller error preference changed' }; ` +
          `Write-Output 'Expected download refusal with restored trust' }`,
        { PULSE_TEST_MARKER: markerPath },
      );
      expect(result.stdout).toContain('Expected download refusal with restored trust');
      await expect(readFile(markerPath)).rejects.toMatchObject({ code: 'ENOENT' });
      expect(requests).toEqual(['/refused/install.ps1']);
    },
    30_000,
  );

  it('does not execute copied removal when the HTTPS certificate is untrusted', async () => {
    await expect(
      runPowerShell(buildWindowsAgentUninstallCommand({ baseUrl: `${baseUrl}/uninstall` }), {
        PULSE_TEST_MARKER: markerPath,
      }),
    ).rejects.toThrow('PowerShell exited');
    await expect(readFile(markerPath)).rejects.toMatchObject({ code: 'ENOENT' });
    expect(requests).toEqual([]);
  }, 30_000);
});
