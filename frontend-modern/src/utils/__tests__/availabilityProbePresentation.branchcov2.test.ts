import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import {
  getAvailabilityProbeMethodLabel,
  getAvailabilityProbePresentation,
  getAvailabilityProbeTargetLabel,
} from '@/utils/availabilityProbePresentation';

// `normalizeAvailabilityProtocol`, `getAvailabilityProbeResultLabel`,
// `getAvailabilityProbeToneClassName`, and `getFailureCountLabel` are module-private
// (non-exported) helpers, so they are exercised indirectly through the three exported
// entry points below, asserting on their observable outputs.

const makeAvailability = (
  overrides?: Partial<ResourceAvailabilityMeta>,
): ResourceAvailabilityMeta => ({
  protocol: 'tcp',
  port: 443,
  available: true,
  latencyMillis: 5,
  lastChecked: '2026-05-06T13:00:00Z',
  ...overrides,
});

const makeResource = (overrides?: Partial<Resource>): Resource => ({
  id: 'availability:probe-1',
  type: 'network-endpoint',
  name: 'probe-target',
  displayName: 'probe-target',
  platformId: 'probe-1',
  platformType: 'availability',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1,
  availability: makeAvailability(),
  platformData: { sources: ['availability'] },
  ...overrides,
});

const AT = new Date('2026-05-06T13:00:20Z').getTime();

describe('normalizeAvailabilityProtocol — branch coverage (via getAvailabilityProbeMethodLabel)', () => {
  // The helper is `(protocol ?? '').trim().toLowerCase()`; each case proves a distinct
  // input shape flows through the nullish-coalesce, trim, and lowercase steps.
  afterEach(() => vi.restoreAllMocks());

  it('coerces an undefined protocol to an empty (unknown) protocol', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: undefined })).toBe('Probe');
  });

  it('coerces a null protocol to an empty (unknown) protocol', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: null as unknown as string })).toBe('Probe');
  });

  it('trims and lowercases a cased/whitespace-padded protocol before matching', () => {
    // '  IcMp ' -> normalize -> 'icmp' -> 'ICMP'
    expect(getAvailabilityProbeMethodLabel({ protocol: '  IcMp ' })).toBe('ICMP');
  });

  it('treats a whitespace-only protocol as empty', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: '   ' })).toBe('Probe');
  });
});

describe('getAvailabilityProbeMethodLabel — branch coverage', () => {
  it('returns plain "TCP" when the protocol is tcp but no port is set', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'tcp' })).toBe('TCP');
  });

  it('returns "TCP <port>" when tcp and a port is set', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'tcp', port: 22 })).toBe('TCP 22');
  });

  it('returns bare "HTTP" when http has no path', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'http' })).toBe('HTTP');
  });

  it('returns bare "HTTPS" when https has no path', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'https' })).toBe('HTTPS');
  });

  it('returns "HTTPS <path>" when https has a path', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'https', path: '/healthz' })).toBe(
      'HTTPS /healthz',
    );
  });

  it('uppercases an unrecognized-but-non-empty protocol', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'snmp' })).toBe('SNMP');
  });

  it('falls back to "Probe" when the availability object has no protocol', () => {
    expect(getAvailabilityProbeMethodLabel({})).toBe('Probe');
  });

  it('falls back to "Probe" when availability is undefined', () => {
    expect(getAvailabilityProbeMethodLabel(undefined)).toBe('Probe');
  });

  it('falls back to "Probe" when availability is null', () => {
    expect(getAvailabilityProbeMethodLabel(null)).toBe('Probe');
  });
});

describe('getAvailabilityProbeTargetLabel — branch coverage', () => {
  it('returns the port string for tcp with a positive finite port', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'tcp', port: 443 })).toBe('443');
  });

  it('returns null for tcp with port 0 (port > 0 guard)', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'tcp', port: 0 })).toBeNull();
  });

  it('returns null for tcp with a negative port', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'tcp', port: -1 })).toBeNull();
  });

  it('returns null for tcp with a non-finite port (Infinity)', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'tcp', port: Infinity })).toBeNull();
  });

  it('returns null for tcp with a non-finite port (NaN)', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'tcp', port: NaN })).toBeNull();
  });

  it('returns null for tcp when port is the wrong type (string)', () => {
    expect(
      getAvailabilityProbeTargetLabel({ protocol: 'tcp', port: 'abc' as unknown as number }),
    ).toBeNull();
  });

  it('returns null for tcp when no port is set', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'tcp' })).toBeNull();
  });

  it('returns the trimmed path for http with a path', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'http', path: '/status' })).toBe('/status');
  });

  it('returns the trimmed path for https with a path', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'https', path: '/ready' })).toBe('/ready');
  });

  it('returns null for http with no path', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'http' })).toBeNull();
  });

  it('returns null for http with a whitespace-only path', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'http', path: '   ' })).toBeNull();
  });

  it('returns null for icmp (non-tcp/http protocol)', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'icmp' })).toBeNull();
  });

  it('returns null for an unrecognized protocol', () => {
    expect(getAvailabilityProbeTargetLabel({ protocol: 'snmp' })).toBeNull();
  });

  it('returns null when availability is undefined', () => {
    expect(getAvailabilityProbeTargetLabel(undefined)).toBeNull();
  });
});

describe('getAvailabilityProbeResultLabel — branch coverage (via presentation.resultLabel)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('returns "reachable" when available is true with no usable latency', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: { protocol: 'icmp', available: true, lastChecked: '2026-05-06T13:00:00Z' },
      }),
    );
    expect(presentation?.resultLabel).toBe('reachable');
  });

  it('returns "not checked" when not failed, no latency, and not reachable', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'unknown',
        availability: { protocol: 'icmp', lastChecked: '2026-05-06T13:00:00Z' },
      }),
    );
    expect(presentation?.resultLabel).toBe('not checked');
  });

  it('returns "failed" on the failure path when lastError has no timeout/HTTP-status token', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'degraded',
        availability: {
          protocol: 'icmp',
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: 'route unreachable',
        },
      }),
    );
    expect(presentation?.resultLabel).toBe('failed');
  });

  it('maps a "timeout" token (not "timed out") to "timed out"', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'http',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: 'Connection timeout exceeded',
        },
      }),
    );
    expect(presentation?.resultLabel).toBe('timed out');
  });

  it('extracts a 4xx HTTP status code from lastError', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'http',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: 'probe got 404 Not Found',
        },
      }),
    );
    expect(presentation?.resultLabel).toBe('404');
  });

  it('rounds fractional latency to the nearest millisecond', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: { protocol: 'tcp', port: 443, latencyMillis: 7.4 },
      }),
    );
    expect(presentation?.resultLabel).toBe('7 ms');
  });

  it('reports "0 ms" at the latency >= 0 boundary', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: { protocol: 'tcp', port: 443, latencyMillis: 0 },
      }),
    );
    expect(presentation?.resultLabel).toBe('0 ms');
  });

  it('falls through to "reachable" when latency is NaN (non-finite) but available is true', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: { protocol: 'tcp', port: 443, available: true, latencyMillis: NaN },
      }),
    );
    expect(presentation?.resultLabel).toBe('reachable');
  });

  it('falls through to "reachable" when latency is negative but available is true', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: { protocol: 'tcp', port: 443, available: true, latencyMillis: -5 },
      }),
    );
    expect(presentation?.resultLabel).toBe('reachable');
  });

  it('falls through to "reachable" when latency is the wrong type but available is true', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'tcp',
          port: 443,
          available: true,
          latencyMillis: 'fast' as unknown as number,
        },
      }),
    );
    expect(presentation?.resultLabel).toBe('reachable');
  });

  it('honours the latency path even when status is "online" and available is true', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: makeAvailability({ latencyMillis: 9.5 }),
      }),
    );
    expect(presentation?.resultLabel).toBe('10 ms');
  });
});

describe('getAvailabilityProbeToneClassName — branch coverage (via presentation.toneClassName)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('returns the warning class for a degraded status that is not hard-failed', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'degraded',
        availability: { protocol: 'icmp', lastChecked: '2026-05-06T13:00:00Z' },
      }),
    );
    expect(presentation?.toneClassName).toBe('text-amber-600 dark:text-amber-300');
  });

  it('returns the unknown class when not failed/degraded and not reachable', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'unknown',
        availability: { protocol: 'icmp', lastChecked: '2026-05-06T13:00:00Z' },
      }),
    );
    expect(presentation?.toneClassName).toBe('text-muted');
  });

  it('returns the error class when available is false even if status is online', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'http',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: '503 Service Unavailable',
        },
      }),
    );
    expect(presentation?.toneClassName).toBe('text-red-600 dark:text-red-300');
  });

  it('returns the success class when available is true even if status is not online', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'unknown',
        availability: {
          protocol: 'tcp',
          port: 443,
          available: true,
          latencyMillis: 5,
          lastChecked: '2026-05-06T13:00:00Z',
        },
      }),
    );
    expect(presentation?.toneClassName).toBe('text-emerald-600 dark:text-emerald-300');
  });
});

describe('getFailureCountLabel — branch coverage (via presentation.detailLabel)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('renders "1 failure" for a single failure with no threshold', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: 1 }),
      }),
    );
    expect(presentation?.detailLabel).toContain('1 failure');
    expect(presentation?.detailLabel).not.toContain('1 failures');
    expect(presentation?.detailLabel).not.toContain('/');
  });

  it('renders "N failures" for multiple failures with no threshold', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: 2 }),
      }),
    );
    expect(presentation?.detailLabel).toContain('2 failures');
    expect(presentation?.detailLabel).not.toContain('/');
  });

  it('omits any failure count when consecutiveFailures is 0', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: 0 }),
      }),
    );
    expect(presentation?.detailLabel).not.toMatch(/failure/);
  });

  it('omits any failure count when consecutiveFailures is negative', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: -3 }),
      }),
    );
    expect(presentation?.detailLabel).not.toMatch(/failure/);
  });

  it('omits any failure count when consecutiveFailures is NaN', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: NaN }),
      }),
    );
    expect(presentation?.detailLabel).not.toMatch(/failure/);
  });

  it('omits any failure count when consecutiveFailures is the wrong type', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          consecutiveFailures: 'bad' as unknown as number,
        }),
      }),
    );
    expect(presentation?.detailLabel).not.toMatch(/failure/);
  });

  it('falls back to "N failures" when failureThreshold is present but invalid (0)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: 3, failureThreshold: 0 }),
      }),
    );
    expect(presentation?.detailLabel).toContain('3 failures');
    expect(presentation?.detailLabel).not.toContain('/');
  });

  it('renders the "N/threshold failures" form when both are positive finite numbers', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ consecutiveFailures: 3, failureThreshold: 4 }),
      }),
    );
    expect(presentation?.detailLabel).toContain('3/4 failures');
  });
});

describe('getAvailabilityProbePresentation — branch coverage', () => {
  afterEach(() => vi.restoreAllMocks());

  it('returns null when neither availability nor platformData.availability is set', () => {
    expect(
      getAvailabilityProbePresentation(
        makeResource({ availability: undefined, platformData: { sources: ['availability'] } }),
      ),
    ).toBeNull();
  });

  it('returns null when availability is unset and platformData itself is missing (optional chain)', () => {
    expect(
      getAvailabilityProbePresentation(
        makeResource({ availability: undefined, platformData: undefined }),
      ),
    ).toBeNull();
  });

  it('prefers resource.availability over platformData.availability (?? left operand wins)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ port: 443 }),
        platformData: {
          sources: ['availability'],
          availability: makeAvailability({ port: 8080 }),
        },
      }),
    );
    expect(presentation?.methodLabel).toBe('TCP 443');
    expect(presentation?.targetLabel).toBe('443');
  });

  it('builds netIoLabel as "<target>: <result>" when a target exists', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(makeResource());
    expect(presentation?.netIoLabel).toBe('443: 5 ms');
  });

  it('builds netIoLabel from just the result when there is no target (icmp)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: { protocol: 'icmp', available: true, lastChecked: '2026-05-06T13:00:00Z' },
      }),
    );
    expect(presentation?.targetLabel).toBeNull();
    expect(presentation?.netIoLabel).toBe(presentation?.resultLabel);
    expect(presentation?.netIoLabel).toBe('reachable');
  });

  it('drops the "checked ..." segment from both rowLabel and detailLabel when lastChecked is missing', () => {
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ lastChecked: undefined }),
      }),
    );
    expect(presentation?.rowLabel).toBe(presentation?.netIoLabel);
    expect(presentation?.rowLabel).toBe('443: 5 ms');
    expect(presentation?.detailLabel).not.toContain('checked');
  });

  it('includes a deterministic "checked ..." segment when lastChecked is set', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(makeResource());
    expect(presentation?.rowLabel).toBe('443: 5 ms - checked 20s ago');
    expect(presentation?.detailLabel).toContain('checked 20s ago');
  });

  it('appends "last success ..." when available is false and lastSuccess parses to a relative time', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'offline',
        availability: {
          protocol: 'icmp',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastSuccess: '2026-05-06T12:51:20Z',
          consecutiveFailures: 1,
        },
      }),
    );
    expect(presentation?.detailLabel).toContain('last success 9 mins ago');
  });

  it('omits "last success ..." when lastSuccess is set but does not parse (sub-guard false)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'offline',
        availability: {
          protocol: 'icmp',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastSuccess: 'not-a-real-date',
        },
      }),
    );
    expect(presentation?.detailLabel).not.toContain('last success');
  });

  it('omits "last success ..." when lastSuccess is set but available is not false (guard skipped)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'icmp',
          available: true,
          latencyMillis: 4,
          lastChecked: '2026-05-06T13:00:00Z',
          lastSuccess: '2026-05-06T12:51:20Z',
        },
      }),
    );
    expect(presentation?.detailLabel).not.toContain('last success');
  });

  it('appends lastError verbatim to detailLabel when present', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'offline',
        availability: {
          protocol: 'icmp',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: 'dial tcp: i/o timeout',
        },
      }),
    );
    expect(presentation?.detailLabel).toContain('dial tcp: i/o timeout');
  });
});
