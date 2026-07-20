import { describe, expect, it } from 'vitest';
import { buildDataHandlingPanelModel } from '../dataHandlingPanelModel';

describe('dataHandlingPanelModel', () => {
  it('summarizes canonical resource policy posture for settings', () => {
    const model = buildDataHandlingPanelModel({
      totalResources: 10,
      sensitivityCounts: {
        public: 1,
        internal: 4,
        sensitive: 3,
        restricted: 2,
      },
      routingCounts: {
        'cloud-summary': 5,
        'local-first': 3,
        'local-only': 2,
      },
      redactionCounts: {
        hostname: 2,
        'ip-address': 1,
      },
    });

    expect(model.totalResources).toBe(10);
    expect(model.localOnlyResources).toBe(2);
    expect(model.redactionHintCount).toBe(3);
    expect(model.hasResources).toBe(true);
    expect(model.hasRedactions).toBe(true);
    expect(model.sensitivityItems.map((item) => [item.key, item.count, item.percentage])).toEqual([
      ['public', 1, 10],
      ['internal', 4, 40],
      ['sensitive', 3, 30],
      ['restricted', 2, 20],
    ]);
    expect(model.routingItems.map((item) => [item.key, item.count, item.percentage])).toEqual([
      ['cloud-summary', 5, 50],
      ['local-first', 3, 30],
      ['local-only', 2, 20],
    ]);
    expect(model.redactionItems.map((item) => [item.key, item.count, item.percentage])).toEqual([
      ['hostname', 2, 67],
      ['ip-address', 1, 33],
    ]);
  });

  it('keeps missing and malformed counts as empty posture without negative totals', () => {
    const model = buildDataHandlingPanelModel({
      totalResources: -4,
      sensitivityCounts: {
        public: -1,
        internal: Number.NaN,
      },
      routingCounts: {
        'local-only': -5,
      },
      redactionCounts: {},
    });

    expect(model.totalResources).toBe(0);
    expect(model.localOnlyResources).toBe(0);
    expect(model.redactionHintCount).toBe(0);
    expect(model.hasResources).toBe(false);
    expect(model.hasRedactions).toBe(false);
    expect(model.sensitivityItems.every((item) => item.count === 0)).toBe(true);
    expect(model.routingItems.every((item) => item.count === 0)).toBe(true);
    expect(model.redactionItems).toEqual([]);
  });
});
