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
        `if ($null -eq (Get-PSDrive -Name "Cert" -ErrorAction SilentlyContinue)) { ` +
        `New-PSDrive -Name "Cert" -PSProvider "Certificate" -Root "\\" | Out-Null ` +
        `}; ` +
        `$certificate=New-SelfSignedCertificate -DnsName "localhost" -CertStoreLocation "Cert:\\CurrentUser\\My" -KeyExportPolicy Exportable; ` +
        `try { ` +
        `$password=ConvertTo-SecureString "pulse-test" -AsPlainText -Force; ` +
        `Export-PfxCertificate -Cert $certificate -FilePath ${quote(pfxPath)} -Password $password | Out-Null; ` +
        `Export-Certificate -Cert $certificate -FilePath ${quote(derPath)} -Type CERT | Out-Null ` +
        `} finally { Remove-Item -LiteralPath ("Cert:\\CurrentUser\\My\\"+$certificate.Thumbprint) }`,
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
