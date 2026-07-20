import { describe, expect, it } from 'vitest';

import { buildPowerShellInstallScriptBootstrap } from '@/utils/agentInstallCommand';

// `normalizeInstallerBaseUrl` and `powerShellQuote` are module-private to the
// bootstrap builder's call graph; their branches are exercised transitively
// through the single exported entry point below, asserting on the observable
// PowerShell string that the builder emits.

describe('buildPowerShellInstallScriptBootstrap — URL validation branch coverage', () => {
  it('throws the required-URL error when baseUrl is the empty string', () => {
    expect(() => buildPowerShellInstallScriptBootstrap('')).toThrow(
      'Pulse install endpoint URL is required.',
    );
  });

  it('throws the required-URL error when baseUrl is whitespace-only', () => {
    // normalizeInstallerBaseUrl does not trim; only the post-normalize .trim()
    // guard catches this. Asserts the guard fires on whitespace-only input.
    expect(() => buildPowerShellInstallScriptBootstrap('   ')).toThrow(
      'Pulse install endpoint URL is required.',
    );
  });

  it('throws the required-URL error when baseUrl is only trailing slashes', () => {
    // normalizeInstallerBaseUrl strips the slashes -> '' -> trim() === '' -> throw.
    expect(() => buildPowerShellInstallScriptBootstrap('///')).toThrow(
      'Pulse install endpoint URL is required.',
    );
  });

  it('does NOT throw on the non-empty happy path (https URL)', () => {
    expect(() => buildPowerShellInstallScriptBootstrap('https://pulse.example')).not.toThrow();
  });

  it('does NOT throw on the non-empty happy path (plain-http URL)', () => {
    expect(() => buildPowerShellInstallScriptBootstrap('http://pulse.example:7655')).not.toThrow();
  });
});

describe('buildPowerShellInstallScriptBootstrap — baseUrl normalization branch coverage', () => {
  it('strips a single trailing slash before appending /install.ps1', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example/');
    expect(script).toContain('$pulseScriptUrl="https://pulse.example/install.ps1"');
    expect(script).not.toContain('https://pulse.example//install.ps1');
  });

  it('strips multiple trailing slashes before appending /install.ps1', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example/base///');
    expect(script).toContain('$pulseScriptUrl="https://pulse.example/base/install.ps1"');
    expect(script).not.toContain('https://pulse.example/base//install.ps1');
    expect(script).not.toContain('https://pulse.example/base///install.ps1');
  });

  it('preserves a baseUrl that already has no trailing slash (no-op normalization arm)', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example:7655');
    expect(script).toContain('$pulseScriptUrl="https://pulse.example:7655/install.ps1"');
  });

  it('does not rewrite the URL scheme (https stays https, http stays http)', () => {
    // The builder performs no scheme upgrade/downgrade — it only strips trailing slashes.
    const httpsScript = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    const httpScript = buildPowerShellInstallScriptBootstrap('http://pulse.example');
    expect(httpsScript).toContain('$pulseScriptUrl="https://pulse.example/install.ps1"');
    expect(httpScript).toContain('$pulseScriptUrl="http://pulse.example/install.ps1"');
  });
});

describe('buildPowerShellInstallScriptBootstrap — bootstrap script wiring', () => {
  it('emits the $pulseScriptUrl assignment as the first statement inside the script block', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script.startsWith('& { $pulseScriptUrl="https://pulse.example/install.ps1"; ')).toBe(
      true,
    );
  });

  it('gates the custom-CA / insecure branch on the PULSE_INSECURE_SKIP_VERIFY env var', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if ($env:PULSE_INSECURE_SKIP_VERIFY -eq "true"');
  });

  it('also gates the custom-CA / insecure branch on the PULSE_CACERT env var (OR arm)', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('-or -not [string]::IsNullOrWhiteSpace($env:PULSE_CACERT))');
  });

  it('reads the custom CA bytes from $env:PULSE_CACERT when populated', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if (-not [string]::IsNullOrWhiteSpace($env:PULSE_CACERT)) {');
    expect(script).toContain(
      '$pulseCustomCaBytes = [System.IO.File]::ReadAllBytes($env:PULSE_CACERT);',
    );
  });

  it('routes a PEM-encoded CA through X509Certificate2::CreateFromPem', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if ($pulseCustomCaText.Contains("-----BEGIN CERTIFICATE-----"))');
    expect(script).toContain(
      '$pulseCustomCa = [System.Security.Cryptography.X509Certificates.X509Certificate2]::CreateFromPem($pulseCustomCaText)',
    );
  });

  it('routes a DER-encoded CA through X509Certificate2::new (else arm)', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      '$pulseCustomCa = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pulseCustomCaBytes)',
    );
  });

  it('installs the X509 chain-validation ServerCertificateValidationCallback', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      '$pulsePrev = [System.Net.ServicePointManager]::ServerCertificateValidationCallback;',
    );
    expect(script).toContain(
      '[System.Net.ServicePointManager]::ServerCertificateValidationCallback = { param($sender, $certificate, $chain, $sslPolicyErrors)',
    );
    expect(script).toContain(
      '} finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $pulsePrev }',
    );
  });

  it('short-circuits the callback to $true when PULSE_INSECURE_SKIP_VERIFY is "true"', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if ($env:PULSE_INSECURE_SKIP_VERIFY -eq "true") { return $true };');
  });

  it('returns the raw sslPolicyErrors verdict when no custom CA was loaded', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      'if ($null -eq $pulseCustomCa) { return $sslPolicyErrors -eq [System.Net.Security.SslPolicyErrors]::None };',
    );
  });

  it('returns $false when the server supplied no certificate', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if ($null -eq $certificate) { return $false };');
  });

  it('builds an X509Chain with NoCheck revocation and the custom CA in ExtraStore', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      '$pulseChain = [System.Security.Cryptography.X509Certificates.X509Chain]::new();',
    );
    expect(script).toContain(
      '$pulseChain.ChainPolicy.RevocationMode = [System.Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck;',
    );
    expect(script).toContain('$null = $pulseChain.ChainPolicy.ExtraStore.Add($pulseCustomCa);');
    expect(script).toContain('$null = $pulseChain.Build($certificate);');
  });

  it('walks ChainElements and trusts the chain when the custom CA Thumbprint matches', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('foreach ($pulseElement in $pulseChain.ChainElements) {');
    expect(script).toContain(
      'if ($pulseElement.Certificate.Thumbprint -eq $pulseCustomCa.Thumbprint) { return $true }',
    );
    expect(script).toContain('return $false };');
  });

  it('fetches the script via `irm $pulseScriptUrl` inside both the custom-trust and the bare else arm', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    // custom-trust arm:
    expect(script).toContain('irm $pulseScriptUrl ');
    // bare else arm (no env-var set):
    expect(script).toContain('} else { irm $pulseScriptUrl } } | iex');
  });

  it('pipes the entire bootstrap through `| iex` so it is executed inline', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script.endsWith('} | iex')).toBe(true);
  });

  it('does not PowerShell-escape a plain https URL (no backticks/quotes injected)', () => {
    // powerShellQuote only transforms `, ", and $ — none appear in a bare URL,
    // so the scriptUrl should pass through verbatim.
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('$pulseScriptUrl="https://pulse.example/install.ps1"');
  });
});
