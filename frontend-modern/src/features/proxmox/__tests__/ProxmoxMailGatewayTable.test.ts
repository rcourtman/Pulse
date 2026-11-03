import { describe, expect, it } from 'vitest';

import { getPlatformTableWeightedColumnWidthStyle } from '@/features/platformPage/sharedPlatformPage';
import {
  MAIL_GATEWAY_COLUMN_WEIGHTS,
  type MailGatewayColumn,
  MAIL_GATEWAY_NARROW_PHONE_COLUMNS,
  MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS,
  MAIL_GATEWAY_PHONE_COLUMNS,
  MAIL_GATEWAY_PHONE_COLUMN_WIDTHS,
} from '../ProxmoxMailGatewayTable';

describe('ProxmoxMailGatewayTable phone presentation', () => {
  it('keeps gateway names distinguishable with five phone tracks', () => {
    // A sixth track (node count) truncated "mail-gateway-eu" and
    // "mail-gateway-us" to the same text; the count lives in the expansion.
    expect(MAIL_GATEWAY_PHONE_COLUMNS).toEqual(['instance', 'uptime', 'mail', 'queue', 'deferred']);
    expect(MAIL_GATEWAY_PHONE_COLUMN_WIDTHS.instance).toBe(40);
    expect(
      MAIL_GATEWAY_PHONE_COLUMNS.reduce(
        (total, column) => total + MAIL_GATEWAY_PHONE_COLUMN_WIDTHS[column],
        0,
      ),
    ).toBe(100);
  });

  it('demotes node count below 360px while keeping five mail-flow fields', () => {
    expect(MAIL_GATEWAY_NARROW_PHONE_COLUMNS).toEqual([
      'instance',
      'uptime',
      'mail',
      'queue',
      'deferred',
    ]);
    expect(
      MAIL_GATEWAY_NARROW_PHONE_COLUMNS.reduce(
        (total, column) => total + MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS[column],
        0,
      ),
    ).toBe(100);
  });

  it('gives every desktop column a weighted share instead of the remainder', () => {
    const columns = Object.keys(MAIL_GATEWAY_COLUMN_WEIGHTS) as MailGatewayColumn[];
    expect(columns).toHaveLength(10);
    const widths = columns.map((column) =>
      Number.parseFloat(
        String(
          getPlatformTableWeightedColumnWidthStyle(column, MAIL_GATEWAY_COLUMN_WEIGHTS, columns)
            .width,
        ),
      ),
    );
    // Version, Spam, Virus, and Quarantine used to split ~14% between them,
    // which clipped a three-digit spam count to "3…" on a 1440px screen.
    expect(Math.min(...widths)).toBeGreaterThan(7);
    expect(widths.reduce((total, width) => total + width, 0)).toBeCloseTo(100, 1);
  });
});
