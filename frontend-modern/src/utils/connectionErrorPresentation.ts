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
  'Verify the certificate or pin a fingerprint in TLS settings, or disable TLS verification temporarily for testing.';
const AUTH_HINT = 'Re-check the API token or username/password.';

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
    match: /\b401\b|unauthorized|authentication failed|invalid (?:credentials|api token)/i,
    headline: 'Authentication failed',
    hint: AUTH_HINT,
  },
  {
    match: /access violation/i,
    headline: 'Connection blocked',
    hint: 'The request was blocked before Pulse could read inventory. Check proxy, firewall, or network policy settings.',
  },
  {
    match: /\b403\b|forbidden|permission denied/i,
    headline: 'Permission denied',
    hint: 'The credentials connected, but the user/token lacks the required role.',
  },
  {
    match: /\b404\b|not found/i,
    headline: 'Endpoint not found',
    hint: 'Pulse reached the host but the API path is missing. Check the URL path or the platform version.',
  },
  {
    match: /\b5\d{2}\b|server error|internal error/i,
    headline: 'Server error',
    hint: 'The platform returned a server error. Check the platform logs for the underlying cause.',
  },
];

const stripImplementationPrefixes = (raw: string): string =>
  raw
    .replace(POLL_PREFIX_PATTERN, '')
    .replace(HTTP_VERB_PATTERN, '')
    .trim();

export const humanizeConnectionError = (raw?: string | null): HumanizedConnectionError | null => {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (!trimmed) return null;

  for (const pattern of HUMANIZED_PATTERNS) {
    if (pattern.match.test(trimmed)) {
      return { headline: pattern.headline, hint: pattern.hint };
    }
  }

  // Unknown error: fall back to the raw text with the most common
  // implementation-leaking prefixes peeled off so it's at least readable.
  const cleaned = stripImplementationPrefixes(trimmed);
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
