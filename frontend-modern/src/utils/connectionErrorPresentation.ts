/**
 * connectionErrorPresentation
 *
 * Translates raw backend connection errors (often Go error chains exposing
 * implementation detail like 'context deadline exceeded' or
 * 'Client.Timeout exceeded while awaiting headers') into short user-facing
 * messages.
 *
 * The connection-table row already shows the connection name and host, so
 * the humanized message focuses on the failure mode and a short remediation
 * hint without restating identity.
 *
 * For unknown errors we fall back to the raw message stripped of the most
 * common implementation prefixes so it's at least readable.
 */

const POLL_PREFIX_PATTERN = /^poll_[a-z_]+\s+failed\s+on\s+[^:]+:\s*/i;
// Patterns from common Go error chains: `Get "https://...":`, `Post "..."`.
const HTTP_VERB_PATTERN = /\b(?:Get|Post|Put|Delete|Patch|Head)\s+"[^"]+":\s*/g;

interface HumanizedConnectionError {
  /** Short headline ('Connection timed out', 'Host not found', etc.). */
  headline: string;
  /** One-line remediation hint, or null when none applies. */
  hint: string | null;
}

const TIMEOUT_HINT =
  'Check the host is reachable, the port is correct, and the network path is open.';
const TLS_HINT =
  "Keep TLS verification enabled. Check the certificate's hostname, expiry and trusted chain. Independently verify its identity before adding or replacing a saved fingerprint.";
const AUTH_HINT =
  'Check the configured user, realm and token ID, and whether the credential has expired or been revoked. Replace it only if invalidity is confirmed.';
const DENIED_HINT =
  'Check read permissions for the intended resource and any proxy policy. Keep intentional access limits. Do not replace credentials or broaden access just to test.';

const HTTP_STATUS_PRESENTATIONS: Record<number, HumanizedConnectionError> = {
  401: { headline: 'Authentication failed', hint: AUTH_HINT },
  403: { headline: 'Permission denied', hint: DENIED_HINT },
  404: {
    headline: 'Endpoint not found',
    hint: 'Pulse reached the host but the API path is missing. Check the URL path or the platform version.',
  },
};
const SERVER_ERROR: HumanizedConnectionError = {
  headline: 'Server error',
  hint: 'The platform returned a server error. Check the platform logs for the underlying cause.',
};

// Use the first explicit response status, not a number in an endpoint or a
// quoted provider explanation. A 403/5xx mentioning authentication is not a 401.
const HTTP_STATUS_PATTERN =
  /\b(?:API error|HTTP(?: error| status(?: code)?)?|status(?: code)?)[\s:]+([45]\d{2})\b|^([45]\d{2})\b/i;

const HUMANIZED_PATTERNS: { match: RegExp; headline: string; hint: string | null }[] = [
  {
    match: /context deadline exceeded|Client\.Timeout|i\/o timeout|deadline exceeded/i,
    headline: 'Connection timed out',
    hint: TIMEOUT_HINT,
  },
  {
    match: /no such host|dns:.*lookup|name does not resolve/i,
    headline: 'Host not found',
    hint: 'Check the hostname or IP address.',
  },
  {
    match: /connection refused|connect:\s*connection refused/i,
    headline: 'Connection refused',
    hint: 'The host is reachable but rejected the connection on this port. Check the port is correct and the service is running.',
  },
  {
    match: /no route to host|network is unreachable/i,
    headline: 'No network route to host',
    hint: 'The host cannot be reached from this Pulse instance. Check VPN, firewall, or VLAN configuration.',
  },
  {
    match: /(?:certificate |tls (?:pinning failed: )?)?fingerprint (?:mismatch|changed)/i,
    headline: 'TLS fingerprint changed',
    hint: TLS_HINT,
  },
  {
    match: /x509:.*signed by unknown authority|certificate signed by unknown/i,
    headline: 'TLS certificate not trusted',
    hint: TLS_HINT,
  },
  {
    match: /x509:.*expired|certificate has expired/i,
    headline: 'TLS certificate expired',
    hint: TLS_HINT,
  },
  {
    match: /tls:\s*handshake failure|tls handshake|bad certificate/i,
    headline: 'TLS handshake failed',
    hint: TLS_HINT,
  },
  {
    match: /x509:|TLS certificate (?:error|validation failed)/i,
    headline: 'TLS certificate validation failed',
    hint: TLS_HINT,
  },
  {
    match: /access violation/i,
    headline: 'Connection blocked',
    hint: 'The request was blocked before Pulse could read inventory. Check proxy, firewall, or network policy settings.',
  },
  {
    match: /forbidden|permission denied/i,
    headline: 'Permission denied',
    hint: DENIED_HINT,
  },
  {
    match: /unauthorized|authentication failed|invalid (?:credentials|api token)/i,
    headline: 'Authentication failed',
    hint: AUTH_HINT,
  },
  {
    match: /not found/i,
    ...HTTP_STATUS_PRESENTATIONS[404],
  },
  {
    match: /server error|internal error/i,
    ...SERVER_ERROR,
  },
];

const stripImplementationPrefixes = (raw: string): string =>
  raw.replace(POLL_PREFIX_PATTERN, '').replace(HTTP_VERB_PATTERN, '').trim();

export const humanizeConnectionError = (raw?: string | null): HumanizedConnectionError | null => {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (!trimmed) return null;

  const cleaned = stripImplementationPrefixes(trimmed);
  const statusMatch = cleaned.match(HTTP_STATUS_PATTERN);
  if (statusMatch) {
    const status = Number(statusMatch[1] ?? statusMatch[2]);
    const presentation = HTTP_STATUS_PRESENTATIONS[status] ?? (status >= 500 ? SERVER_ERROR : null);
    if (presentation) return { ...presentation };
    return { headline: cleaned, hint: null };
  }

  for (const pattern of HUMANIZED_PATTERNS) {
    if (pattern.match.test(cleaned)) {
      return { headline: pattern.headline, hint: pattern.hint };
    }
  }

  // Unknown error: fall back to the raw text with the most common
  // implementation-leaking prefixes peeled off so it's at least readable.
  return { headline: cleaned || 'Connection error', hint: null };
};

/**
 * Render a single-line message suitable for inline display in a table row.
 * Combines headline and hint when both apply.
 */
export const formatConnectionErrorMessage = (raw?: string | null): string | null => {
  const humanized = humanizeConnectionError(raw);
  if (!humanized) return null;
  return humanized.hint ? `${humanized.headline}. ${humanized.hint}` : humanized.headline;
};

// `pmg_queue_status failed on mail-gw: ...`: the monitor's read operation,
// the instance, then the cause.
const READ_OPERATION_PATTERN = /^([a-z]+(?:_[a-z]+)+)\s+failed\s+on\s+[^:]+:\s*/i;

const readOperationLabel = (operation: string): string => {
  const words = operation
    .toLowerCase()
    .replace(/^(?:pmg|pbs|pve)_/, '')
    .replace(/_/g, ' ');
  return words.charAt(0).toUpperCase() + words.slice(1);
};

/**
 * Render a partial-read error: the connection answered, but some readings
 * failed (one per line). The generic cause alone ('Server error') hides which
 * data is missing, so name the failed reading first.
 */
export const formatPartialReadErrorMessage = (raw?: string | null): string | null => {
  const failures = (raw ?? '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
  const first = failures[0];
  if (!first) return null;
  const match = first.match(READ_OPERATION_PATTERN);
  const label = match ? readOperationLabel(match[1]) : 'Some readings';
  const cause = formatConnectionErrorMessage(match ? first.slice(match[0].length) : first);
  const others = failures.length - 1;
  const subject =
    match && others > 0
      ? `${label} and ${others} other ${others === 1 ? 'reading' : 'readings'}`
      : label;
  return cause ? `${subject} could not be read: ${cause}` : `${subject} could not be read.`;
};
