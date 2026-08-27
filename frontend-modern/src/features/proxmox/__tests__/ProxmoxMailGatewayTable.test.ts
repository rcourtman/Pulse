import { describe, expect, it } from 'vitest';

import {
  MAIL_GATEWAY_NARROW_PHONE_COLUMNS,
  MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS,
  MAIL_GATEWAY_PHONE_COLUMNS,
  MAIL_GATEWAY_PHONE_COLUMN_WIDTHS,
} from '../ProxmoxMailGatewayTable';
import mailGatewayDrawerSource from '../ProxmoxMailGatewayDrawer.tsx?raw';

describe('ProxmoxMailGatewayTable phone presentation', () => {
  it('keeps compact drawer statistics on the shared responsive row contract', () => {
    expect(mailGatewayDrawerSource).toContain('InfoCardKeyValueRow');
    expect(mailGatewayDrawerSource).not.toMatch(
      /class="flex items-baseline justify-between[^\"]*">\s*<span class="text-muted">/,
    );
    expect(
      mailGatewayDrawerSource.match(/class="col-span-2 text-\[11px\] sm:col-span-1"/g),
    ).toHaveLength(3);
  });

  it('keeps fleet context and mail-flow counters visible without scrolling', () => {
    expect(MAIL_GATEWAY_PHONE_COLUMNS).toEqual([
      'instance',
      'nodes',
      'uptime',
      'mail',
      'queue',
      'deferred',
    ]);
    expect(MAIL_GATEWAY_PHONE_COLUMN_WIDTHS).toEqual({
      instance: 30,
      nodes: 12,
      uptime: 16,
      mail: 14,
      queue: 14,
      deferred: 14,
    });
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
});
