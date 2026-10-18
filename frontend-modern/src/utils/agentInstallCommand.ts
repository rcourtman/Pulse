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
  // Existing private file for non-terminal hosts; never contains credential bytes.
  tokenFilePath?: string | null;
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
  tokenFilePath,
}: BuildUnixAgentInstallCommandOptions) => {
  const normalizedBaseUrl = normalizeInstallerBaseUrl(baseUrl);
  if (!normalizedBaseUrl.trim()) {
    throw new Error('Pulse install endpoint URL is required.');
  }
  const normalizedCaCertPath = (caCertPath || '').trim();
  const normalizedExtraArgs = extraArgs.map((arg) => arg.trim()).filter((arg) => arg.length > 0);
  const uninstall = normalizedExtraArgs.includes('--uninstall');
  // Removal authenticates with the installed host's saved credential, never
  // with a newly generated installation token (or its private-input prompt).
  const needsToken = !uninstall && Boolean((token || '').trim() || tokenFilePath?.trim());
  const normalizedTokenFilePath = uninstall ? '' : (tokenFilePath || '').trim();
  if (normalizedTokenFilePath && !normalizedTokenFilePath.startsWith('/')) {
    throw new Error('Pulse token file path must be absolute.');
  }
  const installRequiresInsecure =
    insecure || (!uninstall && normalizedBaseUrl.startsWith('http://'));
  const curlFlags = insecure ? '-kfsSL' : '-fsSL';
  // Build shell grammar explicitly: command fields such as pfSense's remove
  // pasted newlines. Never flatten a multiline script (or quoted user data).
  if (
    [normalizedBaseUrl, normalizedTokenFilePath, normalizedCaCertPath, ...normalizedExtraArgs].some(
      (value) => /[\r\n]/.test(value),
    )
  ) {
    throw new Error('Pulse install command values must not contain line breaks.');
  }
  const caCertArg = normalizedCaCertPath ? ` --cacert ${shellQuoteArg(normalizedCaCertPath)}` : '';
  const insecureArg = installRequiresInsecure ? ' --insecure' : '';
  const preflightArgs = [
    `--url ${shellQuoteArg(normalizedBaseUrl)}`,
    '--preflight-only',
    '--output json',
    '--non-interactive',
  ].join(' ');
  const installArgs = [
    ...(!uninstall ? [`--url ${shellQuoteArg(normalizedBaseUrl)}`] : []),
    ...(needsToken ? ['--token-file "$token_file"'] : []),
    ...normalizedExtraArgs,
    '--non-interactive',
  ].join(' ');

  // Keep this reader aligned with configapi/privateBootstrapCommand. Bash owns
  // the silent bounded read in the privileged child, not the caller's shell or
  // sudo argv. A canonical line read silently truncates long pasted tokens.
  const privileged = ['set +xv;', 'set -eu;', 'umask 077;'];
  if (uninstall) {
    // A previous pasted install must not shadow saved state. This runs only
    // in the privileged child; the caller's environment is left unchanged.
    privileged.push(
      'unset PULSE_TOKEN PULSE_TOKEN_FILE PULSE_URL PULSE_AGENT_ID PULSE_HOSTNAME PULSE_INSECURE_SKIP_VERIFY PULSE_CACERT PULSE_SERVER_FINGERPRINT;',
    );
  }
  if (normalizedTokenFilePath) {
    privileged.push(
      `token_file=${shellQuoteArg(normalizedTokenFilePath)};`,
      'token_parent=${token_file%/*};',
      'if [ ! -f "$token_file" ] || [ -L "$token_file" ] || [ ! -O "$token_file" ] || [ ! -d "$token_parent" ] || [ -L "$token_parent" ] || [ ! -O "$token_parent" ] || [ "$(stat -c %a "$token_file" 2>/dev/null || stat -f %Lp "$token_file")" != 600 ] || [ "$(stat -c %a "$token_parent" 2>/dev/null || stat -f %Lp "$token_parent")" != 700 ]; then echo "Use a root-owned 0600 regular token file in a root-owned 0700 directory. No installation was attempted." >&2; exit 1; fi;',
    );
  } else if (needsToken) {
    privileged.push(
      'token_dir=$(mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX);',
      'token_file="$token_dir/token";',
      'unset pulse_token pulse_discard tty_state;',
      'tty_control=();',
      'cleanup() { kill %1 2>/dev/null || :; wait 2>/dev/null || :; unset pulse_token pulse_discard; if [ -n "${tty_state:-}" ]; then "${tty_control[@]}" "$tty_state" </dev/tty 2>/dev/null || :; fi; rm -f -- "$token_file"; rmdir -- "$token_dir"; };',
      "trap cleanup EXIT; trap 'exit 129' HUP; trap 'exit 130' INT; trap 'exit 143' TERM;",
      // Some NAS hosts expose stty only as a BusyBox applet. Keep the selected
      // utility for both mode changes and cleanup; never fall back to echoed
      // input. Missing TTY, missing tool and failed mode changes are distinct.
      'if ! { : </dev/tty; } 2>/dev/null; then echo "A terminal is required to enter the token. Use a private token-file command instead. No installation was attempted." >&2; exit 1; fi;',
      'if command -v stty >/dev/null 2>&1; then tty_control=(stty); elif command -v busybox >/dev/null 2>&1; then tty_control=(busybox stty); else echo "Silent token input requires stty or BusyBox stty. Use a private token-file command instead. No installation was attempted." >&2; exit 1; fi;',
      'if ! tty_state=$("${tty_control[@]}" -g </dev/tty 2>/dev/null) || [ -z "$tty_state" ]; then echo "Cannot read terminal settings with stty or BusyBox stty. Use a private token-file command instead. No installation was attempted." >&2; exit 1; fi;',
      'if ! "${tty_control[@]}" -echo </dev/tty 2>/dev/null; then echo "Cannot disable terminal echo. No token was requested and no installation was attempted." >&2; exit 1; fi;',
      // Keep signal/EXIT cleanup outside Bash's terminal read. Its supervisor
      // waits, then reaps the private input child before restoring the tty.
      // Job 1 belongs to this fresh shell, including before wait has started.
      '( trap - EXIT HUP INT TERM;',
      `if ! IFS= read -r -s -p ${shellQuoteArg('Pulse agent token (paste at this prompt, not in the command): ')} -n 4097 pulse_token </dev/tty; then echo "Token input was interrupted. No installation was attempted." >&2; exit 1; fi;`,
      'if [ "${#pulse_token}" -gt 4096 ]; then while IFS= read -r -s -n 4097 pulse_discard </dev/tty && [ "${#pulse_discard}" -eq 4097 ]; do :; done; unset pulse_discard; fi;',
      'if [ -z "$pulse_token" ] || [ "${#pulse_token}" -gt 4096 ]; then echo "A non-empty token of at most 4096 characters is required." >&2; exit 1; fi;',
      'printf %s "$pulse_token" > "$token_file"; unset pulse_token;',
      ') & wait "$!";',
      'if ! "${tty_control[@]}" "$tty_state" </dev/tty 2>/dev/null; then echo "Cannot restore terminal settings. No installation was attempted." >&2; exit 1; fi; unset tty_state;',
      "printf '\\n' >/dev/tty;",
    );
  }
  privileged.push(`bash "$1" ${installArgs}${caCertArg}${insecureArg};`);
  const child = `bash -c ${shellQuoteArg(privileged.join(' '))} pulse-bootstrap "$install_script";`;

  return [
    '(',
    'set +xv;',
    'set -eu;',
    'umask 077;',
    'if [ "$(id -u)" -eq 0 ]; then :; elif command -v sudo >/dev/null 2>&1; then sudo -v; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi;',
    'bootstrap_dir=$(mktemp -d /tmp/pulse-bootstrap.XXXXXX);',
    'install_script="$bootstrap_dir/install.sh";',
    'cleanup() { rm -f -- "$install_script"; rmdir -- "$bootstrap_dir"; };',
    "trap cleanup EXIT; trap 'exit 129' HUP; trap 'exit 130' INT; trap 'exit 143' TERM;",
    `curl ${curlFlags}${caCertArg} ${shellQuoteArg(`${normalizedBaseUrl}/install.sh`)} -o "$install_script";`,
    // Detachment must not require a downloadable new agent binary.
    ...(!uninstall ? [`bash "$install_script" ${preflightArgs}${caCertArg}${insecureArg};`] : []),
    // Expand the root/sudo prefix once. Repeating the entire quoted reader
    // makes a normal pasted command exceed some terminals' canonical line
    // limit. Positional arguments live only inside this isolated subshell.
    'if [ "$(id -u)" -eq 0 ]; then set --; else set -- sudo; fi;',
    `"$@" ${child}`,
    ')',
  ].join(' ');
};

const buildPowerShellInstallerDownload = (
  baseUrl: string,
  insecure = '$env:PULSE_INSECURE_SKIP_VERIFY -eq "true"',
  caCertPath = '$env:PULSE_CACERT',
) => {
  const normalizedBaseUrl = normalizeInstallerBaseUrl(baseUrl);
  if (!normalizedBaseUrl.trim()) {
    throw new Error('Pulse install endpoint URL is required.');
  }
  const scriptUrl = powerShellQuote(`${normalizedBaseUrl}/install.ps1`);
  return (
    `& { $pulseScriptUrl="${scriptUrl}"; ` +
    `if (${insecure} -or -not [string]::IsNullOrWhiteSpace(${caCertPath})) { ` +
    `${powerShellTlsValidatorBootstrap}` +
    `$pulseCustomCa = $null; ` +
    `if (-not [string]::IsNullOrWhiteSpace(${caCertPath})) { ` +
    `${powerShellLoadCustomCa(caCertPath)}` +
    `}; ` +
    `$pulsePrev = [System.Net.ServicePointManager]::ServerCertificateValidationCallback; ` +
    `try { ` +
    `if (${insecure}) { ` +
    `[System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseInstallerCertificateValidator]::AcceptAnyCallback ` +
    `} else { ` +
    `[System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseInstallerCertificateValidator]::ValidateCustomCaCallback ` +
    `}; ` +
    `irm $pulseScriptUrl ` +
    `} finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $pulsePrev; [PulseInstallerCertificateValidator]::CustomCa = $null } ` +
    `} else { irm $pulseScriptUrl } }`
  );
};

export const buildPowerShellInstallScriptBootstrap = (baseUrl: string) =>
  `${buildPowerShellInstallerDownload(baseUrl)} | iex`;

export const buildWindowsAgentUninstallCommand = ({
  baseUrl,
  insecure = false,
  caCertPath,
  agentId = '',
  hostname = '',
}: {
  baseUrl: string;
  insecure?: boolean;
  caCertPath?: string | null;
  agentId?: string;
  hostname?: string;
}) => {
  const ca = powerShellSingleQuotedLiteral((caCertPath || '').trim());
  const allowInsecure = insecure ? '$true' : '$false';
  // Explicit empty parameters bypass ambient install credentials/identity and
  // let install.ps1 recover its saved connection, token and trust. Download
  // trust is separately bound to the selected endpoint, without changing env.
  const args = [
    '-Uninstall $true',
    "-Url '' -Token '' -TokenFile ''",
    `-AgentId ${powerShellSingleQuotedLiteral(agentId)}`,
    `-Hostname ${powerShellSingleQuotedLiteral(hostname)}`,
    `-Insecure ${allowInsecure} -CACertPath ${ca} -ServerFingerprint ''`,
    "-PreflightOnly $false -Output '' -NonInteractive $true",
  ];
  return `${buildPowerShellInstallerDownload(baseUrl, allowInsecure, ca)} | ForEach-Object { & ([scriptblock]::Create($_)) ${args.join(' ')} }`;
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
