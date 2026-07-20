import { describe, expect, it, vi } from 'vitest';
import {
  formatDiscoveryReadinessBriefingLine,
  formatReadinessAge,
  getDiscoveryReadinessPresentation,
} from '@/utils/resourceDiscoveryReadiness';
import type { ResourceDiscoveryReadiness, ResourceDiscoveryReadinessState } from '@/types/resource';

// `formatDiscoveryAge` (from @/api/discovery) is wall-clock dependent. Mock it
// deterministically so the `observedAt`-driven code paths assert concrete
// strings instead of "X minutes ago". Vitest hoists vi.mock above the imports.
vi.mock('@/api/discovery', () => ({
  formatDiscoveryAge: (updatedAt: string): string => `MOCKED_AGE:${updatedAt}`,
}));

type ReadinessArg = Parameters<typeof getDiscoveryReadinessPresentation>[0];
type AgeArg = Parameters<typeof formatReadinessAge>[0];

const makeReadiness = (
  overrides: Partial<ResourceDiscoveryReadiness> = {},
): ResourceDiscoveryReadiness => ({
  state: 'fresh',
  ...overrides,
});

// A value that is not part of the canonical readiness-state set; used to drive
// the `STATE_COPY[state] ?? fallback` arm. Cast through `unknown` so the strict
// tsconfig accepts it.
const UNKNOWN_STATE = 'bogus' as unknown as ResourceDiscoveryReadinessState;

describe('resourceDiscoveryReadiness — branch coverage (branchcov2)', () => {
  describe('formatReadinessAge', () => {
    it('returns an empty string for every rejected input (non-number / NaN / Infinity / negative)', () => {
      // typeof !== 'number'
      expect(formatReadinessAge()).toBe('');
      expect(formatReadinessAge(undefined)).toBe('');
      expect(formatReadinessAge(null as unknown as AgeArg)).toBe('');
      // Number.isFinite is false
      expect(formatReadinessAge(NaN)).toBe('');
      expect(formatReadinessAge(Infinity)).toBe('');
      expect(formatReadinessAge(-Infinity)).toBe('');
      // seconds < 0
      expect(formatReadinessAge(-1)).toBe('');
      expect(formatReadinessAge(-0.0001)).toBe('');
    });

    it('returns "under a minute old" for the sub-minute band including zero', () => {
      expect(formatReadinessAge(0)).toBe('under a minute old');
      expect(formatReadinessAge(59)).toBe('under a minute old');
      expect(formatReadinessAge(59.999)).toBe('under a minute old');
    });

    it('renders singular vs plural minutes', () => {
      // minutes === 1 (singular arm of the ternary)
      expect(formatReadinessAge(60)).toBe('1 minute old');
      // minutes !== 1 (plural arm)
      expect(formatReadinessAge(120)).toBe('2 minutes old');
      expect(formatReadinessAge(3540)).toBe('59 minutes old');
    });

    it('renders singular vs plural hours', () => {
      // hours === 1
      expect(formatReadinessAge(3600)).toBe('1 hour old');
      // hours !== 1
      expect(formatReadinessAge(7200)).toBe('2 hours old');
      expect(formatReadinessAge(82800)).toBe('23 hours old');
    });

    it('renders singular vs plural days', () => {
      // days === 1
      expect(formatReadinessAge(86400)).toBe('1 day old');
      // days !== 1
      expect(formatReadinessAge(172800)).toBe('2 days old');
      expect(formatReadinessAge(604800)).toBe('7 days old');
    });
  });

  describe('getDiscoveryReadinessPresentation — null/unknown arms', () => {
    it('returns null when readiness is absent and discovery is unsupported', () => {
      expect(getDiscoveryReadinessPresentation(null, false)).toBeNull();
      expect(getDiscoveryReadinessPresentation(undefined, false)).toBeNull();
    });

    it('returns a fully-shaped "unknown" presentation when readiness is absent but supported', () => {
      const expectedUnknown = {
        state: 'unknown',
        label: 'Discovery unknown',
        shortLabel: 'Unknown',
        statusLabel: 'Discovery unknown',
        title: 'Discovery status is not available for this resource yet.',
        detail: 'Discovery status unavailable',
        tone: 'muted',
      } as const;

      // explicit hasDiscoverySupport = true
      expect(getDiscoveryReadinessPresentation(undefined, true)).toStrictEqual(expectedUnknown);
      expect(getDiscoveryReadinessPresentation(null, true)).toStrictEqual(expectedUnknown);
      // default second argument is `true`
      expect(getDiscoveryReadinessPresentation(undefined)).toStrictEqual(expectedUnknown);
      expect(getDiscoveryReadinessPresentation(null)).toStrictEqual(expectedUnknown);
    });
  });

  describe('getDiscoveryReadinessPresentation — STATE_COPY routing', () => {
    it('maps every canonical state to its label/shortLabel/statusLabel/tone', () => {
      const cases: Array<{
        state: ResourceDiscoveryReadinessState;
        label: string;
        shortLabel: string;
        statusLabel: string;
        tone: string;
      }> = [
        {
          state: 'fresh',
          label: 'Discovery fresh',
          shortLabel: 'Fresh',
          statusLabel: 'Discovery fresh',
          tone: 'success',
        },
        {
          state: 'stale',
          label: 'Discovery stale',
          shortLabel: 'Stale',
          statusLabel: 'Discovery stale',
          tone: 'warning',
        },
        {
          state: 'missing',
          label: 'Not discovered',
          shortLabel: 'None',
          statusLabel: 'No discovery data',
          tone: 'muted',
        },
        {
          state: 'running',
          label: 'Discovery running',
          shortLabel: 'Running',
          statusLabel: 'Discovery running',
          tone: 'info',
        },
        {
          state: 'failed',
          label: 'Discovery failed',
          shortLabel: 'Failed',
          statusLabel: 'Discovery failed',
          tone: 'danger',
        },
        {
          state: 'unavailable',
          label: 'Discovery unavailable',
          shortLabel: 'Unavailable',
          statusLabel: 'Discovery unavailable',
          tone: 'warning',
        },
        {
          state: 'unsupported',
          label: 'Not supported',
          shortLabel: 'N/A',
          statusLabel: 'Discovery unsupported',
          tone: 'muted',
        },
      ];

      for (const { state, label, shortLabel, statusLabel, tone } of cases) {
        const presentation = getDiscoveryReadinessPresentation(makeReadiness({ state }));
        expect(presentation?.state).toBe(state);
        expect(presentation?.label).toBe(label);
        expect(presentation?.shortLabel).toBe(shortLabel);
        expect(presentation?.statusLabel).toBe(statusLabel);
        expect(presentation?.tone).toBe(tone);
      }
    });

    it('falls back to the unknown copy when the state is not canonical (STATE_COPY ?? fallback)', () => {
      const presentation = getDiscoveryReadinessPresentation(
        makeReadiness({ state: UNKNOWN_STATE }),
      );
      expect(presentation).toStrictEqual({
        state: UNKNOWN_STATE,
        label: 'Discovery unknown',
        shortLabel: 'Unknown',
        statusLabel: 'Discovery unknown',
        // No reason/service/facts/observed => detail is empty => title === copy.label
        title: 'Discovery unknown',
        detail: '',
        tone: 'muted',
      });
    });
  });

  describe('getDiscoveryReadinessPresentation — details assembly', () => {
    it('joins reason, service, facts and observed age into title and detail', () => {
      const presentation = getDiscoveryReadinessPresentation(
        makeReadiness({
          state: 'fresh',
          reason: 'Within freshness window.',
          serviceName: 'Home Assistant',
          factCount: 7,
          ageSeconds: 120,
        }),
      );
      const expectedDetail =
        'Within freshness window. · Service: Home Assistant · 7 facts · Observed 2 minutes old';
      expect(presentation?.detail).toBe(expectedDetail);
      expect(presentation?.title).toBe(`Discovery fresh: ${expectedDetail}`);
    });

    it('keeps only the reason line when service/facts/observed are absent', () => {
      const presentation = getDiscoveryReadinessPresentation(
        makeReadiness({ state: 'fresh', reason: 'Only reason.' }),
      );
      expect(presentation?.detail).toBe('Only reason.');
      expect(presentation?.title).toBe('Discovery fresh: Only reason.');
    });

    it('drops zero and negative fact counts via the `factCount && factCount > 0` guard', () => {
      expect(
        getDiscoveryReadinessPresentation(makeReadiness({ state: 'fresh', factCount: 0 }))?.detail,
      ).toBe('');
      expect(
        getDiscoveryReadinessPresentation(makeReadiness({ state: 'fresh', factCount: -3 }))?.detail,
      ).toBe('');
    });

    it('collapses title to copy.label when no detail lines survive filtering', () => {
      const presentation = getDiscoveryReadinessPresentation(makeReadiness({ state: 'failed' }));
      expect(presentation?.detail).toBe('');
      expect(presentation?.title).toBe('Discovery failed');
    });
  });

  describe('observedLabel (exercised via getDiscoveryReadinessPresentation)', () => {
    it('routes through formatDiscoveryAge when observedAt is truthy', () => {
      const presentation = getDiscoveryReadinessPresentation(
        makeReadiness({ state: 'fresh', observedAt: '2024-01-02T03:04:05Z' }),
      );
      expect(presentation?.detail).toBe('Observed MOCKED_AGE:2024-01-02T03:04:05Z');
    });

    it('prefers observedAt over ageSeconds when both are populated', () => {
      const presentation = getDiscoveryReadinessPresentation(
        makeReadiness({ state: 'fresh', observedAt: 'X', ageSeconds: 600 }),
      );
      expect(presentation?.detail).toBe('Observed MOCKED_AGE:X');
    });

    it('falls back to formatReadinessAge(ageSeconds) when observedAt is missing or empty', () => {
      expect(
        getDiscoveryReadinessPresentation(makeReadiness({ state: 'fresh', ageSeconds: 600 }))
          ?.detail,
      ).toBe('Observed 10 minutes old');
      // Empty string is falsy, so the observedAt branch is skipped.
      expect(
        getDiscoveryReadinessPresentation(
          makeReadiness({ state: 'fresh', observedAt: '', ageSeconds: 600 }),
        )?.detail,
      ).toBe('Observed 10 minutes old');
    });

    it('omits the Observed line entirely when neither observedAt nor a usable age are present', () => {
      // ageSeconds undefined => formatReadinessAge returns '' => observedLabel returns ''
      const presentation = getDiscoveryReadinessPresentation(
        makeReadiness({ state: 'fresh', reason: 'something' }),
      );
      expect(presentation?.detail).toBe('something');
    });
  });

  describe('formatDiscoveryReadinessBriefingLine', () => {
    it('returns an empty string when readiness is null/undefined (presentation is null)', () => {
      expect(formatDiscoveryReadinessBriefingLine(undefined)).toBe('');
      expect(formatDiscoveryReadinessBriefingLine(null as unknown as ReadinessArg)).toBe('');
    });

    it('joins statusLabel, service, observed and facts in canonical order', () => {
      expect(
        formatDiscoveryReadinessBriefingLine(
          makeReadiness({
            state: 'stale',
            serviceName: 'Home Assistant',
            factCount: 5,
            observedAt: '2024-01-02T03:04:05Z',
          }),
        ),
      ).toBe(
        'Discovery data: Discovery stale, service Home Assistant, observed MOCKED_AGE:2024-01-02T03:04:05Z, 5 facts',
      );
    });

    it('emits only the statusLabel when no optional detail is present', () => {
      expect(formatDiscoveryReadinessBriefingLine(makeReadiness({ state: 'failed' }))).toBe(
        'Discovery data: Discovery failed',
      );
    });

    it('emits the service branch alone when only serviceName is set', () => {
      expect(
        formatDiscoveryReadinessBriefingLine(
          makeReadiness({ state: 'failed', serviceName: 'SNMP' }),
        ),
      ).toBe('Discovery data: Discovery failed, service SNMP');
    });

    it('emits the observed branch alone when only observedAt is set', () => {
      expect(
        formatDiscoveryReadinessBriefingLine(makeReadiness({ state: 'failed', observedAt: 'X' })),
      ).toBe('Discovery data: Discovery failed, observed MOCKED_AGE:X');
    });

    it('emits the facts branch alone when only factCount is set', () => {
      expect(
        formatDiscoveryReadinessBriefingLine(makeReadiness({ state: 'failed', factCount: 3 })),
      ).toBe('Discovery data: Discovery failed, 3 facts');
    });

    it('drops zero and negative fact counts in the briefing details', () => {
      expect(
        formatDiscoveryReadinessBriefingLine(makeReadiness({ state: 'failed', factCount: 0 })),
      ).toBe('Discovery data: Discovery failed');
      expect(
        formatDiscoveryReadinessBriefingLine(makeReadiness({ state: 'failed', factCount: -2 })),
      ).toBe('Discovery data: Discovery failed');
    });
  });
});
