import { spawn } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer, type Server } from 'node:https';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';

import {
  buildPowerShellInstallScriptBootstrap,
  buildWindowsAgentInstallCommand,
} from '@/utils/agentInstallCommand';

const powerShellRuntime = process.platform === 'win32' ? 'powershell.exe' : undefined;

const installerScript = `
param([string]$Url, [string]$TokenFile)
$phase = if ($env:PULSE_PREFLIGHT_ONLY -eq "true") { "preflight" } else { "install" }
Add-Content -LiteralPath $env:PULSE_TEST_MARKER -Value $phase
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
      { env: { ...process.env, ...env }, windowsHide: true },
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

  beforeAll(async () => {
    suiteDirectory = await mkdtemp(join(tmpdir(), 'pulse-windows-tls-suite-'));
    const pfxPath = join(suiteDirectory, 'server.pfx');
    const derPath = join(suiteDirectory, 'server.cer');
    const quote = (value: string) => `'${value.replace(/'/g, "''")}'`;
    await runPowerShell(
      `$ErrorActionPreference="Stop"; ` +
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
  });

  beforeEach(async () => {
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
});
