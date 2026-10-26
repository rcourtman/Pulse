import { describe, expect, it } from 'vitest';

import {
  formatConnectionErrorMessage,
  formatPartialReadErrorMessage,
  humanizeConnectionError,
} from '../connectionErrorPresentation';

describe('humanizeConnectionError', () => {
  it('returns null for empty input', () => {
    expect(humanizeConnectionError(undefined)).toBeNull();
    expect(humanizeConnectionError(null)).toBeNull();
    expect(humanizeConnectionError('')).toBeNull();
    expect(humanizeConnectionError('   ')).toBeNull();
  });

  it('humanizes Go context-deadline timeouts on poll-style errors', () => {
    const raw =
      'poll_nodes failed on delly: Get "https://192.168.0.134:8006/api2/json/nodes": context deadline exceeded (Client.Timeout exceeded while awaiting headers)';
    const result = humanizeConnectionError(raw);
    expect(result?.headline).toBe('Connection timed out');
    expect(result?.hint).toMatch(/host is reachable/i);
  });

  it('classifies DNS lookup failures as host not found', () => {
    const result = humanizeConnectionError(
      'Get "https://truenas.lan/api/v2.0/system/info": dial tcp: lookup truenas.lan: no such host',
    );
    expect(result?.headline).toBe('Host not found');
    expect(result?.hint).toMatch(/hostname or IP/i);
  });

  it('classifies connection refused', () => {
    const result = humanizeConnectionError(
      'Get "https://192.0.2.7:8006/": dial tcp 192.0.2.7:8006: connect: connection refused',
    );
    expect(result?.headline).toBe('Connection refused');
  });

  it('classifies untrusted certs as TLS not trusted with a remediation hint', () => {
    const result = humanizeConnectionError(
      'Get "https://truenas.lan/": x509: certificate signed by unknown authority',
    );
    expect(result?.headline).toBe('TLS certificate not trusted');
    expect(result?.hint).toMatch(/fingerprint|TLS verification/i);
  });

  it('classifies HTTP 401 as authentication failed', () => {
    const result = humanizeConnectionError('Get "https://pve/api2/json/version": 401 Unauthorized');
    expect(result?.headline).toBe('Authentication failed');
  });

  it('classifies HTTP 403 as permission denied', () => {
    const result = humanizeConnectionError(
      'Get "https://pve/api2/json/cluster/status": 403 Forbidden',
    );
    expect(result?.headline).toBe('Permission denied');
  });

  describe('trust-preserving recovery guidance', () => {
    it.each([
      ['x509: certificate signed by unknown authority', 'TLS certificate not trusted'],
      ['x509: certificate has expired or is not yet valid', 'TLS certificate expired'],
      ['remote error: tls: handshake failure', 'TLS handshake failed'],
      [
        'certificate fingerprint mismatch: expected fixture-old, got fixture-new',
        'TLS fingerprint changed',
      ],
      ['truenas tls pinning failed: fingerprint mismatch', 'TLS fingerprint changed'],
      [
        'TLS fingerprint changed - independently verify the certificate before updating the saved fingerprint',
        'TLS fingerprint changed',
      ],
      ['x509: certificate is valid for old.lab, not new.lab', 'TLS certificate validation failed'],
      [
        'TLS certificate error - check SSL settings or add fingerprint',
        'TLS certificate validation failed',
      ],
      ['TLS certificate validation failed', 'TLS certificate validation failed'],
    ])('keeps verification and independently checked identity for %s', (raw, headline) => {
      const result = humanizeConnectionError(raw);
      expect(result?.headline).toBe(headline);
      expect(result?.hint).toContain('Keep TLS verification enabled.');
      expect(result?.hint).toContain('hostname, expiry and trusted chain');
      expect(result?.hint).toContain(
        'Independently verify its identity before adding or replacing',
      );
      expect(result?.hint).not.toMatch(/disable|temporarily|switch.*HTTP/i);
    });

    it.each([
      'API error 403: Forbidden',
      'HTTP status 403: authentication failed; quoted upstream 401',
      'authentication error: API error 403: invalid credentials',
      'permission denied',
    ])('does not infer a successful login or prescribe broader access for %s', (raw) => {
      const result = humanizeConnectionError(raw);
      expect(result?.headline).toBe('Permission denied');
      expect(result?.hint).toContain('intended resource');
      expect(result?.hint).toContain('any proxy policy');
      expect(result?.hint).toContain('Keep intentional access limits');
      expect(result?.hint).toContain('Do not replace credentials or broaden access just to test');
      expect(result?.hint).not.toMatch(/credentials connected|logged in|Administrator/);
    });

    it.each(['401 Unauthorized', 'authentication failed', 'invalid API token'])(
      'makes credential replacement conditional for %s',
      (raw) => {
        const result = humanizeConnectionError(raw);
        expect(result?.headline).toBe('Authentication failed');
        expect(result?.hint).toContain('user, realm and token ID');
        expect(result?.hint).toContain('Replace it only if invalidity is confirmed');
      },
    );

    it.each([500, 502, 595])(
      'does not misclassify HTTP %i provider auth text as a 401',
      (status) => {
        const result = humanizeConnectionError(
          `API error ${status}: Authentication failed; provider mentioned 401 and 403`,
        );
        expect(result?.headline).toBe('Server error');
        expect(result?.hint).not.toMatch(/token|credential/);
      },
    );

    it('ignores status-shaped numbers in the endpoint and poll identity', () => {
      expect(
        humanizeConnectionError(
          'poll_nodes failed on 401: Get "https://pve.lab:403/api/500": something novel',
        ),
      ).toEqual({ headline: 'something novel', hint: null });
    });

    it.each([400, 429])(
      'keeps unclassified HTTP %i responses instead of inferring auth failure',
      (status) => {
        const raw = `API error ${status}: provider mentioned 401 Unauthorized`;
        expect(humanizeConnectionError(raw)).toEqual({ headline: raw, hint: null });
      },
    );
  });

  it('classifies access violations as blocked connections', () => {
    const result = humanizeConnectionError(
      'poll_nodes failed on delly: Get "https://delly:8006/api2/json/nodes": Access violation',
    );
    expect(result?.headline).toBe('Connection blocked');
    expect(result?.hint).toMatch(/proxy/i);
  });

  it('falls back to a cleaned message for unknown errors', () => {
    const result = humanizeConnectionError(
      'poll_nodes failed on delly: Get "https://example.com/foo": something genuinely novel',
    );
    expect(result?.headline).toBe('something genuinely novel');
    expect(result?.hint).toBeNull();
  });

  it('formats a single-line message with hint for known patterns', () => {
    const formatted = formatConnectionErrorMessage(
      'poll_nodes failed on pi: Get "https://pi:8006/api2/json/nodes": context deadline exceeded',
    );
    expect(formatted).toBe(
      'Connection timed out. Check the host is reachable, the port is correct, and the network path is open.',
    );
  });

  it('formats unknown errors as headline only', () => {
    const formatted = formatConnectionErrorMessage('a totally unknown failure mode');
    expect(formatted).toBe('a totally unknown failure mode');
  });
});

describe('formatPartialReadErrorMessage', () => {
  it('names the failed reading before the cause', () => {
    expect(
      formatPartialReadErrorMessage(
        'pmg_queue_status failed on mail-gw: active queue: API error 500: {"errors":"qshape failed"}',
      ),
    ).toBe(
      'Queue status could not be read: Server error. The platform returned a server error. Check the platform logs for the underlying cause.',
    );
  });

  it('counts the other failed readings', () => {
    expect(
      formatPartialReadErrorMessage(
        [
          'pmg_spam_quarantine failed on mail-gw: API error 403: Permission check failed',
          'pmg_virus_quarantine failed on mail-gw: API error 403: Permission check failed',
          'pmg_queue_status failed on mail-gw: API error 500',
        ].join('\n'),
      ),
    ).toMatch(/^Spam quarantine and 2 other readings could not be read: Permission denied\./);
  });

  it('falls back to a generic subject without an operation prefix', () => {
    expect(formatPartialReadErrorMessage('context deadline exceeded')).toMatch(
      /^Some readings could not be read: Connection timed out\./,
    );
    expect(formatPartialReadErrorMessage('')).toBeNull();
    expect(formatPartialReadErrorMessage(undefined)).toBeNull();
  });
});
