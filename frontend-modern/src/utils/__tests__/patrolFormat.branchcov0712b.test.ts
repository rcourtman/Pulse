/**
 * Supplemental branch-coverage tests for `patrolFormat`.
 *
 * The sibling `patrolFormat.test.ts` exercises the happy paths of every export,
 * but several branches of these three functions are never driven:
 *
 *  - `formatPatrolRuntimeFailureDetail`: the `text || ''` / `!raw` empty arms,
 *    five of the eight canonical "return raw" list entries, several keyword
 *    arms inside the classification cascades (tool / credit / rate-limit /
 *    auth / connection / analysis-error), and two redaction regexes (`sk-`
 *    secrets and JSON credential keys).
 *  - `formatPatrolRuntimeFailureSummary`: the "provider analysis failed"
 *    detail-preference sub-branch, the summary===detail fallthrough, the
 *    only-summary / only-detail arms, and the entire `errorCount` fallback
 *    (singular / plural / clamp / undefined).
 *  - `formatScope`: the terminal `return ''` reached when a run has no ids,
 *    no types, and is not the literal `type: 'scoped'`.
 *
 * Each test asserts a concrete output string/shape against the real branch
 * it targets; no truthiness-only assertions are used.
 */
import { describe, expect, it } from 'vitest';

import {
  formatPatrolRuntimeFailureDetail,
  formatPatrolRuntimeFailureSummary,
  formatScope,
} from '@/utils/patrolFormat';

describe('patrolFormat branch coverage (supplemental)', () => {
  describe('formatPatrolRuntimeFailureDetail', () => {
    it('returns "" for undefined input (text || "" falsy arm + !raw guard)', () => {
      expect(formatPatrolRuntimeFailureDetail(undefined)).toBe('');
    });

    it('returns "" for whitespace-only input (trim drives raw to "" -> !raw arm)', () => {
      expect(formatPatrolRuntimeFailureDetail('     ')).toBe('');
    });

    it('collapses internal whitespace before classification, preserving outer case on a canonical match', () => {
      // raw normalises 'Provider   Rate\nLimited' -> 'Provider Rate Limited';
      // lowercased it hits the canonical list and the original casing is returned.
      expect(formatPatrolRuntimeFailureDetail('Provider   Rate\nLimited')).toBe(
        'Provider Rate Limited',
      );
    });

    describe('canonical-list arms that return the raw (trimmed) string verbatim', () => {
      // The sibling test only drives 'provider billing or quota issue',
      // 'selected model does not support patrol tools', and
      // 'provider connection issue' (all via the summary wrapper). These hit
      // the remaining five canonical entries directly.
      it('matches "provider rate limited"', () => {
        expect(formatPatrolRuntimeFailureDetail('Provider rate limited')).toBe(
          'Provider rate limited',
        );
      });

      it('matches "provider authentication issue"', () => {
        expect(formatPatrolRuntimeFailureDetail('Provider authentication issue')).toBe(
          'Provider authentication issue',
        );
      });

      it('matches "provider not ready"', () => {
        expect(formatPatrolRuntimeFailureDetail('Provider not ready')).toBe(
          'Provider not ready',
        );
      });

      it('matches "selected model unavailable"', () => {
        expect(formatPatrolRuntimeFailureDetail('Selected model unavailable')).toBe(
          'Selected model unavailable',
        );
      });

      it('matches "selected model context window too small"', () => {
        expect(
          formatPatrolRuntimeFailureDetail('Selected model context window too small'),
        ).toBe('Selected model context window too small');
      });
    });

    describe('tool-call rejection classification arms', () => {
      const expected =
        'Provider rejected Patrol tool calls. Choose a Patrol model and endpoint with tool-call support.';

      it('matches the standalone "tool calling" keyword', () => {
        expect(formatPatrolRuntimeFailureDetail('Tool calling is disabled')).toBe(expected);
      });

      it('matches the "tools are not supported" phrase', () => {
        expect(formatPatrolRuntimeFailureDetail('tools are not supported here')).toBe(
          expected,
        );
      });

      it('matches the compound "no endpoints found" + "tool" condition (both halves required)', () => {
        // Only "no endpoints found" without "tool" must NOT hit this arm —
        // exercised separately below to prove the conjunction.
        expect(
          formatPatrolRuntimeFailureDetail('no endpoints found for the requested tool'),
        ).toBe(expected);
      });

      it('does NOT classify when "no endpoints found" appears without "tool" (conjunction false arm)', () => {
        // Falls through every classifier to the redaction tail; with no
        // secrets to redact the trimmed raw is returned unchanged.
        expect(formatPatrolRuntimeFailureDetail('no endpoints found in region')).toBe(
          'no endpoints found in region',
        );
      });
    });

    describe('insufficient credits / token budget classification arms', () => {
      const expected =
        'Provider reported insufficient credits or token budget for the requested Patrol analysis.';

      it('matches "insufficient balance"', () => {
        expect(formatPatrolRuntimeFailureDetail('insufficient balance on account')).toBe(
          expected,
        );
      });

      it('matches "payment required"', () => {
        expect(formatPatrolRuntimeFailureDetail('HTTP payment required')).toBe(expected);
      });

      it('matches "quota"', () => {
        expect(formatPatrolRuntimeFailureDetail('daily quota exhausted')).toBe(expected);
      });

      it('matches "credit"', () => {
        expect(formatPatrolRuntimeFailureDetail('out of credit')).toBe(expected);
      });

      it('matches "max_tokens"', () => {
        expect(formatPatrolRuntimeFailureDetail('max_tokens exceeded budget')).toBe(
          expected,
        );
      });
    });

    describe('rate-limit classification arms', () => {
      const expected =
        'Provider rate limit reached. Wait for capacity or adjust provider limits before retrying.';

      it('matches "429"', () => {
        expect(formatPatrolRuntimeFailureDetail('server returned 429')).toBe(expected);
      });

      it('matches "too many requests"', () => {
        expect(formatPatrolRuntimeFailureDetail('too many requests sent')).toBe(expected);
      });

      it('matches "rate limit"', () => {
        expect(formatPatrolRuntimeFailureDetail('rate limit hit')).toBe(expected);
      });
    });

    describe('authentication classification arms', () => {
      const expected =
        'Provider authentication failed. Check the configured provider key and account access.';

      it('matches "401"', () => {
        expect(formatPatrolRuntimeFailureDetail('request failed: 401')).toBe(expected);
      });

      it('matches "403"', () => {
        expect(formatPatrolRuntimeFailureDetail('403 forbidden by gateway')).toBe(expected);
      });

      it('matches "unauthorized"', () => {
        expect(formatPatrolRuntimeFailureDetail('unauthorized request')).toBe(expected);
      });

      it('matches "forbidden"', () => {
        expect(formatPatrolRuntimeFailureDetail('forbidden resource')).toBe(expected);
      });

      it('matches "api key"', () => {
        expect(formatPatrolRuntimeFailureDetail('api key revoked')).toBe(expected);
      });
    });

    describe('connection classification arms', () => {
      const expected =
        'Provider connection failed. Check provider reachability before retrying Patrol.';

      it('matches "failed to connect"', () => {
        expect(formatPatrolRuntimeFailureDetail('failed to connect to upstream')).toBe(
          expected,
        );
      });

      it('matches "connection refused"', () => {
        expect(formatPatrolRuntimeFailureDetail('connection refused by host')).toBe(
          expected,
        );
      });

      it('matches "no such host"', () => {
        expect(formatPatrolRuntimeFailureDetail('no such host: api.example')).toBe(
          expected,
        );
      });

      it('matches "i/o timeout"', () => {
        expect(formatPatrolRuntimeFailureDetail('i/o timeout reading body')).toBe(expected);
      });

      it('matches bare "timeout" (distinct from "timed out")', () => {
        expect(formatPatrolRuntimeFailureDetail('request timeout')).toBe(expected);
      });

      it('matches "returned status 5" (5xx prefix arm)', () => {
        expect(formatPatrolRuntimeFailureDetail('provider returned status 503')).toBe(
          expected,
        );
      });
    });

    describe('provider-analysis classification arms', () => {
      const expected =
        'Provider analysis failed. Check Provider & Models before retrying Patrol.';

      it('matches the exact "agentic patrol failed" sentinel', () => {
        expect(formatPatrolRuntimeFailureDetail('agentic patrol failed')).toBe(expected);
      });

      it('matches the "provider analysis error: agentic patrol failed" prefix', () => {
        expect(
          formatPatrolRuntimeFailureDetail(
            'provider analysis error: agentic patrol failed mid-run',
          ),
        ).toBe(expected);
      });

      it('matches the "agentic patrol failed: provider error" prefix', () => {
        expect(
          formatPatrolRuntimeFailureDetail(
            'agentic patrol failed: provider error something else',
          ),
        ).toBe(expected);
      });
    });

    describe('defensive redaction tail (no classifier matched)', () => {
      it('redacts bare "sk-" secrets', () => {
        const out = formatPatrolRuntimeFailureDetail('using key sk-abcd1234EFGH');
        expect(out).toBe('using key [redacted-secret]');
        expect(out).not.toContain('sk-abcd1234EFGH');
      });

      it('redacts JSON credential keys case-insensitively across naming styles', () => {
        // Exercises the JSON-key regex for snake_case "api_key" and "token".
        const out = formatPatrolRuntimeFailureDetail(
          'payload was {"api_key":"sk-leak123","token":"abc"}',
        );
        expect(out).toBe('payload was {"api_key":"[redacted]","token":"[redacted]"}');
        expect(out).not.toContain('sk-leak123');
        expect(out).not.toContain('"abc"');
      });

      it('redacts kebab/no-separator variants (apikey, x-api-key, authorization)', () => {
        const out = formatPatrolRuntimeFailureDetail(
          'headers {"apikey":"v1","x-api-key":"v2","authorization":"Bearer z"}',
        );
        expect(out).toBe(
          'headers {"apikey":"[redacted]","x-api-key":"[redacted]","authorization":"[redacted]"}',
        );
      });

      it('returns the trimmed raw when nothing sensitive is present (final .trim())', () => {
        expect(formatPatrolRuntimeFailureDetail('  just a benign message  ')).toBe(
          'just a benign message',
        );
      });
    });
  });

  describe('formatPatrolRuntimeFailureSummary', () => {
    describe('summary/detail interaction', () => {
      it('prefers the detail when summary is "Provider analysis failed" and detail starts with "provider " (analysis sub-branch)', () => {
        // summary -> formatPatrolRuntimeFailureDetail('provider analysis error')
        //        -> 'Provider analysis failed. Check Provider & Models before retrying Patrol.'
        // detail  -> 'provider rate limited' (canonical) -> returned verbatim, starts with 'provider '.
        // summary !== detail, so the analysis-failed sub-branch returns `detail`.
        const out = formatPatrolRuntimeFailureSummary({
          errorSummary: 'provider analysis error',
          errorDetail: 'provider rate limited',
        });
        expect(out).toBe('provider rate limited');
      });

      it('returns the single value via `summary || detail` when summary === detail (inequality false arm)', () => {
        // Both classify to the same canonical raw -> equal -> skips the
        // `${summary}: ${detail}` arm and returns one copy.
        const out = formatPatrolRuntimeFailureSummary({
          errorSummary: 'provider rate limited',
          errorDetail: 'provider rate limited',
        });
        expect(out).toBe('provider rate limited');
      });

      it('returns just the summary when detail is absent (summary || detail, detail empty)', () => {
        expect(
          formatPatrolRuntimeFailureSummary({ errorSummary: 'provider rate limited' }),
        ).toBe('provider rate limited');
      });

      it('returns just the detail when summary is absent (summary || detail, summary empty)', () => {
        expect(formatPatrolRuntimeFailureSummary({ errorDetail: '401 unauthorized' })).toBe(
          'Provider authentication failed. Check the configured provider key and account access.',
        );
      });
    });

    describe('errorCount fallback (both summary and detail empty)', () => {
      it('renders the singular form for errorCount === 1', () => {
        expect(formatPatrolRuntimeFailureSummary({ errorCount: 1 })).toBe(
          '1 Patrol runtime error recorded',
        );
      });

      it('renders the plural form for errorCount > 1', () => {
        expect(formatPatrolRuntimeFailureSummary({ errorCount: 3 })).toBe(
          '3 Patrol runtime errors recorded',
        );
      });

      it('returns undefined for errorCount === 0', () => {
        expect(formatPatrolRuntimeFailureSummary({ errorCount: 0 })).toBeUndefined();
      });

      it('returns undefined when errorCount is omitted (errorCount || 0 -> 0)', () => {
        expect(formatPatrolRuntimeFailureSummary({})).toBeUndefined();
      });

      it('clamps a negative errorCount to 0 via Math.max and returns undefined', () => {
        // input.errorCount || 0 -> -2 (truthy), then Math.max(0, -2) -> 0.
        expect(formatPatrolRuntimeFailureSummary({ errorCount: -2 })).toBeUndefined();
      });
    });
  });

  describe('formatScope terminal fallthrough', () => {
    it('returns "" for an empty run object (no ids, no types, not "scoped")', () => {
      // Drives the final `return ''` after idCount===0, types.length===0,
      // and run.type !== 'scoped' all fall through.
      expect(formatScope({})).toBe('');
    });

    it('returns "" when scope_resource_types is an empty array (types.length > 0 false arm)', () => {
      expect(formatScope({ scope_resource_types: [] })).toBe('');
    });

    it('returns "" when scope_resource_ids is an empty array (idCount === 0, not nullish)', () => {
      // [].length is 0 (not undefined), so the `?? 0` keeps 0; with no types
      // and no scoped type the function still bottoms out at ''.
      expect(formatScope({ scope_resource_ids: [] })).toBe('');
    });
  });
});
