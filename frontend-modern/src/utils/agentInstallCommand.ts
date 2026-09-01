const shellQuoteArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`;
export const powerShellQuote = (value: string) =>
  value.replace(/`/g, '``').replace(/"/g, '`"').replace(/\$/g, '`$');

const powerShellSingleQuotedLiteral = (value: string) => `'${value.replace(/'/g, "''")}'`;

// ServicePointManager can invoke its validation callback on a worker thread
// that has no PowerShell runspace. Windows PowerShell 5.1 therefore cannot
// safely use a scriptblock delegate here. Keep the callback and PEM parsing in
// a compiled .NET type that is compatible with both Windows PowerShell 5.1 and
// modern PowerShell runtimes.
const powerShellTlsValidatorSource = `
using System;
using System.IO;
using System.Net.Security;
using System.Security.Cryptography.X509Certificates;
using System.Text.RegularExpressions;

public static class PulseInstallerCertificateValidator
{
    public static X509Certificate2 CustomCa;
    public static readonly RemoteCertificateValidationCallback AcceptAnyCallback = AcceptAny;
    public static readonly RemoteCertificateValidationCallback ValidateCustomCaCallback = ValidateCustomCa;

    public static X509Certificate2 LoadCertificate(string path)
    {
        byte[] bytes = File.ReadAllBytes(path);
        string text = System.Text.Encoding.ASCII.GetString(bytes);
        if (text.Contains("-----BEGIN CERTIFICATE-----"))
        {
            string base64 = Regex.Replace(text, "-----BEGIN CERTIFICATE-----|-----END CERTIFICATE-----|\\\\s", "");
            bytes = Convert.FromBase64String(base64);
        }
        return (X509Certificate2)Activator.CreateInstance(typeof(X509Certificate2), new object[] { bytes });
    }

    private static bool AcceptAny(object sender, X509Certificate certificate, X509Chain chain, SslPolicyErrors errors)
    {
        return true;
    }

    private static bool ValidateCustomCa(object sender, X509Certificate certificate, X509Chain chain, SslPolicyErrors errors)
    {
        if (certificate == null || CustomCa == null ||
            (errors & (SslPolicyErrors.RemoteCertificateNameMismatch | SslPolicyErrors.RemoteCertificateNotAvailable)) != 0)
        {
            return false;
        }

        using (X509Chain candidateChain = new X509Chain())
        {
            candidateChain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
            candidateChain.ChainPolicy.ExtraStore.Add(CustomCa);
            candidateChain.Build(new X509Certificate2(certificate));
            foreach (X509ChainElement element in candidateChain.ChainElements)
            {
                if (String.Equals(element.Certificate.Thumbprint, CustomCa.Thumbprint, StringComparison.OrdinalIgnoreCase))
                {
                    return true;
                }
            }
        }
        return false;
    }
}`.trim();

// The copied command is pasted into a console, and console hosts execute
// pasted input line by line, so any literal newline inside the command breaks
// it in both Windows PowerShell 5.1 and PowerShell 7. C# is whitespace
// insensitive (and the source carries no line comments), so the type
// definition collapses onto the command's single line.
const powerShellTlsValidatorInlineSource = powerShellTlsValidatorSource.replace(
  /\s*\r?\n\s*/g,
  ' ',
);

const powerShellTlsValidatorBootstrap =
  `if ($null -eq ("PulseInstallerCertificateValidator" -as [type])) { ` +
  `Add-Type -TypeDefinition ${powerShellSingleQuotedLiteral(powerShellTlsValidatorInlineSource)} ` +
  `}; `;

const powerShellLoadCustomCa = (pathExpression: string) =>
  `$pulseCustomCa=[PulseInstallerCertificateValidator]::LoadCertificate(${pathExpression}); ` +
  `[PulseInstallerCertificateValidator]::CustomCa=$pulseCustomCa; `;

export const normalizeInstallerBaseUrl = (baseUrl: string) => baseUrl.replace(/\/+$/, '');

export type AgentCommandPlatform = 'linux' | 'macos' | 'freebsd' | 'windows';

// Legacy agents report gopsutil's host.Info().Platform verbatim — a
// descriptive OS caption such as "microsoft windows 11 pro" — so matching
// must tolerate captions, not just exact tokens (refs #1555). Mirrors the
// backend's platformsupport.AgentCommandPlatform; unmatched values are Linux
// distro names, for which the shell installer is correct.
export const resolveAgentCommandPlatform = (platform?: string | null): AgentCommandPlatform => {
  const normalized = platform?.trim().toLowerCase() ?? '';
  if (normalized.includes('windows')) return 'windows';
  if (
    normalized === 'darwin' ||
    normalized === 'mac' ||
    normalized === 'macos' ||
    normalized.includes('mac os') ||
    normalized.includes('os x')
  ) {
    return 'macos';
  }
  if (normalized.includes('freebsd')) return 'freebsd';
  return 'linux';
};

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
  const rootTokenSetup = normalizedToken
    ? `    token_dir=$(mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX)
    token_file="$token_dir/token"
    umask 077
    printf %s ${shellQuoteArg(normalizedToken)} > "$token_file"
`
    : '';
  const sudoTokenSetup = normalizedToken
    ? `    token_dir=$(sudo mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX)
    token_file="$token_dir/token"
    printf %s ${shellQuoteArg(normalizedToken)} | sudo tee "$token_file" >/dev/null
    sudo chmod 0600 "$token_file"
`
    : '';

  return `(
  set -e
  tmp_dir=$(mktemp -d)
  token_dir=""
  install_script="$tmp_dir/install.sh"
  cleanup() {
    rm -rf -- "$tmp_dir"
    if [ -n "${'${token_dir:-}'}" ]; then
      if [ "$(id -u)" -eq 0 ]; then
        rm -rf -- "$token_dir"
      elif command -v sudo >/dev/null 2>&1; then
        sudo rm -rf -- "$token_dir" >/dev/null 2>&1 || true
      fi
    fi
  }
  trap cleanup EXIT HUP INT TERM
  curl ${curlFlags}${normalizedCaCertPath ? ` --cacert ${shellQuoteArg(normalizedCaCertPath)}` : ''} ${shellQuoteArg(`${normalizedBaseUrl}/install.sh`)} -o "$install_script"
  chmod +x "$install_script"
  bash "$install_script" ${preflightArgs}${caCertArg}${insecureArg}
  if [ "$(id -u)" -eq 0 ]; then
${rootTokenSetup}    bash "$install_script" ${installArgs}${caCertArg}${insecureArg}
  elif command -v sudo >/dev/null 2>&1; then
${sudoTokenSetup}    sudo bash "$install_script" ${installArgs}${caCertArg}${insecureArg}
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
    `${powerShellTlsValidatorBootstrap}` +
    `$pulseCustomCa = $null; ` +
    `if (-not [string]::IsNullOrWhiteSpace($env:PULSE_CACERT)) { ` +
    `${powerShellLoadCustomCa('$env:PULSE_CACERT')}` +
    `}; ` +
    `$pulsePrev = [System.Net.ServicePointManager]::ServerCertificateValidationCallback; ` +
    `try { ` +
    `if ($env:PULSE_INSECURE_SKIP_VERIFY -eq "true") { ` +
    `[System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseInstallerCertificateValidator]::AcceptAnyCallback ` +
    `} else { ` +
    `[System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseInstallerCertificateValidator]::ValidateCustomCaCallback ` +
    `}; ` +
    `irm $pulseScriptUrl ` +
    `} finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $pulsePrev; [PulseInstallerCertificateValidator]::CustomCa = $null } ` +
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
  ];
  const certificateValidationCallback = insecure
    ? `[PulseInstallerCertificateValidator]::AcceptAnyCallback`
    : `[PulseInstallerCertificateValidator]::ValidateCustomCaCallback`;
  const customTrustFetch = installerFetchRequiresCustomTrust
    ? `${powerShellTlsValidatorBootstrap}$pulseCustomCa=$null; if (-not [string]::IsNullOrWhiteSpace($pulseCaCertPath)) { ${powerShellLoadCustomCa('$pulseCaCertPath')}}; $pulsePrev=[System.Net.ServicePointManager]::ServerCertificateValidationCallback; try { [System.Net.ServicePointManager]::ServerCertificateValidationCallback=${certificateValidationCallback}; Invoke-WebRequest -Uri $pulseScriptUrl -UseBasicParsing -OutFile $pulseInstallScript } finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback=$pulsePrev; [PulseInstallerCertificateValidator]::CustomCa=$null }`
    : `Invoke-WebRequest -Uri $pulseScriptUrl -UseBasicParsing -OutFile $pulseInstallScript`;

  const tokenBootstrap = normalizedToken
    ? `$pulseTokenFile=Join-Path $pulseTmp "token"; [System.IO.File]::WriteAllText($pulseTokenFile, "${powerShellQuote(normalizedToken)}", [System.Text.Encoding]::ASCII); `
    : '';
  const extraEnvBootstrap = normalizedExtraEnvAssignments.length
    ? `${normalizedExtraEnvAssignments.join('; ')}; `
    : '';
  const runtimeEnvBootstrap =
    `${installRequiresInsecure ? '$env:PULSE_INSECURE_SKIP_VERIFY="true"; ' : ''}` +
    `${normalizedCaCertPath ? `$env:PULSE_CACERT="${powerShellQuote(normalizedCaCertPath)}"; ` : ''}` +
    '$env:PULSE_NON_INTERACTIVE="true"; ';
  const installCommand = `& $pulsePowerShell -NoProfile -ExecutionPolicy Bypass -File $pulseInstallScript ${installArgs.join(' ')}`;

  return (
    `& { $ErrorActionPreference="Stop"; ` +
    `$pulseTmp=Join-Path ([System.IO.Path]::GetTempPath()) ("pulse-agent-install-"+[System.Guid]::NewGuid().ToString("N")); ` +
    `New-Item -ItemType Directory -Force -Path $pulseTmp | Out-Null; ` +
    `$pulseInstallScript=Join-Path $pulseTmp "install.ps1"; ` +
    `$pulseScriptUrl="${scriptUrl}"; ` +
    `$pulseCaCertPath="${powerShellQuote(normalizedCaCertPath)}"; ` +
    `try { ` +
    `${customTrustFetch}; ` +
    `${tokenBootstrap}` +
    `${extraEnvBootstrap}` +
    `${runtimeEnvBootstrap}` +
    `$pulsePowerShell=(Get-Process -Id $PID).Path; ` +
    `$env:PULSE_PREFLIGHT_ONLY="true"; $env:PULSE_OUTPUT="json"; ` +
    `${installCommand}; ` +
    `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; ` +
    `Remove-Item Env:PULSE_PREFLIGHT_ONLY -ErrorAction SilentlyContinue; Remove-Item Env:PULSE_OUTPUT -ErrorAction SilentlyContinue; ` +
    `${installCommand}; ` +
    `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE } ` +
    `} finally { Remove-Item -LiteralPath $pulseTmp -Recurse -Force -ErrorAction SilentlyContinue } }`
  );
};
