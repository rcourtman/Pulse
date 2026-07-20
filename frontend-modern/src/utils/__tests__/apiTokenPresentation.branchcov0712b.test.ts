import { describe, expect, it } from 'vitest';
import type { APITokenUsagePresentationEntry } from '@/utils/apiTokenPresentation';
import {
  getAPITokenDockerPodmanUsageTitle,
  getAPITokenGenerateErrorMessage,
} from '@/utils/apiTokenPresentation';

// Branch-coverage companion to `apiTokenPresentation.test.ts`. The sibling
// suite already pins the happy paths (no-arg default, `Cannot grant scope`
// echo, and `missing_scope` with a known scope id). This file drives the
// remaining arms of `getAPITokenGenerateErrorMessage` (non-object input,
// non-403 status, non-string message, trim path, the `missing_scope` guard
// falling through for an unknown/whitespace/absent scope id, and the
// label-falsy fallback) and of `getAPITokenDockerPodmanUsageTitle`
// (`filter(Boolean)` emptying the list and the single/mixed-label join arms).

describe('apiTokenPresentation — branch coverage (branchcov2)', () => {
  describe('getAPITokenGenerateErrorMessage', () => {
    it('returns the default copy for a non-object primitive (typeof !== "object")', () => {
      // The outer `error && typeof error === 'object'` guard short-circuits,
      // skipping the entire typed block and falling through to the trailing
      // default return.
      expect(getAPITokenGenerateErrorMessage('server exploded')).toBe(
        'Unable to generate the API token.',
      );
      expect(getAPITokenGenerateErrorMessage(503)).toBe('Unable to generate the API token.');
    });

    it('returns the default copy for a 403-shape error whose status is not 403', () => {
      // `typedError.status !== 403` is true -> early default inside the object block.
      const error = { status: 500, message: 'Internal failure' };
      expect(getAPITokenGenerateErrorMessage(error)).toBe('Unable to generate the API token.');
    });

    it('returns the default copy for a 403 whose message is not a string', () => {
      // status === 403 but `typeof typedError.message !== 'string'` is true,
      // so the same early default fires before `.trim()` is ever reached.
      const error = { status: 403, message: 42 } as unknown as Parameters<
        typeof getAPITokenGenerateErrorMessage
      >[0];
      expect(getAPITokenGenerateErrorMessage(error)).toBe('Unable to generate the API token.');
    });

    it('returns the trimmed message when it starts with "Cannot grant scope" (trim + startsWith true arm)', () => {
      // Drives `typedError.message.trim()` and the `startsWith('Cannot grant scope')`
      // true arm: leading/trailing whitespace is stripped before the message is echoed.
      const error = Object.assign(new Error('  Cannot grant scope "ai:execute": denied  '), {
        status: 403,
      });
      expect(getAPITokenGenerateErrorMessage(error)).toBe(
        'Cannot grant scope "ai:execute": denied',
      );
    });

    it('falls through to the default for a 403 message that matches neither sentinel', () => {
      // status === 403, message is a string, but it does not start with
      // "Cannot grant scope" and is not the literal "missing_scope" — so both
      // inner ifs are false and execution reaches the trailing default.
      const error = Object.assign(new Error('something else entirely'), { status: 403 });
      expect(getAPITokenGenerateErrorMessage(error)).toBe('Unable to generate the API token.');
    });

    it('falls through to the default when "missing_scope" is returned without a requiredScope', () => {
      // `message === 'missing_scope'` is true, but `typedError.requiredScope`
      // is undefined -> `requiredScope?.trim()` evaluates to undefined, which
      // is falsy, so the `if (requiredScope)` body is skipped and we fall
      // out of the missing_scope block to the trailing default.
      const error = Object.assign(new Error('missing_scope'), { status: 403 });
      expect(getAPITokenGenerateErrorMessage(error)).toBe('Unable to generate the API token.');
    });

    it('falls through to the default when requiredScope trims to an empty string', () => {
      // requiredScope is whitespace-only -> after `.trim()` it is '' (falsy),
      // so the labelled branch is skipped even though the field was present.
      const error = Object.assign(new Error('missing_scope'), {
        status: 403,
        requiredScope: '   ',
      });
      expect(getAPITokenGenerateErrorMessage(error)).toBe('Unable to generate the API token.');
    });

    it('falls back to the raw scope id when requiredScope has no friendly label (label falsy arm)', () => {
      // `API_SCOPE_LABELS[unknownScope]` is undefined, so the ternary picks the
      // right-hand template that interpolates only the raw id.
      const error = Object.assign(new Error('missing_scope'), {
        status: 403,
        requiredScope: 'totally:unknown:scope',
      });
      expect(getAPITokenGenerateErrorMessage(error)).toBe(
        'This token is missing the required scope: totally:unknown:scope.',
      );
    });
  });

  describe('getAPITokenDockerPodmanUsageTitle', () => {
    it('returns the bare runtimes label when items is empty (labels.length === 0 arm)', () => {
      // `.map` over [] then `.filter(Boolean)` yields [], so the ternary
      // returns the right-hand `API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL`.
      const entry: APITokenUsagePresentationEntry = { count: 0, items: [] };
      expect(getAPITokenDockerPodmanUsageTitle(entry)).toBe('Docker / Podman runtimes');
    });

    it('returns the bare runtimes label when every item label is empty (filter(Boolean) empties the list)', () => {
      // items is non-empty but every label is the falsy '' — `filter(Boolean)`
      // drops them all, exercising the same labels.length === 0 arm via a
      // different runtime path.
      const entry: APITokenUsagePresentationEntry = {
        count: 2,
        items: [{ label: '' }, { label: '' }],
      };
      expect(getAPITokenDockerPodmanUsageTitle(entry)).toBe('Docker / Podman runtimes');
    });

    it('joins a single truthy label after the colon (labels.length > 0, one element)', () => {
      // Single-element join produces no separator, confirming the join(', ')
      // behaves correctly for the n === 1 case.
      const entry: APITokenUsagePresentationEntry = {
        count: 1,
        items: [{ label: 'Edge Node' }],
      };
      expect(getAPITokenDockerPodmanUsageTitle(entry)).toBe('Docker / Podman runtimes: Edge Node');
    });

    it('drops falsy labels and joins the remaining truthy ones with ", "', () => {
      // Mixed truthy/falsy labels: filter(Boolean) keeps 'Docker Edge' and
      // 'Podman Lab' while dropping the empty middle label, so the joined
      // output has exactly two entries with the canonical separator.
      const entry: APITokenUsagePresentationEntry = {
        count: 3,
        items: [{ label: 'Docker Edge' }, { label: '' }, { label: 'Podman Lab' }],
      };
      expect(getAPITokenDockerPodmanUsageTitle(entry)).toBe(
        'Docker / Podman runtimes: Docker Edge, Podman Lab',
      );
    });
  });
});
