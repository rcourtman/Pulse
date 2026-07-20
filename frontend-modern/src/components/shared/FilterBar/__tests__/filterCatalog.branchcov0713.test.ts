import { describe, expect, it, vi } from 'vitest';
import { formatFilterChipValue } from '@/components/shared/FilterBar/filterCatalog';
import type { FilterDef, FilterSelectOption } from '@/components/shared/FilterBar/filterCatalog';

// Branch-coverage companion to FilterBar.test.tsx. The sibling suite renders
// FilterBar with fixtures that never set FilterDef.formatChipValue, so the
// custom-formatter true-arm of formatFilterChipValue is never exercised. These
// cases call the exported helper directly and drive each guard/ternary arm:
// the custom-formatter delegation, the matched-label lookup, the no-match
// `?? value` fallback, the empty-options fallback, the runtime-null-label
// fallback, and the empty-string-label `??` semantics.

// Builder that satisfies the full FilterDef shape while exposing only the
// fields formatFilterChipValue reads (value()/options()/formatChipValue).
// setValue is stubbed with vi.fn() to keep the fixture inert.
const makeFilter = (overrides: {
  value?: string;
  options?: FilterSelectOption[];
  formatChipValue?: (value: string, options: FilterSelectOption[]) => string;
}): FilterDef => ({
  id: 'under-test',
  label: 'UnderTest',
  value: () => overrides.value ?? '',
  setValue: vi.fn(),
  defaultValue: '',
  options: () => overrides.options ?? [],
  ...(overrides.formatChipValue !== undefined
    ? { formatChipValue: overrides.formatChipValue }
    : {}),
});

describe('formatFilterChipValue branch coverage', () => {
  describe('custom formatChipValue arm (truthy guard)', () => {
    it('delegates to filter.formatChipValue and returns its result (guard true-arm)', () => {
      const formatChipValue = vi.fn((v: string) => `custom:${v}`);
      const filter = makeFilter({
        value: 'vm',
        options: [{ value: 'vm', label: 'VMs' }],
        formatChipValue,
      });

      expect(formatFilterChipValue(filter)).toBe('custom:vm');
      expect(formatChipValue).toHaveBeenCalledOnce();
    });

    it('passes the current value and the resolved options array as arguments', () => {
      const seen: Array<{ value: string; opts: FilterSelectOption[] }> = [];
      const options: FilterSelectOption[] = [
        { value: 'all', label: 'All' },
        { value: 'pve1', label: 'pve1' },
      ];
      const filter = makeFilter({
        value: 'pve1',
        options,
        formatChipValue: (value, opts) => {
          seen.push({ value, opts });
          return 'formatted';
        },
      });

      expect(formatFilterChipValue(filter)).toBe('formatted');
      expect(seen).toHaveLength(1);
      expect(seen[0]).toEqual({ value: 'pve1', opts: options });
    });

    it('returns the custom result even when it differs from any option label', () => {
      const filter = makeFilter({
        value: 'all',
        options: [{ value: 'all', label: 'All' }],
        formatChipValue: () => 'Custom label',
      });

      expect(formatFilterChipValue(filter)).toBe('Custom label');
    });

    it('returns an empty string produced by the custom formatter', () => {
      const filter = makeFilter({
        value: 'all',
        options: [{ value: 'all', label: 'All' }],
        formatChipValue: () => '',
      });

      expect(formatFilterChipValue(filter)).toBe('');
    });

    it('skips the default lookup entirely when the custom formatter is present', () => {
      const formatChipValue = vi.fn(() => 'override');
      const filter = makeFilter({
        value: 'no-such-option',
        options: [{ value: 'all', label: 'All' }],
        formatChipValue,
      });

      expect(formatFilterChipValue(filter)).toBe('override');
      expect(formatChipValue).toHaveBeenCalledWith('no-such-option', [
        { value: 'all', label: 'All' },
      ]);
    });
  });

  describe('default lookup arm (formatChipValue absent)', () => {
    it('returns the matching option label (match?.label truthy)', () => {
      const filter = makeFilter({
        value: 'vm',
        options: [
          { value: 'all', label: 'All' },
          { value: 'vm', label: 'VMs' },
        ],
      });

      expect(formatFilterChipValue(filter)).toBe('VMs');
    });

    it('selects the first option whose value matches when duplicates exist', () => {
      const filter = makeFilter({
        value: 'dup',
        options: [
          { value: 'dup', label: 'First' },
          { value: 'dup', label: 'Second' },
        ],
      });

      expect(formatFilterChipValue(filter)).toBe('First');
    });

    it('falls back to the raw value when no option matches (?? value)', () => {
      const filter = makeFilter({
        value: 'orphan',
        options: [{ value: 'all', label: 'All' }],
      });

      expect(formatFilterChipValue(filter)).toBe('orphan');
    });

    it('falls back to the raw value when options is an empty array', () => {
      const filter = makeFilter({ value: 'lonely', options: [] });

      expect(formatFilterChipValue(filter)).toBe('lonely');
    });

    it('falls back to the raw value when the matched label is null at runtime', () => {
      // FilterSelectOption.label is declared `string`, so a null label violates
      // the type; cast through unknown to exercise the `?? value` fallback
      // without silencing a real type error.
      const malformedOptions = [{ value: 'ghost', label: null }] as unknown as FilterSelectOption[];
      const filter = makeFilter({ value: 'ghost', options: malformedOptions });

      expect(formatFilterChipValue(filter)).toBe('ghost');
    });

    it('falls back to the raw value when the matched label is undefined at runtime', () => {
      const malformedOptions = [
        { value: 'ghost', label: undefined },
      ] as unknown as FilterSelectOption[];
      const filter = makeFilter({ value: 'ghost', options: malformedOptions });

      expect(formatFilterChipValue(filter)).toBe('ghost');
    });

    it('returns an empty-string label verbatim because ?? only catches null/undefined', () => {
      // `match?.label ?? value` does not coerce '' to value; an explicit
      // empty label is a distinct, observable outcome.
      const filter = makeFilter({
        value: 'blank',
        options: [{ value: 'blank', label: '' }],
      });

      expect(formatFilterChipValue(filter)).toBe('');
    });

    it('returns the raw value when the value is the empty string with no empty-value option', () => {
      const filter = makeFilter({
        value: '',
        options: [{ value: 'all', label: 'All' }],
      });

      expect(formatFilterChipValue(filter)).toBe('');
    });

    it('returns the matching option label for an option whose value is the empty string', () => {
      const filter = makeFilter({
        value: '',
        options: [
          { value: '', label: 'All nodes' },
          { value: 'pve1', label: 'pve1' },
        ],
      });

      expect(formatFilterChipValue(filter)).toBe('All nodes');
    });
  });
});
