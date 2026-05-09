import { describe, expect, it } from 'vitest';

import {
  formatConnectionErrorMessage,
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
    const result = humanizeConnectionError(
      'Get "https://pve/api2/json/version": 401 Unauthorized',
    );
    expect(result?.headline).toBe('Authentication failed');
  });

  it('classifies HTTP 403 as permission denied', () => {
    const result = humanizeConnectionError(
      'Get "https://pve/api2/json/cluster/status": 403 Forbidden',
    );
    expect(result?.headline).toBe('Permission denied');
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
