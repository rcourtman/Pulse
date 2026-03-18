const shellQuoteArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`;
export const powerShellQuote = (value: string) =>
  value.replace(/`/g, '``').replace(/"/g, '`"').replace(/\$/g, '`$');

const withPrivilegeEscalation = (command: string) => {
  if (!command.includes('| bash -s --')) return command;
  return command.replace(/\|\s*bash -s --([\s\S]*)$/, (_match, args: string) => {
    return `| { if [ "$(id -u)" -eq 0 ]; then bash -s --${args}; elif command -v sudo >/dev/null 2>&1; then sudo bash -s --${args}; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`;
  });
};

export const normalizeInstallerBaseUrl = (baseUrl: string) => baseUrl.replace(/\/+$/, '');

export const resolveInstallerBaseUrl = (customBaseUrl: string, fallbackBaseUrl: string) => {
  const normalizedCustomBaseUrl = normalizeInstallerBaseUrl(customBaseUrl.trim());
  if (normalizedCustomBaseUrl) {
    return normalizedCustomBaseUrl;
  }
  return normalizeInstallerBaseUrl(fallbackBaseUrl.trim());
};

type BuildUnixAgentInstallCommandOptions = {
  baseUrl: string;
  token?: string | null;
  insecure?: boolean;
  caCertPath?: string | null;
  extraArgs?: string[];
};

type BuildWindowsAgentInstallCommandOptions = {
  baseUrl: string;
  token?: string | null;
  insecure?: boolean;
  caCertPath?: string | null;
  extraEnvAssignments?: string[];
};

export const buildUnixAgentInstallCommand = ({
  baseUrl,
  token,
  insecure = false,
  caCertPath,
  extraArgs = [],
}: BuildUnixAgentInstallCommandOptions) => {
  const normalizedBaseUrl = normalizeInstallerBaseUrl(baseUrl);
  if (!normalizedBaseUrl.trim()) {
    throw new Error('Pulse install endpoint URL is required.');
  }
  const normalizedCaCertPath = (caCertPath || '').trim();
  const normalizedToken = (token || '').trim();
  const normalizedExtraArgs = extraArgs
    .map((arg) => arg.trim())
    .filter((arg) => arg.length > 0)
    .join(' ');
  const curlFlags = insecure ? ' -kfsSL' : ' -fsSL';
  let command =
    `curl${curlFlags}${normalizedCaCertPath ? ` --cacert ${shellQuoteArg(normalizedCaCertPath)}` : ''} ${shellQuoteArg(`${normalizedBaseUrl}/install.sh`)} | bash -s -- --url ${shellQuoteArg(normalizedBaseUrl)}` +
    `${normalizedToken ? ` --token ${shellQuoteArg(normalizedToken)}` : ''}`;

  if (normalizedExtraArgs) {
    command += ` ${normalizedExtraArgs}`;
  }

  if (normalizedCaCertPath) {
    command += ` --cacert ${shellQuoteArg(normalizedCaCertPath)}`;
  }

  if (insecure || normalizedBaseUrl.startsWith('http://')) {
    command += ' --insecure';
  }

  return withPrivilegeEscalation(command);
};

export const buildPowerShellInstallScriptBootstrap = (baseUrl: string) => {
  const normalizedBaseUrl = normalizeInstallerBaseUrl(baseUrl);
  if (!normalizedBaseUrl.trim()) {
    throw new Error('Pulse install endpoint URL is required.');
  }
  const scriptUrl = powerShellQuote(`${normalizedBaseUrl}/install.ps1`);
  return (
    `& { $pulseScriptUrl="${scriptUrl}"; ` +
    `if ($env:PULSE_INSECURE_SKIP_VERIFY -eq "true" -or -not [string]::IsNullOrWhiteSpace($env:PULSE_CACERT)) { ` +
    `$pulseCustomCa = $null; ` +
    `if (-not [string]::IsNullOrWhiteSpace($env:PULSE_CACERT)) { ` +
    `$pulseCustomCaBytes = [System.IO.File]::ReadAllBytes($env:PULSE_CACERT); ` +
    `$pulseCustomCaText = [System.Text.Encoding]::ASCII.GetString($pulseCustomCaBytes); ` +
    `if ($pulseCustomCaText.Contains("-----BEGIN CERTIFICATE-----")) { ` +
    `$pulseCustomCa = [System.Security.Cryptography.X509Certificates.X509Certificate2]::CreateFromPem($pulseCustomCaText) ` +
    `} else { ` +
    `$pulseCustomCa = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pulseCustomCaBytes) ` +
    `} ` +
    `}; ` +
    `$pulsePrev = [System.Net.ServicePointManager]::ServerCertificateValidationCallback; ` +
    `try { ` +
    `[System.Net.ServicePointManager]::ServerCertificateValidationCallback = { param($sender, $certificate, $chain, $sslPolicyErrors) ` +
    `if ($env:PULSE_INSECURE_SKIP_VERIFY -eq "true") { return $true }; ` +
    `if ($null -eq $pulseCustomCa) { return $sslPolicyErrors -eq [System.Net.Security.SslPolicyErrors]::None }; ` +
    `if ($null -eq $certificate) { return $false }; ` +
    `$pulseChain = [System.Security.Cryptography.X509Certificates.X509Chain]::new(); ` +
    `$pulseChain.ChainPolicy.RevocationMode = [System.Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck; ` +
    `$null = $pulseChain.ChainPolicy.ExtraStore.Add($pulseCustomCa); ` +
    `$null = $pulseChain.Build($certificate); ` +
    `foreach ($pulseElement in $pulseChain.ChainElements) { ` +
    `if ($pulseElement.Certificate.Thumbprint -eq $pulseCustomCa.Thumbprint) { return $true } ` +
    `}; ` +
    `return $false }; ` +
    `irm $pulseScriptUrl ` +
    `} finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $pulsePrev } ` +
    `} else { irm $pulseScriptUrl } } | iex`
  );
};

export const buildWindowsAgentInstallCommand = ({
  baseUrl,
  token,
  insecure = false,
  caCertPath,
  extraEnvAssignments = [],
}: BuildWindowsAgentInstallCommandOptions) => {
  const normalizedBaseUrl = normalizeInstallerBaseUrl(baseUrl);
  if (!normalizedBaseUrl.trim()) {
    throw new Error('Pulse install endpoint URL is required.');
  }
  const normalizedToken = (token || '').trim();
  const normalizedCaCertPath = (caCertPath || '').trim();
  const envAssignments = [`$env:PULSE_URL="${powerShellQuote(normalizedBaseUrl)}"`];

  if (normalizedToken) {
    envAssignments.push(`$env:PULSE_TOKEN="${powerShellQuote(normalizedToken)}"`);
  }
  if (insecure) {
    envAssignments.push('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
  }
  if (normalizedCaCertPath) {
    envAssignments.push(`$env:PULSE_CACERT="${powerShellQuote(normalizedCaCertPath)}"`);
  }
  envAssignments.push(...extraEnvAssignments.filter((assignment) => assignment.trim().length > 0));

  return `${envAssignments.join('; ')}; ${buildPowerShellInstallScriptBootstrap(normalizedBaseUrl)}`;
};
