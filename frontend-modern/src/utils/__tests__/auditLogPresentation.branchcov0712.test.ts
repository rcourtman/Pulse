import { describe, expect, it } from 'vitest';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import {
  type AuditEventStatusPresentation,
  getAuditEventStatusPresentation,
} from '@/utils/auditLogPresentation';

// `getAuditEventStatusPresentation` is a single ternary over a `boolean` parameter with
// exactly two arms. The sibling `auditLogPresentation.test.ts` only asserts on
// `className` via `toMatchObject`; here we additionally pin the full returned shape
// (including the exact icon *reference*) and exercise the ternary's runtime truthiness
// coercion by feeding non-boolean values through a `boolean`-typed parameter.

describe('getAuditEventStatusPresentation (branch coverage)', () => {
  describe('success arm (truthy)', () => {
    it('returns the full success presentation: CheckCircle reference + emerald className', () => {
      const result: AuditEventStatusPresentation = getAuditEventStatusPresentation(true);
      expect(result).toStrictEqual({
        icon: CheckCircle,
        className: 'w-4 h-4 text-emerald-400',
      });
    });

    it('exposes the exact CheckCircle component by reference (not a clone/wrapper)', () => {
      expect(getAuditEventStatusPresentation(true).icon).toBe(CheckCircle);
    });

    it('selects the success arm for any truthy value (ternary uses JS truthiness)', () => {
      // The parameter is statically `boolean`, but the ternary condition is plain
      // truthiness; a truthy non-boolean therefore lands on the success arm.
      const fromNumber = getAuditEventStatusPresentation(1 as unknown as boolean);
      const fromNonEmptyString = getAuditEventStatusPresentation('true' as unknown as boolean);
      expect(fromNumber).toStrictEqual({
        icon: CheckCircle,
        className: 'w-4 h-4 text-emerald-400',
      });
      expect(fromNonEmptyString.icon).toBe(CheckCircle);
      expect(fromNonEmptyString.className).toBe('w-4 h-4 text-emerald-400');
    });
  });

  describe('failure arm (falsy)', () => {
    it('returns the full failure presentation: XCircle reference + rose className', () => {
      const result: AuditEventStatusPresentation = getAuditEventStatusPresentation(false);
      expect(result).toStrictEqual({
        icon: XCircle,
        className: 'w-4 h-4 text-rose-400',
      });
    });

    it('exposes the exact XCircle component by reference (not a clone/wrapper)', () => {
      expect(getAuditEventStatusPresentation(false).icon).toBe(XCircle);
    });

    it('selects the failure arm for a numeric falsy value (ternary uses JS truthiness)', () => {
      expect(getAuditEventStatusPresentation(0 as unknown as boolean)).toStrictEqual({
        icon: XCircle,
        className: 'w-4 h-4 text-rose-400',
      });
    });

    it('selects the failure arm for an empty string coerced through the boolean parameter', () => {
      expect(getAuditEventStatusPresentation('' as unknown as boolean)).toStrictEqual({
        icon: XCircle,
        className: 'w-4 h-4 text-rose-400',
      });
    });

    it('selects the failure arm for null coerced through the boolean parameter', () => {
      expect(getAuditEventStatusPresentation(null as unknown as boolean)).toStrictEqual({
        icon: XCircle,
        className: 'w-4 h-4 text-rose-400',
      });
    });
  });

  describe('arm discrimination', () => {
    it('returns mutually distinct icon references and classNames across the two arms', () => {
      const success = getAuditEventStatusPresentation(true);
      const failure = getAuditEventStatusPresentation(false);
      expect(success.icon).not.toBe(failure.icon);
      expect(success.className).not.toBe(failure.className);
    });

    it('returns a Solid callable component for the icon on both arms', () => {
      expect(typeof getAuditEventStatusPresentation(true).icon).toBe('function');
      expect(typeof getAuditEventStatusPresentation(false).icon).toBe('function');
    });

    it('returns only the `icon` and `className` keys (no extra enumerable fields)', () => {
      expect(Object.keys(getAuditEventStatusPresentation(true)).sort()).toStrictEqual([
        'className',
        'icon',
      ]);
      expect(Object.keys(getAuditEventStatusPresentation(false)).sort()).toStrictEqual([
        'className',
        'icon',
      ]);
    });
  });
});
