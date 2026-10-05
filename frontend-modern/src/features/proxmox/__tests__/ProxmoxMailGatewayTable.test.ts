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
import mailGatewayDrawerSource from '../ProxmoxMailGatewayDrawer.tsx?raw';
import mailGatewayTableSource from '../ProxmoxMailGatewayTable.tsx?raw';

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

  it('matches gateway alerts across identity aliases and shows the problem in the drawer', () => {
    // Alerts are keyed by the PMG instance id, rows by the unified id.
    expect(mailGatewayTableSource).toContain('getResourceIdentityAliases(resource)');
    expect(mailGatewayTableSource).toContain(
      'filterPlatformResources(resources, search, status, effectiveStatus)',
    );
    expect(mailGatewayDrawerSource).toContain('<DrawerAttentionSection');
  });

  it('keeps alerting rows single-line by marking the metric the alert is about', () => {
    // Platform table rows stay on one line, so an alert colours its own number
    // rather than stacking a reason under the gateway name.
    expect(mailGatewayTableSource).toContain('<AlertedMetric alerts={columnAlerts(');
    expect(mailGatewayTableSource).not.toContain('data-mail-gateway-alert-reason');
  });
});
