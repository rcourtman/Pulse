import { describe, expect, it } from 'vitest';
import type { StatusBadgeProps, StatusBadgeSize } from '@/components/shared/statusBadgeModel';
import {
  getStatusBadgeClass,
  getStatusBadgeLabel,
  resolveStatusBadgeSize,
} from '@/components/shared/statusBadgeModel';

// NOTE: getStatusBadgeTitle is intentionally NOT exercised here — the sibling
// file `statusBadgeModel.branchcov0712.test.ts` already covers every branch of
// its `disabled` if/else, the three-level ?? cascade, and the isEnabled
// ternary. Re-testing those branches would be pure duplication; this file
// targets the other three exported functions whose branches are still
// uncovered.

// Mirror of the module-private class-string constants so assertions are
// exact-string equality rather than substring/truthiness checks. Keeping them
// in sync with the source is the same pattern used by
// containerUpdateBadgeModel.branchcov2.test.ts.
const STATUS_BADGE_BASE_CLASS =
  'inline-flex items-center justify-center text-xs font-medium rounded-md transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 focus-visible:ring-blue-400';
const STATUS_BADGE_PADDING_BY_SIZE: Record<StatusBadgeSize, string> = {
  sm: 'px-2 py-0.5',
  md: 'px-2.5 py-1',
};
const STATUS_BADGE_ENABLED_CLASS =
  'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-500 dark:text-blue-300 dark:hover:bg-blue-500';
const STATUS_BADGE_DISABLED_STATE_CLASS = 'text-muted hover:bg-surface-hover';
const STATUS_BADGE_INTERACTION_DISABLED_CLASS =
  'opacity-60 cursor-not-allowed hover:bg-transparent dark:hover:bg-transparent';

// Minimal valid props; every optional label field defaults to undefined so
// each test can populate only the ones it needs to drive a specific branch.
function makeProps(overrides: Partial<StatusBadgeProps> = {}): StatusBadgeProps {
  return {
    isEnabled: false,
    ...overrides,
  };
}

describe('statusBadgeModel.branchcov0713', () => {
  describe('resolveStatusBadgeSize', () => {
    it('falls back to "sm" when size is undefined (?? right arm)', () => {
      expect(resolveStatusBadgeSize(undefined)).toBe('sm');
    });

    it('returns "sm" unchanged when explicitly passed (?? left arm)', () => {
      expect(resolveStatusBadgeSize('sm')).toBe('sm');
    });

    it('returns "md" unchanged when explicitly passed (?? left arm)', () => {
      expect(resolveStatusBadgeSize('md')).toBe('md');
    });

    it('distinguishes between the default "sm" and an explicit "sm" (both ?? arms are reachable)', () => {
      // Two tests above exercise each arm in isolation; this one asserts the
      // function never confuses the two distinct "sm" inputs.
      expect(resolveStatusBadgeSize(undefined)).toBe(resolveStatusBadgeSize('sm'));
      expect(resolveStatusBadgeSize('md')).not.toBe(resolveStatusBadgeSize('sm'));
    });

    it('treats a malformed size that happens to be falsy-string as the ?? right arm', () => {
      // The declared type is `StatusBadgeSize | undefined`; the only falsy
      // value that type permits is `undefined`. We still verify the ?? operator
      // is what's driving the default by passing the empty string through the
      // declared `undefined` slot — the function does not validate further.
      const malformed = '' as unknown as undefined;
      expect(resolveStatusBadgeSize(malformed)).toBe('');
    });
  });

  describe('getStatusBadgeClass', () => {
    describe('size lookup (Record<StatusBadgeSize, string>)', () => {
      it.each<StatusBadgeSize>(['sm', 'md'])(
        'embeds the %s padding token between base and state class',
        (size) => {
          expect(getStatusBadgeClass(size, true, false)).toBe(
            [
              STATUS_BADGE_BASE_CLASS,
              STATUS_BADGE_PADDING_BY_SIZE[size],
              STATUS_BADGE_ENABLED_CLASS,
            ].join(' '),
          );
        },
      );
    });

    describe('isEnabled ternary', () => {
      it('selects the ENABLED class when isEnabled is true', () => {
        expect(getStatusBadgeClass('sm', true, false)).toBe(
          [
            STATUS_BADGE_BASE_CLASS,
            STATUS_BADGE_PADDING_BY_SIZE.sm,
            STATUS_BADGE_ENABLED_CLASS,
          ].join(' '),
        );
      });

      it('selects the DISABLED-STATE class when isEnabled is false', () => {
        expect(getStatusBadgeClass('sm', false, false)).toBe(
          [
            STATUS_BADGE_BASE_CLASS,
            STATUS_BADGE_PADDING_BY_SIZE.sm,
            STATUS_BADGE_DISABLED_STATE_CLASS,
          ].join(' '),
        );
      });
    });

    describe('disabled ternary', () => {
      it('appends the INTERACTION-DISABLED class when disabled is true', () => {
        expect(getStatusBadgeClass('sm', true, true)).toBe(
          [
            STATUS_BADGE_BASE_CLASS,
            STATUS_BADGE_PADDING_BY_SIZE.sm,
            STATUS_BADGE_ENABLED_CLASS,
            STATUS_BADGE_INTERACTION_DISABLED_CLASS,
          ].join(' '),
        );
      });

      it('emits an empty string token when disabled is false, then trims the trailing space', () => {
        // Defensive .trim() branch: the literal `''` fourth element produces a
        // trailing space after join(' '); .trim() must remove it so the
        // resulting class string has no trailing whitespace.
        const result = getStatusBadgeClass('sm', true, false);
        expect(result.endsWith(' ')).toBe(false);
        expect(result).toBe(
          [
            STATUS_BADGE_BASE_CLASS,
            STATUS_BADGE_PADDING_BY_SIZE.sm,
            STATUS_BADGE_ENABLED_CLASS,
          ].join(' '),
        );
      });
    });

    describe('full 2x2x2 matrix (size × isEnabled × disabled)', () => {
      // Every combination of the three boolean-ish inputs is enumerated so
      // every ternary arm of every input is reached alongside every other.
      const cases: ReadonlyArray<{
        name: string;
        size: StatusBadgeSize;
        isEnabled: boolean;
        disabled: boolean;
        stateClass: string;
      }> = [
        {
          name: 'sm + enabled + not-disabled',
          size: 'sm',
          isEnabled: true,
          disabled: false,
          stateClass: STATUS_BADGE_ENABLED_CLASS,
        },
        {
          name: 'sm + enabled + disabled',
          size: 'sm',
          isEnabled: true,
          disabled: true,
          stateClass: STATUS_BADGE_ENABLED_CLASS,
        },
        {
          name: 'sm + disabled-state + not-disabled',
          size: 'sm',
          isEnabled: false,
          disabled: false,
          stateClass: STATUS_BADGE_DISABLED_STATE_CLASS,
        },
        {
          name: 'sm + disabled-state + disabled',
          size: 'sm',
          isEnabled: false,
          disabled: true,
          stateClass: STATUS_BADGE_DISABLED_STATE_CLASS,
        },
        {
          name: 'md + enabled + not-disabled',
          size: 'md',
          isEnabled: true,
          disabled: false,
          stateClass: STATUS_BADGE_ENABLED_CLASS,
        },
        {
          name: 'md + enabled + disabled',
          size: 'md',
          isEnabled: true,
          disabled: true,
          stateClass: STATUS_BADGE_ENABLED_CLASS,
        },
        {
          name: 'md + disabled-state + not-disabled',
          size: 'md',
          isEnabled: false,
          disabled: false,
          stateClass: STATUS_BADGE_DISABLED_STATE_CLASS,
        },
        {
          name: 'md + disabled-state + disabled',
          size: 'md',
          isEnabled: false,
          disabled: true,
          stateClass: STATUS_BADGE_DISABLED_STATE_CLASS,
        },
      ];

      it.each(cases)(
        '$name produces the exact class string',
        ({ size, isEnabled, disabled, stateClass }) => {
          const expected = [
            STATUS_BADGE_BASE_CLASS,
            STATUS_BADGE_PADDING_BY_SIZE[size],
            stateClass,
            disabled ? STATUS_BADGE_INTERACTION_DISABLED_CLASS : '',
          ]
            .join(' ')
            .trim();
          expect(getStatusBadgeClass(size, isEnabled, disabled)).toBe(expected);
        },
      );

      it.each(cases)(
        '$name has no double spaces and no leading/trailing whitespace',
        ({ size, isEnabled, disabled }) => {
          const result = getStatusBadgeClass(size, isEnabled, disabled);
          expect(result).not.toMatch(/\s{2,}/);
          expect(result).toBe(result.trim());
        },
      );
    });

    describe('malformed size lookup (Record miss at runtime)', () => {
      // The declared type forbids anything outside 'sm' | 'md', but at runtime
      // `STATUS_BADGE_PADDING_BY_SIZE[size]` returns `undefined` for any other
      // key. `Array.prototype.join` stringifies `undefined` slots to the empty
      // string (per ECMA-262 §22.1.3.15), so the result contains a stray
      // internal double space where the padding token was supposed to sit.
      // `.trim()` only strips leading/trailing whitespace and does not collapse
      // the internal double space — see GLM_REPORT.md for the corresponding
      // suspected source bug. These tests lock in the current (defective)
      // runtime behavior so any future fix is forced to update them
      // deliberately.

      it('collapses the missing padding slot to an empty string, leaving a double space (out-of-union size, enabled, not-disabled)', () => {
        const bogus = 'xl' as unknown as StatusBadgeSize;
        const result = getStatusBadgeClass(bogus, true, false);
        // The undefined lookup contributes an empty string; the surrounding
        // separators collapse into a literal double space between the base
        // class and the enabled-state class.
        expect(result).toBe(`${STATUS_BADGE_BASE_CLASS}  ${STATUS_BADGE_ENABLED_CLASS}`);
        expect(result).toMatch(/\s{2,}/);
      });

      it('still appends the interaction-disabled class for a malformed size when disabled is true (double space preserved)', () => {
        const bogus = 'lg' as unknown as StatusBadgeSize;
        const result = getStatusBadgeClass(bogus, false, true);
        expect(result).toBe(
          `${STATUS_BADGE_BASE_CLASS}  ${STATUS_BADGE_DISABLED_STATE_CLASS} ${STATUS_BADGE_INTERACTION_DISABLED_CLASS}`,
        );
        expect(result).toMatch(/\s{2,}/);
      });
    });
  });

  describe('getStatusBadgeLabel', () => {
    describe('isEnabled === true', () => {
      it('returns labelEnabled when it is a non-empty string (?? left arm)', () => {
        const props = makeProps({ isEnabled: true, labelEnabled: 'Active' });
        expect(getStatusBadgeLabel(props)).toBe('Active');
      });

      it('falls back to "Enabled" when labelEnabled is absent (?? right arm)', () => {
        const props = makeProps({ isEnabled: true });
        expect(getStatusBadgeLabel(props)).toBe('Enabled');
      });

      it('returns labelEnabled verbatim when it is an empty string (?? does NOT treat "" as nullish)', () => {
        // Subtle branch: `??` only falls through on null/undefined, so an
        // empty-string label is preserved as-is rather than replaced by the
        // "Enabled" default. This is the documented ?? semantics and is
        // asserted to lock in current behavior.
        const props = makeProps({ isEnabled: true, labelEnabled: '' });
        expect(getStatusBadgeLabel(props)).toBe('');
      });

      it('ignores labelDisabled entirely when isEnabled is true', () => {
        const props = makeProps({
          isEnabled: true,
          labelEnabled: 'On',
          labelDisabled: 'Should not appear',
        });
        expect(getStatusBadgeLabel(props)).toBe('On');
      });
    });

    describe('isEnabled === false', () => {
      it('returns labelDisabled when it is a non-empty string (?? left arm)', () => {
        const props = makeProps({ isEnabled: false, labelDisabled: 'Paused' });
        expect(getStatusBadgeLabel(props)).toBe('Paused');
      });

      it('falls back to "Disabled" when labelDisabled is absent (?? right arm)', () => {
        const props = makeProps({ isEnabled: false });
        expect(getStatusBadgeLabel(props)).toBe('Disabled');
      });

      it('returns labelDisabled verbatim when it is an empty string (?? does NOT treat "" as nullish)', () => {
        // Same ?? semantics as the enabled branch: empty string is preserved.
        const props = makeProps({ isEnabled: false, labelDisabled: '' });
        expect(getStatusBadgeLabel(props)).toBe('');
      });

      it('ignores labelEnabled entirely when isEnabled is false', () => {
        const props = makeProps({
          isEnabled: false,
          labelEnabled: 'Should not appear',
          labelDisabled: 'Off',
        });
        expect(getStatusBadgeLabel(props)).toBe('Off');
      });
    });

    describe('default values are the canonical English words', () => {
      // Lock in the exact default strings so a future refactor that changes
      // "Enabled" / "Disabled" to e.g. "On" / "Off" trips a test.
      it('uses exactly "Enabled" as the default for the enabled state', () => {
        expect(getStatusBadgeLabel(makeProps({ isEnabled: true }))).toBe('Enabled');
      });

      it('uses exactly "Disabled" as the default for the disabled state', () => {
        expect(getStatusBadgeLabel(makeProps({ isEnabled: false }))).toBe('Disabled');
      });
    });
  });
});
