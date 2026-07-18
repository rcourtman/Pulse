import { describe, expect, it } from 'vitest';
import {
  AUDIT_WEBHOOK_SECURITY_NOTE_BODY,
  AUDIT_WEBHOOK_SECURITY_NOTE_TITLE,
  getAuditWebhookDuplicateUrlMessage,
  getAuditWebhookFeatureGateCopy,
  getAuditWebhookInvalidUrlMessage,
  getAuditWebhookSaveErrorMessage,
  getAuditWebhookSaveSuccessMessage,
} from '@/utils/auditWebhookPresentation';

// Branch-coverage companion to auditWebhookPresentation.test.ts.
//
// The sibling suite already pins:
//   - getAuditWebhookFeatureGateCopy() with no args (commercial copy arm),
//   - getAuditWebhookFeatureGateCopy({ showCommercialCopy: false }) (not-enabled arm),
//   - getAuditWebhookFeatureGateCopy({ paidRuntimeRequired: true }) (paid-runtime arm),
//   - getAuditWebhookEmptyStateCopy / getAuditWebhookLoadingState canonical strings,
//   - a small slice of the exported shell-class constants.
//
// v8 reports the module at 100% branch coverage but only ~43% function coverage
// after the sibling suite: four pure message getters
// (getAuditWebhookInvalidUrlMessage, getAuditWebhookDuplicateUrlMessage,
// getAuditWebhookSaveSuccessMessage, getAuditWebhookSaveErrorMessage) are never
// invoked, and the exported SECURITY_NOTE_TITLE / SECURITY_NOTE_BODY strings are
// never asserted. This file targets that residual:
//
//   1. Each currently-unexercised combination of the feature-gate option arms —
//      pinning precedence (paidRuntimeRequired wins even when
//      showCommercialCopy is explicitly false) and the explicit-true /
//      explicit-false / undefined matrix per option.
//   2. The four uncovered message getters, asserting each exact canonical
//      string so a silent rename in the source fails loudly.
//   3. The two exported SECURITY_NOTE vocabulary strings, which no test
//      currently pins.
//
// The module is pure value-in/value-out (no factory returning getter
// properties, no Solid signals) so we mirror the sibling's plain
// direct-invocation pattern — no createRoot required.

describe('auditWebhookPresentation — branch coverage (batch 0718)', () => {
  describe('getAuditWebhookFeatureGateCopy — residual option-matrix arms', () => {
    it('honours paidRuntimeRequired even when showCommercialCopy is explicitly false (precedence: first guard wins)', () => {
      // The `if (options.paidRuntimeRequired)` guard runs before the
      // showCommercialCopy check, so the paid-runtime copy must win regardless
      // of the commercial flag. Locks the precedence order.
      const copy = getAuditWebhookFeatureGateCopy({
        paidRuntimeRequired: true,
        showCommercialCopy: false,
      });
      expect(copy).toEqual({
        title: 'Pulse Pro runtime required',
        body: expect.stringContaining('private Pulse Pro runtime'),
      });
      expect(copy.body).not.toContain('not enabled');
    });

    it('honours paidRuntimeRequired when showCommercialCopy is explicitly true (both flags agree on commercial intent)', () => {
      const copy = getAuditWebhookFeatureGateCopy({
        paidRuntimeRequired: true,
        showCommercialCopy: true,
      });
      expect(copy).toEqual({
        title: 'Pulse Pro runtime required',
        body: expect.stringContaining('Install the private Pulse Pro runtime'),
      });
    });

    it('returns commercial copy for explicit paidRuntimeRequired: false + showCommercialCopy: true', () => {
      // Explicit `false` for paidRuntimeRequired takes the falsy arm of the
      // first guard; explicit `true` for showCommercialCopy means
      // `!== false` is true → commercial copy.
      expect(
        getAuditWebhookFeatureGateCopy({
          paidRuntimeRequired: false,
          showCommercialCopy: true,
        }),
      ).toEqual({
        title: 'Audit Webhooks',
        body: 'Audit webhook delivery is available on paid self-hosted and hosted plans.',
      });
    });

    it('returns not-enabled copy for explicit paidRuntimeRequired: false + showCommercialCopy: false', () => {
      expect(
        getAuditWebhookFeatureGateCopy({
          paidRuntimeRequired: false,
          showCommercialCopy: false,
        }),
      ).toEqual({
        title: 'Audit Webhooks',
        body: 'Audit webhook delivery is not enabled for this instance.',
      });
    });

    it('returns commercial copy for explicit paidRuntimeRequired: false with showCommercialCopy omitted (undefined !== false)', () => {
      // Documents that omitting showCommercialCopy is equivalent to
      // showCommercialCopy: true — the `!== false` default-on behaviour.
      expect(
        getAuditWebhookFeatureGateCopy({ paidRuntimeRequired: false }),
      ).toEqual({
        title: 'Audit Webhooks',
        body: 'Audit webhook delivery is available on paid self-hosted and hosted plans.',
      });
    });

    it('returns commercial copy for explicit showCommercialCopy: true with paidRuntimeRequired omitted', () => {
      // Sister case: only showCommercialCopy is supplied (as true). The first
      // guard's falsy arm is taken (paidRuntimeRequired undefined), then the
      // commercial body is selected.
      expect(getAuditWebhookFeatureGateCopy({ showCommercialCopy: true })).toEqual({
        title: 'Audit Webhooks',
        body: 'Audit webhook delivery is available on paid self-hosted and hosted plans.',
      });
    });

    it('returns commercial copy when called with an explicit empty options object (matches no-arg overload)', () => {
      // The default `options = {}` parameter must behave identically to an
      // explicit `{}` — locks the no-arg / empty-arg equivalence.
      expect(getAuditWebhookFeatureGateCopy({})).toEqual({
        title: 'Audit Webhooks',
        body: 'Audit webhook delivery is available on paid self-hosted and hosted plans.',
      });
    });
  });

  describe('getAuditWebhookInvalidUrlMessage — uncovered getter', () => {
    it('returns the canonical invalid-URL validation message', () => {
      expect(getAuditWebhookInvalidUrlMessage()).toBe('Please enter a valid URL');
    });
  });

  describe('getAuditWebhookDuplicateUrlMessage — uncovered getter', () => {
    it('returns the canonical duplicate-URL validation message', () => {
      expect(getAuditWebhookDuplicateUrlMessage()).toBe('This URL is already configured');
    });
  });

  describe('getAuditWebhookSaveSuccessMessage — uncovered getter', () => {
    it('returns the canonical save-success toast message', () => {
      expect(getAuditWebhookSaveSuccessMessage()).toBe('Audit webhooks updated');
    });
  });

  describe('getAuditWebhookSaveErrorMessage — uncovered getter', () => {
    it('returns the canonical save-error toast message', () => {
      expect(getAuditWebhookSaveErrorMessage()).toBe('Failed to save webhook configuration');
    });
  });

  describe('residual exported vocabulary — SECURITY_NOTE constants', () => {
    // The sibling suite never asserts the SECURITY_NOTE strings; pin them so a
    // future copy edit cannot pass silently.
    it('exposes the canonical SECURITY_NOTE title', () => {
      expect(AUDIT_WEBHOOK_SECURITY_NOTE_TITLE).toBe('Security Note');
    });

    it('exposes the canonical SECURITY_NOTE body verbatim', () => {
      expect(AUDIT_WEBHOOK_SECURITY_NOTE_BODY).toBe(
        'Audit webhooks are dispatched asynchronously to avoid blocking user operations. Endpoints should still verify source trust (for example via an ingest secret) before processing events.',
      );
    });
  });
});
