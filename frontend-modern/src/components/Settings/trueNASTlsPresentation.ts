import type { TrueNASConnectionFormState } from './useTrueNASSettingsPanelState';

type TrustInputs = Pick<
  TrueNASConnectionFormState,
  'host' | 'port' | 'useHttps' | 'insecureSkipVerify' | 'fingerprint'
>;

// Describe requested policy, never a verified peer. Keep the defaults and pin
// semantics aligned with internal/truenas/client.go resolveEndpoint/buildTLSConfig.
export const getTrueNASTlsPolicy = (form: TrustInputs) => {
  const scheme = form.host
    .trim()
    .match(/^([^:]+):\/\//)?.[1]
    .toLowerCase();
  if (scheme && scheme !== 'https' && scheme !== 'http') {
    return {
      tone: 'warning' as const,
      title: 'Endpoint scheme is not supported',
      description:
        'Use an HTTPS endpoint or a hostname. These settings do not establish a connection.',
    };
  }
  const port = form.port.trim();
  if (port && (!Number.isInteger(Number(port)) || Number(port) < 1 || Number(port) > 65535)) {
    return {
      tone: 'warning' as const,
      title: 'Connection port is invalid',
      description:
        'Enter a whole number between 1 and 65535, or leave Port blank for the client default.',
    };
  }
  // The native client uses the explicit URL scheme, then defaults back to
  // HTTPS when the separate port is omitted (even for an http:// Host).
  const https = (scheme ? scheme === 'https' : form.useHttps) || !port;
  if (!https) {
    return {
      tone: 'danger' as const,
      title: 'HTTP does not protect credentials',
      description:
        'HTTP traffic is not encrypted or authenticated by TLS. Neither a fingerprint nor the verification setting protects credentials over HTTP. Use a trusted HTTPS endpoint. Do not switch to HTTP to bypass a certificate error.',
    };
  }
  const pin = form.fingerprint
    .trim()
    .toLowerCase()
    .replace(/^sha256:/, '')
    .replace(/[: ]/g, '');
  if (pin && !/^[0-9a-f]{64}$/.test(pin)) {
    return {
      tone: 'warning' as const,
      title: 'Certificate fingerprint is invalid',
      description:
        'HTTPS requires a SHA-256 pin of 64 hexadecimal characters, optionally separated by colons or spaces and prefixed with SHA256:. This value will be rejected, not treated as verified trust.',
    };
  }
  if (pin) {
    return form.insecureSkipVerify
      ? {
          tone: 'warning' as const,
          title: 'Certificate fingerprint required',
          description:
            'The certificate must match the configured SHA-256 pin. Certificate-chain, hostname and expiry checks are skipped, but the pin is still required when Skip TLS verification is checked.',
        }
      : {
          tone: 'info' as const,
          title: 'Certificate chain and fingerprint required',
          description:
            'The certificate must match the configured SHA-256 pin and pass certificate-chain, hostname and expiry checks using trust available to the Pulse runtime. A pin does not replace these checks while Skip TLS verification is unchecked.',
        };
  }
  return form.insecureSkipVerify
    ? {
        tone: 'danger' as const,
        title: 'Certificate identity not verified',
        description:
          'With no fingerprint and Skip TLS verification checked, these settings do not authenticate the server. Encryption alone does not protect credentials from an impersonated endpoint.',
      }
    : {
        tone: 'info' as const,
        title: 'Trusted certificate chain required',
        description:
          'With no fingerprint, the certificate must pass certificate-chain, hostname and expiry checks using trust available to the Pulse runtime.',
      };
};
