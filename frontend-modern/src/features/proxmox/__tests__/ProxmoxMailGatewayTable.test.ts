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
  it('keeps mail-flow figures on shared detail rows and card chrome', () => {
    expect(mailGatewayDrawerSource).toContain('makeDetailRow(');
    expect(mailGatewayDrawerSource).toContain('<TechnicalDetailsSection');
    expect(mailGatewayDrawerSource).toContain('<InfoCardFrame');
    expect(mailGatewayDrawerSource).not.toContain('function InOutBar');
    expect(mailGatewayDrawerSource).not.toContain('<Card');
    expect(mailGatewayDrawerSource).not.toContain('<h4');
    expect(mailGatewayDrawerSource).not.toMatch(
      /class="flex items-baseline justify-between[^\"]*">\s*<span class="text-muted">/,
    );
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

  it('matches gateway alerts across identity aliases and opens the one resource drawer', () => {
    // Alerts are keyed by the PMG instance id, rows by the unified id.
    expect(mailGatewayTableSource).toContain('getResourceIdentityAliases(resource)');
    expect(mailGatewayTableSource).toContain(
      'filterPlatformResources(resources, search, status, effectiveStatus)',
    );
    // The row opens the canonical resource drawer headed by its own dot; the
    // Mail tab body is headless and leaves status and alerts to that drawer.
    expect(mailGatewayTableSource).toContain('<PlatformResourceDetailTableRow');
    expect(mailGatewayTableSource).toContain('statusIndicator={indicator()}');
    expect(mailGatewayDrawerSource).not.toContain('DrawerAttentionSection');
    expect(mailGatewayDrawerSource).not.toContain('ObjectDrawerHeader');
    expect(mailGatewayDrawerSource).not.toContain('function classifyHealth');
  });

  it('keeps alerting rows single-line by marking the metric the alert is about', () => {
    // Platform table rows stay on one line, so an alert colours its own number
    // rather than stacking a reason under the gateway name.
    expect(mailGatewayTableSource).toContain('<AlertedMetric alerts={columnAlerts(');
    expect(mailGatewayTableSource).not.toContain('data-mail-gateway-alert-reason');
  });
});
