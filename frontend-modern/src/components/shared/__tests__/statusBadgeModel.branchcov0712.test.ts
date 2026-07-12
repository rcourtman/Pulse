import { describe, expect, it } from 'vitest';
import type { StatusBadgeProps } from '@/components/shared/statusBadgeModel';
import { getStatusBadgeTitle } from '@/components/shared/statusBadgeModel';

// Minimal valid props; every optional title field defaults to undefined so each
// test can populate only the ones it needs to drive a specific branch.
function makeProps(overrides: Partial<StatusBadgeProps> = {}): StatusBadgeProps {
  return {
    isEnabled: false,
    ...overrides,
  };
}

describe('statusBadgeModel.branchcov2', () => {
  describe('getStatusBadgeTitle', () => {
    describe('disabled === true (first branch — three-level ?? cascade)', () => {
      it('returns titleWhenDisabled when set (first ?? short-circuits)', () => {
        const props = makeProps({
          isEnabled: false,
          titleWhenDisabled: 'Locked',
          titleDisabled: 'Off',
          titleEnabled: 'On',
        });
        expect(getStatusBadgeTitle(props, true)).toBe('Locked');
      });

      it('falls back to titleDisabled when titleWhenDisabled is absent (second ??)', () => {
        const props = makeProps({
          isEnabled: false,
          titleDisabled: 'Off',
          titleEnabled: 'On',
        });
        expect(getStatusBadgeTitle(props, true)).toBe('Off');
      });

      it('falls back to titleEnabled when titleWhenDisabled and titleDisabled are both absent (third ??)', () => {
        const props = makeProps({
          isEnabled: true,
          titleEnabled: 'On',
        });
        expect(getStatusBadgeTitle(props, true)).toBe('On');
      });

      it('returns the empty string when none of the title fields are set (final ?? default)', () => {
        const props = makeProps({ isEnabled: false });
        expect(getStatusBadgeTitle(props, true)).toBe('');
      });
    });

    describe('disabled === false (else branch — isEnabled ternary)', () => {
      it('returns titleEnabled when isEnabled is true and titleEnabled is set', () => {
        const props = makeProps({
          isEnabled: true,
          titleEnabled: 'On',
        });
        expect(getStatusBadgeTitle(props, false)).toBe('On');
      });

      it('returns the empty string when isEnabled is true but titleEnabled is absent (?? default)', () => {
        const props = makeProps({ isEnabled: true });
        expect(getStatusBadgeTitle(props, false)).toBe('');
      });

      it('returns titleDisabled when isEnabled is false and titleDisabled is set', () => {
        const props = makeProps({
          isEnabled: false,
          titleDisabled: 'Off',
        });
        expect(getStatusBadgeTitle(props, false)).toBe('Off');
      });

      it('returns the empty string when isEnabled is false and titleDisabled is absent (?? default)', () => {
        const props = makeProps({ isEnabled: false });
        expect(getStatusBadgeTitle(props, false)).toBe('');
      });

      it('ignores titleWhenDisabled entirely on the !disabled branch', () => {
        const props = makeProps({
          isEnabled: true,
          titleWhenDisabled: 'Locked',
          titleEnabled: 'On',
        });
        expect(getStatusBadgeTitle(props, false)).toBe('On');
      });
    });
  });
});
