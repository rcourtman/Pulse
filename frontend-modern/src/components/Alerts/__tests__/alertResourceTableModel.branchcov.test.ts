import { describe, expect, it } from 'vitest';

import {
  getAlertResourceMetricDisplayValue,
  type AlertResourceTableResourceLike,
  type AlertResourceThresholdMap,
} from '../alertResourceTableModel';

function makeResource(
  overrides: Partial<AlertResourceTableResourceLike> = {},
): AlertResourceTableResourceLike {
  return {
    id: 'res-1',
    name: 'Test VM',
    ...overrides,
  };
}

describe('getAlertResourceMetricDisplayValue — extract() branch coverage', () => {
  describe('extract over editingThresholds (isEditing = true)', () => {
    it('returns the edited numeric value when present', () => {
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', { cpu: 42 }, true)).toBe(42);
    });

    it('returns literal 0 for an edited value of 0 (no falsy collapse)', () => {
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', { cpu: 0 }, true)).toBe(0);
    });

    it('parses an edited numeric string into a number', () => {
      const edited = { cpu: '42' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(42);
    });

    it('coerces an edited empty string to 0 via Number()', () => {
      const edited = { cpu: '' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(0);
    });

    it('coerces an edited boolean true to 1 via Number()', () => {
      const edited = { cpu: true } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(1);
    });

    it('coerces an edited boolean false to 0 via Number()', () => {
      const edited = { cpu: false } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(0);
    });

    it('coerces a single-element numeric array via Number()', () => {
      const edited = { cpu: [42] } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(42);
    });

    it('coerces an empty array to 0 via Number()', () => {
      const edited = { cpu: [] } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(0);
    });

    it('propagates a numeric NaN edited value through the typeof-number branch', () => {
      const edited = { cpu: NaN } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      const result = getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true);
      expect(Number.isNaN(result)).toBe(true);
    });

    it('propagates a numeric Infinity edited value through the typeof-number branch', () => {
      const edited = { cpu: Infinity } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(Infinity);
    });

    it('falls back to defaults when edited value is null', () => {
      const edited = { cpu: null } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(75);
    });

    it('falls back to defaults when edited value is a non-numeric string', () => {
      const edited = { cpu: 'not-a-number' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(75);
    });

    it('falls back to defaults when edited value is a multi-element array (Number -> NaN)', () => {
      const edited = { cpu: [1, 2] } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(75);
    });

    it('falls back to defaults when edited value is a plain object (Number -> NaN)', () => {
      const edited = { cpu: { v: 1 } } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', edited, true)).toBe(75);
    });

    it('falls back to defaults when metric is absent from editingThresholds', () => {
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', {}, true)).toBe(75);
    });

    it('falls back to defaults when editingThresholds is undefined', () => {
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', undefined, true)).toBe(75);
    });

    it('returns 0 when neither edited value nor defaults are usable', () => {
      const resource = makeResource();
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', {}, true)).toBe(0);
    });

    it('returns 0 when defaults value is explicitly undefined', () => {
      const resource = makeResource({ defaults: { cpu: undefined } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu', {}, true)).toBe(0);
    });
  });

  describe('extract over live thresholds (isEditing = false)', () => {
    it('returns the live numeric value when present', () => {
      const resource = makeResource({ thresholds: { cpu: 80 }, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(80);
    });

    it('returns literal 0 for a live value of 0 (no falsy collapse, skips defaults)', () => {
      const resource = makeResource({ thresholds: { cpu: 0 }, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
    });

    it('parses a live numeric string into a number', () => {
      const thresholds = { cpu: '80' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ thresholds, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(80);
    });

    it('coerces a live empty string to 0 via Number()', () => {
      const thresholds = { cpu: '' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ thresholds, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
    });

    it('falls back to defaults when live value is null', () => {
      const thresholds = { cpu: null } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ thresholds, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(75);
    });

    it('falls back to defaults when live value is a non-numeric string', () => {
      const thresholds = { cpu: 'oops' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ thresholds, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(75);
    });

    it('falls back to defaults when metric is absent from thresholds', () => {
      const resource = makeResource({ thresholds: {}, defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(75);
    });

    it('falls back to defaults when thresholds is undefined', () => {
      const resource = makeResource({ defaults: { cpu: 75 } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(75);
    });

    it('parses a numeric-string default when falling back', () => {
      const defaults = { cpu: '60' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(60);
    });

    it('returns 0 when no usable thresholds or defaults exist', () => {
      const resource = makeResource();
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
    });

    it('returns 0 when defaults value is null', () => {
      const defaults = { cpu: null } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
    });

    it('returns 0 when defaults value is a non-numeric string', () => {
      const defaults = { cpu: 'nope' } as unknown as AlertResourceThresholdMap;
      const resource = makeResource({ defaults });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
    });

    it('returns 0 when defaults value is explicitly undefined', () => {
      const resource = makeResource({ defaults: { cpu: undefined } });
      expect(getAlertResourceMetricDisplayValue(resource, 'cpu')).toBe(0);
    });
  });
});
