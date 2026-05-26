const shellQuoteArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`;
export const powerShellQuote = (value: string) =>
  value.replace(/`/g, '``').replace(/"/g, '`"').replace(/\$/g, '`$');

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
  const normalizedExtraArgs = extraArgs.map((arg) => arg.trim()).filter((arg) => arg.length > 0);
  const installRequiresInsecure = insecure || normalizedBaseUrl.startsWith('http://');
  const curlFlags = insecure ? '-kfsSL' : '-fsSL';
  const caCertArg = normalizedCaCertPath
    ? ` \\\n    --cacert ${shellQuoteArg(normalizedCaCertPath)}`
    : '';
  const insecureArg = installRequiresInsecure ? ` \\\n    --insecure` : '';
  const preflightArgs = [
    `--url ${shellQuoteArg(normalizedBaseUrl)}`,
    '--preflight-only',
    '--output json',
    '--non-interactive',
  ].join(' \\\n    ');
  const installArgs = [
    `--url ${shellQuoteArg(normalizedBaseUrl)}`,
    ...(normalizedToken ? ['--token-file "$token_file"'] : []),
    ...normalizedExtraArgs,
    '--non-interactive',
  ].join(' \\\n    ');

  return `(
  set -e
  tmp_dir=$(mktemp -d)
  install_script="$tmp_dir/install.sh"
  trap 'rm -rf "$tmp_dir"' EXIT
  curl ${curlFlags}${normalizedCaCertPath ? ` --cacert ${shellQuoteArg(normalizedCaCertPath)}` : ''} ${shellQuoteArg(`${normalizedBaseUrl}/install.sh`)} -o "$install_script"
  chmod +x "$install_script"${
    normalizedToken
      ? `
  token_file="$tmp_dir/token"
  umask 077
  printf %s ${shellQuoteArg(normalizedToken)} > "$token_file"`
      : ''
  }
  bash "$install_script" ${preflightArgs}${caCertArg}${insecureArg}
  if [ "$(id -u)" -eq 0 ]; then
    bash "$install_script" ${installArgs}${caCertArg}${insecureArg}
  elif command -v sudo >/dev/null 2>&1; then
    sudo bash "$install_script" ${installArgs}${caCertArg}${insecureArg}
  else
    echo "Root privileges required. Run as root (su -) and retry." >&2
    exit 1
  fi
)`;
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
  const installRequiresInsecure = insecure || normalizedBaseUrl.startsWith('http://');
  const normalizedExtraEnvAssignments = extraEnvAssignments.filter(
    (assignment) => assignment.trim().length > 0,
  );
  const installerFetchRequiresCustomTrust = insecure || Boolean(normalizedCaCertPath);
  const scriptUrl = powerShellQuote(`${normalizedBaseUrl}/install.ps1`);
  const installArgs = [
    `-Url "${powerShellQuote(normalizedBaseUrl)}"`,
    ...(normalizedToken ? ['-TokenFile $pulseTokenFile'] : []),
    ...(installRequiresInsecure ? ['-Insecure $true'] : []),
    ...(normalizedCaCertPath ? [`-CACertPath "${powerShellQuote(normalizedCaCertPath)}"`] : []),
    '-NonInteractive $true',
  ];
  const preflightArgs = [...installArgs, '-PreflightOnly $true', '-Output "json"'];
  const customTrustFetch = installerFetchRequiresCustomTrust
    ? `$pulseCustomCa=$null; if (-not [string]::IsNullOrWhiteSpace($pulseCaCertPath)) { $pulseCustomCaBytes=[System.IO.File]::ReadAllBytes($pulseCaCertPath); $pulseCustomCaText=[System.Text.Encoding]::ASCII.GetString($pulseCustomCaBytes); if ($pulseCustomCaText.Contains("-----BEGIN CERTIFICATE-----")) { $pulseCustomCa=[System.Security.Cryptography.X509Certificates.X509Certificate2]::CreateFromPem($pulseCustomCaText) } else { $pulseCustomCa=[System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pulseCustomCaBytes) } }; $pulsePrev=[System.Net.ServicePointManager]::ServerCertificateValidationCallback; try { [System.Net.ServicePointManager]::ServerCertificateValidationCallback={ param($sender,$certificate,$chain,$sslPolicyErrors) if ($pulseAllowInsecure) { return $true }; if ($null -eq $pulseCustomCa) { return $sslPolicyErrors -eq [System.Net.Security.SslPolicyErrors]::None }; if ($null -eq $certificate) { return $false }; $pulseChain=[System.Security.Cryptography.X509Certificates.X509Chain]::new(); $pulseChain.ChainPolicy.RevocationMode=[System.Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck; $null=$pulseChain.ChainPolicy.ExtraStore.Add($pulseCustomCa); $null=$pulseChain.Build($certificate); foreach ($pulseElement in $pulseChain.ChainElements) { if ($pulseElement.Certificate.Thumbprint -eq $pulseCustomCa.Thumbprint) { return $true } }; return $false }; Invoke-WebRequest -Uri $pulseScriptUrl -UseBasicParsing -OutFile $pulseInstallScript } finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback=$pulsePrev }`
    : `Invoke-WebRequest -Uri $pulseScriptUrl -UseBasicParsing -OutFile $pulseInstallScript`;

  const tokenBootstrap = normalizedToken
    ? `$pulseTokenFile=Join-Path $pulseTmp "token"; [System.IO.File]::WriteAllText($pulseTokenFile, "${powerShellQuote(normalizedToken)}", [System.Text.Encoding]::ASCII); `
    : '';
  const extraEnvBootstrap = normalizedExtraEnvAssignments.length
    ? `${normalizedExtraEnvAssignments.join('; ')}; `
    : '';

  return (
    `& { $ErrorActionPreference="Stop"; ` +
    `$pulseTmp=Join-Path ([System.IO.Path]::GetTempPath()) ("pulse-agent-install-"+[System.Guid]::NewGuid().ToString("N")); ` +
    `New-Item -ItemType Directory -Force -Path $pulseTmp | Out-Null; ` +
    `$pulseInstallScript=Join-Path $pulseTmp "install.ps1"; ` +
    `$pulseScriptUrl="${scriptUrl}"; ` +
    `$pulseAllowInsecure=${insecure ? '$true' : '$false'}; ` +
    `$pulseCaCertPath="${powerShellQuote(normalizedCaCertPath)}"; ` +
    `try { ` +
    `${customTrustFetch}; ` +
    `${tokenBootstrap}` +
    `${extraEnvBootstrap}` +
    `$pulsePowerShell=(Get-Process -Id $PID).Path; ` +
    `& $pulsePowerShell -NoProfile -ExecutionPolicy Bypass -File $pulseInstallScript ${preflightArgs.join(' ')}; ` +
    `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; ` +
    `& $pulsePowerShell -NoProfile -ExecutionPolicy Bypass -File $pulseInstallScript ${installArgs.join(' ')}; ` +
    `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE } ` +
    `} finally { Remove-Item -LiteralPath $pulseTmp -Recurse -Force -ErrorAction SilentlyContinue } }`
  );
};
