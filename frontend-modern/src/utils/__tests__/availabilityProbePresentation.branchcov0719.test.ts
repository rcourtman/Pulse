import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import {
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';

// Residual branch-coverage probes for availabilityProbePresentation.
//
// The sibling tests (availabilityProbePresentation.test.ts and
// availabilityProbePresentation.branchcov2.test.ts) already pin the canonical
// arms of the exported helpers (method/target/result/tone/failureCount and
// the presentation assembly). This file targets the *residual* arms they
// never trip:
//   - `getAvailabilityProbeEndpointLabel` is 0% covered and is the primary
//     target here — every address/port/protocol/path arm is driven directly.
//   - `getAvailabilityCorrelationLabel`, `getAvailabilityFreshnessLabel`,
//     `getAvailabilityProbeToneClassName`, and the 5xx branch of the
//     private result-label helper are exercised indirectly through the
//     assembled presentation, mirroring the sibling test's pattern.

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
  id: 'availability:probe-endpoint',
  type: 'network-endpoint',
  name: 'probe-target',
  displayName: 'probe-target',
  platformId: 'probe-endpoint',
  platformType: 'availability',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1,
  availability: makeAvailability(),
  platformData: { sources: ['availability'] },
  ...overrides,
});

const AT = new Date('2026-05-06T13:00:20Z').getTime();

describe('getAvailabilityProbeEndpointLabel — branch coverage (primary target)', () => {
  // The helper is:
  //   address = (availability?.address ?? '').trim()
  //   addressWithPort = address && isFinitePort(port) && !address.endsWith(`:${port}`)
  //                    ? `${address}:${port}` : address
  //   if (http(s) && availability.path) {
  //     path = path.trim()
  //     if (path && !addressWithPort.endsWith(path)) {
  //       return addressWithPort.replace(/\/+$/, '') + (path startsWith '/' ? path : `/${path}`)
  //     }
  //   }
  //   return addressWithPort

  describe('nullish / empty fallbacks', () => {
    it('returns an empty string when availability is undefined', () => {
      expect(getAvailabilityProbeEndpointLabel(undefined)).toBe('');
    });

    it('returns an empty string when availability is null', () => {
      expect(getAvailabilityProbeEndpointLabel(null)).toBe('');
    });

    it('returns an empty string when no address is set', () => {
      expect(getAvailabilityProbeEndpointLabel({ protocol: 'tcp', port: 443 })).toBe('');
    });

    it('returns an empty string when address is whitespace-only (trimmed to empty)', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: '   ' })).toBe('');
    });
  });

  describe('addressWithPort — port-append arm (truthy branch of the ternary)', () => {
    it('appends a positive finite port to a bare address', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: '10.0.0.1', port: 443 })).toBe(
        '10.0.0.1:443',
      );
    });

    it('appends a port even when the protocol is unrecognized (protocol is orthogonal)', () => {
      expect(
        getAvailabilityProbeEndpointLabel({ address: 'host.local', protocol: 'snmp', port: 161 }),
      ).toBe('host.local:161');
    });

    it('trims surrounding whitespace from the address before appending the port', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: '  10.0.0.2  ', port: 22 })).toBe(
        '10.0.0.2:22',
      );
    });
  });

  describe('addressWithPort — fallback arm (falsy branch of the ternary)', () => {
    it('returns the bare address when port is missing', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: 'example.com' })).toBe('example.com');
    });

    it('returns the bare address when port is 0 (port > 0 guard)', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: 'example.com', port: 0 })).toBe(
        'example.com',
      );
    });

    it('returns the bare address when port is negative', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: 'example.com', port: -1 })).toBe(
        'example.com',
      );
    });

    it('returns the bare address when port is non-finite (Infinity)', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: 'example.com', port: Infinity })).toBe(
        'example.com',
      );
    });

    it('returns the bare address when port is non-finite (NaN)', () => {
      expect(getAvailabilityProbeEndpointLabel({ address: 'example.com', port: NaN })).toBe(
        'example.com',
      );
    });

    it('returns the bare address when port is the wrong type (string)', () => {
      expect(
        getAvailabilityProbeEndpointLabel({ address: 'example.com', port: 'abc' as unknown as number }),
      ).toBe('example.com');
    });

    it('does not duplicate the port when the address already ends with ":<port>"', () => {
      // The `!address.endsWith(`:${port}`)` guard prevents `host:443:443`.
      expect(getAvailabilityProbeEndpointLabel({ address: 'example.com:443', port: 443 })).toBe(
        'example.com:443',
      );
    });
  });

  describe('http(s) path-append arm', () => {
    it('appends an absolute path to an http address with a port', () => {
      expect(
        getAvailabilityProbeEndpointLabel({
          address: '10.0.0.5',
          protocol: 'http',
          port: 8080,
          path: '/healthz',
        }),
      ).toBe('10.0.0.5:8080/healthz');
    });

    it('appends an absolute path to an https address with no port', () => {
      expect(
        getAvailabilityProbeEndpointLabel({
          address: 'api.example.com',
          protocol: 'https',
          path: '/v1/status',
        }),
      ).toBe('api.example.com/v1/status');
    });

    it('prepends a leading "/" when the stored path does not start with one', () => {
      expect(
        getAvailabilityProbeEndpointLabel({
          address: 'api.example.com',
          protocol: 'https',
          path: 'ready',
        }),
      ).toBe('api.example.com/ready');
    });

    it('strips trailing slashes from the address before appending the path', () => {
      // `addressWithPort.replace(/\/+$/, '')` — `host//` -> `host` before path join.
      expect(
        getAvailabilityProbeEndpointLabel({
          address: 'api.example.com//',
          protocol: 'https',
          path: '/status',
        }),
      ).toBe('api.example.com/status');
    });

    it('falls through when the path is whitespace-only (trim() yields empty)', () => {
      // The inner `if (path && ...)` guard — whitespace path is treated as no path.
      expect(
        getAvailabilityProbeEndpointLabel({
          address: 'api.example.com',
          protocol: 'https',
          path: '   ',
        }),
      ).toBe('api.example.com');
    });

    it('does not re-append the path when addressWithPort already ends with it', () => {
      // The `!addressWithPort.endsWith(path)` guard prevents `host/status/status`.
      expect(
        getAvailabilityProbeEndpointLabel({
          address: 'api.example.com/status',
          protocol: 'https',
          path: '/status',
        }),
      ).toBe('api.example.com/status');
    });

    it('still appends the port to an http address before joining the path', () => {
      // Combined arm: port-append + path-append + path-starts-with-/ branch.
      expect(
        getAvailabilityProbeEndpointLabel({
          address: '10.0.0.9',
          protocol: 'http',
          port: 5000,
          path: '/ready',
        }),
      ).toBe('10.0.0.9:5000/ready');
    });

    it('does not join the path for a non-http(s) protocol even when a path is set', () => {
      // The `(protocol === 'http' || protocol === 'https')` guard — tcp path is ignored.
      expect(
        getAvailabilityProbeEndpointLabel({
          address: '10.0.0.9',
          protocol: 'tcp',
          port: 5432,
          path: '/should-be-ignored',
        }),
      ).toBe('10.0.0.9:5432');
    });
  });
});

describe('getAvailabilityCorrelationLabel — residual arms (via presentation.correlationLabel)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('renders "Resource link is unresolved" for the "unresolved" correlation state', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ correlationState: 'unresolved' }),
      }),
    );
    expect(presentation?.correlationLabel).toBe('Resource link is unresolved');
    expect(presentation?.detailLabel).toContain('Resource link is unresolved');
  });

  it('renders "Standalone endpoint" for the "standalone" correlation state', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ correlationState: 'standalone' }),
      }),
    );
    expect(presentation?.correlationLabel).toBe('Standalone endpoint');
    expect(presentation?.detailLabel).toContain('Standalone endpoint');
  });

  it('collapses ambiguous-with-exactly-one-candidate to the generic ambiguous label', () => {
    // `correlationCandidates > 1` is false at the boundary value of 1.
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ correlationState: 'ambiguous', correlationCandidates: 1 }),
      }),
    );
    expect(presentation?.correlationLabel).toBe('Resource match is ambiguous');
    expect(presentation?.detailLabel).not.toContain('possible resource matches');
  });

  it('collapses ambiguous-with-no-candidates to the generic ambiguous label', () => {
    // `availability.correlationCandidates && ...` is falsy when the field is absent.
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ correlationState: 'ambiguous' }),
      }),
    );
    expect(presentation?.correlationLabel).toBe('Resource match is ambiguous');
  });

  it('returns a null correlation label for the default switch arm (no correlation state)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ correlationState: undefined }),
      }),
    );
    expect(presentation?.correlationLabel).toBeNull();
  });

  it('returns a null correlation label for the "attached" correlation state', () => {
    // `attached` is on the union but has no case clause, so it falls to `default`.
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({ correlationState: 'attached' }),
      }),
    );
    expect(presentation?.correlationLabel).toBeNull();
  });
});

describe('getAvailabilityFreshnessLabel — residual arms (via presentation.freshnessLabel)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('reports "fresh" when evidence.validUntil is at or after now (>= boundary)', () => {
    const now = new Date('2026-05-06T13:00:00Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          evidence: {
            id: 'evidence-fresh',
            source: { provider: 'availability', collector: 'availability-poller' },
            subject: { resourceId: 'availability:probe-endpoint' },
            observedAt: '2026-05-06T12:55:00Z',
            ingestedAt: '2026-05-06T12:55:00Z',
            validUntil: '2026-05-06T13:00:00Z',
            completeness: 'complete',
            confidence: 'confirmed',
            permissions: 'sufficient',
          },
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('fresh');
  });

  it('reports "fresh" via lastChecked + pollIntervalSeconds when within 2x the interval', () => {
    // checked 10s ago, pollInterval 30s -> 10 + 30*2 = 70s ahead of now -> fresh.
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: 30,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('fresh');
  });

  it('reports "stale" via lastChecked + pollIntervalSeconds when past 2x the interval', () => {
    // checked 10s ago, pollInterval 2s -> 10 + 2*2 = 4s ahead, but now is past it -> stale.
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: 2,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('stale');
  });

  it('falls through to the pollInterval arm when evidence.validUntil is unparseable', () => {
    // validUntil exists but Date.parse yields NaN; lastChecked+pollInterval still says fresh.
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: 30,
          evidence: {
            id: 'evidence-bad',
            source: { provider: 'availability', collector: 'availability-poller' },
            subject: { resourceId: 'availability:probe-endpoint' },
            observedAt: '2026-05-06T12:55:00Z',
            ingestedAt: '2026-05-06T12:55:00Z',
            validUntil: 'not-a-real-date',
            completeness: 'complete',
            confidence: 'confirmed',
            permissions: 'sufficient',
          },
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('fresh');
  });

  it('reports "freshness unknown" when lastChecked is missing and no validUntil', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: undefined,
          pollIntervalSeconds: 30,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });

  it('reports "freshness unknown" when lastChecked is unparseable', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: 'not-a-real-date',
          pollIntervalSeconds: 30,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });

  it('reports "freshness unknown" when pollIntervalSeconds is missing', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: undefined,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });

  it('reports "freshness unknown" when pollIntervalSeconds is zero (<= 0 guard)', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: 0,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });

  it('reports "freshness unknown" when pollIntervalSeconds is negative (<= 0 guard)', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: -5,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });

  it('reports "freshness unknown" when pollIntervalSeconds is non-finite', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: Infinity,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });

  it('reports "freshness unknown" when pollIntervalSeconds is the wrong type', () => {
    const now = new Date('2026-05-06T13:00:10Z');
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        availability: makeAvailability({
          lastChecked: '2026-05-06T13:00:00Z',
          pollIntervalSeconds: 'soon' as unknown as number,
        }),
      }),
      now,
    );
    expect(presentation?.freshnessLabel).toBe('freshness unknown');
  });
});

describe('getAvailabilityProbeToneClassName — residual warning arm (via presentation.toneClassName)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('returns the warning class for the "unresolved" correlation state', () => {
    // Neither hard-failed nor degraded nor stale, but correlationState === 'unresolved'
    // is one of the ||'d warning triggers the sibling tests never trip.
    vi.spyOn(Date, 'now').mockReturnValue(AT);
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: makeAvailability({
          available: true,
          latencyMillis: 5,
          lastChecked: '2026-05-06T13:00:00Z',
          correlationState: 'unresolved',
        }),
      }),
    );
    expect(presentation?.toneClassName).toBe('text-amber-600 dark:text-amber-300');
  });
});

describe('getAvailabilityProbeResultLabel — residual 5xx status arm (via presentation.resultLabel)', () => {
  afterEach(() => vi.restoreAllMocks());

  it('extracts a 5xx HTTP status code from lastError', () => {
    // The sibling test covers the 4xx (404) arm; this pins the `[45]\d{2}` 5xx arm.
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'http',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: 'upstream returned 503 Service Unavailable',
        },
      }),
    );
    expect(presentation?.resultLabel).toBe('503');
  });

  it('extracts a 5xx HTTP status code that appears at the start of lastError', () => {
    // Word-boundary regex `\b([45]\d{2})\b` — confirm it matches at offset 0.
    const presentation = getAvailabilityProbePresentation(
      makeResource({
        status: 'online',
        availability: {
          protocol: 'http',
          available: false,
          lastChecked: '2026-05-06T13:00:00Z',
          lastError: '500 Internal Server Error',
        },
      }),
    );
    expect(presentation?.resultLabel).toBe('500');
  });
});
