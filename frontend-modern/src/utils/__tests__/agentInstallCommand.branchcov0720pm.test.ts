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

  it('loads the custom CA through the Windows PowerShell 5.1-compatible helper', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if (-not [string]::IsNullOrWhiteSpace($env:PULSE_CACERT)) {');
    expect(script).toContain(
      '[PulseInstallerCertificateValidator]::LoadCertificate($env:PULSE_CACERT)',
    );
  });

  it('decodes PEM certificates without the newer X509Certificate2::CreateFromPem API', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if (text.Contains("-----BEGIN CERTIFICATE-----"))');
    expect(script).toContain('bytes = Convert.FromBase64String(base64);');
    expect(script).not.toContain('CreateFromPem');
  });

  it('passes PEM-decoded or raw DER bytes into X509Certificate2', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      'Activator.CreateInstance(typeof(X509Certificate2), new object[] { bytes })',
    );
  });

  it('installs the compiled X509 chain-validation callback', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      '$pulsePrev = [System.Net.ServicePointManager]::ServerCertificateValidationCallback;',
    );
    expect(script).toContain(
      '[System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseInstallerCertificateValidator]::ValidateCustomCaCallback',
    );
    expect(script).toContain(
      '} finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $pulsePrev; [PulseInstallerCertificateValidator]::CustomCa = $null }',
    );
  });

  it('uses the compiled accept-any callback when PULSE_INSECURE_SKIP_VERIFY is "true"', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain(
      'if ($env:PULSE_INSECURE_SKIP_VERIFY -eq "true") { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseInstallerCertificateValidator]::AcceptAnyCallback',
    );
  });

  it('does not embed a PowerShell scriptblock callback', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).not.toContain('ServerCertificateValidationCallback = { param(');
    expect(script).not.toContain('GetNewClosure');
  });

  it('fails closed when the server supplied no certificate or no custom CA', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('if (certificate == null || CustomCa == null ||');
    expect(script).toContain('SslPolicyErrors.RemoteCertificateNameMismatch');
    expect(script).toContain('SslPolicyErrors.RemoteCertificateNotAvailable');
    expect(script).toContain('return false;');
  });

  it('builds an X509Chain with NoCheck revocation and the custom CA in ExtraStore', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('using (X509Chain candidateChain = new X509Chain())');
    expect(script).toContain(
      'candidateChain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;',
    );
    expect(script).toContain('candidateChain.ChainPolicy.ExtraStore.Add(CustomCa);');
    expect(script).toContain('candidateChain.Build(new X509Certificate2(certificate));');
  });

  it('walks ChainElements and trusts the chain when the custom CA Thumbprint matches', () => {
    const script = buildPowerShellInstallScriptBootstrap('https://pulse.example');
    expect(script).toContain('foreach (X509ChainElement element in candidateChain.ChainElements)');
    expect(script).toContain(
      'String.Equals(element.Certificate.Thumbprint, CustomCa.Thumbprint, StringComparison.OrdinalIgnoreCase)',
    );
    expect(script).toContain('return true;');
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
